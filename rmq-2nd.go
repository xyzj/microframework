package wmfw

import (
	"strings"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/xyzj/gopsu"
	"github.com/xyzj/gopsu/config"
	"github.com/xyzj/gopsu/mq"
)

func (fw *WMFrameWorkV2) loadMQConfig2nd() {
	fw.rmqCtl2nd.addr = fw.wmConf.GetDefault(&config.Item{Key: "mq_2nd_addr", Value: "127.0.0.1:5672", Comment: "mq服务地址,ip:port格式"}).String()
	fw.rmqCtl2nd.user = fw.wmConf.GetDefault(&config.Item{Key: "mq_2nd_user", Value: "arx7", Comment: "mq连接用户名"}).String()
	fw.rmqCtl2nd.pwd = fw.wmConf.GetDefault(&config.Item{Key: "mq_2nd_pwd", Value: "WcELCNqP5dCpvMmMbKDdvgb", Comment: "mq连接密码"}).TryDecode()
	fw.rmqCtl2nd.vhost = fw.wmConf.GetDefault(&config.Item{Key: "mq_2nd_vhost", Value: "", Comment: "mq虚拟域名"}).String()
	fw.rmqCtl2nd.exchange = fw.wmConf.GetDefault(&config.Item{Key: "mq_2nd_exchange", Value: "luwak_topic", Comment: "mq交换机名称"}).String()
	fw.rmqCtl2nd.queueRandom = true //, _ = strconv.ParseBool(fw.wmConf.GetDefault(&config.Item{Key:"mq_queue_random", "true", "随机队列名，true-用于独占模式（此时 mq_durable=false,mq_autodel=true），false-负载均衡"))
	fw.rmqCtl2nd.durable = false    //, _ = strconv.ParseBool(fw.wmConf.GetDefault(&config.Item{Key:"mq_durable", "false", "队列是否持久化"))
	fw.rmqCtl2nd.autodel = true     // , _ = strconv.ParseBool(fw.wmConf.GetDefault(&config.Item{Key:"mq_autodel", "true", "队列在未使用时是否删除"))
	fw.rmqCtl2nd.enable = fw.wmConf.GetDefault(&config.Item{Key: "mq_2nd_enable", Value: "false", Comment: "第二个mq生产者，对接用"}).TryBool()
	fw.rmqCtl2nd.usetls = false //, _ = strconv.ParseBool(fw.wmConf.GetItemDefault("mq_tls", "true", "是否使用证书连接rabbitmq服务"))
	fw.rmqCtl2nd.protocol = "amqps"
	if !fw.rmqCtl2nd.usetls {
		fw.rmqCtl2nd.addr = strings.Replace(fw.rmqCtl2nd.addr, "5671", "5672", 1)
		fw.rmqCtl2nd.protocol = "amqp"
	}
	if fw.rmqCtl2nd.queueRandom {
		fw.rmqCtl2nd.durable = false
		fw.rmqCtl2nd.autodel = true
	}
	fw.wmConf.ToFile()
	fw.rmqCtl2nd.show(fw.rootPath)
}

// newMQProducer2nd NewRabbitfw.rmqCtl2nd.mqProducer
func (fw *WMFrameWorkV2) newMQProducer2nd() bool {
	fw.loadMQConfig2nd()
	if !fw.rmqCtl2nd.enable {
		return false
	}
	opt := &mq.RabbitMQOpt{
		ExchangeName:       "luwak_topic",
		ExchangeDurable:    true,
		ExchangeAutoDelete: true,
		Addr:               fw.rmqCtl2nd.addr,
		Username:           fw.rmqCtl2nd.user,
		Passwd:             fw.rmqCtl2nd.pwd,
		VHost:              fw.rmqCtl2nd.vhost,
	}
	fw.rmqCtl2nd.sender = mq.NewRMQProducer(opt, fw.wmLog)
	return true
	// fw.rmqCtl2nd.mqProducer = mq.NewProducer(fw.rmqCtl2nd.exchange, fmt.Sprintf("%s://%s:%s@%s/%s", fw.rmqCtl2nd.protocol, fw.rmqCtl2nd.user, fw.rmqCtl2nd.pwd, fw.rmqCtl2nd.addr, fw.rmqCtl2nd.vhost), false)
	// fw.rmqCtl2nd.mqProducer.SetLogger(&StdLogger{
	// 	Name:        "MQP2nd",
	// 	LogReplacer: strings.NewReplacer("[", "", "]", ""),
	// 	LogWriter:   fw.coreWriter,
	// })
	// if fw.rmqCtl2nd.usetls {
	// 	return fw.rmqCtl2nd.mqProducer.StartTLS(&tls.Config{InsecureSkipVerify: true})
	// }
	// return fw.rmqCtl2nd.mqProducer.Start()
}

// ProducerIsReady2nd 返回ProducerIsReady可用状态
func (fw *WMFrameWorkV2) ProducerIsReady2nd() bool {
	if fw.rmqCtl2nd.mqProducer != nil {
		return fw.rmqCtl2nd.mqProducer.IsReady()
	}
	return false
}

// WriteRabbitMQ2nd 写mq2nd，不会追加头
func (fw *WMFrameWorkV2) WriteRabbitMQ2nd(key string, value []byte, expire time.Duration, msgproto ...proto.Message) error {
	fw.rmqCtl2nd.sender.Send(key, value, time.Second*30)
	// if !fw.ProducerIsReady() {
	// 	return fmt.Errorf("mq 2nd producer is not ready")
	// }
	// err := fw.rmqCtl2nd.mqProducer.SendCustom(&mq.RabbitMQData{
	// 	RoutingKey: key,
	// 	Data: &amqp.Publishing{
	// 		ContentType:  "text/plain",
	// 		DeliveryMode: amqp.Persistent,
	// 		Expiration:   strconv.Itoa(int(expire.Nanoseconds() / 1000000)),
	// 		Timestamp:    time.Now(),
	// 		Body:         value,
	// 	},
	// })
	// if err != nil {
	// 	fw.WriteError("MQP2nd", "SErr:"+key+"|"+err.Error())
	// 	return err
	// }
	fw.WriteInfo("MQP2nd", "S:"+key+"|"+gopsu.String(value))
	return nil
}

// ViewRabbitMQConfig2nd 查看rabbitmq配置,返回json字符串
func (fw *WMFrameWorkV2) ViewRabbitMQConfig2nd() string {
	return fw.rmqCtl2nd.forshow
}
