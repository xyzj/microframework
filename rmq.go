package wmfw

import (
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"github.com/xyzj/gopsu"
	"github.com/xyzj/gopsu/config"
	"github.com/xyzj/gopsu/loopfunc"
	"github.com/xyzj/gopsu/mq"
)

// rabbitmq配置
type rabbitConfigure struct {
	forshow string
	// rmq服务地址
	addr string
	// 登录用户名
	user string
	// 登录密码
	pwd string
	// 虚拟域名
	vhost string
	// 交换机名称
	exchange string
	// 队列名称
	queue string
	// 是否使用随机队列名
	queueRandom bool
	// 队列是否持久化
	durable bool
	// 队列是否在未使用时自动删除
	autodel bool
	// 是否启用tls
	usetls bool
	// protocol
	protocol string
	// 是否启用rmq
	enable bool
	// mqProducer 生产者
	mqProducer *mq.Session
	// mqConsumer 消费者
	mqConsumer *mq.Session
	// gpsConsumer 消费者
	gpsConsumer *mq.Session
	sender      *mq.RMQProducer
	cEnable     *bool
}

func (conf *rabbitConfigure) show(rootPath string) string {
	conf.forshow, _ = sjson.Set("", "addr", conf.addr)
	conf.forshow, _ = sjson.Set(conf.forshow, "user", CWorker.Encrypt(conf.user))
	conf.forshow, _ = sjson.Set(conf.forshow, "pwd", CWorker.Encrypt(conf.pwd))
	conf.forshow, _ = sjson.Set(conf.forshow, "vhost", conf.vhost)
	conf.forshow, _ = sjson.Set(conf.forshow, "exchange", conf.exchange)
	conf.forshow, _ = sjson.Set(conf.forshow, "protocol", conf.protocol)
	conf.forshow, _ = sjson.Set(conf.forshow, "use_tls", conf.usetls)
	conf.forshow, _ = sjson.Set(conf.forshow, "root_path", rootPath)
	return conf.forshow
}

func (fw *WMFrameWorkV2) loadMQConfigProducer() {
	fw.rmqCtl.addr = fw.baseConf.GetDefault(&config.Item{Key: "mq_addr", Value: "127.0.0.1:5671", Comment: "mq服务地址,ip:port格式"}).String()
	fw.rmqCtl.user = fw.baseConf.GetDefault(&config.Item{Key: "mq_user", Value: "arx7", Comment: "mq连接用户名"}).String()
	fw.rmqCtl.pwd = fw.baseConf.GetDefault(&config.Item{Key: "mq_pwd", Value: "WcELCNqP5dCpvMmMbKDdvgb", Comment: "mq连接密码"}).TryDecode()
	fw.rmqCtl.vhost = fw.baseConf.GetDefault(&config.Item{Key: "mq_vhost", Value: "", Comment: "mq虚拟域名"}).String()
	fw.rmqCtl.exchange = fw.baseConf.GetDefault(&config.Item{Key: "mq_exchange", Value: "luwak_topic", Comment: "mq交换机名称"}).String()
	fw.rmqCtl.enable = !*disableRMQ // fw.baseConf.GetDefault(&config.Item{Key: "mq_enable", Value: "true", Comment: "是否启用rabbitmq"}).TryBool()
	fw.rmqCtl.usetls = fw.baseConf.GetDefault(&config.Item{Key: "mq_tls", Value: "true", Comment: "是否使用证书连接rabbitmq服务"}).TryBool()
	fw.rmqCtl.protocol = "amqps"
	if !fw.rmqCtl.usetls {
		fw.rmqCtl.addr = strings.Replace(fw.rmqCtl.addr, "5671", "5672", 1)
		fw.rmqCtl.protocol = "amqp"
	}

	fw.rmqCtl.show(fw.rootPath)
}
func (fw *WMFrameWorkV2) loadMQConfig(queueAutoDel bool) {
	fw.rmqCtl.addr = fw.baseConf.GetDefault(&config.Item{Key: "mq_addr", Value: "127.0.0.1:5671", Comment: "mq服务地址,ip:port格式"}).String()
	fw.rmqCtl.user = fw.baseConf.GetDefault(&config.Item{Key: "mq_user", Value: "arx7", Comment: "mq连接用户名"}).String()
	fw.rmqCtl.pwd = fw.baseConf.GetDefault(&config.Item{Key: "mq_pwd", Value: "WcELCNqP5dCpvMmMbKDdvgb", Comment: "mq连接密码"}).TryDecode()
	fw.rmqCtl.vhost = fw.baseConf.GetDefault(&config.Item{Key: "mq_vhost", Value: "", Comment: "mq虚拟域名"}).String()
	fw.rmqCtl.exchange = fw.baseConf.GetDefault(&config.Item{Key: "mq_exchange", Value: "luwak_topic", Comment: "mq交换机名称"}).String()
	fw.rmqCtl.queueRandom = fw.baseConf.GetDefault(&config.Item{Key: "mq_queue_random", Value: "true", Comment: "随机队列名，true-用于独占模式（此时 mq_autodel=true），false-负载均衡"}).TryBool()
	fw.rmqCtl.durable = false
	fw.rmqCtl.autodel = true
	fw.rmqCtl.enable = !*disableRMQ //fw.baseConf.GetDefault(&config.Item{Key: "mq_enable", Value: "true", Comment: "是否启用rabbitmq"}).TryBool()
	fw.rmqCtl.usetls = fw.baseConf.GetDefault(&config.Item{Key: "mq_tls", Value: "true", Comment: "是否使用证书连接rabbitmq服务"}).TryBool()
	fw.rmqCtl.protocol = "amqps"
	if !fw.rmqCtl.usetls {
		fw.rmqCtl.addr = strings.Replace(fw.rmqCtl.addr, "5671", "5672", 1)
		fw.rmqCtl.protocol = "amqp"
	}
	if fw.rmqCtl.queueRandom {
		fw.rmqCtl.durable = false
		fw.rmqCtl.autodel = true
	}

	fw.rmqCtl.show(fw.rootPath)
}

