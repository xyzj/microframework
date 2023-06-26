package wmfw

import (
	"fmt"
	"strconv"
	"strings"
	"time"

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
	if !fw.mqttCtl.enable || len(bindkeys) == 0 {
		return
	}
	var csub = false
	opt := mqtt.NewClientOptions()
	opt.AddBroker("tcp://" + fw.mqttCtl.addr)
	opt.SetClientID(fw.serverName + "_" + gopsu.GetRandomString(10, true))
	opt.SetUsername(fw.mqttCtl.username)
	opt.SetPassword(fw.mqttCtl.password)
	opt.SetWriteTimeout(time.Second * 3) // 发送3秒超时
	opt.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		fw.WriteError("MQTT", "connection lost, "+err.Error())
		csub = false
	})
	var mapkeys = make(map[string]byte)
	for _, key := range bindkeys {
		mapkeys[key] = 0
	}
	fw.mqttCtl.client = mqtt.NewClient(opt)
	loopfunc.LoopFunc(func(params ...interface{}) {
		if token := fw.mqttCtl.client.Connect(); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
		fw.mqttCtl.client.SubscribeMultiple(mapkeys, func(client mqtt.Client, msg mqtt.Message) {
			defer func() {
				if err := recover(); err != nil {
					fw.WriteError("MQTT", fmt.Sprintf("%+v", errors.WithStack(err.(error))))
				}
			}()
			fRecv(msg.Topic(), msg.Payload())
		})
		t := time.NewTicker(time.Second * 15)
		for {
			if !csub && fw.mqttCtl.client.IsConnectionOpen() {
				fw.mqttCtl.client.SubscribeMultiple(mapkeys, func(client mqtt.Client, msg mqtt.Message) {
					defer func() {
						if err := recover(); err != nil {
							fw.WriteError("MQTT", fmt.Sprintf("%+v", errors.WithStack(err.(error))))
						}
					}()
					fRecv(msg.Topic(), msg.Payload())
				})
				csub = true
			}
			select {
			case <-t.C:
			}
		}
	}, "mqtt", fw.LogDefaultWriter())
}

// WriteMQTT 发送mqtt消息
func (fw *WMFrameWorkV2) WriteMQTT(key string, msg []byte, appendhead bool) {
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
