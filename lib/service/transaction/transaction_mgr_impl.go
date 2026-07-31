package transaction

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/api/sharedstruct"
	"github.com/Iori372552686/GoOne/lib/service/bus"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
	"github.com/golang/protobuf/proto"
)

type TransactionMgr struct {
	started     bool
	cmdHandlers map[g1_protocol.CMD]cmd_handler.CmdHandlerFunc

	config       TransactionMgrConfig
	useSerialKey bool
	maxTransNum  int32
	shards       []*transactionShard
	roundRobin   atomic.Uint32

	activeTransactions atomic.Int64
	pendingPackets     atomic.Int64
	droppedPackets     atomic.Int64
	closing            atomic.Bool
	closeCh            chan struct{}
	closeOnce          sync.Once
	shardWG            sync.WaitGroup
}

type transactionShard struct {
	mgr   *TransactionMgr
	index int

	chanInPacket chan *sharedstruct.SSPacket
	chanTransRet chan uint32

	nextTransID    uint32
	transIDStep    uint32
	transMap       map[uint32]*Transaction
	keyInProcess   map[uint64]bool
	pendingPackets map[uint64][]*sharedstruct.SSPacket
}

func (m *TransactionMgr) InitAndRun(maxTrans int32, useUidLock bool, maxUidPendingPacket int) {
	m.InitAndRunWithConfig(TransactionMgrConfig{
		MaxTrans:         maxTrans,
		ShardCount:       1,
		MaxPendingPerKey: maxUidPendingPacket,
	})
	m.useSerialKey = useUidLock
}

func (m *TransactionMgr) InitAndRunWithConfig(cfg TransactionMgrConfig) {
	if m.started {
		logger.Errorf("transmgr can only be InitAndRun once")
		return
	}

	cfg = normalizeConfig(cfg)
	m.started = true
	m.config = cfg
	m.useSerialKey = true
	m.maxTransNum = cfg.MaxTrans
	m.closeCh = make(chan struct{})
	registerTransactionMgr(m)

	shardBufSize := perShardBufferSize(cfg.MaxTrans, cfg.ShardCount)
	m.shards = make([]*transactionShard, 0, cfg.ShardCount)
	for i := 0; i < cfg.ShardCount; i++ {
		shard := &transactionShard{
			mgr:            m,
			index:          i,
			chanInPacket:   make(chan *sharedstruct.SSPacket, shardBufSize),
			chanTransRet:   make(chan uint32, shardBufSize),
			nextTransID:    uint32(i + 1),
			transIDStep:    uint32(cfg.ShardCount),
			transMap:       make(map[uint32]*Transaction, shardBufSize),
			keyInProcess:   make(map[uint64]bool),
			pendingPackets: make(map[uint64][]*sharedstruct.SSPacket),
		}
		m.shards = append(m.shards, shard)
		m.shardWG.Add(1)
		go shard.run()
	}
}

// RegisterCmd 是兼容入口：委托 RegisterCmdE 并仅记录错误（不 Fatal、不覆盖，）。
func (m *TransactionMgr) RegisterCmd(cmd g1_protocol.CMD, cmdHandler cmd_handler.CmdHandlerFunc) {
	if err := m.RegisterCmdE(cmd, cmdHandler); err != nil {
		logger.Errorf("RegisterCmd(%d) failed: %v", cmd, err)
	}
}

// RegisterCmdE 注册一个 cmd handler，返回明确哨兵错误：
//   - nil handler → ErrNilCmdHandler
//   - 重复 cmd → ErrDuplicateCmd（不再 last-write-wins 静默覆盖）
//   - InitAndRun 之后注册 → ErrRegisterAfterStart
//
// 历史缺陷：RegisterCmd 在 m.started 时 logger.Fatalf（杀进程），且不检测重复
//（last-write-wins 静默覆盖）。
func (m *TransactionMgr) RegisterCmdE(cmd g1_protocol.CMD, cmdHandler cmd_handler.CmdHandlerFunc) error {
	if cmdHandler == nil {
		return ErrNilCmdHandler
	}
	if m.started {
		return ErrRegisterAfterStart
	}
	if m.cmdHandlers == nil {
		m.cmdHandlers = make(map[g1_protocol.CMD]cmd_handler.CmdHandlerFunc)
	}
	if _, exists := m.cmdHandlers[cmd]; exists {
		return ErrDuplicateCmd
	}
	m.cmdHandlers[cmd] = cmdHandler
	return nil
}

func (m *TransactionMgr) ProcessSSPacket(packet *sharedstruct.SSPacket) {
	if packet == nil {
		return
	}
	if m.closing.Load() && packet.Header.DstTransID == 0 {
		observeTransactionPacket("request", packet.Header.Cmd, "rejected_shutdown")
		logger.Warningf("transmgr is shutting down, reject request {header:%#v}", packet.Header)
		return
	}
	shard := m.selectShard(packet)
	if shard == nil {
		logger.Errorf("transmgr is not initialized, drop packet {header:%#v}", packet.Header)
		return
	}

	// Bounded enqueue: never block the bus consumer goroutine forever.
	// A shard stuck for longer than the timeout indicates handler starvation;
	// dropping with a metric is preferable to backpressuring the entire MQ
	// consumption chain.
	if !packet.SendToChan(shard.chanInPacket, 3*time.Second) {
		m.onPacketDropped()
		observeTransactionPacket("request", packet.Header.Cmd, "dropped_queue_full")
		logger.Errorf("transmgr shard queue full, drop packet {shard:%d, header:%#v}", shard.index, packet.Header)
	}
}

