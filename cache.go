package wmfw

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xyzj/gopsu"
	json "github.com/xyzj/gopsu/json"
)

// CacheType 缓存类型
type CacheType int

const (
	// CacheTypeMEM 内存缓存
	CacheTypeMEM CacheType = iota
	// CacheTypeRedis redis 缓存
	CacheTypeRedis
	// CacheTypeFile 文件缓存
	CacheTypeFile
)

var cacheExpire = time.Minute * 30

// WriteCacheMEM gin框架写内存缓存
func (fw *WMFrameWorkV2) WriteCacheMEM(c *gin.Context, data interface{}) {
	cachetag := fw.cacheHead + MD5Worker.Hash(gopsu.Bytes(c.Param("_userTokenName")+c.Request.RequestURI+c.Param("_body")))
	if err := fw.WriteCache(CacheTypeMEM, cachetag, data); err != nil {
		return
	}
	c.Set("cachetag", cachetag)
}

// ReadCacheMEM 读取内存缓存
func (fw *WMFrameWorkV2) ReadCacheMEM(cachetag string, out interface{}) error {
	return fw.ReadCache(CacheTypeMEM, cachetag, out)
}

// WriteCache 写入缓存
//
//	data: 缓存数据
//
// return: cachetag，error
func (fw *WMFrameWorkV2) WriteCache(t CacheType, cachetag string, data interface{}) error {
	if data == nil {
		return fmt.Errorf("no data can be cached")
	}
	if cachetag == "" {
		cachetag = fw.cacheHead + MD5Worker.Hash([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	}
	go func(t CacheType, data interface{}, cachetag string) {
		lo := &sync.WaitGroup{}
		lo.Add(1)
		fw.cacheLocker.Store(cachetag, lo)
		defer func() {
			lo.Done()
			fw.cacheLocker.Delete(cachetag)
		}()
		b, err := json.Marshal(data)
		if err != nil {
			fw.WriteError("CACHE", "encode cache data error: "+err.Error())
			return
		}
		switch t {
		case CacheTypeRedis:
			err = fw.WriteRedis(fw.serverName+"/datacache/"+cachetag, CodeGzip.Compress(b), cacheExpire)
		case CacheTypeFile:
			err = os.WriteFile(filepath.Join(gopsu.DefaultCacheDir, cachetag), CodeGzip.Compress(b), 0664)
		case CacheTypeMEM:
			fw.cacheMem.Set(cachetag, CodeGzip.Compress(b), cacheExpire)
		}
		if err != nil {
			fw.WriteError("CACHE", "encode cache data error: "+err.Error())
		}
	}(t, data, cachetag)
	return nil
}

// ReadCache 读取缓存
//
//	t: 缓存类型
//	cachetag: 缓存标签
//
// return: 错误
func (fw *WMFrameWorkV2) ReadCache(t CacheType, cachetag string, out interface{}) error {
	b, err := fw.readCacheData(t, cachetag)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

// ReadCacheToString 读取缓存，返回字符串
func (fw *WMFrameWorkV2) ReadCacheToString(t CacheType, cachetag string) string {
	b, err := fw.readCacheData(t, cachetag)
	if err != nil {
		return ""
	}
	return gopsu.String(b)
}

func (fw *WMFrameWorkV2) readCacheData(t CacheType, cachetag string) ([]byte, error) {
	if cachetag == "" {
		return nil, fmt.Errorf("cachetag is empty")
	}
	if a, ok := fw.cacheLocker.Load(cachetag); ok { // 检查写入锁
		a.(*sync.WaitGroup).Wait()
	}
	var err error
	var s string
	var b []byte
	switch t {
	case CacheTypeRedis:
		s, err = fw.ReadRedis(fw.serverName + "/datacache/" + cachetag)
		b = []byte(s)
	case CacheTypeFile:
		b, err = os.ReadFile(filepath.Join(gopsu.DefaultCacheDir, cachetag))
	case CacheTypeMEM:
		if bb, ok := fw.cacheMem.GetAndExpire(cachetag, cacheExpire); ok {
			b = bb.([]byte)
		} else {
			err = fmt.Errorf("cache not found")
		}
	default:
		return nil, fmt.Errorf("unsupported cache type")
	}
	if err != nil {
		return nil, err
	}
	return CodeGzip.Uncompress(b), nil
}

func (fw *WMFrameWorkV2) clearCache() {
	files, err := os.ReadDir(gopsu.DefaultCacheDir)
	if err != nil {
		return
	}
	tt := time.Now()
	for _, d := range files {
		if d.IsDir() {
			continue
		}
		file, err := d.Info()
		if err != nil {
			continue
		}
		switch fw.cacheHead {
		case "8fe5c971": // backend 检查整个文件夹
			if !strings.HasPrefix(file.Name(), "_") && tt.Sub(file.ModTime()).Hours() > 24 { // 除保护文件以外的其他文件，超过24小时，删
				os.Remove(filepath.Join(gopsu.DefaultCacheDir, file.Name()))
			}
		default: //  其他子系统，只检查自己的缓存文件
			if !strings.HasPrefix(file.Name(), fw.cacheHead) {
				continue
			}
			// 删除文件
			if file.ModTime().Add(cacheExpire).Before(tt) {
				os.Remove(filepath.Join(gopsu.DefaultCacheDir, file.Name()))
			}
		}
	}
}
