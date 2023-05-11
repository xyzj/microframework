package wmfw

import (
	"fmt"
	"strings"
	"time"

	mariadb "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func (fw *WMFrameWorkV2) formatDSN() []string {
	var connstr = make([]string, 0)
	for _, v := range strings.Split(fw.dbCtl.databases, ",") {
		if v == "" {
			continue
		}
		switch fw.dbCtl.driver {
		case "mssql":
			s := fmt.Sprintf("user id=%s;"+
				"password=%s;"+
				"server=%s;"+
				"database=%s;"+
				"connection timeout=10",
				fw.dbCtl.user, fw.dbCtl.pwd, fw.dbCtl.addr, v)
			if fw.dbCtl.tls != "false" {
				s += ";encrypt=true;trustservercertificate=true"
			}
			connstr = append(connstr, s)
		case "mysql":
			sqlcfg := &mariadb.Config{
				Collation:            "utf8_general_ci",
				Loc:                  time.UTC,
				MaxAllowedPacket:     4 << 20,
				AllowNativePasswords: true,
				CheckConnLiveness:    true,
				Net:                  "tcp",
				Addr:                 fw.dbCtl.addr,
				User:                 fw.dbCtl.user,
				Passwd:               fw.dbCtl.pwd,
				DBName:               v,
				MultiStatements:      true,
				ParseTime:            true,
				Timeout:              time.Minute,
				ColumnsWithAlias:     false,
				ClientFoundRows:      true,
				InterpolateParams:    true,
				TLSConfig:            fw.dbCtl.tls,
			}
			connstr = append(connstr, sqlcfg.FormatDSN())
		}
	}
	return connstr
}
func (fw *WMFrameWorkV2) newORMEngines() {
	connstrs := fw.formatDSN()
	for _, v := range connstrs {
		if eng, err := gorm.Open(mysql.Open(v), &gorm.Config{}); err == nil {
			fw.dborms = append(fw.dborms, eng)
		} else {
			fw.WriteError("ORM", "Start ORM engine error: "+err.Error())
		}
	}
	fw.WriteSystem("ORM", fmt.Sprintf("Success start %d ORM engines", len(fw.dborms)))
}

// ORMEngine 返回ORM引擎，可指定引擎序号
func (fw *WMFrameWorkV2) ORMEngine(id ...int) *gorm.DB {
	if len(fw.dborms) == 0 {
		return nil
	}
	if len(id) == 0 {
		return fw.dborms[0]
	}
	if id[0] >= len(fw.dborms) {
		return fw.dborms[0]
	}
	return fw.dborms[id[0]]
}
