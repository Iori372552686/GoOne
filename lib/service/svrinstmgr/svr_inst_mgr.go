package svrinstmgr

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/service/bus"

	"github.com/samuel/go-zookeeper/zk"
)

// 路由方法
const (
	SvrRouterRule_Random           = 1 + iota // 随机路由
	SvrRouterRule_Hash_UID                    // 根据UID取模
	SvrRouterRule_Hash_ZoneID                 // 根据ZoneID取模
	SvrRouterRule_Hash_RouterID               // 根据自定义RouterID取模
	SvrRouterRule_IoCache_RouterID            // 根据自定义RouterID io cache
	SvrRouterRule_Master
)

type ServerInstanceMgr struct {
	routeRules map[uint32]uint32

	mapSvrTypeToIns map[uint32][]uint32
	conn            *zk.Conn
	lock            sync.RWMutex
}

// -------------------------------- public --------------------------------

// parameters:
//
//	routeRules: ServerType->SvrRouterRule
func (s *ServerInstanceMgr) InitAndRun(selfBusID string, routeRules map[uint32]uint32, zookeeperAddr string) error {
	zkConn, chanConnect, err := zk.Connect([]string{zookeeperAddr}, time.Second*30)
	if err != nil {
		return errors.New("init zookeeper error")
	}
	s.conn = zkConn

	s.routeRules = routeRules

	go s.runSvrInsMgr(selfBusID, chanConnect)
	return nil
}

func (s *ServerInstanceMgr) Close() {
	logger.Errorf("zk close")
	s.conn.Close()
}

// 根据ServerType和预先设定的RouterRule，获取一个ServerInstance
func (s *ServerInstanceMgr) GetSvrInsBySvrType(serverType, zone uint32, uid uint64, routerId uint64) (uint32, uint64) {
	if rule, in := s.routeRules[serverType]; in {
		switch rule {
		case SvrRouterRule_Random:
			return s.getSvrInsByRandom(serverType), 0
		case SvrRouterRule_Hash_UID:
			return s.getSvrInsByHash(serverType, uid), uid
		case SvrRouterRule_Hash_ZoneID:
			return s.getSvrInsByHash(serverType, uint64(zone)), uint64(zone)
		case SvrRouterRule_Hash_RouterID:
			return s.getSvrInsByHash(serverType, routerId), routerId
		case SvrRouterRule_Master:
			return s.getSvrInsByMaster(serverType), 0
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

func (s *ServerInstanceMgr) runSvrInsMgr(selfBusID string, chanConnect <-chan zk.Event) {
	var err error
	var chanNodeEvent <-chan zk.Event
	for {
		select {
		case eventConnect, ok := <-chanConnect: // ZooKeeper连接事件
			if !ok {
				logger.Fatalf("chanConnect is closed")
				return
			}

			if eventConnect.Type == zk.EventSession && eventConnect.State == zk.StateHasSession {
				nodeName := string("/online/") + selfBusID + "."

				// 这里创建临时节点，断开连接的时候会自动删除
				s.conn.Create("/online", []byte{}, 0, zk.WorldACL(zk.PermAll))
				_, err = s.conn.Create(nodeName, []byte{}, zk.FlagEphemeral|zk.FlagSequence, zk.WorldACL(zk.PermAll))
				if err != nil {
					logger.Fatalf("create node %s error | %v", nodeName, err)
					panic("failed to create node")
					return
				}

				// 监听子节点的变化
				chanNodeEvent, err = s.watchAndRefreshNodes()
				if err != nil {
					logger.Fatalf("failed to watch online nodes | %v", err)
					panic("failed to watch online node")
					return
				}
			}
		case eventNode, ok := <-chanNodeEvent: // 节点变化的事件
			if ok && eventNode.Type == zk.EventNodeChildrenChanged {
				logger.Warningf("online nodes changed: %#v, %#v", eventNode, ok)

				chanNodeEvent, err = s.watchAndRefreshNodes()
				if err != nil {
					if err == zk.ErrNoServer {
						logger.Warningf("failed to watch online nodes | %v", err)
						// 如果连接断了，会出这种错误。这种情况就交给上面的断线重连的处理去watch
					} else {
						logger.Fatalf("failed to watch online nodes | %v", err)
						panic("failed to watch online node")
						return
					}
				}
			}
		}
	}
}

func (s *ServerInstanceMgr) watchAndRefreshNodes() (<-chan zk.Event, error) {
	children, _, chanEvent, err := s.conn.ChildrenW("/online")
	if err != nil {
		return nil, err // 这里必须把err不加修改地直接传递出去，因为外面会判断zk.ErrNoServer
	}

	s.refreshNode(children)
	return chanEvent, nil
}

// 刷新在线的svr状态，这里要用到读写锁
func (s *ServerInstanceMgr) refreshNode(children []string) {
	logger.Infof("refresh nodes: %v\n", children)

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
	for _, child := range children {
		busID, _, _, severType, _ := bus.ParseBusID(child)
		s.mapSvrTypeToIns[severType] = append(s.mapSvrTypeToIns[severType], busID)
		logger.Infof("add %s to type %d", child, severType)
		newIns[busID] = true
	}

	// 排序、去重、输出日志
	// （这里有个坑，必须要用下标引用来修改map的内容）
	for k := range s.mapSvrTypeToIns {
		// 排序去重
		sort.Slice(s.mapSvrTypeToIns[k], func(i, j int) bool { return s.mapSvrTypeToIns[k][i] < s.mapSvrTypeToIns[k][j] })
		s.mapSvrTypeToIns[k] = Uint32SliceDeduplicateSorted(s.mapSvrTypeToIns[k])

		// 输出日志
		var b bytes.Buffer
		fmt.Fprintf(&b, "Server instances: {type:%v, nodes:[", k)
		for i, u := range s.mapSvrTypeToIns[k] {
			if i > 0 {
				fmt.Fprint(&b, ", ")
			}
			fmt.Fprint(&b, bus.IpIntToString(u))
		}
		fmt.Fprint(&b, "]}\n")
		logger.Infof(b.String())
	}
	s.lock.Unlock()

	logger.Infof("refresh finish")

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
