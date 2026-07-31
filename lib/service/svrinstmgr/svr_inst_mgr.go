package svrinstmgr

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/contrib/registry"
	regfactory "github.com/Iori372552686/GoOne/lib/contrib/registry/factory"
	"github.com/Iori372552686/GoOne/lib/service/bus"
)

// 路由方法
const (
	SvrRouterRule_Random           = 1 + iota // 随机路由
	SvrRouterRule_Hash_UID                    // 根据UID取模
	SvrRouterRule_Hash_ZoneID                 // 根据ZoneID取模
	SvrRouterRule_Hash_RouterID               // 根据自定义RouterID取模
	SvrRouterRule_IoCache_RouterID            // 根据自定义RouterID io cache
	SvrRouterRule_Master

	// --- new: consistent-hash routing rules (do NOT replace Hash_* modulo rules) ---
	SvrRouterRule_ConsistentHash_UID
	SvrRouterRule_ConsistentHash_ZoneID
	SvrRouterRule_ConsistentHash_RouterID
)

// Force-use constants that are meant to be configured at runtime.
// Some IDE analyzers incorrectly report them as unused.
var _ = SvrRouterRule_IoCache_RouterID

type ServerInstanceMgr struct {
	routeRules map[uint32]uint32

	mapSvrTypeToIns    map[uint32][]uint32
	consistentHashRing map[uint32]*consistentHash
	client             registry.Client
	reg                registry.Registrar
	discovery          registry.Discovery

	// watcher/cancel/registry 的创建与 Close 全部在 closeMu 下进行，
	// 消除「Close 与 watcher 创建并发」的迟到 watcher / use-after-close 竞态。
	closeMu   sync.Mutex
	watcher   registry.Watcher
	stopWatch context.CancelFunc
	watchWG   sync.WaitGroup // join runWatch goroutine
	closed    bool
	lock      sync.RWMutex
}

// -------------------------------- public --------------------------------

// parameters:
//
//	routeRules: ServerType->SvrRouterRule
func (s *ServerInstanceMgr) InitAndRun(selfBusID string, routeRules map[uint32]uint32, zookeeperAddr string) error {
	// Registry address supports:
	//   - "127.0.0.1:2181"                 (defaults to zk)
	//   - "zk://127.0.0.1:2181?..."
	//   - "etcd://127.0.0.1:2379?..."
	c, cfg, err := regfactory.NewFromAddr(zookeeperAddr)
	if err != nil {
		return fmt.Errorf("init registry error: %w", err)
	}
	s.client = c
	s.reg = c
	s.discovery = c
	s.routeRules = routeRules
	s.mapSvrTypeToIns = make(map[uint32][]uint32)
	s.consistentHashRing = make(map[uint32]*consistentHash)

	// Register self into /online/<selfBusID> (ephemeral node).
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if err := s.reg.Register(ctx, &registry.ServiceInstance{
		ID:        selfBusID,
		Name:      cfg.ServiceName,
		Version:   "",
		Metadata:  map[string]string{"bus_id": selfBusID},
		Endpoints: nil,
	}); err != nil {
		return fmt.Errorf("register self into registry error: %w", err)
	}

	s.watchWG.Add(1)
	go s.runWatch(cfg.ServiceName)
	return nil
}

// Close 取消 watch context、join runWatch goroutine 并关闭 registry client。
// 与 runWatch 的 watcher 创建互斥，避免迟到 watcher 与 use-after-close。
func (s *ServerInstanceMgr) Close() {
	s.closeMu.Lock()
	s.closed = true
	if s.stopWatch != nil {
		s.stopWatch()
		s.stopWatch = nil
	}
	if s.watcher != nil {
		_ = s.watcher.Stop()
		s.watcher = nil
	}
	s.closeMu.Unlock()

	s.watchWG.Wait()

	s.closeMu.Lock()
	if s.client != nil {
		_ = s.client.Close()
		s.client = nil
	}
	s.closeMu.Unlock()
}

