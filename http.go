package wmfw

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"html/template"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	gingzip "github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"github.com/xyzj/gopsu"
	"github.com/xyzj/gopsu/db"
	"github.com/xyzj/gopsu/excel"
	"github.com/xyzj/gopsu/games"
	ginmiddleware "github.com/xyzj/gopsu/gin-middleware"
	"github.com/xyzj/gopsu/gocmd"
	"github.com/xyzj/gopsu/json"
	"github.com/xyzj/gopsu/loopfunc"
	"github.com/xyzj/gopsu/pathtool"
	yaaggin "github.com/xyzj/yaag/gin"
	"github.com/xyzj/yaag/yaag"
)

const (
	// TPLHEAD html模板头
	TPLHEAD = `<html lang="zh-cn">
<head>
<meta content="text/html; charset=utf-8" http-equiv="content-type" />
{{template "css"}}
</head>
{{template "body" .}}
</html>`
	// TPLCSS html模板css
	TPLCSS = `{{define "css"}}
<style type="text/css">
a {
  color: #4183C4;
  font-size: 16px; }
h1, h2, h3, h4, h5, h6 {
  margin: 20px 0 10px;
  padding: 0;
  font-weight: bold;
  -webkit-font-smoothing: antialiased;
  cursor: text;
  position: relative; }
h1 {
  font-size: 28px;
  color: black; }
h2 {
  font-size: 24px;
  border-bottom: 1px solid #cccccc;
  color: black; }
h3 {
  font-size: 18px; }
h4 {
  font-size: 16px; }
h5 {
  font-size: 14px; }
h6 {
  color: #777777;
  font-size: 14px; }
table {
  padding: 0; }
	table tr {
	  border-top: 1px solid #cccccc;
	  background-color: white;
	  margin: 0;
	  padding: 0; }
	  table tr:nth-child(2n) {
		background-color: #f8f8f8; }
	  table tr th {
		font-weight: bold;
		border: 1px solid #cccccc;
		text-align: center;
		margin: 0;
		padding: 6px 13px; }
	  table tr td {
		border: 1px solid #cccccc;
		text-align: left;
		margin: 0;
		padding: 6px 13px; }
	  table tr th :first-child, table tr td :first-child {
		margin-top: 0; }
	  table tr th :last-child, table tr td :last-child {
		margin-bottom: 0; }
</style>
{{end}}`
	// TPLBODY html模板body
	TPLBODY = `{{define "body"}}
<body>
<h3>服务器系统时间：</h3><a>{{.timer}}</a>
<h3>服务启动时间：</h3><a>{{.upTime}}</a>
<h3>{{.key}}：</h3><a>{{range $idx, $elem := .value}}
{{$elem}}<br>
{{end}}</a>
</body>
</html>
{{end}}`
)

// //go:embed yaag
// var apirec embed.FS

var (
	apidocPath   = "docs/apidoc.html"
	yaagConfig   *yaag.Config
	rever        = strings.NewReplacer("{\n", "", "}", "", `"`, "", ",", "")
	vipUsers     = []string{"root", "sephiroth"}
	allowSchemas = []string{"http://", "https://", "ws://", "wss://"}
)

func apidoc(c *gin.Context) {
	switch c.Param("switch") {
	case "on":
		yaagConfig.On = true
		c.String(200, "API record is set to on.")
	case "off":
		yaagConfig.On = false
		c.String(200, "API record is set to off.")
	case "reset":
		yaagConfig.ResetDoc()
		c.String(200, "API record reset done.")
	default:
		p := pathtool.JoinPathFromHere("docs", "apirecord-"+c.Param("switch")+".html")
		if pathtool.IsExist(p) {
			b, _ := os.ReadFile(p)
			c.Header("Content-Type", "text/html")
			c.Status(http.StatusOK)
			c.Writer.Write(b)
		} else {
			c.String(200, "The API record file was not found, you may not have the API record function turned on.")
		}
	}
}