// newMQProducer NewRabbitfw.rmqCtl.mqProducer
func (fw *WMFrameWorkV2) newMQProducer() bool {
	fw.loadMQConfig(true)
	if !fw.rmqCtl.enable {
		return false
	}
	opt := &mq.RabbitMQOpt{
		ExchangeName:       fw.rmqCtl.exchange,
		ExchangeDurable:    true,
		ExchangeAutoDelete: true,
		Addr:               fw.rmqCtl.addr,
		Username:           fw.rmqCtl.user,
		Passwd:             fw.rmqCtl.pwd,
		VHost:              fw.rmqCtl.vhost,
	}
	if fw.rmqCtl.usetls {
		opt.TLSConf = &tls.Config{InsecureSkipVerify: true}
	}
	fw.rmqCtl.sender = mq.NewRMQProducer(opt, fw.wmLog)
	// fw.rmqCtl.mqProducer = mq.NewProducer(fw.rmqCtl.exchange, fmt.Sprintf("%s://%s:%s@%s/%s", fw.rmqCtl.protocol, fw.rmqCtl.user, fw.rmqCtl.pwd, fw.rmqCtl.addr, fw.rmqCtl.vhost), false)
	// fw.rmqCtl.mqProducer.SetLogger(&StdLogger{
	// 	Name:        "MQP",
	// 	LogReplacer: strings.NewReplacer("[", "", "]", ""),
	// 	LogWriter:   fw.coreWriter,
	// })
	// if fw.rmqCtl.usetls {
	// 	return fw.rmqCtl.mqProducer.StartTLS(&tls.Config{InsecureSkipVerify: true})
	// }
	// return fw.rmqCtl.mqProducer.Start()
	return true
}

// Newfw.rmqCtl.mqConsumer Newfw.rmqCtl.mqConsumer
func (fw *WMFrameWorkV2) newMQConsumerV2(queueAutoDel bool, bindkeys []string, recv func(key string, value []byte)) {
	fw.loadMQConfig(queueAutoDel)
	// 若不启用mq功能，则退出
	if !fw.rmqCtl.enable {
		return
	}

	fw.rmqCtl.queue = fw.rootPath + "_" + fw.serverName
	if fw.rmqCtl.queueRandom {
		fw.rmqCtl.queue += "_" + gopsu.GetRandomString(19, true)
		fw.rmqCtl.durable = false
		fw.rmqCtl.autodel = true
	}
	opt := &mq.RabbitMQOpt{
		QueueAutoDelete:    true,
		QueueDurable:       false,
		ExchangeAutoDelete: true,
		ExchangeDurable:    true,
		ExchangeName:       fw.rmqCtl.exchange,
		QueueName:          fw.rmqCtl.queue,
		VHost:              fw.rmqCtl.vhost,
		Username:           fw.rmqCtl.user,
		Passwd:             fw.rmqCtl.pwd,
		Addr:               fw.rmqCtl.addr,
		Subscribe:          bindkeys,
	}
	if fw.rmqCtl.usetls {
		opt.TLSConf = &tls.Config{InsecureSkipVerify: true}
	}
	fw.rmqCtl.cEnable = mq.NewRMQConsumer(opt, fw.wmLog, recv)
}

