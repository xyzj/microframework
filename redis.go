package wmfw

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/xyzj/gopsu"
	"github.com/xyzj/gopsu/config"
	"github.com/xyzj/gopsu/loopfunc"
)

var (
	redisCtxTimeo = 3 * time.Second
)

// redis配置
type redisConfigure struct {
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
	mainver int
	ready   bool
}

// NewRedisClient 新的redis client
func (fw *WMFrameWorkV2) newRedisClient() {
	if fw.redisCtl.ready {
		fw.tryRedisVer()
		return
	}
	fw.redisCtl.addr = fw.baseConf.GetDefault(&config.Item{Key: "redis_addr", Value: "127.0.0.1:6379", Comment: "redis服务地址,ip:port格式"}).String()
	fw.redisCtl.pwd = fw.baseConf.GetDefault(&config.Item{Key: "redis_pwd", Value: "WcELCNqP5dCpvMmMbKDdvgb", Comment: "redis连接密码"}).TryDecode()
	fw.redisCtl.database = int(fw.baseConf.GetDefault(&config.Item{Key: "redis_db", Value: "0", Comment: "redis数据库id"}).TryInt64())
	fw.redisCtl.enable = !*disableRedis // fw.baseConf.GetDefault(&config.Item{Key: "redis_enable", Value: "true", Comment: "是否启用redis"}).TryBool()

	if !fw.redisCtl.enable {
		return
	}
	fConn := func() {
		fw.redisCtl.client = redis.NewClient(&redis.Options{
			Addr:            fw.redisCtl.addr,
			Password:        fw.redisCtl.pwd,
			DB:              fw.redisCtl.database,
			PoolFIFO:        true,
			MinIdleConns:    3,
			ConnMaxIdleTime: time.Minute,
			// OnConnect: func(ctx context.Context, cn *redis.Conn) error {
			// 	if !fw.redisCtl.ready {
			// 		fw.redisCtl.ready = true
			// 		fw.WriteSystem("REDIS", fmt.Sprintf("Connect to server %s is ready", fw.redisCtl.addr))
			// 	}
			// 	return nil
			// },
		})
		if err := fw.tryRedisVer(); err == nil {
			fw.WriteSystem("REDIS", fmt.Sprintf("Connect to server %s is ready, use db %d", fw.redisCtl.addr, fw.redisCtl.database))
			fw.redisCtl.ready = true
		}
	}
	fConn()
	go loopfunc.LoopFunc(func(params ...interface{}) {
		for {
			time.Sleep(time.Second * 10)
			fw.WriteRedis(fw.serverName+"_write_test", "", time.Second)
			if !fw.redisCtl.ready {
				fw.WriteError("REDIS", "connection maybe lost")
				fConn()
			}
		}
	}, "redis check", fw.LogDefaultWriter(), nil)
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
	if fw.redisCtl.mainver < 4 {
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
		fw.redisCtl.mainver = 0
		fw.redisCtl.ready = false
	}
	params = append(params, err.Error())
	fw.WriteError("REDIS", fmt.Sprintf(formatstr+"| %s", params...))
	return err
}
func (fw *WMFrameWorkV2) tryRedisVer() error {
	if fw.redisCtl.mainver > 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), redisCtxTimeo)
	defer cancel()
	a, err := fw.redisCtl.client.Info(ctx, "Server").Result()
	if err == nil {
	LOOP:
		for _, v := range strings.Split(a, "\r\n") {
			switch {
			case strings.HasPrefix(v, "redis_version:"):
				fw.redisCtl.mainver = gopsu.String2Int(strings.Split(strings.Split(v, ":")[1], ".")[0], 10)
				break LOOP
			case strings.HasPrefix(v, "godis_version:"):
				fw.redisCtl.mainver = 4
				break LOOP
			}
		}
	}
	return fw.logRedisError(err, "failed get server info %s", fw.redisCtl.addr)
}

// DelHashRedisByFieldPrefix 删redis hash key 模糊前缀
func (fw *WMFrameWorkV2) DelHashRedisByFieldPrefix(key string, fieldPrefix string) error {
	if !fw.redisCtl.ready {
		return errRedisNotReady
	}
	if len(fieldPrefix) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisCtxTimeo)
	defer cancel()
	// 获取哈希表的所有字段
	fields, err := fw.redisCtl.client.HKeys(ctx, fw.AppendRootPathRedis(key)).Result()
	if err != nil {
		return fw.logRedisError(err, "failed to get hash fields: %s", key)
	}
	// 遍历字段，检查并删除匹配前缀的字段
	for _, field := range fields {
		if strings.HasPrefix(field, fieldPrefix) {
			err := fw.DelHashRedis(key, field)
			if err != nil {
				return fw.logRedisError(err, "failed to del hash fields: %s", key)
			}
		}
	}
	return fw.logRedisError(err, "failed del redis hash data: %s, %+v", key, fields)
}