// NewHTTPEngineWithYaagSkip 创建gin引擎 并设置yaag忽略路由
func (fw *WMFrameWorkV2) NewHTTPEngineWithYaagSkip(skip []string, f ...gin.HandlerFunc) *gin.Engine {
	if !*debug {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.ForwardedByClientIP = true
	// 404,405
	r.HandleMethodNotAllowed = true
	r.NoMethod(ginmiddleware.Page405)
	r.NoRoute(ginmiddleware.Page404Rand)
	// 中间件
	//cors
	corsopt := cors.Config{
		MaxAge:           time.Hour * 24,
		AllowAllOrigins:  true,
		AllowCredentials: true,
		AllowWildcard:    true,
		AllowWebSockets:  true,
		AllowMethods:     []string{"*"},
		AllowHeaders:     []string{"*"},
	}
	if origins := fw.ReadConfigItem("origin", "", "允许的域，多个域名使用`,`隔开"); origins != "" {
		corsopt.AllowAllOrigins = false
		for _, v := range strings.Split(origins, ",") {
			if len(gopsu.SlicesIntersect([]string{v}, allowSchemas)) > 0 {
				corsopt.AllowOrigins = append(corsopt.AllowOrigins, v)
				continue
			}
			for _, vv := range allowSchemas {
				corsopt.AllowOrigins = append(corsopt.AllowOrigins, vv+v)
			}
		}
	}
	r.Use(cors.New(corsopt))
	// r.Use(ginmiddleware.CFConnectingIP())
	// 数据压缩
	r.Use(gingzip.Gzip(9))
	// 日志
	r.Use(ginmiddleware.LogToWriter(fw.httpWriter, "/favicon.ico", "/downloadLog", "/showroutes", "/config/view", "/sha256", "/yaag"))
	// logName := ""
	// if *logLevel > 1 {
	// 	logName = fw.loggerMark + ".http"
	// }
	// r.Use(ginmiddleware.LoggerWithRolling(gopsu.DefaultLogDir, logName, *logDays))
	// 错误恢复
	r.Use(ginmiddleware.Recovery())
	// 黑名单
	r.Use(ginmiddleware.Blacklist(""))
	// 其他中间件
	if f != nil {
		r.Use(f...)
	}
	r.GET("/favicon.ico", func(c *gin.Context) {
		c.Header("Cache-Control", "private, max-age=86400")
		c.Writer.Write(favicon)
	})
	// 基础路由
	r.GET("/whoami", func(c *gin.Context) {
		c.String(200, c.ClientIP())
	})
	r.GET("/name", func(c *gin.Context) {
		c.String(200, fw.serverName)
	})
	r.GET("/sha256", func(c *gin.Context) {
		b, err := os.ReadFile(os.Args[0])
		if err != nil {
			c.String(400, err.Error())
			return
		}
		c.String(200, SHA256Worker.Hash(b))
	})
	r.GET("/health", ginmiddleware.PageDefault)
	r.GET("/health/mod", fw.pageModCheck)
	r.POST("/health/mod", fw.pageModCheck)
	r.GET("/clearlog", fw.CheckRequired("name"), ginmiddleware.Clearlog)
	r.GET("/uptime", fw.pageStatus)
	r.POST("/uptime", fw.pageStatus)
	r.StaticFS("/downloadLog", http.Dir(gopsu.DefaultLogDir))
	r.GET("/config/view", ginmiddleware.BasicAuth(), func(c *gin.Context) {
		configInfo := make(map[string]interface{})
		configInfo["upTime"] = fw.upTime
		configInfo["timer"] = time.Now().Format("2006-01-02 15:04:05 Mon")
		configInfo["key"] = "服务配置信息"
		configInfo["value"] = fw.wmConf.Print()
		c.Header("Content-Type", "text/html")
		t, _ := template.New("viewconfig").Parse(TPLHEAD + TPLCSS + TPLBODY)
		h := render.HTML{
			Name:     "viewconfig",
			Data:     configInfo,
			Template: t,
		}
		h.WriteContentType(c.Writer)
		h.Render(c.Writer)
	})
	r.GET("/crash/:do", func(c *gin.Context) {
		p := pathtool.JoinPathFromHere(pathtool.GetExecNameWithoutExt() + ".crash.log")
		if !pathtool.IsExist(p) {
			c.String(200, "seems good")
			return
		}
		switch c.Param("do") {
		case "view":
			b, err := os.ReadFile(p)
			if err != nil {
				c.String(200, "read crash file error: "+err.Error())
				return
			}
			c.String(200, string(b))
		case "download":
			c.FileAttachment(p, pathtool.GetExecNameWithoutExt()+".crash.log")
		default:
			ginmiddleware.Page404Big(c)
		}
	})

	// 静态资源路由
	for k, v := range dirs {
		if strings.Contains(v, ":") {
			r.Static("/"+strings.Split(v, ":")[0], strings.Split(v, ":")[1])
			println(fmt.Sprintf("dir %d. /%s/", k+1, strings.Split(v, ":")[0]))
			skip = append(skip, "/"+strings.Split(v, ":")[0]+"/")
		}
	}
	// 轻松一下
	r.GET("/xgame/:game", games.GameGroup)
	r.GET("/devquotes", ginmiddleware.PageDev)
	// apirecord
	// r.StaticFS("/apirec", http.FS(apirec))
	r.GET("/apirecord/:switch", apidoc)
	// 生成api访问文档
	apidocPath = pathtool.JoinPathFromHere("docs", "apirecord-"+fw.serverName+".html")
	os.MkdirAll(pathtool.JoinPathFromHere("docs"), 0755)
	yaagConfig = &yaag.Config{
		On:       false,
		DocTitle: "API Record for " + fw.serverAlias,
		DocPath:  apidocPath,
		BaseUrls: map[string]string{
			"Server Name": fw.serverName,
			"Core Author": "X.Yuan",
			// "Last Update": time.Now().Format(gopsu.LongTimeFormat),
		},
	}
	yaag.Init(yaagConfig)
	r.Use(yaaggin.Document(skip...))

	return r
}

// NewHTTPEngine 创建gin引擎
func (fw *WMFrameWorkV2) NewHTTPEngine(f ...gin.HandlerFunc) *gin.Engine {
	return fw.NewHTTPEngineWithYaagSkip([]string{}, f...)
}

// NewHTTPService 启动HTTP服务
func (fw *WMFrameWorkV2) newHTTPService(r *gin.Engine) {
	var sss string
	var findRoot bool
	// var findXRoot bool
	for _, v := range r.Routes() {
		// if v.Path == "/xroot" {
		// 	findXRoot = true
		// 	continue
		// }
		if v.Path == "/" {
			findRoot = true
			continue
		}
		if v.Method == "HEAD" ||
			strings.HasSuffix(v.Path, "*filepath") ||
			strings.HasPrefix(v.Path, "/rd") ||
			strings.HasPrefix(v.Path, "/plain") {
			continue
		}
		if strings.ContainsAny(v.Path, "*") && !strings.HasSuffix(v.Path, "filepath") {
			continue
		}
		sss += fmt.Sprintf(`<a>%s: %s</a><br><br>`, v.Method, v.Path)
	}
	// if !findXRoot {
	// 	r.GET("/xroot", ginmiddleware.PageDefault)
	// }
	if !findRoot {
		if *appcompatible {
			r.GET("/", ginmiddleware.PageEmpty)
		} else {
			r.GET("/", ginmiddleware.PageAbort)
		}
	}
	if sss != "" {
		r.GET("/showroutes", ginmiddleware.BasicAuth("whowants2seethis?:iam,yourcreator."),
			func(c *gin.Context) {
				c.Header("Content-Type", "text/html")
				c.Status(http.StatusOK)
				render.WriteString(c.Writer, sss, nil)
			})
	}
	// 读取特权名单
	if b, err := os.ReadFile(pathtool.JoinPathFromHere(".vip")); err == nil {
		ss := strings.Split(gopsu.DecodeString(string(b)), ",")
		if len(ss) > 0 {
			vipUsers = ss
		}
	}
	// if *debug || *cert == "" || *key == "" {
	// 	err = fw.listenAndServeTLS(*webPort, r, "", "", "")
	// } else {
	err := fw.listenAndServeTLS(*webPort, r, fw.httpCert, fw.httpKey, "")
	// }
	if err != nil {
		// panic(fmt.Errorf("Failed start HTTP(S) server at :" + strconv.Itoa(*webPort) + " | " + err.Error()))
		fw.WriteError("WEB", "Failed start web server at :"+strconv.Itoa(*webPort)+" | "+err.Error()+". >>> QUIT ...")
		gocmd.SignalQuit()
		return
	}
}
func (fw *WMFrameWorkV2) listenAndServeTLS(port int, h *gin.Engine, certfile, keyfile string, clientca string) error {
	// 路由处理
	var findRoot = false
	for _, v := range h.Routes() {
		if v.Path == "/" {
			findRoot = true
			break
		}
	}
	if !findRoot {
		h.GET("/", ginmiddleware.PageDefault)
	}
	// 设置全局超时
	st := ginmiddleware.GetSocketTimeout()
	// 初始化
	s := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      h,
		ReadTimeout:  st,
		WriteTimeout: st,
		IdleTimeout:  st,
	}
	// 启动http服务
	if certfile == "" && keyfile == "" {
		fw.WriteSystem("WEB", fmt.Sprintf("Success start HTTP server at :%d", port))
		return s.ListenAndServe()
	}
	// 初始化证书
	var tc = &tls.Config{
		Certificates: make([]tls.Certificate, 1),
	}
	var err error
	tc.Certificates[0], err = tls.LoadX509KeyPair(certfile, keyfile)
	if err != nil {
		return err
	}
	if len(clientca) > 0 {
		pool := x509.NewCertPool()
		caCrt, err := os.ReadFile(clientca)
		if err == nil {
			pool.AppendCertsFromPEM(caCrt)
			tc.ClientCAs = pool
			tc.ClientAuth = tls.NoClientCert
		}
	}
	s.TLSConfig = tc
	// 添加手动更新路由
	h.GET("/cert/:do", func(c *gin.Context) {
		if do, ok := c.Params.Get("do"); ok && do == "renew" {
			var spath = pathtool.JoinPathFromHere("sslrenew")
			if gopsu.OSNAME == "windows" {
				spath += ".exe"
			}
			if !pathtool.IsExist(spath) {
				c.String(400, "no sslrenew found")
				return
			}
			cmd := exec.Command(spath)
			err := cmd.Start()
			if err != nil {
				c.String(400, err.Error())
				return
			}
			time.Sleep(time.Second)
			cmd.Process.Signal(syscall.SIGINT)
			cmd.Wait()
			c.Writer.WriteString("sslrenew done\n")
		}
		fw.RenewCA()
		c.String(200, "the certificate file has been reloaded")
	})
	// 启动证书维护线程
	go fw.renewCA(s, certfile, keyfile)
	// 启动https
	fw.WriteSystem("WEB", fmt.Sprintf("Success start HTTPS server at :%d", port))
	return s.ListenAndServeTLS("", "")
}

