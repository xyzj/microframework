package wmfw

import (
	"crypto/tls"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/streadway/amqp"
	"github.com/xyzj/gopsu"
	"github.com/xyzj/microframework/mq"
)

func (fw *WMFrameWorkV2) loadMQConfig2nd() {
	fw.rmqCtl2nd.addr = fw.wmConf.GetItemDefault("mq_2nd_addr", "127.0.0.1:5672", "mq服务地址,ip:port格式")
	fw.rmqCtl2nd.user = fw.wmConf.GetItemDefault("mq_2nd_user", "arx7", "mq连接用户名")
	fw.rmqCtl2nd.pwd = gopsu.DecodeString(fw.wmConf.GetItemDefault("mq_2nd_pwd", "WcELCNqP5dCpvMmMbKDdvgb", "mq连接密码"))
	fw.rmqCtl2nd.vhost = fw.wmConf.GetItemDefault("mq_2nd_vhost", "", "mq虚拟域名")
	fw.rmqCtl2nd.exchange = fw.wmConf.GetItemDefault("mq_2nd_exchange", "luwak_topic", "mq交换机名称")
	fw.rmqCtl2nd.queueRandom = true //, _ = strconv.ParseBool(fw.wmConf.GetItemDefault("mq_queue_random", "true", "随机队列名，true-用于独占模式（此时 mq_durable=false,mq_autodel=true），false-负载均衡"))
	fw.rmqCtl2nd.durable = false    //, _ = strconv.ParseBool(fw.wmConf.GetItemDefault("mq_durable", "false", "队列是否持久化"))
	fw.rmqCtl2nd.autodel = true     // , _ = strconv.ParseBool(fw.wmConf.GetItemDefault("mq_autodel", "true", "队列在未使用时是否删除"))
	fw.rmqCtl2nd.enable, _ = strconv.ParseBool(fw.wmConf.GetItemDefault("mq_2nd_enable", "false", "第二个mq生产者，对接用"))
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
	fw.wmConf.Save()
	fw.rmqCtl2nd.show(fw.rootPath)
}

// newMQProducer2nd NewRabbitfw.rmqCtl2nd.mqProducer
func (fw *WMFrameWorkV2) newMQProducer2nd() bool {
	fw.loadMQConfig2nd()
	if !fw.rmqCtl2nd.enable {
		return false
	}
	fw.rmqCtl2nd.mqProducer = mq.NewProducer(fw.rmqCtl2nd.exchange, fmt.Sprintf("%s://%s:%s@%s/%s", fw.rmqCtl2nd.protocol, fw.rmqCtl2nd.user, fw.rmqCtl2nd.pwd, fw.rmqCtl2nd.addr, fw.rmqCtl2nd.vhost), false)
	fw.rmqCtl2nd.mqProducer.SetLogger(&StdLogger{
		Name:        "MQP2nd",
		LogReplacer: strings.NewReplacer("[", "", "]", ""),
		LogWriter:   fw.coreWriter,
	})
	if fw.rmqCtl2nd.usetls {
		return fw.rmqCtl2nd.mqProducer.StartTLS(&tls.Config{InsecureSkipVerify: true})
	}
	return fw.rmqCtl2nd.mqProducer.Start()
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
	if !fw.ProducerIsReady() {
		return fmt.Errorf("mq 2nd producer is not ready")
	}
	err := fw.rmqCtl2nd.mqProducer.SendCustom(&mq.RabbitMQData{
		RoutingKey: key,
		Data: &amqp.Publishing{
			ContentType:  "text/plain",
			DeliveryMode: amqp.Persistent,
			Expiration:   strconv.Itoa(int(expire.Nanoseconds() / 1000000)),
			Timestamp:    time.Now(),
			Body:         value,
		},
	})
	if err != nil {
		fw.WriteError("MQP2nd", "SErr:"+key+"|"+err.Error())
		return err
	}
	fw.WriteInfo("MQP2nd", "S:"+key+"|"+gopsu.String(value))
	return nil
}

// ViewRabbitMQConfig2nd 查看rabbitmq配置,返回json字符串
func (fw *WMFrameWorkV2) ViewRabbitMQConfig2nd() string {
	return fw.rmqCtl2nd.forshow
}
