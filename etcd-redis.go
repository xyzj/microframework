package wmfw

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/xyzj/gopsu"
	"github.com/xyzj/gopsu/json"
	"github.com/xyzj/gopsu/loopfunc"
)

// type mapETCD struct {
// 	locker sync.RWMutex
// 	data   map[string]*EtcdInfo
// }

// func (m *mapETCD) Store(key string, value *EtcdInfo) {
// 	if value.Fulladdr == "" {
// 		value.Fulladdr = fmt.Sprintf("%s://%s:%s", value.Intfc, value.IP, value.Port)
// 	}
// 	m.locker.Lock()
// 	m.data[key] = value
// 	m.locker.Unlock()
// }
// func (m *mapETCD) Update(key string, t int64) {
// 	m.locker.Lock()
// 	m.data[key].TimeUpdate = t
// 	m.locker.Unlock()
// }
// func (m *mapETCD) Delete(key string) {
// 	m.locker.Lock()
// 	delete(m.data, key)
// 	m.locker.Unlock()
// }
// func (m *mapETCD) Has(key string) bool {
// 	m.locker.RLock()
// 	_, ok := m.data[key]
// 	m.locker.RUnlock()
// 	return ok
// }
// func (m *mapETCD) ForEach(f func(key string, value *EtcdInfo) bool) {
// 	m.locker.RLock()
// 	x := deepcopy.Copy(m.data).(map[string]*EtcdInfo)
// 	m.locker.RUnlock()
// 	for k, v := range x {
// 		if !f(k, v) {
// 			break
// 		}
// 	}
// }
// func (m *mapETCD) Load(key string) (*EtcdInfo, bool) {
// 	m.locker.RLock()
// 	v, ok := m.data[key]
// 	m.locker.RUnlock()
// 	if ok {
// 		return v, true
// 	}
// 	return nil, false
// }
// func (m *mapETCD) Clean() {
// 	m.locker.Lock()
// 	m.data = make(map[string]*EtcdInfo)
// 	m.locker.Unlock()
// }

// type etcdInfoRedis struct {
// 	update   int64
// 	EtcdInfo *EtcdInfo
// }