// RenewCA 更新ca证书
func (fw *WMFrameWorkV2) RenewCA() bool {
	fw.chanSSLRenew <- struct{}{}
	return true
}

// 后台更新证书
func (fw *WMFrameWorkV2) renewCA(s *http.Server, certfile, keyfile string) {
	loopfunc.LoopFunc(func(params ...interface{}) {
		t := time.NewTicker(time.Hour * 10)
		for {
			select {
			case <-fw.chanSSLRenew:
				newcert, err := tls.LoadX509KeyPair(certfile, keyfile)
				if err == nil {
					s.TLSConfig.Certificates[0] = newcert
					s.TLSConfig.ClientAuth = tls.NoClientCert
				}
			case <-t.C:
				fw.chanSSLRenew <- struct{}{}
			}
		}
	}, "renew ca", fw.httpWriter)
}

// DoRequestWithTimeout 进行http request请求
func (fw *WMFrameWorkV2) DoRequestWithTimeout(req *http.Request, timeo time.Duration, params ...interface{}) (int, []byte, map[string]string, error) {
	// 处理url，`/`开头的，尝试从etcd获取地址
	x := req.URL.String()
	if strings.HasPrefix(x, "/") {
		s, err := fw.PickerDetail(strings.Split(x, "/")[1])
		if err != nil {
			fw.WriteError("ETCD", err.Error())
			return 501, nil, nil, fmt.Errorf("wrong url: " + x)
		}
		if req.URL, err = url.Parse(s + x); err != nil {
			return 501, nil, nil, err
		}
	}
	// 处理头
	if req.Header.Get("Content-Type") == "" {
		switch req.Method {
		case "GET":
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		case "POST":
			req.Header.Set("Content-Type", "application/json")
		}
	}
	// 处理参数
	var notlog bool
	for _, v := range params {
		if v.(string) == "notlog" {
			notlog = true
			continue
		}
	}
	// 超时
	ctx, cancel := context.WithTimeout(context.Background(), timeo)
	defer cancel()
	// 请求
	start := time.Now()
	resp, err := fw.httpClientPool.Do(req.WithContext(ctx))
	if err != nil {
		fw.WriteError("HTTP-REQ ERR", fmt.Sprintf("%s %s▸%s", req.Method, req.URL.String(), err.Error()))
		return 502, nil, nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		fw.WriteError("HTTP-REQ ERR", fmt.Sprintf("%s %s▸%s", req.Method, req.URL.String(), err.Error()))
		return 502, nil, nil, err
	}
	end := time.Since(start).String()
	// 处理头
	h := make(map[string]string)
	h["resp_from"] = req.Host
	h["resp_duration"] = end
	for k := range resp.Header {
		h[k] = resp.Header.Get(k)
	}
	sc := resp.StatusCode
	if !notlog {
		fw.WriteInfo("HTTP-REQ", fmt.Sprintf("|%d| %-13s |%s %s ▸%s", sc, end, req.Method, req.URL.String(), gopsu.String(b)))
	}
	return sc, b, h, nil
}

