package wmfw

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/xyzj/gopsu/logger"
	"github.com/xyzj/gopsu/loopfunc"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/pkg/errors"
	"github.com/tidwall/sjson"
	"github.com/xyzj/gopsu"
)

// mqtt配置
type mqttConfigure struct {
	forshow string
	// mqtt服务地址, ip:port
	addr string
	// mqtt用户名
	username string
	// mqtt密码
	password string
	// 是否启用mqtt
	enable bool
	// 数据接收方法
	client mqtt.Client
}

func (conf *mqttConfigure) show(rootPath string) string {
	conf.forshow, _ = sjson.Set("", "addr", conf.addr)
	conf.forshow, _ = sjson.Set(conf.forshow, "enable", conf.addr)
	conf.forshow, _ = sjson.Set(conf.forshow, "root_path", rootPath)
	return conf.forshow
}

func (fw *WMFrameWorkV2) newMQTTClient(bindkeys []string, fRecv func(topic string, body []byte)) {
	fw.mqttCtl.addr = fw.wmConf.GetItemDefault("mqtt_addr", "127.0.0.1:1883", "mqtt服务地址,ip:port格式")
	fw.mqttCtl.username = fw.wmConf.GetItemDefault("mqtt_user", "", "mqtt用户名")
	fw.mqttCtl.password = fw.wmConf.GetItemDefault("mqtt_pwd", "", "mqtt密码")
	fw.mqttCtl.enable, _ = strconv.ParseBool(fw.wmConf.GetItemDefault("mqtt_enable", "false", "是否启用mqtt"))
	fw.wmConf.Save()
	if !fw.mqttCtl.enable {
		return
	}
	fw.mqttCtl.client = MoreMQTTClient(fw.mqttCtl.addr,
		fw.mqttCtl.username,
		fw.mqttCtl.password,
		bindkeys,
		fRecv,
		fw.wmLog)
	// var mapkeys = make(map[string]byte)
	// for _, key := range bindkeys {
	// 	mapkeys[key] = 0
	// }
	// var needSub = len(mapkeys) > 0
	// var clientSub = false
	// opt := mqtt.NewClientOptions()
	// opt.AddBroker("tcp://" + fw.mqttCtl.addr)
	// opt.SetClientID(fw.serverName + "_" + gopsu.GetRandomString(10, true))
	// opt.SetUsername(fw.mqttCtl.username)
	// opt.SetPassword(fw.mqttCtl.password)
	// opt.SetWriteTimeout(time.Second * 3) // 发送3秒超时
	// opt.SetConnectionLostHandler(func(client mqtt.Client, err error) {
	// 	fw.WriteError("MQTT", "connection lost, "+err.Error())
	// 	clientSub = false
	// })
	// opt.SetOnConnectHandler(func(client mqtt.Client) {
	// 	fw.WriteSystem("MQTT", "Success connect to "+fw.mqttCtl.addr)
	// })
	// fw.mqttCtl.client = mqtt.NewClient(opt)
	// loopfunc.LoopFunc(func(params ...interface{}) {
	// 	if token := fw.mqttCtl.client.Connect(); token.Wait() && token.Error() != nil {
	// 		panic(token.Error())
	// 	}
	// 	t := time.NewTicker(time.Second * 20)
	// 	for {
	// 		if needSub && !clientSub && fw.mqttCtl.client.IsConnectionOpen() {
	// 			fw.mqttCtl.client.SubscribeMultiple(mapkeys, func(client mqtt.Client, msg mqtt.Message) {
	// 				defer func() {
	// 					if err := recover(); err != nil {
	// 						fw.WriteError("MQTT", fmt.Sprintf("%+v", errors.WithStack(err.(error))))
	// 					}
	// 				}()
	// 				fw.WriteDebug("MQTT-R", "topic: "+msg.Topic()+" | body:"+gopsu.String(msg.Payload()))
	// 				fRecv(msg.Topic(), msg.Payload())
	// 			})
	// 			clientSub = true
	// 		}
	// 		select {
	// 		case <-t.C:
	// 		}
	// 	}
	// }, "mqtt", fw.LogDefaultWriter())
}

// WriteMQTT 发送mqtt消息
func (fw *WMFrameWorkV2) WriteMQTT(key string, msg []byte, appendhead bool) {
	if !fw.mqttCtl.client.IsConnectionOpen() {
		return
	}
	if appendhead && !strings.HasPrefix(key, fw.rootPath) {
		key = fw.rootPath + "/" + key
	}
	token := fw.mqttCtl.client.Publish(key, 0, false, msg)
	if token.Wait() && token.Error() != nil {
		fw.WriteError("MQTT-Err", token.Error().Error())
		return
	}
	fw.WriteInfo("MQTT-S", key+" | "+gopsu.String(msg))
}

// MoreMQTTClient 创建一个mqtt客户端
func MoreMQTTClient(addr, username, passwd string, bindkeys []string, fRecv func(topic string, body []byte), logg logger.Logger) mqtt.Client {
	if fRecv == nil {
		fRecv = func(topic string, body []byte) {}
	}
	if logg == nil {
		logg = &logger.NilLogger{}
	}
	var mapkeys = make(map[string]byte)
	for _, key := range bindkeys {
		mapkeys[key] = 0
	}
	var needSub = len(mapkeys) > 0
	var clientSub = false
	opt := mqtt.NewClientOptions()
	opt.AddBroker("tcp://" + addr)
	opt.SetClientID(gopsu.RealIP(false) + "_" + gopsu.GetRandomString(10, true))
	opt.SetUsername(username)
	opt.SetPassword(passwd)
	opt.SetWriteTimeout(time.Second * 3) // 发送3秒超时
	opt.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		logg.Error("[MQTT] connection lost, " + err.Error())
		clientSub = false
	})
	opt.SetOnConnectHandler(func(client mqtt.Client) {
		logg.System("[MQTT] Success connect to " + addr)
	})
	client := mqtt.NewClient(opt)
	go loopfunc.LoopFunc(func(params ...interface{}) {
		if token := client.Connect(); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
		t := time.NewTicker(time.Second * 20)
		for {
			if needSub && !clientSub && client.IsConnectionOpen() {
				client.SubscribeMultiple(mapkeys, func(client mqtt.Client, msg mqtt.Message) {
					defer func() {
						if err := recover(); err != nil {
							logg.Error("[MQTT] " + fmt.Sprintf("%+v", errors.WithStack(err.(error))))
						}
					}()
					fRecv(msg.Topic(), msg.Payload())
				})
				clientSub = true
			}
			select {
			case <-t.C:
			}
		}
	}, "mqtt", logg.DefaultWriter())
	return client
}