// Newfw.rmqCtl.mqConsumer Newfw.rmqCtl.mqConsumer
func (fw *WMFrameWorkV2) newMQConsumer(queueAutoDel bool) bool {
	fw.loadMQConfig(queueAutoDel)
	// 若不启用mq功能，则退出
	if !fw.rmqCtl.enable {
		return false
	}

	fw.rmqCtl.mqConsumer = mq.NewConsumer(fw.rmqCtl.exchange,
		fmt.Sprintf("%s://%s:%s@%s/%s", fw.rmqCtl.protocol, fw.rmqCtl.user, fw.rmqCtl.pwd, fw.rmqCtl.addr, fw.rmqCtl.vhost),
		fw.rmqCtl.queue,
		fw.rmqCtl.durable,
		fw.rmqCtl.autodel,
		false)
	fw.rmqCtl.mqConsumer.SetLogger(fw.wmLog)
	if fw.rmqCtl.usetls {
		return fw.rmqCtl.mqConsumer.StartTLS(&tls.Config{InsecureSkipVerify: true})
	}
	return fw.rmqCtl.mqConsumer.Start()
}

// RecvRabbitMQ 接收消息
// f: 消息处理方法，key为消息过滤器，body为消息体
func (fw *WMFrameWorkV2) recvRabbitMQ(f func(key string, body []byte), msgproto ...proto.Message) {
	loopfunc.LoopWithWait(func(params ...interface{}) {
		rcvMQ, err := fw.rmqCtl.mqConsumer.Recv()
		if err != nil {
			panic(err)
		}
		for d := range rcvMQ {
			if d.ContentType == "" && d.DeliveryTag == 0 { // 接收错误，可能服务断开
				panic(errors.New("rcv err: possible service error"))
			}
			if fw.Debug() {
				if gjson.ValidBytes(d.Body) {
					fw.WriteDebug("MQC", "DR:"+fw.rmqCtl.addr+"|"+d.RoutingKey+"|"+gopsu.String(d.Body))
				} else {
					if msgproto == nil {
						fw.WriteDebug("MQC", "DR:"+fw.rmqCtl.addr+"|"+d.RoutingKey+"|"+base64.StdEncoding.EncodeToString(d.Body))
					} else {
						fw.WriteDebug("MQC", "DR:"+fw.rmqCtl.addr+"|"+d.RoutingKey+"|"+msgFromBytes(d.Body, msgproto[0]))
					}
				}
			}
			f(d.RoutingKey, d.Body)
		}
	}, "MQC", fw.LogDefaultWriter(), time.Second*20)
	// RECV:
	//
	//	func() {
	//		defer func() {
	//			if err := recover(); err != nil {
	//				fw.WriteError("MQC", "Consumer core crash: "+errors.WithStack(err.(error)).Error())
	//			}
	//		}()
	//		rcvMQ, err := fw.rmqCtl.mqConsumer.Recv()
	//		if err != nil {
	//			fw.WriteError("MQC", "RErr: "+err.Error())
	//			return
	//		}
	//		for d := range rcvMQ {
	//			if fw.Debug() {
	//				if gjson.ValidBytes(d.Body) {
	//					fw.WriteDebug("MQC", "DR:"+fw.rmqCtl.addr+"|"+d.RoutingKey+"|"+gopsu.String(d.Body))
	//				} else {
	//					if msgproto == nil {
	//						fw.WriteDebug("MQC", "DR:"+fw.rmqCtl.addr+"|"+d.RoutingKey+"|"+base64.StdEncoding.EncodeToString(d.Body))
	//					} else {
	//						fw.WriteDebug("MQC", "DR:"+fw.rmqCtl.addr+"|"+d.RoutingKey+"|"+msgFromBytes(d.Body, msgproto[0]))
	//					}
	//				}
	//			}
	//			f(d.RoutingKey, d.Body)
	//		}
	//	}()
	//	time.Sleep(time.Second * 15)
	//	goto RECV
}

// ProducerIsReady 返回ProducerIsReady可用状态
func (fw *WMFrameWorkV2) ProducerIsReady() bool {
	return fw.rmqCtl.sender.Enable()
	// if fw.rmqCtl.mqProducer != nil {
	// 	return fw.rmqCtl.mqProducer.IsReady()
	// }
	// return false
}

// ConsumerIsReady 返回ProducerIsReady可用状态
func (fw *WMFrameWorkV2) ConsumerIsReady() bool {
	return *fw.rmqCtl.cEnable
	// if fw.rmqCtl.mqConsumer != nil {
	// 	return fw.rmqCtl.mqConsumer.IsReady()
	// }
	// return false
}

// AppendRootPathRabbit 向rabbitmq的key追加头
func (fw *WMFrameWorkV2) AppendRootPathRabbit(key string) string {
	if !strings.HasPrefix(key, fw.rootPathMQ) {
		return fw.rootPathMQ + key
	}
	return key
}

