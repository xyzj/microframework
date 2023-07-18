package wmfw

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/tidwall/sjson"
	"github.com/xyzj/gopsu"
)

var (
	redisCtxTimeo = 3 * time.Second
)

// redis配置
type redisConfigure struct {
	forshow string
	// redis服务地址
	addr string
	// 访问密码
	pwd string
	// 数据库
	database int
	// 是否启用redis
	enable bool
	// client
	client *redis.Client
	// 主版本号
	mianver int
	ready   bool
}

func (conf *redisConfigure) show(rootPath string) string {
	conf.forshow, _ = sjson.Set("", "addr", conf.addr)
	conf.forshow, _ = sjson.Set(conf.forshow, "pwd", CWorker.Encrypt(conf.pwd))
	conf.forshow, _ = sjson.Set(conf.forshow, "dbname", conf.database)
	conf.forshow, _ = sjson.Set(conf.forshow, "root_path", rootPath)
	return conf.forshow
}

// NewRedisClient 新的redis client
func (fw *WMFrameWorkV2) newRedisClient() bool {
	if fw.redisCtl.ready {
		fw.tryRedisVer()
		return true
	}
	fw.redisCtl.addr = fw.wmConf.GetItemDefault("redis_addr", "127.0.0.1:6379", "redis服务地址,ip:port格式")
	fw.redisCtl.pwd = gopsu.DecodeString(fw.wmConf.GetItemDefault("redis_pwd", "WcELCNqP5dCpvMmMbKDdvgb", "redis连接密码"))
	fw.redisCtl.database, _ = strconv.Atoi(fw.wmConf.GetItemDefault("redis_db", "0", "redis数据库id"))
	fw.redisCtl.enable, _ = strconv.ParseBool(fw.wmConf.GetItemDefault("redis_enable", "true", "是否启用redis"))
	fw.wmConf.Save()
	fw.redisCtl.show(fw.rootPath)
	if !fw.redisCtl.enable {
		return false
	}
	fw.redisCtl.client = redis.NewClient(&redis.Options{
		Addr:            fw.redisCtl.addr,
		Password:        fw.redisCtl.pwd,
		DB:              fw.redisCtl.database,
		PoolFIFO:        true,
		MinIdleConns:    3,
		ConnMaxIdleTime: time.Minute,
		OnConnect: func(ctx context.Context, cn *redis.Conn) error {
			if !fw.redisCtl.ready {
				fw.redisCtl.ready = true
				fw.WriteSystem("REDIS", fmt.Sprintf("Connect to server %s is ready", fw.redisCtl.addr))
			}
			return nil
		},
	})
	fw.redisCtl.ready = true
	fw.WriteSystem("REDIS", fmt.Sprintf("Connect to server %s is ready, use db %d", fw.redisCtl.addr, fw.redisCtl.database))
	// fw.WriteSystem("REDIS", fmt.Sprintf("Connect to server %s is ready, use db %d", fw.redisCtl.addr, fw.redisCtl.database))
	fw.tryRedisVer()
	// fw.redisCtl.ready = true
	// fw.WriteSystem("REDIS", fmt.Sprintf("Success connect to server %s, use db %d", fw.redisCtl.addr, fw.redisCtl.database))
	return true
}

// AppendRootPathRedis 向redis的key追加头
func (fw *WMFrameWorkV2) AppendRootPathRedis(key string) string {
	if !strings.HasPrefix(key, fw.rootPathRedis) {
		if strings.HasPrefix(key, "/") {
			return fw.rootPathRedis + key[1:]
		}
		return fw.rootPathRedis + key
	}
	return key
}

// ExpireRedis 更新redis有效期
func (fw *WMFrameWorkV2) ExpireRedis(key string, expire time.Duration) bool {
	if !fw.redisCtl.ready {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), redisCtxTimeo)
	defer cancel()
	return fw.redisCtl.client.Expire(ctx, fw.AppendRootPathRedis(key), expire).Val()
}

// ExistsRedis 检查key是否存在
func (fw *WMFrameWorkV2) ExistsRedis(key string) (bool, error) {
	if !fw.redisCtl.ready {
		return false, errRedisNotReady
	}
	ctx, cancel := context.WithTimeout(context.Background(), redisCtxTimeo)
	defer cancel()
	x := fw.redisCtl.client.Exists(ctx, fw.AppendRootPathRedis(key))
	fw.logRedisError(x.Err(), "failed check redis key: %v ", key)
	return x.Val() == 1, x.Err()
}

// EraseRedis 删redis
func (fw *WMFrameWorkV2) EraseRedis(key ...string) error {
	if !fw.redisCtl.ready {
		return errRedisNotReady
	}
	if len(key) == 0 {
		return errRedisNil
	}
	keys := make([]string, len(key))
	for k, v := range key {
		keys[k] = fw.AppendRootPathRedis(v)
	}
	ctx, cancel := context.WithTimeout(context.Background(), redisCtxTimeo)
	defer cancel()
	fw.WriteInfo("REDIS", fmt.Sprintf("erase redis data:%+v", keys))
	err := fw.redisCtl.client.Del(ctx, keys...).Err()
	return fw.logRedisError(err, "failed erase redis data: %+v", keys)
}

