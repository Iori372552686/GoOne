package http_sign

import (
	"sync"

	"github.com/Iori372552686/GoOne/lib/api/logger"
)

// defaultSignKey 为未指定 key 时返回的 HttpSign 实例名。
const defaultSignKey = "default"

// SignMgr 是按名注册的 HttpSign 实例表，并发安全，供各服务共享。
type SignMgr struct {
	mu        sync.RWMutex
	instances map[string]*HttpSign
}

// NewSignMgr 返回一个空的 SignMgr。
func NewSignMgr() *SignMgr {
	return &SignMgr{instances: make(map[string]*HttpSign)}
}

// SetSignIns 在 key 下注册（或替换）一个 HttpSign 实例。
func (m *SignMgr) SetSignIns(key string, impl *HttpSign) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.instances[key] = impl
}

// GetSignIns 返回指定 key 的实例；未传 key 时返回 "default" 实例。
// 不存在时返回 nil。
func (m *SignMgr) GetSignIns(keys ...string) *HttpSign {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(keys) == 0 {
		return m.instances[defaultSignKey]
	}
	return m.instances[keys[0]]
}

// Count 返回已注册的实例数量。
func (m *SignMgr) Count() int {
	if m == nil {
		return 0
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.instances)
}

// InitAndRun 为每个 Config 项构建并注册一个 HttpSign 实例，
// 是服务启动时调用的主入口。
func (m *SignMgr) InitAndRun(cfgs []Config) {
	logger.Infof("SignMgr InsInit..")
	for _, c := range cfgs {
		ins := BuildHttpSign(
			c.SignName, c.PrivateKey, int64(c.ExpiredTime),
			c.TimestampName, c.RequestIDName, c.VersionType,
		).WithSignType(toSignType(c.SignType))
		m.SetSignIns(c.IndexName, ins)
	}
	logger.Infof("SignMgr InsInit... Done !")
}
