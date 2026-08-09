package ssrpc

import g1_protocol "github.com/Iori372552686/g1_common/protocol"

// Session captures transport-neutral request metadata that middleware and
// observability code can rely on without depending on a concrete transport.
type Session struct {
	Transport Transport
	Cmd       g1_protocol.CMD
	Method    string

	UID  uint64
	Zone uint32
	RID  uint64

	SrcBusID uint32
	PeerIP   uint32
	PeerFlag uint32
	TransID  uint32
}