// DoRequest 进行http request请求
func (fw *WMFrameWorkV2) DoRequest(req *http.Request) (int, []byte, map[string]string, error) {
	return fw.DoRequestWithTimeout(req, fw.reqTimeo)
}

// DoRequestWithFixedToken 使用固定token进行http请求
//
//	req：请求
//	time：超时时间
//	username：用于获取固定token的用户名
func (fw *WMFrameWorkV2) DoRequestWithFixedToken(req *http.Request, timeo time.Duration, username string) (int, []byte, map[string]string, error) {
	if req.Header.Get("User-Token") == "" {
		uuid, ok := fw.ft.Load(username)
		if !ok {
			uuid, ok = fw.GoUUID("", username)
			if !ok {
				return 401, nil, nil, fmt.Errorf("can not get the token with the name " + username)
			}
		}
		req.Header.Set("User-Token", uuid)
	}
	return fw.DoRequestWithTimeout(req, timeo)
	// sc, b, h, err := fw.DoRequestWithTimeout(req, timeo)
	// if sc == http.StatusUnauthorized {
	// 	fw.ft.Delete(username)
	// }
	// return sc, b, h, err

}
func (fw *WMFrameWorkV2) pageModCheck(c *gin.Context) {
	var tbody = `{{define "body"}}
<body>
<h3>服务器时间：</h3><a>{{.timer}}</a>
<h3>服务模块状态：</h3>
<table>
<thead>
<tr>
<th>启用的模块</th>
<th>模块状态</th>
</tr>
</thead>
<tbody>
	{{range $idx, $elem := .clients}}
	<tr>
		{{range $key,$value:=$elem}}
			<td>{{$value}}</td>
		{{end}}
	</tr>
	{{end}}
</tbody>
</table>
</body>
{{end}}`
	var serviceCheck = make([][]string, 0)
	// 版本
	serviceCheck = append(serviceCheck, []string{"ver", gjson.Parse(fw.verJSON).Get("version").String()})
	// 检查etc
	serviceCheck = append(serviceCheck, []string{"etcd", func() string {
		if fw.cnf.UseETCD == nil || !fw.cnf.UseETCD.Activation {
			return "---"
		}
		if _, err := fw.Picker(fw.serverName); err != nil {
			return "bad"
		}
		return "ok"
	}()})
	// 检查redis
	serviceCheck = append(serviceCheck, []string{"redis", func() string {
		if fw.cnf.UseRedis == nil || !fw.cnf.UseRedis.Activation {
			return "---"
		}
		if err := fw.WriteRedis(gopsu.GetUUID1(), "value interface{}", time.Second); err != nil {
			return "bad"
		}
		return "ok"
	}()})
	// 检查mq生产者
	serviceCheck = append(serviceCheck, []string{"mq_producer", func() string {
		if fw.cnf.UseMQProducer == nil || !fw.cnf.UseMQProducer.Activation {
			return "---"
		}
		if !fw.ProducerIsReady() {
			return "bad"
		}
		return "ok"
	}()})
	// 检查mq消费者
	serviceCheck = append(serviceCheck, []string{"mq_consumer", func() string {
		if fw.cnf.UseMQConsumer == nil || !fw.cnf.UseMQConsumer.Activation {
			return "---"
		}
		if !fw.ConsumerIsReady() {
			return "bad"
		}
		return "ok"
	}()})
	// 检查sql
	serviceCheck = append(serviceCheck, []string{"sql", func() string {
		if fw.cnf.UseSQL == nil || !fw.cnf.UseSQL.Activation {
			return "---"
		}
		if !fw.MysqlIsReady() {
			return "bad"
		}
		return "ok"
	}()})
	// 检查mqtt
	serviceCheck = append(serviceCheck, []string{"mqtt", func() string {
		if fw.cnf.UseMQTT == nil || !fw.cnf.UseMQTT.Activation {
			return "---"
		}
		if !fw.MQTTIsReady() {
			return "bad"
		}
		return "ok"
	}()})
	if c.Request.Method == "GET" {
		var d = gin.H{
			"timer":   gopsu.Stamp2Time(time.Now().Unix()),
			"clients": serviceCheck,
		}
		t, _ := template.New("modcheck").Parse(TPLHEAD + TPLCSS + tbody)
		h := render.HTML{
			Name:     "modcheck",
			Data:     d,
			Template: t,
		}
		h.WriteContentType(c.Writer)
		h.Render(c.Writer)
		return
	}
	var js string
	for _, v := range serviceCheck {
		js, _ = sjson.Set(js, v[0], v[1])
	}
	returnJSON(200, c, gjson.Parse(js).Value())
	// c.JSON(200, gjson.Parse(js).Value())
}

