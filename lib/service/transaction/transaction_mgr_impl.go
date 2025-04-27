package transaction

import (
	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/api/sharedstruct"
	"github.com/Iori372552686/GoOne/lib/service/bus"
	g1_protocol "github.com/gdsgog/poker_protocol/protocol"
	"github.com/golang/protobuf/proto"

	"time"
)

// 使用：
//   . 通过InitAndRun初始化
//   . RegisterCmd注册：指定的Cmd所对应的继承自TransBase的事务
//   . 外部通过调用ProcessSSPacket，将收到的SSPacket传给transmgr，
//     从而使transmgr会根据packet.DstTransId，进行开启协程处理新事务，或将消息转给已有协程。

// 并发模型：
//   . 主处理逻辑由一个单独的协程完成runTransMgr（接收chan：chanInPacket）
//   . 收到消息后，会根据一定规则“开启新子协程”或“使用已有子协程”去处理这个消息。
//     . 一般规则：根据消息中的DstTransId。（为0开启，>0发给指定协程）
//     . 特殊规则：设置了useUidLock时，一个uid最多只关联一个协程。（后面的消息进队列）

// -------------------------------- public --------------------------------

// 初始化
func (m *TransactionMgr) InitAndRun(maxTrans int32, useUidLock bool, maxUidPendingPacket int) {
	if m.started {
		logger.Errorf("transmgr can only be InitAndRun once")
		return
	}

	m.started = true

	m.chanInPacket = make(chan *sharedstruct.SSPacket, maxTrans)

	m.curTransID = 1
	m.maxTransNum = maxTrans
	m.transMap = make(map[uint32]*Transaction, maxTrans)
	m.chanTransRet = make(chan uint32, maxTrans)

	m.useUidLock = useUidLock
	m.uidInProcess = make(map[uint64]bool, 0)
	m.maxUidPendingPacket = maxUidPendingPacket
	m.uidPendingPackets = make(map[uint64][]*sharedstruct.SSPacket, 0)

	go m.run()
}

// 注册命令字
func (m *TransactionMgr) RegisterCmd(cmd g1_protocol.CMD, cmdHandler cmd_handler.CmdHandlerFunc) {
	if m.started {
		// TransactionMgr的协程安全，是通过把所有操作放到run里，来实现的。
		// 所以run启动之后，外部不能再操作TransactionMgr中的字段。
		// RegisterCmd是在初始化时完成的，所以一般来说问题不大。
		// 但如果出现问题，可以把处理通过chan发给run来处理。
		logger.Fatalf("RegisterCmd must be invoked before InitAndRun")
	}

	if nil == m.cmdHandlers {
		m.cmdHandlers = make(map[g1_protocol.CMD]cmd_handler.CmdHandlerFunc, 0)
	}
	m.cmdHandlers[cmd] = cmdHandler
}