// newRedisETCDClient newRedisETCDClient
func (fw *WMFrameWorkV2) newRedisETCDClient() {
	fw.etcdCtl.regAddr = fw.wmConf.GetItemDefault("etcd_reg", "", "服务注册地址,ip[:port]格式，不指定port时，自动使用http启动参数的端口")
	fw.etcdCtl.enable, _ = strconv.ParseBool(fw.wmConf.GetItemDefault("etcd_enable", "true", "是否启用服务注册"))
	fw.etcdCtl.v6 = false //, _ = strconv.ParseBool(fw.wmConf.GetItemDefault("etcd_v6", "false", "是否优先使用v6地址"))
	xaddr := fw.wmConf.GetItemDefault("etcd_reg_"+strings.ReplaceAll(fw.serverName, "-"+*nameTail, ""), "", "配置该项时，忽略etcd_reg设置。服务注册地址,ip[:port]格式，不指定port时，自动使用http启动参数的端口")
	if xaddr != "" {
		fw.etcdCtl.regAddr = xaddr
	}
	if fw.etcdCtl.regAddr == "127.0.0.1" || fw.etcdCtl.regAddr == "" {
		fw.etcdCtl.regAddr = gopsu.RealIP(fw.etcdCtl.v6)
		// fw.wmConf.UpdateItem("etcd_reg", fw.etcdCtl.regAddr)
	}
	fw.wmConf.Save()
	if !fw.etcdCtl.enable {
		return
	}
	fw.etcdCtl.show(fw.rootPath)
	// 处理信息
	// var httpType = "https"
	// if *debug || fw.httpCert == "" || fw.httpKey == "" {
	// 	httpType = "http"
	// }
	a, b, err := net.SplitHostPort(fw.etcdCtl.regAddr)
	if err != nil {
		a = fw.etcdCtl.regAddr
	}
	if b == "" {
		b = strconv.Itoa(*webPort)
	}
	etcd := &optEtcd{
		Name:      fw.serverName,
		Host:      a,
		Port:      b,
		Interface: strings.ReplaceAll(fw.httpProtocol, "://", ""),
		Protocol:  "json",
		Alias:     fw.serverAlias,
	}
	fw.etcdRedis(etcd)
}
func (fw *WMFrameWorkV2) etcdRedis(etcd *optEtcd) {
	if !fw.newRedisClient() {
		return
	}
	fw.etcdCtl.ready = true
	realip := gopsu.RealIP(fw.etcdCtl.v6)
	key := fmt.Sprintf("/%s/etcd/%s/%s_%s", fw.rootPath, fw.serverName, fw.serverName, gopsu.GetUUID1())
	keypath := fmt.Sprintf("/%s/etcd/*", fw.rootPath)
	fw.etcdCtl.useetcd = false
	ei := &EtcdInfo{
		Name:        etcd.Name,
		Alias:       etcd.Alias,
		IP:          etcd.Host,
		Port:        etcd.Port,
		Intfc:       etcd.Interface,
		Protocol:    etcd.Protocol,
		TimeConnect: time.Now().Unix(),
		Source:      realip,
		Data:        "redis",
	}
	fetcdReg := func() error {
		// if ok := fw.ExpireRedis(key, time.Second*7); ok {
		// 	return nil
		// }
		ei.TimeActive = time.Now().Unix()
		b, _ := json.Marshal(ei)
		return fw.WriteRedis(key, b, time.Second*12)
	}
	fetcdRead := func() {
		t := time.Now().Unix()
		keys := fw.ReadAllRedisKeys(keypath)
		for _, v := range keys {
			b, err := fw.ReadRedis(v)
			if err != nil {
				// fw.mapEtcd.delete(v)
				continue
			}
			if d, ok := fw.mapEtcd.Load(v); ok {
				d.TimeUpdate = t
				fw.mapEtcd.Store(v, d)
				continue
			}
			// if fw.mapEtcd.Has(v) {
			// 	fw.mapEtcd.update(v, t)
			// 	continue
			// }
			body := &EtcdInfo{}
			err = json.UnmarshalFromString(b, body)
			if err != nil {
				continue
			}
			body.TimeUpdate = t
			if body.Fulladdr == "" {
				body.Fulladdr = fmt.Sprintf("%s://%s:%s", body.Intfc, body.IP, body.Port)
			}
			fw.mapEtcd.Store(v, body)
		}
		// 清理旧的
		fw.mapEtcd.ForEach(func(key string, value *EtcdInfo) bool {
			if value.TimeUpdate != t {
				fw.mapEtcd.Delete(key)
			}
			return true
		})
	}

	go loopfunc.LoopFunc(func(params ...interface{}) {
		treg := time.NewTicker(time.Second * 5) // 注册
		tget := time.NewTicker(time.Second * 6) // 读取
		fetcdRead()
		fw.chanRegDone <- struct{}{}
		if err := fetcdReg(); err == nil {
			fw.WriteSystem("ETCD", fmt.Sprintf("Registration to redis-server %v as `%s://%s:%s/%s` success.", fw.redisCtl.addr, etcd.Interface, etcd.Host, etcd.Port, etcd.Name))
		}
		for {
			select {
			case <-treg.C:
				fetcdReg()
			case <-tget.C:
				fetcdRead()
			}
		}
	}, "etcd redis", fw.wmLog.DefaultWriter())
}

// Picker 选取服务地址
func (fw *WMFrameWorkV2) redisPicker(svrName string) (string, error) {
	return fw.redisPickerDetail(svrName)
}

// AllServices 返回所有服务列表
func (fw *WMFrameWorkV2) redisAllServices() (string, error) {
	var s = make(map[string][]string)
	fw.mapEtcd.ForEach(func(key string, value *EtcdInfo) bool {
		s[key] = []string{
			// fmt.Sprintf("%s://%s:%s", v.EtcdInfo.Intfc, v.EtcdInfo.IP, v.EtcdInfo.Port),
			value.Fulladdr,
			value.Source,
			value.Alias,
			value.Data,
		}
		return true
	})
	return json.MarshalToString(s)
}

// PickerDetail 选取服务地址,带http(s)前缀
func (fw *WMFrameWorkV2) redisPickerDetail(svrName string) (string, error) {
	addr := ""
	err := fmt.Errorf(`no matching server was found with the name %s`, svrName)
	fw.mapEtcd.ForEach(func(key string, value *EtcdInfo) bool {
		if value.Name == svrName {
			addr = value.Fulladdr
			err = nil
			return false
		}
		return true
	})
	return addr, err
}

// RedisETCDKeys 循环
func (fw *WMFrameWorkV2) RedisETCDKeys() []string {
	ss := make([]string, 0)
	fw.mapEtcd.ForEach(func(key string, value *EtcdInfo) bool {
		ss = append(ss, key)
		return true
	})
	return ss
}
