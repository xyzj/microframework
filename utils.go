/*
Package wmfw : wlst的微服务框架第二版
*/
package wmfw

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xyzj/gopsu"
	"github.com/xyzj/gopsu/cache"
	"github.com/xyzj/gopsu/config"
	json "github.com/xyzj/gopsu/json"
	"github.com/xyzj/gopsu/logger"
	"github.com/xyzj/gopsu/mapfx"
	"gorm.io/gorm"
)

// 启动参数
var (
	// pyroscope debug
	pyroscope = flag.Bool("pyroscope", false, "set true to enable pyroscope debug, should only be used in DEV-LAN")
	// forceHTTP 强制http
	forceHTTP = flag.Bool("forcehttp", false, "[Discarded] set true to use HTTP anyway.")
	//  是否启用调试模式
	debug = flag.Bool("debug", false, "set if enable debug info.")
	// logLevel 日志等级，可选项1,10,20,30,40
	logLevel1 = flag.Int("loglevel", 20, "[Discarded] use base.conf")
	// logDays 日志文件保留天数，默认10
	logDays1 = flag.Int("logdays", 10, "[Discarded] use base.conf")
	// webPort 主端口
	webPort = flag.Int("http", 6819, "set http port to listen on.")
	// portable 把日志，缓存等目录创建在当前目录下，方便打包带走
	portable = flag.Bool("portable", false, "把日志，配置，缓存目录创建在当前目录下")
	// 配置文件
	conf = flag.String("conf", "", "set the config file path.")
	// 服务名增加随机字符，用于调试时名称不重复
	nameTail = flag.String("nametail", "", "Add a string tail after the service name")
	// 证书crt文件
	cert = flag.String("cert", "", "set https cert file path")
	// 证书key文件
	key          = flag.String("key", "", "set https key file path")
	disableRedis = flag.Bool("disable-redis", false, "only used for program debug, force disable redis client")
	disableRMQ   = flag.Bool("disable-rmq", false, "only used for program debug, force disable rabbitmq client")
	disableDB    = flag.Bool("disable-db", false, "only used for program debug, force disable db client")
	disableMqtt  = flag.Bool("disable-mqtt", false, "only used for program debug, force disable mqtt client")
	ignoreBase   = flag.Bool("ignore-base-config", false, "do not load `base.conf`, only use `-conf=`")
)

var (
	logLevel = 20
	logDays  = 10
	// CWorker 加密
	CWorker *gopsu.CryptoWorker // = gopsu.GetNewCryptoWorker(gopsu.CryptoAES128CBC)
	// MD5Worker md5计算
	MD5Worker = gopsu.GetNewCryptoWorker(gopsu.CryptoMD5)
	// CodeGzip 压缩
	CodeGzip = gopsu.GetNewArchiveWorker(gopsu.ArchiveGZip)
	// SHA256Worker sha256计算
	SHA256Worker = gopsu.GetNewCryptoWorker(gopsu.CryptoSHA256)
)

// OptionETCD ETCD配置
type OptionETCD struct {
	// 服务名称
	SvrName string
	// 服务别名（显示用名称）
	SvrAlias string
	// 服务类型，留空时默认为http或https
	SvrType string
	// 交互协议，留空默认json
	SvrProtocol string
	// 启用
	Activation bool
	ForceOff   bool
}

// OptionSQL 数据库配置
type OptionSQL struct {
	// 启动分表线程
	DoMERGE bool
	// 启用
	Activation bool
	// 设置升级脚本
	DBUpgrade []byte
	// 设置初始化脚本
	DBInit []byte
	// 启用xorm支持
	SupportORM bool
}

// OptionRedis redis配置
type OptionRedis struct {
	// 启用
	Activation bool
}

// OptionMQProducer rmq配置
type OptionMQProducer struct {
	// 启用
	Activation bool
}

// OptionMQGPSTimer rmq gps timer 配置
type OptionMQGPSTimer struct {
	// 启用
	Activation bool
}

// OptionMQConsumer rmq配置
type OptionMQConsumer struct {
	// 消费者绑定key
	BindKeys []string
	// 消费者key获取方法
	BindKeysFunc func() ([]string, bool)
	// 消费者数据处理方法
	RecvFunc func(key string, body []byte)
	// 启用
	Activation bool
	// 队列在无消费者时是否保留
	KeepQueue bool
}

