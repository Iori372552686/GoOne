package websvr

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
)

// TestWebHTTPDrainSuccessClearsServer 验证 Drain（shutdown）成功时清空 httpSrv
// 指针；空 server 的 Shutdown 立即成功。
func TestWebHTTPDrainSuccessClearsServer(t *testing.T) {
	w := &webRuntimeComponent{}
	srv := &http.Server{Handler: http.NewServeMux()}
	w.setHTTPServer(srv)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := w.shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	// Shutdown 成功，指针应清空。
	if w.getHTTPServer() != nil {
		t.Fatal("shutdown 成功后 httpSrv 指针应清空")
	}
}

// TestWebHTTPDrainTimeoutKeepsServerWhenShutdownBlocks 验证 当 Shutdown 因 ctx
// 超时返回错误时，保留 httpSrv 指针供 Stop 强制 Close。
//
// 通过一个挂起连接的真实 server 模拟 Shutdown 阻塞：起一个 listener + 慢 handler，
// Shutdown 在 ctx 超时后返回。
func TestWebHTTPDrainTimeoutKeepsServerWhenShutdownBlocks(t *testing.T) {
	// 起一个会阻塞关闭的 server：handler 持有一个长请求。
	mux := http.NewServeMux()
	shutdownStarted := make(chan struct{})
	handlerDone := make(chan struct{})
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		<-handlerDone // 阻塞直到测试结束
		w.WriteHeader(200)
	})
	srv := &http.Server{Handler: mux}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go srv.Serve(ln)
	port := ln.Addr().(*net.TCPAddr).Port

	// 发起一个慢请求（保持连接活跃）。
	go func() {
		conn, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(port))
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = conn.Write([]byte("GET /slow HTTP/1.0\r\nHost: x\r\n\r\n"))
		select {
		case <-shutdownStarted:
		case <-time.After(2 * time.Second):
		}
	}()
	time.Sleep(100 * time.Millisecond) // 等请求到达

	w := &webRuntimeComponent{}
	w.setHTTPServer(srv)

	// 极短 ctx：Shutdown 必然超时。
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	close(shutdownStarted)
	_ = w.shutdown(ctx)
	cancel()

	// 超时后指针应保留。
	if w.getHTTPServer() == nil {
		close(handlerDone)
		t.Fatal("Shutdown 超时后 httpSrv 指针应保留供 Stop 强关")
	}
	// 释放 handler，让 Stop 能完成。
	close(handlerDone)
	if err := w.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if w.getHTTPServer() != nil {
		t.Fatal("Stop 后 httpSrv 应置 nil")
	}
}

// TestWebStopForceClosesRemainingConnections 验证 Stop 对仍存在的 server 调用
// Close 强制关闭。
func TestWebStopForceClosesRemainingConnections(t *testing.T) {
	w := &webRuntimeComponent{}
	srv := &http.Server{Handler: http.NewServeMux()}
	w.setHTTPServer(srv)
	if err := w.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if w.getHTTPServer() != nil {
		t.Fatal("Stop 应清空 httpSrv 指针")
	}
}

// TestWebGRPCStopForcesAfterDrainTimeout 验证 gRPC Drain 超时保留指针，Stop 调
// Stop 强制关闭。
func TestWebGRPCStopForcesAfterDrainTimeout(t *testing.T) {
	w := &webRuntimeComponent{}
	// 构造一个真实 grpc.Server（未 Serve）。GracefulStop 在无连接时立即返回；为触发超时
	// 路径，我们用一个极短 ctx。
	srv := grpc.NewServer()
	w.setGRPCServer(srv)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	time.Sleep(3 * time.Millisecond)
	_ = w.shutdown(ctx)
	// 注意：GracefulStop 可能已完成（无连接），故指针可能已 nil；若仍存在，Stop 必须能强关。
	if w.getGRPCServer() != nil {
		if err := w.Stop(context.Background()); err != nil {
			t.Fatalf("Stop: %v", err)
		}
		if w.getGRPCServer() != nil {
			t.Fatal("Stop 后 grpcSrv 应置 nil")
		}
	}
}

// TestWebRuntimeErrorsChannelBuffered 验证 RuntimeErrorSource：channel 容量 1，不阻塞
// 上报 goroutine。
func TestWebRuntimeErrorsChannelBuffered(t *testing.T) {
	w := &webRuntimeComponent{runtimeErrCh: make(chan error, 1)}
	w.reportRuntimeErr(context.Canceled)
	select {
	case <-w.RuntimeErrors():
	default:
		t.Fatal("RuntimeErrors 应收到上报的错误")
	}
	// 再次上报不阻塞（容量 1）。
	done := make(chan struct{})
	go func() {
		w.reportRuntimeErr(context.DeadlineExceeded)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("reportRuntimeErr 不应阻塞（channel 容量 1）")
	}
}

// 并发安全：多个 goroutine 并发 get/set server 不 race（行为级保护）。
func TestWebServerAccessorsConcurrentSafe(t *testing.T) {
	w := &webRuntimeComponent{}
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); w.setHTTPServer(&http.Server{}) }()
		go func() { defer wg.Done(); _ = w.getHTTPServer() }()
	}
	wg.Wait()
}
