package Topics

import (
	"GinSkeleton/App/Utils/Config"
	"github.com/streadway/amqp"
	"time"
)

func CreateConsumer() (*consumer, error) {
	// 获取配置信息
	configFac := Config.CreateYamlFactory()
	conn, err := amqp.Dial(configFac.GetString("RabbitMq.Topics.Addr"))
	exchange_type := configFac.GetString("RabbitMq.Topics.ExchangeType")
	exchange_name := configFac.GetString("RabbitMq.Topics.ExchangeName")
	queue_name := configFac.GetString("RabbitMq.Topics.QueueName")
	dura := configFac.GetBool("RabbitMq.Topics.Durable")
	reconnect_interval_sec := configFac.GetDuration("RabbitMq.Topics.OffLineReconnectIntervalSec")
	retry_times := configFac.GetInt("RabbitMq.Topics.RetryCount")

	if err != nil {
		//log.Panic(err.Error())
		return nil, err
	}

	v_consumer := &consumer{
		connect:                     conn,
		exchangeType:                exchange_type,
		exchangeName:                exchange_name,
		queueName:                   queue_name,
		durable:                     dura,
		connErr:                     conn.NotifyClose(make(chan *amqp.Error, 1)),
		offLineReconnectIntervalSec: reconnect_interval_sec,
		retryTimes:                  retry_times,
	}
	return v_consumer, nil
}

//  定义一个消息队列结构体：Topics 模型
type consumer struct {
	connect                     *amqp.Connection
	exchangeType                string
	exchangeName                string
	queueName                   string
	durable                     bool
	occurError                  error // 记录初始化过程中的错误
	connErr                     chan *amqp.Error
	routeKey                    string                     //  断线重新连接刷新回调函数使用
	callbackForReceived         func(received_data string) //  断线重新连接刷新回调函数使用
	offLineReconnectIntervalSec time.Duration
	retryTimes                  int
}

// 接收、处理消息
func (c *consumer) Received(route_key string, callback_fun_deal_smg func(received_data string)) {
	defer func() {
		c.connect.Close()
	}()
	// 将回调函数地址赋值给结构体变量，用于掉线重连使用
	c.routeKey = route_key
	c.callbackForReceived = callback_fun_deal_smg

	blocking := make(chan bool)

	go func(key string) {

		ch, err := c.connect.Channel()
		c.occurError = errorDeal(err)
		defer ch.Close()

		// 声明exchange交换机
		err = ch.ExchangeDeclare(
			c.exchangeName, //exchange name
			c.exchangeType, //exchange kind
			c.durable,      //数据是否持久化
			!c.durable,     //所有连接断开时，交换机是否删除
			false,
			false,
			nil,
		)
		// 声明队列
		v_queue, err := ch.QueueDeclare(
			c.queueName,
			c.durable,
			!c.durable,
			false,
			false,
			nil,
		)
		c.occurError = errorDeal(err)

		//队列绑定
		err = ch.QueueBind(
			v_queue.Name,
			key, //  Topics 模式,生产者会将消息投递至交换机的route_key， 消费者匹配不同的key获取消息、处理
			c.exchangeName,
			false,
			nil,
		)
		c.occurError = errorDeal(err)

		msgs, err := ch.Consume(
			v_queue.Name, // 队列名称
			"",           //  消费者标记，请确保在一个消息频道唯一
			true,         //是否自动响应确认，这里设置为false，手动确认
			false,        //是否私有队列，false标识允许多个 consumer 向该队列投递消息，true 表示独占
			false,        //RabbitMQ不支持noLocal标志。
			false,        // 队列如果已经在服务器声明，设置为 true ，否则设置为 false；
			nil,
		)
		c.occurError = errorDeal(err)

		for msg := range msgs {
			// 通过回调处理消息
			callback_fun_deal_smg(string(msg.Body))
		}

	}(route_key)

	<-blocking

}

//消费者端，掉线重连监听器
func (c *consumer) OffLineReconnectionListener(callback_offline_err func(error_args *amqp.Error)) {

	select {
	case err := <-c.connErr:
		for i := 1; i <= c.retryTimes; i++ {
			// 自动重连机制，需要继续完善
			time.Sleep(c.offLineReconnectIntervalSec * time.Second)
			v_conn, err := CreateConsumer()
			if err != nil {
				continue
			} else {
				v_conn.Received(c.routeKey, c.callbackForReceived)
				break
			}
		}
		callback_offline_err(err)
	}

}
