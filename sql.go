package wmfw

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/tidwall/sjson"
	"github.com/xyzj/gopsu"
	"github.com/xyzj/gopsu/db"
	"github.com/xyzj/gopsu/pathtool"
)

// 数据库配置
type dbConfigure struct {
	forshow string
	// 数据库地址
	addr string
	// 登录用户名
	user string
	// 登录密码
	pwd string
	// 数据库名称
	database  string
	databases string
	// 数据库驱动模式，mssql/mysql
	driver string
	// 是否启用数据库
	enable bool
	// 使用mrg_myisam引擎的总表名称
	mrgTables []string
	// mrg_myisam引擎最大分表数量
	mrgMaxSubTables int
	// mrg_myisam分表大小（MB），默认1800
	mrgSubTableSize int64
	// mrg_myisam分表行数，默认4800000
	mrgSubTableRows int64
	// client
	client *db.SQLPool
	// tls 是否启用ssl连接
	tls string
}

var (
	// 检查升级文件
	upsql = pathtool.JoinPathFromHere(pathtool.GetExecName() + ".dbupg")
)

func (conf *dbConfigure) show() string {
	conf.forshow, _ = sjson.Set("", "addr", conf.addr)
	conf.forshow, _ = sjson.Set(conf.forshow, "user", CWorker.Encrypt(conf.user))
	conf.forshow, _ = sjson.Set(conf.forshow, "pwd", CWorker.Encrypt(conf.pwd))
	conf.forshow, _ = sjson.Set(conf.forshow, "dbname", conf.database)
	conf.forshow, _ = sjson.Set(conf.forshow, "driver", conf.driver)
	conf.forshow, _ = sjson.Set(conf.forshow, "enable", conf.enable)
	return conf.forshow
}

// Newfw.dbCtl.client mariadb client
func (fw *WMFrameWorkV2) newDBClient(dbinit, dbupgrade string) bool {
	fw.dbCtl.addr = fw.wmConf.GetItemDefault("db_addr", "127.0.0.1:3306", "sql服务地址,ip[:port[/instance]]格式")
	fw.dbCtl.user = fw.wmConf.GetItemDefault("db_user", "root", "sql用户名")
	fw.dbCtl.pwd = gopsu.DecodeString(fw.wmConf.GetItemDefault("db_pwd", "SsWAbSy8H1EOP3n5LdUQqls", "sql密码"))
	fw.dbCtl.driver = fw.wmConf.GetItemDefault("db_drive", "mysql", "sql数据库驱动，mysql 或 mssql")
	fw.dbCtl.enable, _ = strconv.ParseBool(fw.wmConf.GetItemDefault("db_enable", "true", "是否启用sql"))
	fw.dbCtl.tls = fw.wmConf.GetItemDefault("db_tls", "false", "是否启用ssl加密, false or skip-verify")
	fw.dbCtl.databases = fw.wmConf.GetItemDefault("db_name", "", "sql数据库名称，多个库用，仅支持orm`,`分割")
	cdb := fw.wmConf.GetItemDefault("db_name_"+strings.ReplaceAll(fw.serverName, "-"+*nameTail, ""), "", "sql数据库名称，高优先级，用于多个服务共用一个配置时区分指定数据库名称，当设置时，优先级高于db_name")
	if cdb != "" {
		fw.dbCtl.databases = cdb
	}

	if fw.dbCtl.databases == "" {
		fw.dbCtl.databases = "v5db_" + fw.serverName
		fw.wmConf.UpdateItem("db_name", fw.dbCtl.databases)
		fw.wmConf.Save()
	}
	// 兼容orm，常规连接仅采用第一个数据库
	if strings.Contains(fw.dbCtl.databases, ",") {
		fw.dbCtl.database = strings.Split(fw.dbCtl.databases, ",")[0]
	} else {
		fw.dbCtl.database = fw.dbCtl.databases
	}
	dbcache := true //, _ := strconv.ParseBool(fw.wmConf.GetItemDefault("db_cache", "true", "是否启用结果集缓存"))
	fw.dbCtl.show()
	if !fw.dbCtl.enable {
		return false
	}
	var dbname = fw.dbCtl.database
DBCONN:
	fw.dbCtl.client = &db.SQLPool{
		User:         fw.dbCtl.user,
		Server:       fw.dbCtl.addr,
		Passwd:       fw.dbCtl.pwd,
		DataBase:     fw.dbCtl.database,
		EnableCache:  dbcache,
		MaxOpenConns: 200,
		CacheDir:     gopsu.DefaultCacheDir,
		Timeout:      120,
		Logger:       fw.wmLog,
		// Logger: &StdLogger{
		// 	Name:        "DB",
		// 	LogReplacer: strings.NewReplacer("[", "", "]", ""),
		// 	LogWriter:   fw.coreWriter,
		// },
	}
	switch fw.dbCtl.driver {
	case "mssql":
		fw.dbCtl.client.DriverType = db.DriverMSSQL
	default:
		fw.dbCtl.client.DriverType = db.DriverMYSQL
	}
	err := fw.dbCtl.client.New(fw.dbCtl.tls)
	if err != nil {
		if strings.Contains(err.Error(), "Unknown database") {
			fw.dbCtl.database = ""
			fw.WriteError("DB", err.Error()+" Try to create one...")
			goto DBCONN
		}
		fw.dbCtl.enable = false
		fw.WriteError("DB", "Failed connect to server "+fw.dbCtl.addr+"|"+err.Error())
		return false
	}
	if fw.dbCtl.database == "" && dbname != "" {
		if _, _, err := fw.dbCtl.client.Exec("CREATE DATABASE IF NOT EXISTS `" + dbname + "`;USE `" + dbname + "`;"); err != nil {
			fw.dbCtl.enable = false
			fw.WriteError("DB", "Create Database error: "+fw.dbCtl.addr+"|"+err.Error())
			return false
		}
		fw.WriteError("DB", "Create Database on "+fw.dbCtl.addr)
		os.Remove(upsql)
		if len(dbinit) > 0 {
			if _, _, err := fw.dbCtl.client.Exec(dbinit); err != nil {
				fw.dbCtl.enable = false
				fw.WriteError("DB", "Create Tables error: "+fw.dbCtl.addr+"|"+err.Error())
				return false
			}
			fw.WriteError("DB", "Create Tables on "+fw.dbCtl.addr)
		}
		fw.dbCtl.database = dbname
		goto DBCONN
	}
	fw.dbUpgrade(dbupgrade)
	return true
}