// ProcessSSPacket将获得packet的所有权
func (m *TransactionMgr) ProcessSSPacket(packet *sharedstruct.SSPacket) {
	m.chanInPacket <- packet
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

// -------------------------------- private --------------------------------

type transRet struct {
	transID uint32
	ret     int32
}

type TransactionMgr struct {
	started     bool                                           // TransMgr已启动
	cmdHandlers map[g1_protocol.CMD]cmd_handler.CmdHandlerFunc // 这里注册的命令字

	chanInPacket chan *sharedstruct.SSPacket // 外部通过此通道，把其他服务器发来的消息传给transmgr进行处理。

	curTransID   uint32
	maxTransNum  int32
	transMap     map[uint32]*Transaction // 保存所有的trans。（此处用独立的transInfo，而不是Transaction。是为了避免trans_mgr与Transaction的协程访问同一份Transaction数据）
	chanTransRet chan uint32             // 这个channel用来接收trans执行完后返回的结果

	useUidLock bool // 每个uid同时只能有一个协程在处理。（后面的消息进等待队列）
	// 状态服务器（内存中留有uid相关数据），这个应该是一定要为true的，不然两个协程同时处理一个uid会有问题。
	maxUidPendingPacket int                                 // 当useUidLock=true时，此值为每个uid的消息等待队列的长度。
	uidInProcess        map[uint64]bool                     // uid有正在处理的事务
	uidPendingPackets   map[uint64][]*sharedstruct.SSPacket // 每个uid的待处理消息队列。
}

func (m *TransactionMgr) run() {
Loop:
	for {
		select {
		case packet, ok := <-m.chanInPacket:
			if !ok {
				logger.Error("m.chanInPacket is closed")
				break Loop
			}
			if packet != nil {
				m.processSSPacket(packet)
			}
		case aTransRet, ok := <-m.chanTransRet:
			if !ok {
				logger.Error("m.chanTransRet is closed")
				break Loop
			}
			m.processTransactionRet(aTransRet)
		}
	}
}

func (m *TransactionMgr) processSSPacket(packet *sharedstruct.SSPacket) int32 {
	uid := packet.Header.Uid
	rid := packet.Header.RouterID
	dstTransID := packet.Header.DstTransID
	cmd := packet.Header.Cmd
	logger.CmdDebugf(cmd, "Recv uid: %v | SrcBusID: %v |  cmd [%v] ", uid, bus.IpIntToString(packet.Header.SrcBusID), g1_protocol.CMD(packet.Header.Cmd))

	if dstTransID == 0 {
		if m.useUidLock && m.uidInProcess[rid] { // pending or drop
			packets := m.uidPendingPackets[rid]
			if packets == nil {
				packets = make([]*sharedstruct.SSPacket, 0)
			}
			if len(packets) >= m.maxUidPendingPacket {
				logger.Errorf("Drop a packet for uid lock {packet:%#v}", packet)
				return -1
			} else {
				//logger.Debugf("pending a packet{uid:%v, packet:%#v}", uid, packet)
				m.uidPendingPackets[rid] = append(packets, packet)
				return 0
			}
		}

		if len(m.transMap) >= int(m.maxTransNum) {
			logger.Errorf("reach transaction count limit {max:%v, packetHeader:%v}",
				m.maxTransNum, packet.Header)
			return -5
		}

		cmdHandler, in := m.cmdHandlers[g1_protocol.CMD(cmd)]
		if !in {
			logger.Errorf("no reg cmd {cmd:0x%x}", cmd)
			return -2
		}

		myTransID := m.curTransID //todo: id冲突。有可能很早以前的id还在运行中。
		m.curTransID += 1

		// 这里开启协程执行trans，每一个trans都会创建一个新的结构体来运行
		transaction := newTransaction(myTransID, packet.Header, make(chan *sharedstruct.SSPacket, 1))
		m.transMap[myTransID] = transaction
		m.uidInProcess[rid] = true
		//logger.Debugf("Create a transaction {transId:%v, packet:%#v}", myTransID, packet.Header)
		go transaction.run(cmdHandler, packet, m.chanTransRet)
	} else {
		// 恢复挂起的trans
		if trans, in := m.transMap[dstTransID]; in {
			if !packet.SendToChan(trans.chanIn, 3*time.Second) {
				logger.Errorf("timeout to send message to transaction {header: %#v}", packet.Header)
				return -4
			}
		} else {
			logger.Errorf("received a response can't be handled by any transaction{header:%#v}", packet.Header)
			return -3
		}
	}

	return 0
}

// 这里处理每一个transaction routine的返回值
func (m *TransactionMgr) processTransactionRet(aTransRet uint32) {
	trans, in := m.transMap[aTransRet]
	if !in || trans == nil {
		logger.Errorf("no trans in map {transId:%d}\n", aTransRet)
		return
	}
	//trans.Debugf("Transaction return {cmd:0x%x, ret:%v}", trans.Cmd(), aTransRet)

	//uid := trans.Uid()
	rid := trans.Rid()
	close(trans.chanIn)
	delete(m.transMap, aTransRet)
	delete(m.uidInProcess, rid)

	// 这里如果uid对应的队列里面还有其他未处理的包，则继续处理
	if m.useUidLock {
		if packets, in := m.uidPendingPackets[rid]; in {
			if len(packets) <= 0 {
				delete(m.uidPendingPackets, rid)
			} else {
				p := packets[0]
				m.uidPendingPackets[rid] = packets[1:]
				m.processSSPacket(p)
			}
		}
	}
	//logger.Debugf("Transaction finished {transId:%v}", aTransRet.transID)
}
