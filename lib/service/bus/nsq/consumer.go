package nsq

import (
	"sync"
	"time"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/nsqio/go-nsq"
)

// cb  handler
type MsgHandler func(srcBusID uint32, data []byte) error

/*
*  nsqHandler
*  @Description: handler
 */
type ConsumerHandler struct {
	nsqConsumer *nsq.Consumer

	messagesReceived int
	onRecv           MsgHandler
	rwLocker         sync.RWMutex
}

/**
* @Description: 处理消息函数
* @receiver: nh
* @param: msg
* @return: error
* @Author: Iori
* @Date: 2022-04-22 12:02:04
**/
func (ch *ConsumerHandler) HandleMessage(msg *nsq.Message) error {
	if logger.DebugEnabled() {
		logger.Debugf("nsq receive ID:%s, addr:%s, bodyLen:%d", msg.ID, msg.NSQDAddress, len(msg.Body))
	}
	if ch.onRecv != nil {
		ch.onRecv(0, msg.Body)
	}

	return nil
}

/**
* @Description:  创建消费者实例
* @param: topic
* @param: channel
* @param: lookupAddr
* @param: addr
* @param: cb
* @return: *nsq.Consumer
* @return: error
* @Author: Iori
* @Date: 2022-04-25 14:24:32
**/
func NewConsumer(topic, channel, addr string, lookupAddr []string, concurrency int, cb MsgHandler) (*nsq.Consumer, error) {
	return NewConsumerWithOpts(topic, channel, addr, lookupAddr, concurrency, 0, 0, cb)
}

// NewConsumerWithOpts 创建消费者实例，支持配置 MaxInFlight 和 LookupdPollInterval。
// maxInFlight<=0 时用默认值 2；lookupdPollInterval<=0 时用默认值 3s。
func NewConsumerWithOpts(topic, channel, addr string, lookupAddr []string, concurrency, maxInFlight int, lookupdPollInterval time.Duration, cb MsgHandler) (*nsq.Consumer, error) {
	cfg := nsq.NewConfig()
	cfg.LookupdPollInterval = 3 * time.Second
	if lookupdPollInterval > 0 {
		cfg.LookupdPollInterval = lookupdPollInterval
	}
	cfg.MaxInFlight = 2
	if maxInFlight > 0 {
		cfg.MaxInFlight = maxInFlight
	}

	c, err := nsq.NewConsumer(topic, channel, cfg)
	if err != nil {
		logger.Errorf("init Consumer NewConsumer error: %v", err)
		return nil, err
	}

	//add handler
	handler := &ConsumerHandler{nsqConsumer: c}
	if concurrency <= 0 || concurrency > 100 {
		concurrency = 1
	}
	c.AddConcurrentHandlers(handler, concurrency)
	if cb != nil {
		handler.onRecv = cb
	}

	//conn
	if len(lookupAddr) > 0 {
		err = c.ConnectToNSQLookupds(lookupAddr)
	} else {
		err = c.ConnectToNSQD(addr)
	}
	if err != nil {
		logger.Errorf("init Consumer ConnectToNSQLookupd error: %v", err)
		return nil, err
	}
	return c, nil
}
