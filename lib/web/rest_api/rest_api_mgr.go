package rest_api

import (
	"sync"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/web/http_sign"
)

// defaultRestKey 为未指定 key 时返回的 RestApi 实例名。
const defaultRestKey = "default"

// RestApiMgr 是按服务名注册的 RestApi 实例表，并发安全。
type RestApiMgr struct {
	mu        sync.RWMutex
	instances map[string]*RestApi
}

// NewRestApiMgr 返回一个空的 RestApiMgr。
func NewRestApiMgr() *RestApiMgr {
	return &RestApiMgr{instances: make(map[string]*RestApi)}
}

// SetRestIns 在 key 下注册（或替换）一个 RestApi 实例。
// key 为空或 impl 为 nil 时为空操作。
func (m *RestApiMgr) SetRestIns(key string, impl *RestApi) {
	if key == "" || impl == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.instances[key] = impl
}

// GetRestIns 返回指定 key 的实例；未传 key 时返回 "default" 实例。
// 不存在时返回 nil。
func (m *RestApiMgr) GetRestIns(keys ...string) *RestApi {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(keys) == 0 {
		return m.instances[defaultRestKey]
	}
	return m.instances[keys[0]]
}

// Count 返回已注册的实例数量。
func (m *RestApiMgr) Count() int {
	if m == nil {
		return 0
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.instances)
}

// Init 为每个 Config 项构建并注册一个 RestApi 实例，
// 并从 signs 解析其可选的 HttpSign 依赖。
func (m *RestApiMgr) Init(cfgs []Config, signs *http_sign.SignMgr) {
	logger.Infof("RestApiMgr InsInit..")
	for _, c := range cfgs {
		m.SetRestIns(c.ServiceName, NewRestInstances(c, signs))
	}
	logger.Infof("RestApiMgr InsInit... Done !")
}