// EraseAllRedis 模糊删除
func (fw *WMFrameWorkV2) EraseAllRedis(key string) error {
	if !fw.redisCtl.ready {
		return errRedisNotReady
	}
	ctx, cancel := context.WithTimeout(context.Background(), redisCtxTimeo)
	defer cancel()
	val := fw.redisCtl.client.Keys(ctx, fw.AppendRootPathRedis(key))
	if val.Err() != nil {
		return val.Err()
	}
	var err error
	if len(val.Val()) > 0 {
		ctx2, cancel := context.WithTimeout(context.Background(), redisCtxTimeo)
		defer cancel()
		fw.WriteInfo("REDIS", fmt.Sprintf("erase all redis data:%s", fw.AppendRootPathRedis(key)))
		err = fw.redisCtl.client.Del(ctx2, val.Val()...).Err()
	}
	return fw.logRedisError(err, "failed erase all redis data: %s", key)
}

// ReadRedis 读redis
func (fw *WMFrameWorkV2) ReadRedis(key string) (string, error) {
	if !fw.redisCtl.ready {
		return "", errRedisNotReady
	}
	key = fw.AppendRootPathRedis(key)
	ctx, cancel := context.WithTimeout(context.Background(), redisCtxTimeo)
	defer cancel()
	val := fw.redisCtl.client.Get(ctx, key)
	if val.Err() != nil {
		fw.logRedisDebug(val.Err(), "failed read redis data: %s", key)
		if strings.Contains(val.Err().Error(), " nil") {
			return "", errRedisNil
		}
		return "", val.Err()
	}
	return val.Val(), nil
}

// ReadHashRedis 读取所有hash数据
func (fw *WMFrameWorkV2) ReadHashRedis(key, field string) (string, error) {
	if !fw.redisCtl.ready {
		return "", errRedisNotReady
	}
	key = fw.AppendRootPathRedis(key)
	ctx, cancel := context.WithTimeout(context.Background(), redisCtxTimeo)
	defer cancel()
	val := fw.redisCtl.client.HGet(ctx, key, field)
	if val.Err() != nil {
		fw.logRedisDebug(val.Err(), "failed read redis hash data: %s, %s", key, field)
		if strings.Contains(val.Err().Error(), " nil") {
			return "", errRedisNil
		}
		return "", val.Err()
	}
	return val.Val(), nil
}

// ReadHashAllRedis 读取所有hash数据
func (fw *WMFrameWorkV2) ReadHashAllRedis(key string) (map[string]string, error) {
	if !fw.redisCtl.ready {
		return nil, errRedisNotReady
	}
	key = fw.AppendRootPathRedis(key)
	ctx, cancel := context.WithTimeout(context.Background(), redisCtxTimeo)
	defer cancel()
	val := fw.redisCtl.client.HGetAll(ctx, key)
	if val.Err() != nil {
		fw.logRedisDebug(val.Err(), "failed read redis all hash data: %s", key)
		if strings.Contains(val.Err().Error(), " nil") {
			return nil, errRedisNil
		}
		return nil, val.Err()
	}
	return val.Val(), nil
}

// WriteRedis 写redis
func (fw *WMFrameWorkV2) WriteRedis(key string, value interface{}, expire time.Duration) error {
	if !fw.redisCtl.ready {
		return errRedisNotReady
	}
	ctx, cancel := context.WithTimeout(context.Background(), redisCtxTimeo)
	defer cancel()
	err := fw.redisCtl.client.Set(ctx, fw.AppendRootPathRedis(key), value, expire).Err()
	return fw.logRedisError(err, "failed write redis data: %s ", key)
}

// WriteHashFieldRedis 修改或添加redis hashmap中的值
func (fw *WMFrameWorkV2) WriteHashFieldRedis(key, field string, value interface{}) error {
	if !fw.redisCtl.ready {
		return errRedisNotReady
	}
	key = fw.AppendRootPathRedis(key)
	ctx, cancel := context.WithTimeout(context.Background(), redisCtxTimeo)
	defer cancel()
	err := fw.redisCtl.client.HSet(ctx, key, field, value).Err()
	return fw.logRedisError(err, "failed write redis hash data: %s, %s", key, field)
}

// WriteHashRedis 向redis写hashmap数据
func (fw *WMFrameWorkV2) WriteHashRedis(key string, hashes map[string]string) error {
	if !fw.redisCtl.ready {
		return errRedisNotReady
	}
	if len(hashes) == 0 {
		return nil
	}

	args := make([]string, len(hashes)*2)
	var idx = 0
	for f, v := range hashes {
		args[idx] = f
		args[idx+1] = v
		idx += 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), redisCtxTimeo)
	defer cancel()
	var err error
	if fw.redisCtl.mianver < 4 {
		err = fw.redisCtl.client.HMSet(ctx, fw.AppendRootPathRedis(key), args).Err()
	} else {
		err = fw.redisCtl.client.HSet(ctx, fw.AppendRootPathRedis(key), args).Err()
	}
	return fw.logRedisError(err, "failed write redis hash data: %s", key)
}

