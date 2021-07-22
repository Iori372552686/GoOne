package redis

import (
	"fmt"
	"sync"
	"time"
	`bian/src/common/logger`
	`github.com/mediocregopher/radix/v3`
)

const POOL_SIZE = 100

type RedisMgr struct {
	clients	sync.Map	// map[uint32]radix.Client
}

func NewRedisMgr() *RedisMgr {
	m := new(RedisMgr)
	return m
}

func (m *RedisMgr) AddInstance(instID uint32, ip string, port uint16, password string, dbIndex int, isCluster bool) error {

	addr := fmt.Sprintf("%v:%v", ip, port)
	connFunc := func(network, addr string) (radix.Conn, error) {
		if isCluster {
			return radix.Dial(network, addr,
				radix.DialTimeout(10 * time.Second),
				radix.DialAuthPass(password),
			)
		} else {
			return radix.Dial(network, addr,
				radix.DialTimeout(10 * time.Second),
				radix.DialAuthPass(password),
				radix.DialSelectDB(dbIndex),
			)
		}
	}
	poolFunc := func(network, addr string) (radix.Client, error) {
		pingOpt := radix.PoolPingInterval(time.Second)
		return radix.NewPool(network, addr, POOL_SIZE, radix.PoolConnFunc(connFunc), pingOpt)
	}

	var err error
	var client radix.Client
	if isCluster {
		client, err = radix.NewCluster([]string{addr}, radix.ClusterPoolFunc(poolFunc))
	} else {
		client, err = poolFunc("tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("failed to create redis client | %w", err)
	}

	m.clients.Load(instID)

	if v, exist := m.clients.Load(instID); exist {
		logger.Warningf("overwrite a redis instance")
		if oldClient, ok := v.(radix.Client); ok {
			_ = oldClient.Close()
		}
		m.clients.Delete(instID)
	}

	m.clients.Store(instID, client)

	return nil
}

func (m *RedisMgr) GetClient(instID uint32) radix.Client {
	v, exist := m.clients.Load(instID)
	client, ok := v.(radix.Client)
	if !exist || !ok || client == nil {
		logger.Errorf("failed to get a redis client")
		return nil
	}
	return client
}

func (m *RedisMgr) DoFlatCmd(instID uint32, result interface{}, cmd, key string, args ...interface{}) error {
	client := m.GetClient(instID)
	if client == nil {
		return fmt.Errorf("RedisDoCmd cannot get a client {id:%v, cmd:%s}", instID, cmd)
	}
	return client.Do(radix.FlatCmd(result, cmd, key, args...))
}

func (m *RedisMgr) DoCmd(instID uint32, result interface{}, cmd string, args ...string) error {
	client := m.GetClient(instID)
	if client == nil {
		return fmt.Errorf("RedisDoCmd cannot get a client {id:%v, cmd:%s}", instID, cmd)
	}
	return client.Do(radix.Cmd(result, cmd, args...))
}



func (m *RedisMgr) SetBytes(instID uint32, key string, value []byte) error {
	return m.DoFlatCmd(instID, nil, "SET", key, value)
}

func (m *RedisMgr) GetBytes(instID uint32, key string) ([]byte, error) {
	var result []byte
	mn := radix.MaybeNil{Rcv: &result}
	err := m.DoFlatCmd(instID, &mn, "GET", key)
	if err != nil {
		return nil, err
	}
	if mn.Nil {
		return nil, nil
	}
	return result, nil
}

func (m *RedisMgr) MGetBytes(instID uint32, keys []string) ([]string, error) {
	var result []string
	mn := radix.MaybeNil{Rcv: &result}
	err := m.DoCmd(instID, &mn, "MGET", keys...)
	if err != nil {
		return nil, err
	}
	if mn.Nil {
		return nil, nil
	}
	return result, nil
}

func (m *RedisMgr) DelKey(instID uint32, key string) error {
	return m.DoFlatCmd(instID, nil, "DEL", key)
}

func (m *RedisMgr) ZsetSet(instID uint32, setName string, key string, score int32) error {
	return m.DoFlatCmd(instID, nil, "ZADD", setName, key, score)
}

func (m *RedisMgr) ZsetRange(instID uint32, setName string, beginIdx, endIdx int32) ([]string, error) {
	var result []string
	mn := radix.MaybeNil{Rcv: &result}
	err := m.DoFlatCmd(instID, &mn, "ZRANGE", setName, beginIdx, endIdx)
	if err != nil {
		return nil, err
	}
	if mn.Nil {
		return nil, nil
	}
	return result, nil
}

func (m *RedisMgr) IncrKey(instID uint32, key string) (int64, error) {
	result := int64(0)
	err := m.DoCmd(instID, &result, "INCR", key)
	return result, err
}
