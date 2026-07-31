// Capacity load generator for GoOne connsvr.
//
// Opens N TCP connections, sends a login frame (binds UID), and keeps each
// connection alive with periodic heartbeats. Reports login success rate.
// Run against a full service stack (connsvr:11001 + mainsvr/mysqlsvr via bus).
//
//	go run ./tools/tester/cmd/capacity -addr 127.0.0.1:11001 -conns 1000 -duration 60s
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:11001", "connsvr TCP address")
	conns := flag.Int("conns", 1000, "number of concurrent connections")
	startUID := flag.Uint64("start-uid", 300001, "first UID (incremented per conn)")
	duration := flag.Duration("duration", 30*time.Second, "hold duration")
	hbInterval := flag.Duration("hb", 5*time.Second, "heartbeat interval per conn")
	flag.Parse()

	var (
		loginOK   atomic.Int64
		loginFail atomic.Int64
		hbOK      atomic.Int64
		hbFail    atomic.Int64
	)
	var wg sync.WaitGroup
	deadline := time.Now().Add(*duration)

	// Open connections with a small ramp to avoid SYN bursts.
	for i := 0; i < *conns; i++ {
		uid := *startUID + uint64(i)
		c, err := net.DialTimeout("tcp", *addr, 5*time.Second)
		if err != nil {
			loginFail.Add(1)
			continue
		}
		// CSPacketHeader login frame: minimal valid frame binding this UID.
		// Layout matches sharedstruct.CSPacketHeader (28 bytes) + empty body.
		var hdr [28]byte
		binary.LittleEndian.PutUint64(hdr[0:], uid)   // Uid
		binary.LittleEndian.PutUint32(hdr[8:], 0x00020001) // Cmd = login
		binary.LittleEndian.PutUint32(hdr[24:], 0)    // BodyLen = 0
		if _, err := c.Write(hdr[:]); err != nil {
			loginFail.Add(1)
			c.Close()
			continue
		}
		loginOK.Add(1)

		wg.Add(1)
		go func(c net.Conn, uid uint64) {
			defer wg.Done()
			defer c.Close()
			// Drain any responses (best-effort).
			buf := make([]byte, 1024)
			_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
			c.Read(buf)
			_ = c.SetReadDeadline(time.Time{})

			hb := time.NewTicker(*hbInterval)
			defer hb.Stop()
			for {
				select {
				case <-time.After(time.Until(deadline)):
					return
				case <-hb.C:
					// Heartbeat frame (same header layout, different cmd).
					var h [28]byte
					binary.LittleEndian.PutUint64(h[0:], uid)
					binary.LittleEndian.PutUint32(h[8:], 0x00020002) // heartbeat cmd
					binary.LittleEndian.PutUint32(h[24:], 0)
					_ = c.SetWriteDeadline(time.Now().Add(3 * time.Second))
					if _, err := c.Write(h[:]); err != nil {
						hbFail.Add(1)
						return
					}
					hbOK.Add(1)
				}
			}
		}(c, uid)

		// Ramp: ~200 conns/sec
		if i%200 == 0 {
			time.Sleep(time.Second)
		}
	}

	fmt.Fprintf(os.Stderr, "[capacity] opened %d conns (login ok=%d fail=%d), holding %v\n",
		*conns, loginOK.Load(), loginFail.Load(), *duration)

	// Periodic status until deadline.
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-time.After(time.Until(deadline)):
			wg.Wait()
			fmt.Fprintf(os.Stderr, "[capacity] done. login ok=%d fail=%d | heartbeat ok=%d fail=%d\n",
				loginOK.Load(), loginFail.Load(), hbOK.Load(), hbFail.Load())
			return
		case <-tick.C:
			fmt.Fprintf(os.Stderr, "[capacity] heartbeat ok=%d fail=%d\n", hbOK.Load(), hbFail.Load())
		}
	}
}