// DelHashRedis 删redis hash key
func (fw *WMFrameWorkV2) DelHashRedis(key string, fields ...string) error {
	if !fw.redisCtl.ready {
		return errRedisNotReady
	}
	if len(fields) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisCtxTimeo)
	defer cancel()
	fw.WriteInfo("REDIS", fmt.Sprintf("del redis hash data:%+v", key))
	err := fw.redisCtl.client.HDel(ctx, fw.AppendRootPathRedis(key), fields...).Err()
	return fw.logRedisError(err, "failed del redis hash data: %s, %+v", key, fields)
}

// ReadAllRedisKeys 模糊读取所有匹配的key
func (fw *WMFrameWorkV2) ReadAllRedisKeys(key string) []string {
	if !fw.redisCtl.ready {
		return []string{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), redisCtxTimeo)
	defer cancel()
	val := fw.redisCtl.client.Keys(ctx, fw.AppendRootPathRedis(key))
	fw.logRedisDebug(val.Err(), "failed read redis keys: %s", key)
	return val.Val()
}

// ReadAllRedis 模糊读redis
func (fw *WMFrameWorkV2) ReadAllRedis(key string) (map[string]string, error) {
	var s = make(map[string]string, 0)
	if !fw.redisCtl.ready {
		return s, errRedisNotReady
	}
	key = fw.AppendRootPathRedis(key)
	ctx, cancel := context.WithTimeout(context.Background(), redisCtxTimeo)
	defer cancel()
	val := fw.redisCtl.client.Keys(ctx, key)
	if val.Err() != nil {
		fw.logRedisDebug(val.Err(), "failed read all redis data: %s", key)
		if strings.Contains(val.Err().Error(), " nil") {
			return s, errRedisNil
		}
		return s, val.Err()
	}
	for _, v := range val.Val() {
		ctx, cancel := context.WithTimeout(context.Background(), redisCtxTimeo)
		defer cancel()
		vv := fw.redisCtl.client.Get(ctx, v)
		if vv.Err() == nil {
			s[v] = vv.Val()
		}
	}
	return s, nil
}

// ReadRedisPTTL 读取key的有效期，返回时间
func (fw *WMFrameWorkV2) ReadRedisPTTL(key string) time.Duration {
	if !fw.redisCtl.ready {
		return 0
	}
	key = fw.AppendRootPathRedis(key)
	ctx, cancel := context.WithTimeout(context.Background(), redisCtxTimeo)
	defer cancel()
	val := fw.redisCtl.client.PTTL(ctx, key)
	if val.Err() != nil {
		fw.logRedisDebug(val.Err(), "failed read redis pttl: %s", key)
		return 0
	}
	return val.Val()
}

// RedisClient 返回redis客户端
func (fw *WMFrameWorkV2) RedisClient() *redis.Client {
	return fw.redisCtl.client
}

// RedisIsReady 返回redis可用状态
func (fw *WMFrameWorkV2) RedisIsReady() bool {
	return fw.redisCtl.ready
}

// ViewRedisConfig 查看redis配置,返回json字符串
func (fw *WMFrameWorkV2) ViewRedisConfig() string {
	return fw.redisCtl.forshow
}

// ExpireUserToken 更新token有效期
func (fw *WMFrameWorkV2) ExpireUserToken(token string) {
	// 更新redis的对应键值的有效期
	fw.ExpireRedis("usermanager/legal/"+MD5Worker.Hash(gopsu.Bytes(token)), fw.tokenLife)
}

func (fw *WMFrameWorkV2) logRedisDebug(err error, formatstr string, params ...interface{}) error {
	if err == nil {
		return nil
	}
	// if strings.Contains(err.Error(), "connect: connection refused") {
	// fw.redisCtl.ready = false
	// }
	params = append(params, err.Error())
	fw.WriteDebug("REDIS", fmt.Sprintf(formatstr+"| %s", params...))
	return err
}
func (fw *WMFrameWorkV2) logRedisError(err error, formatstr string, params ...interface{}) error {
	if err == nil {
		return nil
	}
	// if strings.Contains(err.Error(), "connect: connection refused") {
	// fw.redisCtl.ready = false
	// }
	if strings.Contains(err.Error(), "dial tcp") {
		fw.redisCtl.ready = false
	}
	params = append(params, err.Error())
	fw.WriteError("REDIS", fmt.Sprintf(formatstr+"| %s", params...))
	return err
}
func (fw *WMFrameWorkV2) tryRedisVer() {
	if !fw.redisCtl.ready || fw.redisCtl.mianver > 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), redisCtxTimeo)
	defer cancel()
	a, err := fw.redisCtl.client.Info(ctx, "Server").Result()
	if err == nil {
		for _, v := range strings.Split(a, "\r\n") {
			if strings.HasPrefix(v, "redis_version:") {
				fw.redisCtl.mianver = gopsu.String2Int(strings.Split(strings.Split(v, ":")[1], ".")[0], 10)
				break
			}
		}
	}
	fw.logRedisError(err, "failed get server info %s", fw.redisCtl.addr)
}