func (fw *WMFrameWorkV2) pageStatus(c *gin.Context) {
	var statusInfo = make(map[string]interface{})
	statusInfo["upTime"] = fw.upTime
	statusInfo["timer"] = time.Now().Format("2006-01-02 15:04:05 Mon")
	statusInfo["key"] = "服务运行信息"
	fmtver, _ := json.MarshalIndent(gjson.Parse(fw.verJSON).Value(), "", "")
	statusInfo["value"] = strings.Split(rever.Replace(gopsu.String(fmtver)), "\n")
	switch c.Request.Method {
	case "GET":
		c.Header("Content-Type", "text/html")
		t, _ := template.New("runtime").Parse(TPLHEAD + TPLCSS + TPLBODY)
		h := render.HTML{
			Name:     "runtime",
			Data:     statusInfo,
			Template: t,
		}
		h.WriteContentType(c.Writer)
		h.Render(c.Writer)
	case "POST":
		c.Set("server_time", statusInfo["timer"].(string))
		c.Set("start_at", statusInfo["upTime"].(string))
		c.Set("ver", gjson.Parse(fw.verJSON).Value())
		c.Set("conf", fw.wmConf.Print())
		// fw.DealWithSuccessOK(c)
		returnJSON(200, c, c.Keys)
	}
}

