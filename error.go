package wmfw

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// ErrorV2 自定义错误类型
type ErrorV2 struct {
	fmtDetail  string
	Detail     string      `json:"detail,omitempty"`
	Status     int         `json:"status"`
	Xfile      int         `json:"xfile,omitempty"`
	XfileArgs  interface{} `json:"xfile_args,omitempty"`
	httpStatus int
}

// Error 返回错误详情
func (e *ErrorV2) Error() string {
	if e.Detail == "" {
		e.Detail = e.fmtDetail
	}
	return e.Detail
}

// HSt 返回http状态码
func (e *ErrorV2) HSt() int {
	return e.httpStatus
}

// XArgs 填充xargs
func (e *ErrorV2) XArgs(s ...string) *ErrorV2 {
	if len(s) == 0 {
		return e
	}
	if len(s)%2 != 0 {
		s = s[:len(s)-1]
	}
	var js string
	for i := 0; i < len(s); i += 2 {
		js, _ = sjson.Set(js, s[i], s[i+1])
	}
	e.XfileArgs = gjson.Parse(js).Value()
	return e
}

// Format 格式化错误内容
func (e *ErrorV2) Format(a ...interface{}) *ErrorV2 {
	if len(a) == 0 {
		return e
	}
	c := strings.Count(e.fmtDetail, "%")
	if c == 0 {
		e.Detail = fmt.Sprintf("%v", a)
		return e
	}
	if len(a) < c {
		c = len(a)
	}
	e.Detail = fmt.Sprintf(e.fmtDetail, a[:c]...)
	return e
}

// ErrorType 预定义错误类型
type ErrorType int

// ErrNoDataFound 找不到数据
// ErrNoDataFound
// ErrRedisNil redis读取空
// ErrRedisNil
// ErrSQL 数据库执行错误
// ErrSQL
// ErrNotAuthorized 未经过认证
// ErrNotAuthorized
const (
	// ErrLostRequired 缺少必填参数
	ErrLostRequired ErrorType = iota
	// ErrServerOffline 服务离线
	ErrServerOffline
	// ErrTokenExpired token过期
	ErrTokenExpired
	// ErrTokenIllegal token非法
	ErrTokenIllegal
	// ErrTokenNotFound token 不存在
	ErrTokenNotFound
	// ErrTokenNotAcceptable token 权限不足
	ErrTokenNotAcceptable
	// ErrTokenNotUnderstand token信息无法解析
	ErrTokenNotUnderstand
	// ErrBadRequest 请求失败
	ErrBadRequest
	// ErrRedisNil redis空
	ErrRedisNil
	// ErrRedisNotReady redis未连接
	ErrRedisNotReady
	// ErrNotAuthorized 未认证
	ErrNotAuthorized
	// ErrNoDataFound 无数据
	ErrNoDataFound
	// ErrSQL 错误
	ErrSQL
	// ErrSQLDetail 数据库错误
	ErrSQLDetail
	// ErrSQLDuplicateEntry sql主键重复
	ErrSQLDuplicateEntry
	// ErrTokenIPChanged token ip变化
	ErrTokenIPChanged
)

// NewErr 返回一个err类型
func NewErr(err ErrorType) *ErrorV2 {
	switch err {
	case ErrLostRequired:
		return &ErrorV2{fmtDetail: "key name `%s` is required", Status: 0, Xfile: 5, httpStatus: http.StatusBadRequest}
	case ErrServerOffline:
		return &ErrorV2{fmtDetail: "server `%s` offline", Status: 0, Xfile: 1, httpStatus: http.StatusServiceUnavailable}
	case ErrTokenExpired:
		return &ErrorV2{fmtDetail: "User-Token has expired %s", Status: 0, Xfile: 11, httpStatus: http.StatusUnauthorized}
	case ErrTokenIllegal:
		return &ErrorV2{fmtDetail: "User-Token illegal %s", Status: 0, Xfile: 11, httpStatus: http.StatusUnauthorized}
	case ErrTokenNotFound:
		return &ErrorV2{fmtDetail: "User-Token not found %s", Status: 0, Xfile: 11, httpStatus: http.StatusUnauthorized}
	case ErrTokenNotAcceptable:
		return &ErrorV2{fmtDetail: "User-Token auth not acceptable %s", Status: 0, Xfile: 11, httpStatus: http.StatusNotAcceptable}
	case ErrTokenNotUnderstand:
		return &ErrorV2{fmtDetail: "User-Token not understand %s", Status: 0, Xfile: 11, httpStatus: http.StatusUnauthorized}
	case ErrSQLDetail:
		return &ErrorV2{fmtDetail: "sql error: %s", Status: 0, httpStatus: http.StatusInternalServerError}
	case ErrBadRequest:
		return errBadRequest
	case ErrRedisNil:
		return errRedisNil
	case ErrRedisNotReady:
		return errRedisNotReady
	case ErrNotAuthorized:
		return errNotAuthorized
	case ErrNoDataFound:
		return errNoDataFound
	case ErrSQL:
		return errSQL
	case ErrSQLDuplicateEntry:
		return errSQLDuplicateEntry
	case ErrTokenIPChanged:
		return &ErrorV2{fmtDetail: "User-Token IP has changed %s", Status: 0, Xfile: 11, httpStatus: http.StatusUnauthorized}
	default:
		return errUndefine
	}
}

var (
	errUndefine = &ErrorV2{fmtDetail: "undefine error", Status: 0, httpStatus: http.StatusTeapot}
	// ErrRedisNil redis读取空
	errRedisNil = &ErrorV2{Detail: "redis: nil", Status: 0, httpStatus: http.StatusOK}
	// ErrRedisNil redis读取空
	errRedisNotReady = &ErrorV2{Detail: "redis is not ready", Status: 0, httpStatus: http.StatusOK}
	// ErrBadRequest 请求失败
	errBadRequest = &ErrorV2{fmtDetail: "", Status: 0, Xfile: 5, httpStatus: http.StatusBadRequest}
	// ErrNotAuthorized 未经过认证
	errNotAuthorized = &ErrorV2{Detail: "your auth is not match", Status: 0, httpStatus: http.StatusUnauthorized}
	// ErrNotAuthorized 未经过认证
	errNotAcceptable = &ErrorV2{Detail: "your auth is not acceptable", Status: 0, httpStatus: http.StatusNotAcceptable}
	// ErrNoDataFound 找不到数据
	errNoDataFound = &ErrorV2{Detail: "no data can be found", Status: 0, Xfile: 100, httpStatus: http.StatusBadRequest}
	// ErrSQL 数据库执行错误
	errSQL = &ErrorV2{Detail: "sql error", Status: 0, Xfile: 3, httpStatus: http.StatusInternalServerError}
	// ErrSQLDuplicateEntry 主键重复错误
	errSQLDuplicateEntry = &ErrorV2{Detail: "sql error: Duplicate key", Status: 0, Xfile: 3, httpStatus: http.StatusInternalServerError}
)