// MaintainMrgTables 维护mrg引擎表
func (fw *WMFrameWorkV2) MaintainMrgTables() {
	// 延迟一下，确保sql已连接
	time.Sleep(time.Minute)
	if !fw.dbCtl.enable {
		return
	}

MAINTAIN:
	func() {
		defer func() {
			if err := recover(); err != nil {
				fw.WriteError("DB", err.(error).Error())
			}
		}()
		t := time.NewTicker(time.Hour)
		for range t.C {
			if time.Now().Hour() != 13 {
				continue
			}
			// 重新刷新配置
			fw.dbCtl.mrgTables = strings.Split(fw.wmConf.GetItemDefault("db_mrg_tables", "", "使用mrg_myisam引擎分表的总表名称，用`,`分割多个总表"), ",")
			fw.dbCtl.mrgMaxSubTables = gopsu.String2Int(fw.wmConf.GetItemDefault("db_mrg_maxsubtables", "10", "分表子表数量，最小为1"), 10)
			fw.dbCtl.mrgSubTableSize = gopsu.String2Int64(fw.wmConf.GetItemDefault("db_mrg_subtablesize", "1800", "子表最大磁盘空间容量（MB），当超过该值时，进行分表操作,推荐默认值1800"), 10)
			if fw.dbCtl.mrgSubTableSize < 1 {
				fw.dbCtl.mrgSubTableSize = 10
			}
			fw.dbCtl.mrgSubTableRows = gopsu.String2Int64(fw.wmConf.GetItemDefault("db_mrg_subtablerows", "4500000", "子表最大行数，当超过该值时，进行分表操作，推荐默认值4500000"), 10)
			fw.wmConf.Save()
			for _, v := range fw.dbCtl.mrgTables {
				tableName := strings.TrimSpace(v)
				if tableName == "" {
					continue
				}
				_, _, size, rows, err := fw.dbCtl.client.ShowTableInfo(tableName)
				if err != nil {
					fw.WriteError("DB", "SHOW table "+tableName+" "+err.Error())
					continue
				}
				if size >= fw.dbCtl.mrgSubTableSize || rows >= fw.dbCtl.mrgSubTableRows {
					err = fw.dbCtl.client.MergeTable(tableName, fw.dbCtl.mrgMaxSubTables)
					if err != nil {
						fw.WriteError("DB", "MRG table "+tableName+" "+err.Error())
						continue
					}
				}
			}
		}
	}()
	time.Sleep(time.Minute)
	goto MAINTAIN
}

// MysqlIsReady 返回mysql可用状态
func (fw *WMFrameWorkV2) MysqlIsReady() bool {
	return fw.dbCtl.enable
}

// ViewSQLConfig 查看sql配置,返回json字符串
func (fw *WMFrameWorkV2) ViewSQLConfig() string {
	return fw.dbCtl.forshow
}

// DBUpgrade 检查是否需要升级数据库
//
//	返回是否执行过升级，true-执行了升级，false-不需要升级
func (fw *WMFrameWorkV2) dbUpgrade(sql string) bool {
	if !fw.dbCtl.enable || sql == "" {
		return false
	}
	// 校验升级脚本
	b, _ := os.ReadFile(upsql)
	if gopsu.String(b) == gopsu.GetMD5(sql) { // 升级脚本已执行过，不再重复升级
		return false
	}
	// 执行升级脚本
	var err error
	fw.WriteInfo("DBUP", "Try to update database")
	var sep = ";"
	if strings.Contains(sql, ";|") {
		sep = ";|"
	}
	for _, v := range strings.Split(sql, sep) {
		s := strings.TrimSpace(v)
		if s == "" || strings.HasPrefix(s, "--") {
			continue
		}
		if _, _, err = fw.dbCtl.client.Exec(s + ";"); err != nil {
			if strings.Contains(err.Error(), "Duplicate") ||
				strings.Contains(err.Error(), "Multiple") {
				continue
			}
			fw.WriteError("DBUP", s+" | "+err.Error())
		}
	}
	// 标记脚本，下次启动不再重复升级
	err = os.WriteFile(upsql, gopsu.Bytes(gopsu.GetMD5(sql)), 0664)
	if err != nil {
		fw.WriteError("DBUP", "mark database update error: "+err.Error())
	}
	return true
}
