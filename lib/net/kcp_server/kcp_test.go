package kcp_server

import (
	"io"
	"testing"
	"time"

	Kcp "github.com/xtaci/kcp-go/v5"
)

// Test_main runs a bounded KCP echo round-trip over loopback.
func Test_main(t *testing.T) {
	listener, err := Kcp.ListenWithOptions("127.0.0.1:0", nil, 10, 3)
	if err != nil {
		t.Fatalf("listen kcp failed: %v", err)
	}
	defer listener.Close()

	go func() {
		for {
			s, acceptErr := listener.AcceptKCP()
			if acceptErr != nil {
				return
			}
			go handleEcho(s)
		}
	}()

	sess, err := Kcp.DialWithOptions(listener.Addr().String(), nil, 10, 3)
	if err != nil {
		t.Fatalf("dial kcp failed: %v", err)
	}
	defer sess.Close()
	_ = sess.SetDeadline(time.Now().Add(10 * time.Second))

	for i := 0; i < 3; i++ {
		data := []byte(time.Now().String())
		if _, err := sess.Write(data); err != nil {
			t.Fatalf("write failed: %v", err)
		}

		buf := make([]byte, len(data))
		if _, err := io.ReadFull(sess, buf); err != nil {
			t.Fatalf("read failed: %v", err)
		}
		if string(buf) != string(data) {
			t.Fatalf("echo mismatch: sent %q, got %q", data, buf)
		}
	}
}

// handleEcho send back everything it received
func handleEcho(conn *Kcp.UDPSession) {
	buf := make([]byte, 4096)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			return
		}

		_, err = conn.Write(buf[:n])
		if err != nil {
			return
		}
	}
}