// BindRabbitMQ 绑定消费者key
func (fw *WMFrameWorkV2) BindRabbitMQ(keys ...string) {
	kk := make([]string, 0)
	for _, v := range keys {
		if strings.TrimSpace(v) == "" {
			continue
		}
		kk = append(kk, fw.AppendRootPathRabbit(v))
	}
	if err := fw.rmqCtl.mqConsumer.BindKey(kk...); err != nil {
		fw.WriteError("MQC", err.Error())
	}
}

// UnBindRabbitMQ 解除绑定消费者key
func (fw *WMFrameWorkV2) UnBindRabbitMQ(keys ...string) {
	kk := make([]string, 0)
	for _, v := range keys {
		if strings.TrimSpace(v) == "" {
			continue
		}
		kk = append(kk, fw.AppendRootPathRabbit(v))
	}
	if err := fw.rmqCtl.mqConsumer.UnBindKey(kk...); err != nil {
		fw.WriteError("MQC", err.Error())
	}
}

// WriteRabbitMQ 写mq
func (fw *WMFrameWorkV2) WriteRabbitMQ(key string, value []byte, expire time.Duration, msgproto ...proto.Message) error {
	if fw.rmqCtl.sender == nil {
		return fmt.Errorf("sender not ready")
	}
	if !fw.rmqCtl.sender.Enable() {
		return fmt.Errorf("sender connection lost")
	}
	key = fw.AppendRootPathRabbit(key)
	fw.rmqCtl.sender.Send(key, value, expire)
	// if !fw.ProducerIsReady() {
	// 	return fmt.Errorf("mq producer is not ready")
	// }
	// key = fw.AppendRootPathRabbit(key)
	// err := fw.rmqCtl.mqProducer.SendCustom(&mq.RabbitMQData{
	// 	RoutingKey: key,
	// 	Data: &amqp.Publishing{
	// 		ContentType:  "text/plain",
	// 		DeliveryMode: amqp.Persistent,
	// 		Expiration:   strconv.Itoa(int(expire.Milliseconds())),
	// 		Timestamp:    time.Now(),
	// 		Body:         value,
	// 	},
	// })
	// if err != nil {
	// 	fw.WriteError("MQP", "SErr:"+key+"|"+err.Error())
	// 	return err
	// }
	if len(msgproto) > 0 {
		// fw.WriteInfo("MQP", "S:"+key+"|"+gopsu.PB2String(msgFromBytes(value, msgproto[0])))
		fw.WriteInfo("RMQ", "S:"+key+"|"+msgFromBytes(value, msgproto[0]))
	} else {
		fw.WriteInfo("RMQ", "S:"+key+"|"+string(value))
	}
	return nil
}

// PubEvent 事件id，状态，过滤器，用户名，详细，来源ip，额外数据
func (fw *WMFrameWorkV2) PubEvent(eventid, status int, key, username, detail, from, jsdata string) {
	js, _ := sjson.Set("", "time", time.Now().Unix())
	js, _ = sjson.Set(js, "event_id", eventid)
	js, _ = sjson.Set(js, "user_name", username)
	js, _ = sjson.Set(js, "detail", detail)
	js, _ = sjson.Set(js, "from", from)
	js, _ = sjson.Set(js, "status", status)
	gjson.Parse(jsdata).ForEach(func(key, value gjson.Result) bool {
		if key.String() == "data" {
			js, _ = sjson.Set(js, "data", value.String())
			return true
		}
		if value.IsObject() {
			value.ForEach(func(key1 gjson.Result, value1 gjson.Result) bool {
				if value1.Int() > 999999 {
					js, _ = sjson.Set(js, key.String()+"."+key1.String(), value1.String())
				} else {
					js, _ = sjson.Set(js, key.String()+"."+key1.String(), value1.Value())
				}
				return true
			})
			return true
		}
		js, _ = sjson.Set(js, key.String(), value.Value())
		return true
	})
	fw.WriteRabbitMQ(key, gopsu.Bytes(js), time.Minute*10)
}

// ClearQueue 清空队列
func (fw *WMFrameWorkV2) ClearQueue() {
	if !fw.ConsumerIsReady() {
		return
	}
	fw.rmqCtl.mqConsumer.ClearQueue()
}

// ViewRabbitMQConfig 查看rabbitmq配置,返回json字符串
func (fw *WMFrameWorkV2) ViewRabbitMQConfig() string {
	return fw.rmqCtl.forshow
}
func msgFromBytes(b []byte, pb proto.Message) string {
	err := proto.Unmarshal(b, pb)
	if err != nil {
		return ""
	}
	return gopsu.PB2String(pb) // pb.String()
}

func (fw *WMFrameWorkV2) PBBytes2String(b []byte, pb proto.Message) string {
	return msgFromBytes(b, pb)
}
