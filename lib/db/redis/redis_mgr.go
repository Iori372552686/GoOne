package redis

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Iori372552686/GoOne/lib/api/logger"

	"github.com/mediocregopher/radix/v3"
)

const POOL_SIZE = 100

/*
*  RedisMgr
*  @Description:
 */
type RedisMgr struct {
	clients sync.Map // map[uint32]radix.Client
}

/**
* @Description:  new redis mgr
* @return: *RedisMgr
* @Author: Iori
* @Date: 2022-02-26 11:42:47
**/
func NewRedisMgr() *RedisMgr {
	m := new(RedisMgr)
	return m
}

/**
* @Description: InitAndRun
* @receiver: self
* @param: dbIns
* @return: error
* @Author: Iori
* @Date: 2022-02-26 11:42:42
**/
func (self *RedisMgr) InitAndRun(dbIns []Config) error {
	safe := make([]string, len(dbIns))
	for i, ds := range dbIns {
		safe[i] = ds.SafeString()
	}
	logger.Infof("RedisMgr   InsInit.. | %v", safe)

	// 记录已成功添加的实例 ID，用于中途失败时逆序关闭（V3-P0-05：多实例初始化失败
	// 必须回滚已成功实例，避免连接泄漏）。
	added := make([]uint32, 0, len(dbIns))
	for _, ds := range dbIns {
		instID := uint32(ds.InstanceID)
		err := self.AddInstance(instID, ds.IP, ds.Port, ds.Password, ds.DbIndex, ds.IsCluster)
		if err != nil {
			// 逆序关闭已成功添加的实例（清理路径，忽略各自的 Close error）。
			for i := len(added) - 1; i >= 0; i-- {
				_ = self.closeInstance(added[i])
			}
			return err
		}
		added = append(added, instID)
	}

	logger.Infof("RedisMgr   InsInit... Done !")
	return nil
}

// Close 关闭所有 Redis 实例的连接池。幂等：重复调用不 panic、不重复释放
// （V3-P0-05：Redis Manager 增加 Close，使资源由 Component 统一关闭）。
//
// 错误聚合（V4 P0-05）：聚合每个实例 Close 的 error，不再静默忽略。任一实例关闭失败时
// 返回 errors.Join 的聚合错误；全部成功返回 nil。
func (m *RedisMgr) Close() error {
	var errs []error
	m.clients.Range(func(key, value any) bool {
		if err := m.closeInstance(key.(uint32)); err != nil {
			errs = append(errs, fmt.Errorf("redis instance %d close: %w", key.(uint32), err))
		}
		return true
	})
	return errors.Join(errs...)
}

// closeInstance 关闭并移除指定实例。已不存在时安全返回。返回 client.Close 的 error
// （V4 P0-05）。
func (m *RedisMgr) closeInstance(instID uint32) error {
	v, ok := m.clients.LoadAndDelete(instID)
	if !ok {
		return nil
	}
	if client, ok := v.(radix.Client); ok {
		return client.Close()
	}
	return nil
}

