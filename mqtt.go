package wmfw

import (
	"strings"

	"github.com/tidwall/sjson"
	"github.com/xyzj/gopsu"
	config "github.com/xyzj/gopsu/confile"
	"github.com/xyzj/gopsu/mq"
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
	client *mq.MqttClient
}

func (conf *mqttConfigure) show(rootPath string) string {
	conf.forshow, _ = sjson.Set("", "addr", conf.addr)
	conf.forshow, _ = sjson.Set(conf.forshow, "enable", conf.addr)
	conf.forshow, _ = sjson.Set(conf.forshow, "root_path", rootPath)
	return conf.forshow
}

func (fw *WMFrameWorkV2) newMQTTClient(bindkeys []string, fRecv func(topic string, body []byte)) {
	fw.mqttCtl.addr = fw.wmConf.GetDefault(&config.Item{Key: "mqtt_addr", Value: "127.0.0.1:1883", Comment: "mqtt服务地址,ip:port格式"}).String()
	fw.mqttCtl.username = fw.wmConf.GetDefault(&config.Item{Key: "mqtt_user", Value: "", Comment: "mqtt用户名"}).String()
	fw.mqttCtl.password = fw.wmConf.GetDefault(&config.Item{Key: "mqtt_pwd", Value: "", Comment: "mqtt密码"}).TryDecode()
	fw.mqttCtl.enable = fw.wmConf.GetDefault(&config.Item{Key: "mqtt_enable", Value: "false", Comment: "是否启用mqtt"}).TryBool()
	fw.wmConf.ToFile()
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
func (fw *WMFrameWorkV2) WriteMQTT(key string, msg []byte, appendhead bool) {
	if !fw.mqttCtl.client.IsConnectionOpen() {
		return
	}
	if appendhead && !strings.HasPrefix(key, fw.rootPath) {
		key = fw.rootPath + "/" + key
	}
	err := fw.mqttCtl.client.Write(key, msg)
	if err != nil {
		fw.WriteError("[MQTT] E:", err.Error())
		return
	}
	fw.WriteInfo("[MQTT] S:", key+" | "+gopsu.String(msg))
}

// MQTTIsReady mqtt是否就绪
func (fw *WMFrameWorkV2) MQTTIsReady() bool {
	return fw.mqttCtl.client.IsConnectionOpen()
}