// 根据ServerType和预先设定的RouterRule，获取一个ServerInstance
func (s *ServerInstanceMgr) GetSvrInsBySvrType(serverType, zone uint32, uid uint64, routerId uint64) (uint32, uint64) {
	if rule, in := s.routeRules[serverType]; in {
		switch rule {
		case SvrRouterRule_Random:
			return s.getSvrInsByRandom(serverType), uid
		case SvrRouterRule_Hash_UID:
			return s.getSvrInsByHash(serverType, uid), uid
		case SvrRouterRule_Hash_ZoneID:
			return s.getSvrInsByHash(serverType, uint64(zone)), uint64(zone)
		case SvrRouterRule_Hash_RouterID:
			return s.getSvrInsByHash(serverType, routerId), routerId
		case SvrRouterRule_IoCache_RouterID:
			return s.getSvrInsByConsistentHash(serverType, routerId), routerId
		case SvrRouterRule_ConsistentHash_UID:
			return s.getSvrInsByConsistentHash(serverType, uid), uid
		case SvrRouterRule_ConsistentHash_ZoneID:
			return s.getSvrInsByConsistentHash(serverType, uint64(zone)), uint64(zone)
		case SvrRouterRule_ConsistentHash_RouterID:
			return s.getSvrInsByConsistentHash(serverType, routerId), routerId
		case SvrRouterRule_Master:
			return s.getSvrInsByMaster(serverType), uid
		default:
			logger.Error("wrong svr router config ", serverType)
		}
	}

	return 0, 0
}

// 根据RouterID，获取一个ServerInstance
func (s *ServerInstanceMgr) GetSvrInsByRouterID(serverType uint32, rid uint64) uint32 {
	return s.getSvrInsByHash(serverType, rid)
}

// 根据svrtype获取所有的svrinstance
func (s *ServerInstanceMgr) GetAllSvrInsBySvrType(severType uint32) []uint32 {
	var instances []uint32
	s.lock.RLock()
	defer s.lock.RUnlock()

	for _, val := range s.mapSvrTypeToIns[severType] {
		instances = append(instances, val)
	}
	return instances
}

// -------------------------------- private --------------------------------

func (s *ServerInstanceMgr) runWatch(serviceName string) {
	defer s.watchWG.Done()
	ctx, cancel := context.WithCancel(context.Background())

	s.closeMu.Lock()
	s.stopWatch = cancel
	s.closeMu.Unlock()

	for {
		// Close 已触发：不再创建新 watcher。
		s.closeMu.Lock()
		if s.closed {
			s.closeMu.Unlock()
			return
		}
		s.closeMu.Unlock()

		w, err := s.discovery.Watch(ctx, serviceName)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			logger.Warningf("registry watch create failed: %v", err)
			// 可被 Close 取消的退避，替代裸 time.Sleep。
			if !sleepOrCancel(ctx, time.Second) {
				return
			}
			continue
		}

		s.closeMu.Lock()
		if s.closed {
			s.closeMu.Unlock()
			_ = w.Stop()
			return
		}
		s.watcher = w
		s.closeMu.Unlock()

		for {
			services, err := w.Next()
			if err != nil {
				s.closeMu.Lock()
				if s.watcher == w {
					_ = w.Stop()
					s.watcher = nil
				}
				s.closeMu.Unlock()
				if ctx.Err() != nil {
					return
				}
				logger.Warningf("registry watch next failed: %v", err)
				if !sleepOrCancel(ctx, time.Second) {
					return
				}
				break // recreate watcher
			}
			s.refreshServices(services)
		}
	}
}