func (m *TransactionMgr) Close(ctx context.Context) error {
	if !m.started {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	m.closeOnce.Do(func() {
		m.closing.Store(true)
		close(m.closeCh)
	})

	done := make(chan struct{})
	go func() {
		m.shardWG.Wait()
		close(done)
	}()

	select {
	case <-done:
		unregisterTransactionMgr(m)
		m.shards = nil
		m.started = false
		return nil
	case <-ctx.Done():
		return errors.Join(errors.New("transaction manager shutdown timed out"), ctx.Err())
	}
}

// 发给自己（SelfBusId）的消息直接调用ProcessSSPacket，而不到网络上转一圈
func (m *TransactionMgr) SendPbMsgToMyself(selfBusId uint32, rid uint64, uid uint64, zone uint32, cmd g1_protocol.CMD, pbMsg proto.Message) {
	data, err := proto.Marshal(pbMsg)
	if err != nil {
		logger.Errorf("Failed to SendMsgToMyself {uid:%v, cmd,%v, msg:%v}", uid, cmd, pbMsg)
		return
	}

	packet := &sharedstruct.SSPacket{
		Header: sharedstruct.SSPacketHeader{
			SrcBusID:   selfBusId,
			DstBusID:   selfBusId,
			SrcTransID: 0,
			DstTransID: 0,
			Uid:        uid,
			RouterID:   rid,
			Cmd:        uint32(cmd),
			Zone:       zone,
			Ip:         0,
			Flag:       0,
			BodyLen:    uint32(len(data)),
			CmdSeq:     0,
		},
		Body: data,
	}

	m.ProcessSSPacket(packet)
}

func (m *TransactionMgr) StatsSnapshot() TransactionMgrStats {
	return TransactionMgrStats{
		ShardCount:         len(m.shards),
		ActiveTransactions: m.activeTransactions.Load(),
		PendingPackets:     m.pendingPackets.Load(),
		DroppedPackets:     m.droppedPackets.Load(),
	}
}

func (m *TransactionMgr) selectShard(packet *sharedstruct.SSPacket) *transactionShard {
	if len(m.shards) == 0 {
		return nil
	}
	if len(m.shards) == 1 {
		return m.shards[0]
	}

	if packet.Header.DstTransID != 0 {
		idx := int((packet.Header.DstTransID - 1) % uint32(len(m.shards)))
		return m.shards[idx]
	}

	if key, ok := m.serialKeyFromHeader(packet.Header); ok {
		idx := int(key % uint64(len(m.shards)))
		return m.shards[idx]
	}

	next := m.roundRobin.Add(1)
	idx := int((next - 1) % uint32(len(m.shards)))
	return m.shards[idx]
}

func (m *TransactionMgr) serialKeyFromHeader(header sharedstruct.SSPacketHeader) (uint64, bool) {
	if !m.useSerialKey {
		return 0, false
	}
	if header.RouterID != 0 {
		return header.RouterID, true
	}
	if header.Uid != 0 {
		return header.Uid, true
	}
	return 0, false
}

func (m *TransactionMgr) tryAcquireTransSlot() bool {
	for {
		cur := m.activeTransactions.Load()
		if cur >= int64(m.maxTransNum) {
			return false
		}
		if m.activeTransactions.CompareAndSwap(cur, cur+1) {
			return true
		}
	}
}

func (m *TransactionMgr) releaseTransSlot() {
	m.activeTransactions.Add(-1)
}

func (m *TransactionMgr) onPendingPacketAdded() {
	m.pendingPackets.Add(1)
}

func (m *TransactionMgr) onPendingPacketRemoved() {
	m.pendingPackets.Add(-1)
}

func (m *TransactionMgr) onPacketDropped() {
	m.droppedPackets.Add(1)
}

func (s *transactionShard) run() {
	defer s.mgr.shardWG.Done()
	for {
		select {
		case packet, ok := <-s.chanInPacket:
			if !ok {
				logger.Error("transaction shard chanInPacket is closed")
				return
			}
			s.processSSPacket(packet)
		case transID, ok := <-s.chanTransRet:
			if !ok {
				logger.Error("transaction shard chanTransRet is closed")
				return
			}
			s.processTransactionRet(transID)
		case <-s.mgr.closeCh:
			if s.canStop() {
				return
			}
			if !s.drainOne() {
				time.Sleep(time.Millisecond)
			}
		}
	}
}

func (s *transactionShard) canStop() bool {
	return len(s.chanInPacket) == 0 && len(s.chanTransRet) == 0 && len(s.transMap) == 0 && len(s.pendingPackets) == 0
}

func (s *transactionShard) drainOne() bool {
	select {
	case packet := <-s.chanInPacket:
		if packet != nil {
			s.processSSPacket(packet)
		}
		return true
	case transID := <-s.chanTransRet:
		s.processTransactionRet(transID)
		return true
	default:
		return false
	}
}

func (s *transactionShard) processSSPacket(packet *sharedstruct.SSPacket) int32 {
	uid := packet.Header.Uid
	rid := packet.Header.RouterID
	dstTransID := packet.Header.DstTransID
	cmd := packet.Header.Cmd
	if logger.DebugEnabled() {
		logger.CmdDebugf(cmd, "Recv uid: %v | SrcBusID: %v | cmd [%v]", uid, bus.IpIntToString(packet.Header.SrcBusID), g1_protocol.CMD(packet.Header.Cmd))
	}

	if dstTransID != 0 {
		trans, in := s.transMap[dstTransID]
		if !in {
			observeTransactionPacket("response", cmd, "missing_transaction")
			logger.Errorf("received a response can't be handled by any transaction{header:%#v}", packet.Header)
			return -3
		}
		if !packet.SendToChan(trans.chanIn, 3*time.Second) {
			observeTransactionPacket("response", cmd, "dispatch_timeout")
			observeTransactionTimeout("dispatch_response", g1_protocol.CMD(cmd))
			logger.Errorf("timeout to send message to transaction {header: %#v}", packet.Header)
			return -4
		}
		observeTransactionPacket("response", cmd, "delivered")
		return 0
	}

	// 级联超时：请求携带的截止时间已过则直接丢弃，不再浪费下游算力
	// （调用方早已超时返回，处理结果也无人消费）。
	if packet.Header.DeadlineExceeded(time.Now().UnixMilli()) {
		s.mgr.onPacketDropped()
		observeTransactionPacket("request", cmd, "dropped_deadline_exceeded")
		logger.Warningf("Drop an expired request {uid:%d, rid:%d, cmd:%d, deadlineMs:%d}",
			uid, rid, cmd, packet.Header.DeadlineUnixMs)
		return -6
	}

	serialKey, hasSerialKey := s.mgr.serialKeyFromHeader(packet.Header)
	if hasSerialKey && s.keyInProcess[serialKey] {
		packets := s.pendingPackets[serialKey]
		if len(packets) >= s.mgr.config.MaxPendingPerKey {
			s.mgr.onPacketDropped()
			observeTransactionPacket("request", cmd, "dropped_pending_full")
			logger.Errorf("Drop a packet for serial key {key:%d, uid:%d, rid:%d, cmd:%d}", serialKey, uid, rid, cmd)
			return -1
		}

		s.pendingPackets[serialKey] = append(packets, packet)
		s.mgr.onPendingPacketAdded()
		observeTransactionPacket("request", cmd, "queued")
		return 0
	}

	cmdHandler, in := s.mgr.cmdHandlers[g1_protocol.CMD(cmd)]
	if !in {
		observeTransactionPacket("request", cmd, "missing_handler")
		logger.Errorf("no reg cmd {cmd:0x%x}", cmd)
		return -2
	}

	if !s.mgr.tryAcquireTransSlot() {
		observeTransactionPacket("request", cmd, "rejected_max_trans")
		logger.Errorf("reach transaction count limit {max:%v, packetHeader:%v}", s.mgr.maxTransNum, packet.Header)
		return -5
	}

	transID := s.nextTransID
	s.nextTransID += s.transIDStep

	transaction := newTransaction(transID, packet.Header, make(chan *sharedstruct.SSPacket, 1))
	s.transMap[transID] = transaction
	if hasSerialKey {
		s.keyInProcess[serialKey] = true
	}

	observeTransactionPacket("request", cmd, "started")
	go transaction.run(cmdHandler, packet, s.chanTransRet)
	return 0
}

func (s *transactionShard) processTransactionRet(transID uint32) {
	trans, in := s.transMap[transID]
	if !in || trans == nil {
		logger.Errorf("no trans in map {transId:%d}", transID)
		return
	}

	close(trans.chanIn)
	delete(s.transMap, transID)
	s.mgr.releaseTransSlot()

	serialKey, hasSerialKey := s.mgr.serialKeyFromHeader(trans.OriPacketHeader)
	if !hasSerialKey {
		return
	}

	delete(s.keyInProcess, serialKey)

	packets, in := s.pendingPackets[serialKey]
	if !in {
		return
	}
	if len(packets) == 0 {
		delete(s.pendingPackets, serialKey)
		return
	}

	nextPacket := packets[0]
	if len(packets) == 1 {
		delete(s.pendingPackets, serialKey)
	} else {
		s.pendingPackets[serialKey] = packets[1:]
	}
	s.mgr.onPendingPacketRemoved()
	s.processSSPacket(nextPacket)
}

func perShardBufferSize(maxTrans int32, shardCount int) int {
	if shardCount <= 0 {
		return 1
	}

	size := int(maxTrans) / shardCount
	if int(maxTrans)%shardCount != 0 {
		size++
	}
	if size <= 0 {
		return 1
	}
	return size
}
