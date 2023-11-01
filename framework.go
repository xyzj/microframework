package wmfw

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/xyzj/gopsu"
	"github.com/xyzj/gopsu/cache"
	"github.com/xyzj/gopsu/config"
	"github.com/xyzj/gopsu/db"
	"github.com/xyzj/gopsu/gocmd"
	json "github.com/xyzj/gopsu/json"
	"github.com/xyzj/gopsu/logger"
	"github.com/xyzj/gopsu/mapfx"
	"github.com/xyzj/gopsu/pathtool"

	// 载入资源
	_ "embed"
)

//go:embed ca/ca.pem
var ca []byte

//go:embed ca/localhost.pem
var caCert []byte

//go:embed ca/localhost-key.pem
var caKey []byte

//go:embed ca/localhost.pfx
var caPfx []byte

//go:embed nothere.webp
var nothere []byte

//go:embed favicon.webp
var favicon []byte

var signalQuit = gocmd.NewSignalQuit()

// NewFrameWorkV2 初始化一个新的framework
func NewFrameWorkV2(versionInfo string) *WMFrameWorkV2 {
	// gocmd.NewProgram(&gocmd.Info{
	// 	Ver: versionInfo}).AddCommand(
	// 	gocmd.CmdStart).AddCommand(
	// 	gocmd.CmdRestart).AddCommand(
	// 	gocmd.CmdStatus).AddCommand(
	// 	gocmd.CmdStop).AddCommand(
	// 	&gocmd.Command{
	// 		Name:     "run",
	// 		Descript: "run the program.",
	// 		RunWithExitCode: func(pinfo *gocmd.ProcInfo) int {
	// 			pinfo.Pid = os.Getpid()
	// 			pinfo.Save()
	// 			signalQuit.SignalCapture(nil)
	// 			return -1
	// 		},
	// 	}).ExecuteDefault("run")
	gocmd.DefaultProgram(&gocmd.Info{
		Ver: versionInfo,
	}).ExecuteDefault("run")
	if !flag.Parsed() {
		flag.Parse()
	}
	// 初始化
	fw := &WMFrameWorkV2{
		rootPath:   "micro-svr",
		tokenLife:  time.Minute * 30,
		appConf:    &config.File{},
		serverName: "xserver",
		upTime:     time.Now().Format("2006-01-02 15:04:05 Mon"),
		verJSON:    versionInfo,
		etcdCtl:    &etcdConfigure{},
		redisCtl:   &redisConfigure{},
		dbCtl:      &dbConfigure{},
		rmqCtl:     &rabbitConfigure{},
		rmqCtl2nd:  &rabbitConfigure{},
		mqttCtl:    &mqttConfigure{},
		reqTimeo:   time.Second * 30,
		tokenCache: cache.NewCache(3000),
		httpClientPool: &http.Client{
			// Timeout: time.Duration(time.Second * 60),
			Transport: &http.Transport{
				// DialContext: (&net.Dialer{
				// 	Timeout: time.Second * 2,
				// }).DialContext,
				// TLSHandshakeTimeout: time.Second * 2,
				IdleConnTimeout:     time.Second * 2,
				MaxConnsPerHost:     777,
				MaxIdleConnsPerHost: 2,
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		},
		chanSSLRenew: make(chan struct{}, 2),
		chanRegDone:  make(chan struct{}, 2),
		cacheHead:    gopsu.CalcCRC32String([]byte("microsvrv2")),
		cacheLocker:  &sync.Map{},
		ft:           mapfx.NewBaseMap[string](),
		mapEtcd:      mapfx.NewStructMap[string, EtcdInfo](), //     &mapETCD{locker: sync.RWMutex{}, data: make(map[string]*EtcdInfo)},
		httpProtocol: "http://",
	}
	// 处置版本，检查机器码
	fw.checkMachine()
	// 写版本信息
	os.WriteFile(pathtool.GetExecName()+".ver", []byte(versionInfo), 0444)
	// p, _ := os.Executable()
	// f, _ := os.OpenFile(p+".ver", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	// defer f.Close()
	// f.Write(fmtver)
	// 处置目录
	if *portable {
		gopsu.DefaultConfDir, gopsu.DefaultLogDir, gopsu.DefaultCacheDir = gopsu.MakeRuntimeDirs(".")
	} else {
		gopsu.DefaultConfDir, gopsu.DefaultLogDir, gopsu.DefaultCacheDir = gopsu.MakeRuntimeDirs("..")
	}
	// 设置基础路径
	fw.baseCAPath = filepath.Join(gopsu.DefaultConfDir, "ca")
	if !pathtool.IsExist(fw.baseCAPath) {
		os.MkdirAll(fw.baseCAPath, 0755)
	}
	fw.tlsCert = filepath.Join(fw.baseCAPath, "localhost.pem")
	fw.tlsKey = filepath.Join(fw.baseCAPath, "localhost-key.pem")
	fw.tlsRoot = filepath.Join(fw.baseCAPath, "ca.pem")
	fw.httpCert = ""
	fw.httpKey = ""
	// fw.httpCert = filepath.Join(fw.baseCAPath, "localhost.pem")
	// fw.httpKey = filepath.Join(fw.baseCAPath, "localhost-key.pem")
	if !pathtool.IsExist(fw.tlsRoot) {
		os.WriteFile(fw.tlsRoot, ca, 0644)
	}
	if !pathtool.IsExist(fw.tlsCert) {
		os.WriteFile(fw.tlsCert, caCert, 0644)
	}
	if !pathtool.IsExist(fw.tlsKey) {
		os.WriteFile(fw.tlsKey, caKey, 0644)
	}
	if !pathtool.IsExist(filepath.Join(fw.baseCAPath, "localhost.pfx")) {
		os.WriteFile(filepath.Join(fw.baseCAPath, "localhost.pfx"), caPfx, 0644)
	}
	return fw
}

// Start 运行框架
// 启动模组，不阻塞
func (fw *WMFrameWorkV2) Start(opv2 *OptionFrameWorkV2) {
	// 设置日志
	fw.cnf = opv2
	if opv2.UseETCD != nil {
		if opv2.UseETCD.SvrName != "" {
			fw.serverName = opv2.UseETCD.SvrName
		}
		if *nameTail != "" {
			fw.serverName += "-" + *nameTail
		}
		fw.serverAlias = func(alias string) string {
			if alias == "" {
				return fw.serverName
			}
			return alias
		}(opv2.UseETCD.SvrAlias)

		fw.cacheHead = gopsu.CalcCRC32String([]byte(fw.serverName))
	}
	if fw.loggerMark == "" {
		fw.loggerMark = fmt.Sprintf("%s-%05d", fw.serverName, *webPort)
	}
	// 载入配置
	if opv2.ConfigFile == "" {
		opv2.ConfigFile = *conf
	}
	var cfpath string
	if opv2.ConfigFile != "" {
		if strings.ContainsAny(opv2.ConfigFile, "\\/") {
			cfpath = opv2.ConfigFile
		} else {
			cfpath = filepath.Join(gopsu.DefaultConfDir, opv2.ConfigFile)
		}
		if !pathtool.IsExist(cfpath) {
			println("no config file found, try to create new one")
		}
	}
	fw.loadConfigure(cfpath)
	// 日志
	cl := []byte{40, 90}
	if logLevel == 10 {
		cl = append(cl, 10, 20, 30)
	}
	fw.wmLog = logger.NewLogger(gopsu.DefaultLogDir,
		func(level int) string {
			if level > 1 {
				return fw.loggerMark + ".core"
			}
			return ""
		}(logLevel),
		logLevel,
		logDays,
		true,
		cl...)
	fw.httpWriter = logger.NewWriter(&logger.OptLog{
		AutoRoll: logLevel > 1,
		FileDir:  gopsu.DefaultLogDir,
		Filename: func(level int) string {
			if level > 1 {
				return fw.loggerMark + ".http"
			}
			return ""
		}(logLevel),
		MaxDays:    logDays,
		ZipFile:    logDays > 10,
		DelayWrite: true,
	})
	fw.cacheMem = cache.NewCacheWithWriter(70000, fw.wmLog.DefaultWriter())
	// 前置处理方法，用于预初始化某些内容
	if opv2.FrontFunc != nil {
		opv2.FrontFunc()
	}
	// redis
	if !*disableRedis {
		fw.newRedisClient()
	}
	// if opv2.UseRedis != nil {
	// 	if opv2.UseRedis.Activation {
	// 		fw.newRedisClient()
	// 	}
	// }
	// etcd
	if opv2.UseETCD != nil {
		if !opv2.UseETCD.ForceOff {
			fw.newRedisETCDClient()
		}
		// if opv2.UseETCD.Activation {
		// 	go fw.newETCDClient()
		// }
	}
	// 生产者
	if opv2.UseMQProducer != nil {
		if opv2.UseMQProducer.Activation {
			fw.newMQProducer()
		}
	}
	// 消费者
	if opv2.UseMQConsumer != nil {
		if opv2.UseMQConsumer.Activation {
			// if fw.newMQConsumer(!opv2.UseMQConsumer.KeepQueue) {
			// 	if opv2.UseMQConsumer.BindKeysFunc != nil {
			// 		if ss, ok := opv2.UseMQConsumer.BindKeysFunc(); ok {
			// 			opv2.UseMQConsumer.BindKeys = ss
			// 		}
			// 	}
			// 	fw.BindRabbitMQ(opv2.UseMQConsumer.BindKeys...)
			// 	go fw.recvRabbitMQ(opv2.UseMQConsumer.RecvFunc)
			// }
			if opv2.UseMQConsumer.BindKeysFunc != nil {
				if ss, ok := opv2.UseMQConsumer.BindKeysFunc(); ok {
					opv2.UseMQConsumer.BindKeys = ss
				}
			}
			var xss = make([]string, 0, len(opv2.UseMQConsumer.BindKeys))
			for _, v := range opv2.UseMQConsumer.BindKeys {
				xss = append(xss, fw.AppendRootPathRabbit(v))
			}
			fw.newMQConsumerV2(!opv2.UseMQConsumer.KeepQueue, xss, opv2.UseMQConsumer.RecvFunc)
		}
	}
	// mqtt
	if opv2.UseMQTT != nil {
		if opv2.UseMQTT.Activation {
			if opv2.UseMQTT.RecvFunc == nil {
				opv2.UseMQTT.RecvFunc = func(key string, msg []byte) {
					fw.WriteDebug("MQTT-D", "key:"+key+" | body:"+gopsu.String(msg))
				}
			}
			fw.newMQTTClient(opv2.UseMQTT.BindKeys, opv2.UseMQTT.RecvFunc)
		}
	}
	// sql
	if opv2.UseSQL != nil {
		if opv2.UseSQL.Activation {
			if fw.newDBClient(gopsu.String(opv2.UseSQL.DBInit), gopsu.String(opv2.UseSQL.DBUpgrade)) {
				// 分表维护线程
				if opv2.UseSQL.DoMERGE {
					go fw.MaintainMrgTables()
				}
			} else {
				// sql无法连接直接退出程序
				fw.appConf.ToFile()
				fw.baseConf.ToFile()
				time.Sleep(time.Second)
				os.Exit(1)
			}
			if opv2.UseSQL.SupportORM {
				fw.newORMEngines()
			}
		}
	}
	// gin http
	if opv2.UseHTTP != nil {
		if opv2.UseHTTP.Activation {
			if opv2.UseHTTP.EngineFunc == nil {
				opv2.UseHTTP.EngineFunc = func() *gin.Engine {
					return fw.NewHTTPEngine()
				}
			}
			go fw.newHTTPService(opv2.UseHTTP.EngineFunc())
		}
	}
	// gpstimer
	if fw.gpsTimer > 0 {
		go fw.newGPSConsumer()
	}
	if fw.mqP2nd {
		go fw.newMQProducer2nd()
	}
	// 执行额外方法
	if opv2.ExpandFuncs != nil {
		select {
		case <-fw.chanRegDone:
		case <-time.After(time.Second * 3):
		}
		for _, v := range opv2.ExpandFuncs {
			v()
		}
	}
	// 处理缓存文件
	go func() {
		t := time.NewTicker(time.Hour)
		for range t.C {
			fw.clearCache()
		}
	}()
	// 启用性能调试，仅可用于开发过程中
	// if *pyroscope {
	// profiler.Start(profiler.Config{
	// 	ApplicationName: fw.serverName + "_" + gopsu.RealIP(false),
	// 	ServerAddress:   "http://192.168.50.83:10097",
	// })
	// }
	fw.appConf.ToFile()
	// fw.baseConf.ToFile()
	fw.WriteSystem("SYS", "Service start:"+fw.verJSON)
}

// Run 运行框架
// 启动模组，阻塞
func (fw *WMFrameWorkV2) Run(opv2 *OptionFrameWorkV2) {
	fw.Start(opv2)
	select {}
}

// LoadConfigure 初始化配置
func (fw *WMFrameWorkV2) loadConfigure(f string) {
	fw.appConf = config.NewConfig(f)
	if *ignoreBase {
		fw.baseConf = fw.appConf
	} else {
		if baseConf := pathtool.JoinPathFromHere("base.conf"); pathtool.IsExist(baseConf) {
			fw.baseConf = config.NewConfig(baseConf)
		} else {
			fw.baseConf = fw.appConf
		}
	}
	fw.rootPath = fw.baseConf.GetDefault(&config.Item{Key: "root_path", Value: "micro-svr", Comment: "etcd/mq/redis注册根路径"}).String()
	fw.rootPathRedis = "/" + fw.rootPath + "/"
	fw.rootPathMQ = fw.rootPath + "."
	fw.gpsTimer = fw.baseConf.GetDefault(&config.Item{Key: "gpstimer", Value: "0", Comment: "是否使用广播的gps时间进行对时操作,0-不启用，1-启用（30～900s内进行矫正），2-忽略误差范围强制矫正"}).TryInt64()
	fw.mqP2nd = fw.baseConf.GetItem("mq_2nd_enable").TryBool()

	// 检查高优先级输入参数，覆盖
	if do := fw.appConf.GetItem("domain_name").String(); do != "" {
		if pathtool.IsExist(filepath.Join(fw.baseCAPath, do+".crt")) && pathtool.IsExist(filepath.Join(fw.baseCAPath, do+".key")) {
			fw.httpCert = filepath.Join(fw.baseCAPath, do+".crt")
			fw.httpKey = filepath.Join(fw.baseCAPath, do+".key")
		}
	}
	if *cert != "" && *key != "" && pathtool.IsExist(*cert) && pathtool.IsExist(*key) {
		fw.httpCert = *cert
		fw.httpKey = *key
		fw.httpProtocol = "https://"
	}
	// 以下参数不自动生成，影响dorequest性能
	if tr := fw.baseConf.GetItem("tr_timeo").TryInt64(); tr > 5 {
		fw.reqTimeo = time.Second * time.Duration(tr)
	}
	switch fw.baseConf.GetDefault(&config.Item{Key: "log_level", Value: "info", Comment: "log level, enable value: debug, info, warn, error"}).String() {
	case "debug":
		logLevel = 10
	case "warn":
		logLevel = 30
	case "error":
		logLevel = 40
	default:
		logLevel = 20
	}
	switch x := fw.baseConf.GetDefault(&config.Item{Key: "log_days", Value: "10", Comment: "log file keep kays, 1~30"}).TryInt64(); x / 30 {
	case 0, 1:
		if x == 0 {
			logDays = 10
		} else {
			logDays = int(x)
		}
	default:
		logDays = 10
	}
	if logDays > 30 {
		logDays = 30
	}
	if *debug {
		logLevel = 10
	}
}

// GetLogger 返回日志模块
func (fw *WMFrameWorkV2) GetLogger() logger.Logger {
	return fw.wmLog
}

// LogDefaultWriter 返回日志writer
func (fw *WMFrameWorkV2) LogDefaultWriter() io.Writer {
	return fw.wmLog.DefaultWriter()
}

// ConfClient 配置文件实例
func (fw *WMFrameWorkV2) ConfClient() *config.File {
	return fw.appConf
}

// ReadConfigItem 读取配置参数
func (fw *WMFrameWorkV2) ReadConfigItem(key, value, remark string) string {
	if fw.appConf == nil {
		return ""
	}
	return fw.appConf.GetDefault(&config.Item{Key: key, Value: config.VString(value), Comment: remark}).String()
}

// ReadConfigKeys 获取配置所有key
func (fw *WMFrameWorkV2) ReadConfigKeys() []string {
	s := fw.appConf.Keys()
	if fw.appConf != fw.baseConf {
		s = append(s, fw.baseConf.Keys()...)
	}
	return s
}

// ReadConfigAll 获取配置所有item
func (fw *WMFrameWorkV2) ReadConfigAll() string {
	s := fw.appConf.Print()

	if fw.appConf != fw.baseConf {
		s += "\n" + fw.baseConf.Print()
	}
	return s
}

// ReloadConfig 重新读取
func (fw *WMFrameWorkV2) ReloadConfig() error {
	return fw.appConf.FromFile("")
}

// WriteConfigItem 更新key
func (fw *WMFrameWorkV2) WriteConfigItem(key, value string) {
	fw.appConf.PutItem(&config.Item{Key: key, Value: config.VString(value)})
}

// WriteConfig 保存配置
func (fw *WMFrameWorkV2) WriteConfig() {
	fw.appConf.ToFile()
}

// Tag 版本标签
func (fw *WMFrameWorkV2) Tag() string {
	if fw.tag == "" {
		fw.tag = gjson.Parse(fw.verJSON).Get("version").String()
	}
	return fw.tag
}

// VersionInfo 获取版本信息
func (fw *WMFrameWorkV2) VersionInfo() string {
	return fw.verJSON
}

// SetVersionInfo 更新版本信息
func (fw *WMFrameWorkV2) SetVersionInfo(v string) {
	fw.verJSON = v
}

// WebPort http 端口
func (fw *WMFrameWorkV2) WebPort() int {
	return *webPort
}

// ServerName 服务名称
func (fw *WMFrameWorkV2) ServerName() string {
	return fw.serverName
}

// SetServerName 设置服务名称
func (fw *WMFrameWorkV2) SetServerName(s string) {
	fw.serverName = s
}

// SetLoggerMark 设置日志文件标识
func (fw *WMFrameWorkV2) SetLoggerMark(s string) {
	fw.loggerMark = s
}

// SetHTTPTimeout 设置http超时
func (fw *WMFrameWorkV2) SetHTTPTimeout(second int) {
	fw.httpClientPool.Timeout = time.Second * time.Duration(second)
}

// Debug 返回是否debug模式
func (fw *WMFrameWorkV2) Debug() bool {
	return *debug
}

// DBClient dbclient
func (fw *WMFrameWorkV2) DBClient() *db.SQLPool {
	return fw.dbCtl.client
}

// HTTPProtocol http协议
func (fw *WMFrameWorkV2) HTTPProtocol() string {
	return fw.httpProtocol
}

// ServerAlias 返回服务别名
func (fw *WMFrameWorkV2) ServerAlias() string {
	return fw.serverAlias
}

// RootPath 返回根路径
func (fw *WMFrameWorkV2) RootPath() string {
	return fw.rootPath
}

// Marshal 序列化为字节数组
func (fw *WMFrameWorkV2) Marshal(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte{}
	}
	return b
}

// MarshalToString 序列化为字符串
func (fw *WMFrameWorkV2) MarshalToString(v interface{}) string {
	b, err := json.MarshalToString(v)
	if err != nil {
		return ""
	}
	return b
}

// SetTokenLife 设置User-Token的有效期，默认30分钟
func (fw *WMFrameWorkV2) SetTokenLife(t time.Duration) {
	fw.tokenLife = t
}
