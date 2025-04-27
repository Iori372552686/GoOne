package nsq

import (
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/nsqio/go-nsq"
)

/*
*  nsqProducer
*  @Description:
 */
type nsqProducer struct {
	*nsq.Producer
}

/**
* @Description: 初始化生产者
* @param: addr
* @return: *nsqProducer
* @return: error
* @Author: Iori
* @Date: 2022-04-22 14:05:53
**/
func NewProducer(addr string) (*nsqProducer, error) {
	logger.Infof(" new and init producer address: %v", addr)
	producer, err := nsq.NewProducer(addr, nsq.NewConfig())
	if err != nil {
		return nil, err
	}
	return &nsqProducer{producer}, nil
}

/**
* @Description: 发布消息
* @receiver: np
* @param: topic
* @param: message
* @return: error
* @Author: Iori
* @Date: 2022-04-22 14:05:45
**/
func (np *nsqProducer) Public(topic, message string) error {
	err := np.Publish(topic, []byte(message))
	if err != nil {
		logger.Errorf("nsq public error | %v", err)
		return err
	}
	return nil
}