/**
* @Description: AddInstance
* @receiver: m
* @param: instID
* @param: ip
* @param: port
* @param: password
* @param: dbIndex
* @param: isCluster
* @return: error
* @Author: Iori
* @Date: 2022-02-26 11:42:37
**/
func (m *RedisMgr) AddInstance(instID uint32, ip string, port int, password string, dbIndex int, isCluster bool) error {

	addr := fmt.Sprintf("%v:%v", ip, port)
	connFunc := func(network, addr string) (radix.Conn, error) {
		if isCluster {
			return radix.Dial(network, addr,
				radix.DialTimeout(10*time.Second),
				radix.DialAuthPass(password),
			)
		} else {
			return radix.Dial(network, addr,
				radix.DialTimeout(10*time.Second),
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

/**
* @Description: GetClient
* @receiver: m
* @param: instID
* @return: radix.Client
* @Author: Iori
* @Date: 2022-02-26 11:42:31
**/
func (m *RedisMgr) GetClient(instID uint32) radix.Client {
	v, exist := m.clients.Load(instID)
	client, ok := v.(radix.Client)
	if !exist || !ok || client == nil {
		logger.Errorf("failed to get a redis client")
		return nil
	}
	return client
}

func (m *RedisMgr) InstanceCount() int {
	if m == nil {
		return 0
	}
	count := 0
	m.clients.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}

/**
* @Description: DoFlatCmd
* @receiver: m
* @param: instID
* @param: result
* @param: cmd
* @param: key
* @param: args
* @return: error
* @Author: Iori
* @Date: 2022-02-26 11:42:27
**/
func (m *RedisMgr) DoFlatCmd(instID uint32, result interface{}, cmd, key string, args ...interface{}) error {
	client := m.GetClient(instID)
	if client == nil {
		return fmt.Errorf("RedisDoCmd cannot get a client {id:%v, cmd:%s}", instID, cmd)
	}
	finish := beginRedisObserve(instID, cmd)
	err := client.Do(radix.FlatCmd(result, cmd, key, args...))
	finish(err)
	return err
}

/**
* @Description: DoCmd
* @receiver: m
* @param: instID
* @param: result
* @param: cmd
* @param: args
* @return: error
* @Author: Iori
* @Date: 2022-02-26 11:42:22
**/
func (m *RedisMgr) DoCmd(instID uint32, result interface{}, cmd string, args ...string) error {
	client := m.GetClient(instID)
	if client == nil {
		return fmt.Errorf("RedisDoCmd cannot get a client {id:%v, cmd:%s}", instID, cmd)
	}
	finish := beginRedisObserve(instID, cmd)
	err := client.Do(radix.Cmd(result, cmd, args...))
	finish(err)
	return err
}

/**
* @Description: SetBytes
* @receiver: m
* @param: instID
* @param: key
* @param: value
* @return: error
* @Author: Iori
* @Date: 2022-02-26 11:42:18
**/
func (m *RedisMgr) SetBytes(instID uint32, key string, value []byte) error {
	return m.DoFlatCmd(instID, nil, "SET", key, value)
}
func (m *RedisMgr) SetBytesEx(instID uint32, key string, value []byte, second int64) error {
	return m.DoFlatCmd(instID, nil, "SET", key, value, "ex", second)
}

/**
* @Description: GetBytes
* @receiver: m
* @param: instID
* @param: key
* @return: []byte
* @return: error
* @Author: Iori
* @Date: 2022-02-26 11:42:14
**/
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

/**
* @Description: MGetBytes
* @receiver: m
* @param: instID
* @param: keys
* @return: []string
* @return: error
* @Author: Iori
* @Date: 2022-02-26 11:42:10
**/
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

/**
* @Description: DelKey
* @receiver: m
* @param: instID
* @param: key
* @return: error
* @Author: Iori
* @Date: 2022-02-26 11:42:06
**/
func (m *RedisMgr) DelKey(instID uint32, key string) error {
	return m.DoFlatCmd(instID, nil, "DEL", key)
}

/**
* @Description: ZsetSet
* @receiver: m
* @param: instID
* @param: setName
* @param: key
* @param: score
* @return: error
* @Author: Iori
* @Date: 2022-02-26 11:42:01
**/
func (m *RedisMgr) ZsetSet(instID uint32, setName string, key string, score int32) error {
	return m.DoFlatCmd(instID, nil, "ZADD", setName, key, score)
}

/**
* @Description: ZsetRange
* @receiver: m
* @param: instID
* @param: setName
* @param: beginIdx
* @param: endIdx
* @return: []string
* @return: error
* @Author: Iori
* @Date: 2022-02-26 11:41:55
**/
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

/**
* @Description: INCR（自增）
* @receiver: m
* @param: instID
* @param: key
* @return: int64
* @return: error
* @Author: Iori
* @Date: 2022-02-26 11:41:35
**/
func (m *RedisMgr) IncrKey(instID uint32, key string) (int64, error) {
	result := int64(0)
	err := m.DoCmd(instID, &result, "INCR", key)
	return result, err
}

/**
* @Description: INCRBY（自增自定义数）
* @receiver: m
* @param: instID
* @param: key
* @return: int64
* @return: error
* @Author: Iori
* @Date: 2022-02-26 11:41:35
**/
func (m *RedisMgr) IncrByKey(instID uint32, key string, value int64) (int64, error) {
	result := int64(0)
	err := m.DoFlatCmd(instID, &result, "INCRBY", key, value)
	return result, err
}
