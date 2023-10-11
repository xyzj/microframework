package wmfw

import (
	"strings"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/xyzj/gopsu"
	"github.com/xyzj/gopsu/config"
	"github.com/xyzj/gopsu/mq"
)

// mqtt配置
type mqttConfigure struct {
	// mqtt服务地址, ip:port
	addr string
	// mqtt用户名
	username string
	// mqtt密码
	password string
	// 是否启用mqtt
	enable bool
	// 数据接收方法
	client *mq.MqttClient
}

func (fw *WMFrameWorkV2) newMQTTClient(bindkeys []string, fRecv func(topic string, body []byte)) {
	fw.mqttCtl.addr = fw.baseConf.GetDefault(&config.Item{Key: "mqtt_addr", Value: "127.0.0.1:1883", Comment: "mqtt服务地址,ip:port格式"}).String()
	fw.mqttCtl.username = fw.baseConf.GetDefault(&config.Item{Key: "mqtt_user", Value: "", Comment: "mqtt用户名"}).String()
	fw.mqttCtl.password = fw.baseConf.GetDefault(&config.Item{Key: "mqtt_pwd", Value: "", Comment: "mqtt密码"}).TryDecode()
	fw.mqttCtl.enable = !*disableMqtt // fw.appConf.GetDefault(&config.Item{Key: "mqtt_enable", Value: "false", Comment: "是否启用mqtt"}).TryBool()

	if !fw.mqttCtl.enable {
		return
	}
	var mapkeys = make(map[string]byte)
	for _, v := range bindkeys {
		mapkeys[v] = 0
	}
	opt := &mq.MqttOpt{
		Subscribe: mapkeys,
		ClientID:  fw.serverName,
		Addr:      fw.mqttCtl.addr,
		Username:  fw.mqttCtl.username,
		Passwd:    fw.mqttCtl.password,
	}
	fw.mqttCtl.client = mq.NewMQTTClient(opt,
		fw.wmLog,
		fRecv)
}

// WriteMQTT 发送mqtt消息
func (fw *WMFrameWorkV2) WriteMQTT(key string, value []byte, appendhead bool) {
	if fw.mqttCtl.client == nil {
		return
	}
	if !fw.mqttCtl.client.IsConnectionOpen() {
		return
	}
	if appendhead && !strings.HasPrefix(key, fw.rootPath) {
		key = fw.rootPath + "/" + key
	}
	err := fw.mqttCtl.client.Write(key, value)
	if err != nil {
		fw.WriteError("[MQTT] E:", err.Error())
		return
	}
	fw.WriteInfo("[MQTT] S:", key+" | "+gopsu.String(value))
}

// MQTTIsReady mqtt是否就绪
func (fw *WMFrameWorkV2) MQTTIsReady() bool {
	return fw.mqttCtl.client.IsConnectionOpen()
}

// WriteMQ 发送消息到rabbitmq和mqtt，如果启用的话
func (fw *WMFrameWorkV2) WriteMQ(key string, value []byte, timeout time.Duration, msgproto ...proto.Message) {
	fw.WriteRabbitMQ(key, value, timeout, msgproto...)
	fw.WriteMQTT(key, value, true)
}