// OptionHTTP http配置
type OptionHTTP struct {
	// 路由引擎组合方法，推荐使用这个方法代替GinEngine值，可以避免过早初始化
	EngineFunc func() *gin.Engine
	// 启用
	Activation bool
}

// OptionMQTT mqtt配置
type OptionMQTT struct {
	// 消费者绑定key
	BindKeys []string
	// 消费者数据处理方法
	RecvFunc func(key string, body []byte)
	// 启用
	Activation bool
}

// OptionFrameWorkV2 wlst 微服务框架配置v2版
type OptionFrameWorkV2 struct {
	// 配置文件路径
	ConfigFile string
	// 启用ETCD模块
	UseETCD *OptionETCD
	// 启用SQL模块
	UseSQL *OptionSQL
	// 启用Redis模块
	UseRedis *OptionRedis
	// 启用mq生产者模块
	UseMQProducer *OptionMQProducer
	// 启用mq消费者模块
	UseMQConsumer *OptionMQConsumer
	// 启用mqtt模块
	UseMQTT *OptionMQTT
	// 启用http服务模块
	UseHTTP *OptionHTTP
	// 启动参数处理方法，在功能模块初始化之前执行
	// 提交方法名称时最后不要加`()`，表示把方法作为参数，而不是把方法的执行结果回传
	FrontFunc func()
	// 扩展方法列表，用于处理额外的数据或变量，在主要模块启动完成后依次执行
	// 非线程顺序执行，注意不要阻塞
	// sample：
	// []func(){
	//	 FuncA,
	//	 go FuncB
	// }
	ExpandFuncs []func()
}

// WMFrameWorkV2 v2版微服务框架
type WMFrameWorkV2 struct {
	dborms         []*gorm.DB
	chanSSLRenew   chan struct{}
	chanRegDone    chan struct{}
	wmLog          logger.Logger // 日志
	httpWriter     io.Writer
	cacheLocker    *sync.Map
	cacheMem       *cache.XCache
	ft             *mapfx.BaseMap[string]             // 固定token
	mapEtcd        *mapfx.StructMap[string, EtcdInfo] //*mapETCD
	appConf        *config.File                       // 配置
	baseConf       *config.File                       // 公共配置，只读，不写
	etcdCtl        *etcdConfigure
	redisCtl       *redisConfigure
	dbCtl          *dbConfigure
	rmqCtl         *rabbitConfigure
	rmqCtl2nd      *rabbitConfigure
	mqttCtl        *mqttConfigure
	httpClientPool *http.Client
	cnf            *OptionFrameWorkV2
	tokenCache     *cache.XCache
	tokenLife      time.Duration
	reqTimeo       time.Duration
	serverName     string
	serverAlias    string
	loggerMark     string
	verJSON        string
	tag            string
	upTime         string
	rootPath       string
	rootPathRedis  string
	rootPathMQ     string
	httpProtocol   string
	baseCAPath     string // tls配置
	tlsCert        string //  = filepath.Join(baseCAPath, "client-cert.pem")
	tlsKey         string //  = filepath.Join(baseCAPath, "client-key.pem")
	tlsRoot        string //  = filepath.Join(baseCAPath, "rootca.pem")
	httpCert       string
	httpKey        string
	cacheHead      string // 缓存
	gpsTimer       int64  // 启用gps校时,0-不启用，1-启用（30～900s内进行矫正），2-强制对时
	mqP2nd         bool
}

func init() {
	// 设置使用的cpu核心数量
	runtime.GOMAXPROCS(runtime.NumCPU())
	// CWorker 加密
	CWorker = gopsu.GetNewCryptoWorker(gopsu.CryptoAES128CBC)
	CWorker.SetKey("(NMNle+XW!ykVjf1", "Zq0V+,.2u|3sGAzH")
	os.Setenv("GOCMD_LOG_SIGNAL_TO_FILE", "1")
}

// NotHere 返回nothere图片
func NotHere() []byte {
	return nothere
}

type AssetID string

func (aid AssetID) String() string {
	return string(aid)
}

func (aid *AssetID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*aid = AssetID(s)
	return nil
}
func (aid *AssetID) MarshalJSON() ([]byte, error) {
	if a := string(*aid); a == "" || a == "00000000000000000000" {
		return []byte("\"\""), nil
	}
	return []byte("\"" + fmt.Sprintf("%020s", *aid) + "\""), nil
}

// EmptyAID 判断aid是否为空
func (fw *WMFrameWorkV2) EmptyAID(aid string) bool {
	return aid == "" || aid == "00000000000000000000"
}