// PrepareToken 获取User-Token信息
//
//	shouldAbort: token非法时是否退出接口，true-退出，false-不退出
//	auth：访问需要的权限，如，`r`，`w`
func (fw *WMFrameWorkV2) PrepareToken(shouldAbort bool, auth ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		uuid := c.GetHeader("User-Token")
		if len(uuid) != 36 {
			ex := NewErr(ErrTokenNotFound).Format(uuid)
			if shouldAbort {
				returnJSON(ex.HSt(), c, ex)
				// c.AbortWithStatusJSON(ex.HSt(), ex)
			}
			c.AddParam("_error", ex.Error())
			return
		}
		tokenPath := fw.AppendRootPathRedis("usermanager/legal/" + MD5Worker.Hash(gopsu.Bytes(uuid)))
		x, err := fw.ReadRedis(tokenPath)
		if err != nil {
			ex := NewErr(ErrTokenIllegal).Format(uuid)
			if shouldAbort {
				returnJSON(ex.HSt(), c, ex)
				// c.AbortWithStatusJSON(ex.HSt(), ex)
			}
			c.AddParam("_error", ex.Error())
			return
		}
		ans := gjson.Parse(x)
		if !ans.Exists() { // token内容异常
			ex := NewErr(ErrTokenNotUnderstand).Format(uuid)
			if shouldAbort {
				returnJSON(ex.HSt(), c, ex)
				// c.AbortWithStatusJSON(ex.HSt(), ex)
			}
			c.AddParam("_error", ex.Error())
			fw.EraseRedis(tokenPath)
			return
		}
		if ans.Get("expire").Int() > 0 && ans.Get("expire").Int() < time.Now().Unix() { // 用户过期
			ex := NewErr(ErrTokenExpired).Format(uuid)
			if shouldAbort {
				returnJSON(ex.HSt(), c, ex)
				// c.AbortWithStatusJSON(ex.HSt(), ex)
			}
			c.AddParam("_error", ex.Error())
			fw.EraseRedis(tokenPath)
			return
		}
		// auth信息查询
		authbinding := make([]string, 0)
		for _, v := range ans.Get("auth_binding").Array() {
			authbinding = append(authbinding, v.String())
		}
		// 获取token用户名
		tokenname := ans.Get("link_name").String()
		if tokenname == "" {
			tokenname = ans.Get("user_name").String()
		}
		// 需要匹配权限,不匹配则强制退出，但token不会失效
		if len(auth) > 0 && tokenname != "root" {
			if len(gopsu.SlicesIntersect(authbinding, auth)) == 0 { // 这里采用`或`处理权限 < len(auth) {
				returnJSON(errNotAcceptable.HSt(), c, errNotAcceptable)
				// c.AbortWithStatusJSON(errNotAcceptable.HSt(), errNotAcceptable)
				return
			}
		}
		c.Params = append(c.Params, gin.Param{
			Key:   "_userTokenName",
			Value: tokenname,
		})
		c.Params = append(c.Params, gin.Param{
			Key:   "_authBinding",
			Value: strings.Join(authbinding, ","),
		})
		// token信息查询
		c.Params = append(c.Params, gin.Param{
			Key:   "_userTokenPath",
			Value: tokenPath,
		})
		c.Params = append(c.Params, gin.Param{
			Key:   "_userDepID",
			Value: ans.Get("userinfo.dep_id").String(),
		})
		c.Params = append(c.Params, gin.Param{
			Key:   "_userTokenAlias",
			Value: ans.Get("useralias").String(),
		})
		// 检查特权用户
		for _, v := range vipUsers {
			if v == tokenname {
				c.Params = append(c.Params, gin.Param{
					Key:   "_vip",
					Value: "1",
				})
				break
			}
		}
		// asadmin信息查询
		asadmin := ans.Get("asadmin").String()
		if asadmin == "0" {
			asadmin = ans.Get("userinfo.user_admin").String()
		}
		c.Params = append(c.Params, gin.Param{
			Key:   "_userAsAdmin",
			Value: asadmin,
		})
		// 角色信息查询
		c.Params = append(c.Params, gin.Param{
			Key:   "_userRoleID",
			Value: ans.Get("role_id").String(),
		})
		// api信息查询
		enableapi := make([]string, 0)
		for _, v := range ans.Get("enable_api").Array() {
			enableapi = append(enableapi, v.String())
		}
		c.Params = append(c.Params, gin.Param{
			Key:   "_enableAPI",
			Value: strings.Join(enableapi, ","),
		})
		// // 更新redis的对应键值的有效期
		// if ans.Get("source").String() != "local" {
		// 	fw.ExpireUserToken(uuid)
		// }
	}
}

// PrepareTokenFromParams 从提交参数获取token信息
func (fw *WMFrameWorkV2) PrepareTokenFromParams() gin.HandlerFunc {
	return func(c *gin.Context) {
		uuid := c.Param("User-Token")
		if len(uuid) == 36 {
			c.Request.Header.Set("User-Token", uuid)
		}
	}
}

// RenewToken 更新uuid时效
func (fw *WMFrameWorkV2) RenewToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		go func() {
			if c.GetHeader("Token_alive") == "0" {
				return
			}
			uuid := c.GetHeader("User-Token")
			if len(uuid) != 36 {
				return
			}
			x, err := fw.ReadRedis("usermanager/legal/" + MD5Worker.Hash(gopsu.Bytes(uuid)))
			if err != nil {
				return
			}
			// 更新redis的对应键值的有效期
			if gjson.Parse(x).Get("source").String() == "local" {
				return
			}
			if _, ok := fw.tokenCache.Get(uuid); ok { // 1分钟内不重复刷新
				return
			}
			fw.ExpireUserToken(uuid)
			fw.tokenCache.Set(uuid, struct{}{}, time.Minute)
		}()
	}
}

