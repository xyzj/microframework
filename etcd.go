package wmfw

import (
	"fmt"
)

// etcd配置
type etcdConfigure struct {
	// etcd服务地址
	addr string
	// 是否启用tls
	usetls bool
	// 是否启用etcd
	enable bool
	// 优先v6
	v6 bool
	// 对外公布注册地址
	regAddr string
	// enable auth
	useauth bool
	// user
	username string
	// passwd
	password string
	// Client
	// client  *microgo.Etcdv3Client
	ready   bool
	useetcd bool
}

// OptEtcd etcd注册配置
type optEtcd struct {
	Name      string
	Alias     string
	Host      string
	Port      string
	Protocol  string // json or pb2
	Interface string // http or https
}

// EtcdInfo 注册信息体
type EtcdInfo struct {
	TimeConnect int64  `json:"timeConnect"`
	TimeActive  int64  `json:"timeActive"`
	TimeUpdate  int64  `json:"-"`
	IP          string `json:"ip"`
	Port        string `json:"port"`
	Name        string `json:"name"`
	Alias       string `json:"alias"`
	Intfc       string `json:"INTFC"`
	Protocol    string `json:"protocol"`
	Source      string `json:"source"`
	Data        string `json:"data,omitempty"`
	Fulladdr    string `json:"-"`
}

// ETCDIsReady 返回ETCD可用状态
func (fw *WMFrameWorkV2) ETCDIsReady() bool {
	return fw.etcdCtl.ready
}

// Picker 选取服务地址
func (fw *WMFrameWorkV2) Picker(svrName string) (string, error) {
	if !fw.etcdCtl.ready {
		return "", fmt.Errorf("etcd client not ready")
	}
	// if fw.etcdCtl.useetcd {
	// 	return fw.v3Picker(svrName)
	// }
	return fw.redisPicker(svrName)
}

// AllServices 返回所有服务列表
func (fw *WMFrameWorkV2) AllServices() (string, error) {
	if !fw.etcdCtl.ready {
		return "", fmt.Errorf("etcd client not ready")
	}
	// if fw.etcdCtl.useetcd {
	// 	return fw.v3AllServices()
	// }
	return fw.redisAllServices()
}

// PickerDetail 选取服务地址,带http(s)前缀
func (fw *WMFrameWorkV2) PickerDetail(svrName string) (string, error) {
	if !fw.etcdCtl.ready {
		return "", fmt.Errorf("etcd client not ready")
	}
	// if fw.etcdCtl.useetcd {
	// 	return fw.v3PickerDetail(svrName)
	// }
	return fw.redisPickerDetail(svrName)
}
