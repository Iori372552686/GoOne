package plug

import (
	"bytes"
	"net/http"
	"sync"
	"time"

	"github.com/bytedance/sonic/encoder"
)

// fatalNotifier 负责 fatal 日志的外发通知。
//
// 出于安全考虑，通知地址不再硬编码在仓库中（历史版本曾硬编码一个真实可用的钉钉机器人
// webhook Token，已轮换）。未调用 ConfigureFatalHook 配置地址时，UploadFatalToDingHook
// 不发起任何网络请求，直接丢弃消息，绝不回退到任何内置地址。
var (
	fatalHookMu  sync.RWMutex
	fatalHookURL string

	// httpDo 执行实际的 HTTP 请求。默认实现使用带 5s 超时的 client；测试通过
	// swapHTTPDo 注入桩函数以避免真实网络副作用。读写均经 fatalHookMu 保护以满足
	// race 检测（go test -race）。
	httpDo = defaultHTTPDo
)

// defaultHTTPDo 是 httpDo 的生产默认实现。
func defaultHTTPDo(req *http.Request) error {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// ConfigureFatalHook 设置 fatal 通知的完整 webhook 地址；addr 为空表示关闭通知。
// 该函数仅供服务启动期调用一次，配置值来自外部注入（环境变量或配置文件），不得硬编码。
func ConfigureFatalHook(addr string) {
	fatalHookMu.Lock()
	defer fatalHookMu.Unlock()
	fatalHookURL = addr
}

// fatalHookAddr 返回当前配置的地址，供测试观测。
func fatalHookAddr() string {
	fatalHookMu.RLock()
	defer fatalHookMu.RUnlock()
	return fatalHookURL
}

// UploadFatalToDingHook 向已配置的 webhook 发送 fatal 通知。
// 未配置地址时直接返回，不发起网络请求，也不回退到任何内置地址。
func UploadFatalToDingHook(msgbody string) {
	addr := fatalHookAddr()
	if addr == "" {
		return
	}

	body := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": "logger",
			"text":  msgbody,
		},
	}

	var data = bytes.NewBuffer(nil)
	_ = encoder.NewStreamEncoder(data).Encode(body)

	req, err := http.NewRequest(http.MethodPost, addr, data)
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	fatalHookMu.RLock()
	do := httpDo
	fatalHookMu.RUnlock()
	_ = do(req)
}