// GoUUID 获取特定uuid
func (fw *WMFrameWorkV2) GoUUID(uuid, username string) (string, bool) {
	// 若提交的uuid=="", 则清除缓存，强制查询
	if uuid == "" {
		fw.ft.Delete(username)
	}
	// 若找到缓存的id，返回
	if uid, ok := fw.ft.Load(username); ok {
		return uid, true
	}
	// 没找到缓存id，进行查询
	addr, err := fw.PickerDetail("usermanager")
	if err != nil {
		fw.WriteError("ETCD", "can not found server usermanager")
		return "", false
	}
	var req *http.Request
	req, _ = http.NewRequest("GET", addr+"/usermanager/v1/user/fixed/login?user_name="+username, strings.NewReader(""))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Legal-High", gopsu.CalculateSecurityCode("m", time.Now().Month().String(), 0)[0])
	resp, err := fw.httpClientPool.Do(req)
	if err != nil {
		fw.WriteError("CORE", "get uuid error:"+err.Error())
		return "", false
	}
	defer resp.Body.Close()
	var b bytes.Buffer
	_, err = b.ReadFrom(resp.Body)
	if err != nil {
		fw.WriteError("CORE", "read uuid error:"+err.Error())
		return "", false
	}
	// 缓存
	fw.ft.Store(username, b.String())
	return b.String(), true
}

// DealWithSQLError 统一处理sql执行错误问题
func (fw *WMFrameWorkV2) DealWithSQLError(c *gin.Context, err error) bool {
	if err != nil {
		fw.WriteError("DB", c.Request.RequestURI+"|"+err.Error())
		if strings.Contains(err.Error(), "Duplicate entry") {
			errx := NewErr(ErrSQLDetail).Format(err.Error())
			returnJSON(errx.Status, c, errx)
			// c.AbortWithStatusJSON(errx.Status, errx)
			return true
		}
		returnJSON(errSQL.Status, c, errSQL)
		// c.AbortWithStatusJSON(errSQL.Status, errSQL)
		return true
	}
	return false
}

// DealWithXFileMessage 处理自定义失败信息
// xfileargs 为选填，但填充必须为双数，key1,value1,key2,value2,...样式
func (fw *WMFrameWorkV2) DealWithXFileMessage(c *gin.Context, detail string, xfile int, xfileArgs ...string) {
	err := &ErrorV2{
		Status: 200,
		Detail: detail,
		Xfile:  xfile,
	}
	if len(xfileArgs)%2 == 0 {
		js := ""
		for i := 0; i < len(xfileArgs); i += 2 {
			js, _ = sjson.Set("", xfileArgs[i], xfileArgs[i+1])
		}
		err.XfileArgs = gjson.Parse(js).Value()
	}
	returnJSON(err.Status, c, err)
	// c.AbortWithStatusJSON(err.Status, err)
}

// DealWithFailedMessage 处理标准失败信息
func (fw *WMFrameWorkV2) DealWithFailedMessage(c *gin.Context, detail string, status ...int) {
	if len(status) == 0 {
		c.Set("status", 0)
	} else {
		c.Set("status", status[0])
	}
	c.Set("detail", detail)
	if len(status) > 0 && status[0] == 2 {
		c.Set("xfile", 10000)
	}
	returnJSON(200, c, c.Keys)
}

// DealWithSuccessOK 处理标准成功信息，可选添加detail信息
func (fw *WMFrameWorkV2) DealWithSuccessOK(c *gin.Context, detail ...string) {
	if _, ok := c.Keys["status"]; !ok {
		c.Set("status", 1)
	}
	l := len(detail)
	if l == 1 {
		c.Set("detail", detail[0])
		goto END
	}
	if l%2 == 0 {
		for i := 0; i < l; i += 2 {
			c.Set(detail[i], detail[i+1])
		}
	}
END:
	returnJSON(200, c, c.Keys)
}

func returnJSON(code int, c *gin.Context, body any) {
	c.Writer.Header().Set("Content-Type", "application/json")
	b, _ := json.Marshal(body)
	// if err != nil {
	// 	code = 400
	// 	js, _ := sjson.SetBytes([]byte{}, "status", 0)
	// 	js, _ = sjson.SetBytes(js, "detail", err.Error())
	// 	b = js
	// }
	if code >= 400 {
		c.Abort()
	}
	c.Writer.WriteHeader(code)
	c.Writer.Write(b)
	// c.Writer.Flush()
}

// JSON2Key json字符串赋值到gin.key
func (fw *WMFrameWorkV2) JSON2Key(c *gin.Context, s string) {
	if b := gjson.Parse(s); b.IsObject() {
		b.ForEach(func(key, value gjson.Result) bool {
			c.Set(key.String(), value.Value())
			return true
		})
	}
}

// SetTokenLife 设置User-Token的有效期，默认30分钟
func (fw *WMFrameWorkV2) SetTokenLife(t time.Duration) {
	fw.tokenLife = t
}

