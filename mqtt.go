package wmfw

import (
	"strconv"
	"strings"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/tidwall/sjson"
	"github.com/xyzj/gopsu"
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
		fRecv,
		fw.wmLog)
}

// WriteMQTT 发送mqtt消息
func (fw *WMFrameWorkV2) WriteMQTT(key string, msg []byte, appendhead bool) {
	if fw.mqttCtl.client == nil {
		return
	}
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

// MQTTIsReady mqtt是否就绪
func (fw *WMFrameWorkV2) MQTTIsReady() bool {
	if fw.mqttCtl.client == nil {
		return false
	}
	return fw.mqttCtl.client.IsConnectionOpen()
}
