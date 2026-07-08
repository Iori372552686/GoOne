package tcp_server

import (
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/Iori372552686/GoOne/lib/api/sharedstruct"
)

// echoHandler echoes every parsed CSPacket back to the client through the
// server's write channel, exercising the full gateway hot path:
// read loop -> bytes.Buffer -> frame split -> WriteData (alloc+copy+timer).
type echoHandler struct {
	svr *TcpPacketSvr
}

func (h *echoHandler) OnConn(conn net.Conn) {}

func (h *echoHandler) OnPacket(conn net.Conn, data []byte) {
	headerLen := sharedstruct.ByteLenOfCSPacketHeader()
	_ = h.svr.WriteData(conn, data[:headerLen], data[headerLen:])
}

func (h *echoHandler) OnClose(conn net.Conn) {}

func startEchoServer(tb testing.TB) (addr string, stop func()) {
	tb.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		tb.Fatalf("listen failed: %v", err)
	}
	port := lis.Addr().(*net.TCPAddr).Port
	_ = lis.Close()

	svr := &TcpPacketSvr{}
	handler := &echoHandler{svr: svr}
	packetInfo := TcpPacketInfo{
		HeaderLen: sharedstruct.ByteLenOfCSPacketHeader(),
		BodyLen:   sharedstruct.ByteLenOfCSPacketBody,
	}
	if err := svr.InitAndRun("127.0.0.1", port, packetInfo, handler); err != nil {
		tb.Fatalf("server init failed: %v", err)
	}

	return fmt.Sprintf("127.0.0.1:%d", port), func() {}
}

func buildFrame(bodySize int) []byte {
	body := make([]byte, bodySize)
	header := sharedstruct.CSPacketHeader{
		Uid:     10001,
		Cmd:     0x00020001,
		BodyLen: uint32(len(body)),
	}
	frame := append(header.ToBytes(), body...)
	return frame
}

// BenchmarkTCPEchoRoundTrip measures single-connection request/response
// round-trip latency and allocations through the gateway framing path.
func BenchmarkTCPEchoRoundTrip(b *testing.B) {
	for _, bodySize := range []int{64, 1024, 16 * 1024} {
		b.Run(fmt.Sprintf("body_%d", bodySize), func(b *testing.B) {
			addr, stop := startEchoServer(b)
			defer stop()

			conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
			if err != nil {
				b.Fatalf("dial failed: %v", err)
			}
			defer conn.Close()

			frame := buildFrame(bodySize)
			recv := make([]byte, len(frame))

			b.ReportAllocs()
			b.SetBytes(int64(len(frame)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := conn.Write(frame); err != nil {
					b.Fatalf("write failed: %v", err)
				}
				if _, err := io.ReadFull(conn, recv); err != nil {
					b.Fatalf("read failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkTCPEchoPipelined measures server-side throughput with N in-flight
// frames per iteration, closer to a real gateway load profile.
func BenchmarkTCPEchoPipelined(b *testing.B) {
	const window = 64
	addr, stop := startEchoServer(b)
	defer stop()

	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		b.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	frame := buildFrame(256)

	var wg sync.WaitGroup
	wg.Add(1)
	total := b.N * len(frame)
	go func() {
		defer wg.Done()
		buf := make([]byte, 64*1024)
		read := 0
		for read < total {
			n, err := conn.Read(buf)
			if err != nil {
				return
			}
			read += n
		}
	}()

	b.ReportAllocs()
	b.SetBytes(int64(len(frame)))
	b.ResetTimer()

	batch := make([]byte, 0, len(frame)*window)
	for i := 0; i < b.N; i += window {
		batch = batch[:0]
		n := window
		if b.N-i < n {
			n = b.N - i
		}
		for j := 0; j < n; j++ {
			batch = append(batch, frame...)
		}
		if _, err := conn.Write(batch); err != nil {
			b.Fatalf("write failed: %v", err)
		}
	}
	wg.Wait()
}