// sleepOrCancel 等待 d，ctx 取消时立即返回 false（替代裸 time.Sleep）。
func sleepOrCancel(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// 刷新在线的svr状态，这里要用到读写锁
func (s *ServerInstanceMgr) refreshServices(services []*registry.ServiceInstance) {
	children := make([]string, 0, len(services))
	for _, si := range services {
		if si == nil {
			continue
		}
		// ID is used as the node key: /online/<ID>
		children = append(children, si.ID)
	}
	logger.Debugf("refresh nodes: %v", children)

	oldIns := make(map[uint32]bool)
	newIns := make(map[uint32]bool)

	for _, m := range s.mapSvrTypeToIns {
		for _, v := range m {
			oldIns[v] = true
		}
	}

	s.lock.Lock()

	// 所有的busID加到ServerType->ServerInstance的map中
	s.mapSvrTypeToIns = make(map[uint32][]uint32)
	s.consistentHashRing = make(map[uint32]*consistentHash)
	for _, child := range children {
		busID, _, _, severType, _ := bus.ParseBusID(child)
		s.mapSvrTypeToIns[severType] = append(s.mapSvrTypeToIns[severType], busID)
		logger.Debugf("add %s to type %d", child, severType)
		newIns[busID] = true
	}

	// 排序、去重、输出日志
	// （这里有个坑，必须要用下标引用来修改map的内容）
	for k := range s.mapSvrTypeToIns {
		// 排序去重
		sort.Slice(s.mapSvrTypeToIns[k], func(i, j int) bool { return s.mapSvrTypeToIns[k][i] < s.mapSvrTypeToIns[k][j] })
		s.mapSvrTypeToIns[k] = Uint32SliceDeduplicateSorted(s.mapSvrTypeToIns[k])
		if shouldBuildConsistentHashRing(s.routeRules[k]) {
			s.consistentHashRing[k] = newConsistentHashSorted(s.mapSvrTypeToIns[k], defaultConsistentHashVirtualNodes)
		}

		// 输出日志（strings.Builder，避免 fmt ignored-error 告警）
		var b strings.Builder
		b.WriteString("Server instances: {type:")
		b.WriteString(fmt.Sprint(k))
		b.WriteString(", nodes:[")
		for i, u := range s.mapSvrTypeToIns[k] {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(bus.IpIntToString(u))
		}
		b.WriteString("]}")
		logger.Infof("%s", b.String())
	}
	s.lock.Unlock()

	logger.Debugf("refresh finish")

	// 打印出删除和发现的svr
	for k := range oldIns {
		if _, in := newIns[k]; !in {
			logger.Infof("svr instance deleted: 0x%x", k)
		}
	}
	for k := range newIns {
		if _, in := oldIns[k]; !in {
			logger.Infof("svr instance added: 0x%x", k)
		}
	}
}

func shouldBuildConsistentHashRing(rule uint32) bool {
	switch rule {
	case SvrRouterRule_IoCache_RouterID,
		SvrRouterRule_ConsistentHash_UID,
		SvrRouterRule_ConsistentHash_ZoneID,
		SvrRouterRule_ConsistentHash_RouterID:
		return true
	default:
		return false
	}
}

// 随机获取svr
func (s *ServerInstanceMgr) getSvrInsByRandom(svrType uint32) uint32 {
	s.lock.RLock()
	defer s.lock.RUnlock()

	svrs := s.mapSvrTypeToIns[svrType]
	if len(svrs) == 0 {
		return 0
	}

	idx := rand.Int31n(int32(len(svrs)))
	return svrs[idx]
}

// 通过UID获取一个svr，这里对uid取模
func (s *ServerInstanceMgr) getSvrInsByConsistentHash(svrType uint32, key uint64) uint32 {
	s.lock.RLock()
	svrs := s.mapSvrTypeToIns[svrType]
	if len(svrs) == 0 {
		s.lock.RUnlock()
		return 0
	}
	ring := s.consistentHashRing[svrType]
	s.lock.RUnlock()

	if ring != nil {
		return ring.get(key)
	}
	return newConsistentHash(svrs, defaultConsistentHashVirtualNodes).get(key)
}

// 兼容旧名字：Hash_* 路由仍然是取模（不要改语义）
func (s *ServerInstanceMgr) getSvrInsByHash(svrType uint32, id uint64) uint32 {
	s.lock.RLock()
	defer s.lock.RUnlock()

	svrs := s.mapSvrTypeToIns[svrType]
	if len(svrs) == 0 {
		return 0
	}

	return svrs[id%uint64(len(svrs))]
}

// 主备模式，永远取第一个svr
func (s *ServerInstanceMgr) getSvrInsByMaster(svrType uint32) uint32 {
	s.lock.RLock()
	defer s.lock.RUnlock()

	svrs := s.mapSvrTypeToIns[svrType]
	if len(svrs) == 0 {
		return 0
	}

	return svrs[0]
}

func Uint32SliceDeduplicateSorted(s []uint32) []uint32 {
	if s == nil || len(s) <= 1 {
		return s
	}

	out := []uint32{s[0]}
	for i := 1; i < len(s); i++ {
		if s[i] != out[len(out)-1] {
			out = append(out, s[i])
		}
	}

	return out
}