// CheckRequired 校验必填项
func (fw *WMFrameWorkV2) CheckRequired(params ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, v := range params {
			if gopsu.TrimString(v) == "" {
				continue
			}
			if _, ok := c.Params.Get(v); !ok {
				ex := NewErr(ErrLostRequired).Format(v).XArgs("key_name", v)
				returnJSON(ex.HSt(), c, ex)
				// c.AbortWithStatusJSON(ex.HSt(), ex)
				return
			}
		}
	}
}

// ReadCacheJSON 读取数据库缓存
func ReadCacheJSON(mydb db.SQLInterface) gin.HandlerFunc {
	return func(c *gin.Context) {
		if mydb != nil {
			cachetag := c.Param("cachetag")
			if cachetag != "" {
				cachestart := gopsu.String2Int(c.Param("cachestart"), 10)
				cacherows := gopsu.String2Int(c.Param("cacherows"), 10)
				ans := mydb.QueryCacheJSON(cachetag, cachestart, cacherows)
				if gjson.Parse(ans).Get("total").Int() > 0 {
					c.Params = append(c.Params, gin.Param{
						Key:   "_cacheData",
						Value: ans,
					})
				}
			}
		}
	}
}

// ReadCachePB2 读取数据库缓存
func ReadCachePB2(mydb db.SQLInterface) gin.HandlerFunc {
	return func(c *gin.Context) {
		if mydb != nil {
			cachetag := c.Param("cachetag")
			if cachetag != "" {
				cachestart := gopsu.String2Int(c.Param("cachestart"), 10)
				cacherows := gopsu.String2Int(c.Param("cacherows"), 10)
				ans := mydb.QueryCachePB2(cachetag, cachestart, cacherows)
				if ans.Total > 0 {
					var s string
					if b, err := json.Marshal(ans); err != nil {
						s = gopsu.String(b)
					}
					// s, _ := json.MarshalToString(ans)
					c.Params = append(c.Params, gin.Param{
						Key:   "_cacheData",
						Value: s,
					})
				}
			}
		}
	}
}

// UserTokenShouldMatch 检查token是否匹配
func (fw *WMFrameWorkV2) UserTokenShouldMatch() gin.HandlerFunc {
	return func(c *gin.Context) {
		userasadmin := c.Param("_userAsAdmin")
		username, ok := c.Params.Get("user_name")
		if !ok {
			username = c.Param("_userTokenName")
			c.AddParam("user_name", username)
		}
		if username == "" {
			returnJSON(errNotAuthorized.HSt(), c, errNotAuthorized)
			// c.AbortWithStatusJSON(errNotAuthorized.HSt(), errNotAuthorized)
			return
		}
		if userasadmin != "1" && username != c.Param("_userTokenName") {
			c.Set("status", 0)
			c.Set("detail", "User-Token dose not match")
			c.Set("xfile", 12)
			returnJSON(400, c, c.Keys)
			// c.AbortWithStatusJSON(400, c.Keys)
			return
		}
	}
}

// UserMustAdmin 需要admin权限
func (fw *WMFrameWorkV2) UserMustAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Param("_userAsAdmin") == "1" || c.Param("_vip") == "1" {
			return
		}
		c.Set("status", 0)
		c.Set("detail", "you need to be an admin")
		c.Set("xfile", 12)
		returnJSON(400, c, c.Keys)
		// c.AbortWithStatusJSON(400, c.Keys)
	}
}

// DealWithXlsx xlsx 文件下载
func (fw *WMFrameWorkV2) DealWithXlsx(c *gin.Context, xlsx *excel.FileData, filename string) {
	if !strings.HasSuffix(filename, ".xlsx") {
		filename += ".xlsx"
	}
	filename = url.PathEscape(filename)
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet") // "application/octet-stream")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Access-Control-Expose-Headers", "Content-Disposition")
	xlsx.Write(c.Writer)
}

// DealWithDownload 返回文件下载
func (fw *WMFrameWorkV2) DealWithDownload(c *gin.Context, data []byte, filename string) {
	filename = url.PathEscape(filename)
	if len(data) > 512 {
		c.Header("Content-Type", http.DetectContentType(data[:512])) // "application/octet-stream")
	} else {
		x := mime.TypeByExtension(filepath.Ext(filename))
		if x == "" {
			c.Header("Content-Type", "application/octet-stream")
		} else {
			c.Header("Content-Type", x) // "application/octet-stream")
		}
	}
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Length", fmt.Sprintf("%d", len(data)))
	c.Header("Access-Control-Expose-Headers", "Content-Disposition")
	c.Writer.Write(data)
}

// DealWithImage 返回文件下载
func (fw *WMFrameWorkV2) DealWithImage(c *gin.Context, data []byte, filename string) {
	filename = url.PathEscape(filename)
	if len(data) > 512 {
		c.Header("Content-Type", http.DetectContentType(data[:512])) // "application/octet-stream")
	} else {
		x := mime.TypeByExtension(filepath.Ext(filename))
		if x == "" {
			c.Header("Content-Type", "application/octet-stream")
		} else {
			c.Header("Content-Type", x) // "application/octet-stream")
		}
	}
	c.Header("Cache-Control", "private, max-age=86400")
	c.Writer.Write(data)
}
