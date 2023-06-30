package wmfw

import (
	"fmt"
)

// Panic 写error日志并触发panic
func (fw *WMFrameWorkV2) Panic(name, msg string) {
	fw.WriteLog(name, msg, 40)
	panic(fmt.Errorf(msg))
}

// WriteDebug debug日志
func (fw *WMFrameWorkV2) WriteDebug(name, msg string) {
	fw.WriteLog(name, msg, 10)
}

// WriteInfo Info日志
func (fw *WMFrameWorkV2) WriteInfo(name, msg string) {
	fw.WriteLog(name, msg, 20)
}

// WriteWarning Warning日志
func (fw *WMFrameWorkV2) WriteWarning(name, msg string) {
	fw.WriteLog(name, msg, 30)
}

// WriteError Error日志
func (fw *WMFrameWorkV2) WriteError(name, msg string) {
	fw.WriteLog(name, msg, 40)
}

// WriteSystem System日志
func (fw *WMFrameWorkV2) WriteSystem(name, msg string) {
	fw.WriteLog(name, msg, 90)
}

// WriteLog 写公共日志
// name： 日志类别，如sys，mq，db这种
// msg： 日志信息
// level： 日志级别10,20，30,40,90
func (fw *WMFrameWorkV2) WriteLog(name, msg string, level int) {
	if name != "" {
		name = "[" + name + "] "
	}
	msg = name + msg
	switch level {
	case 10:
		fw.wmLog.Debug(msg)
	case 20:
		fw.wmLog.Info(msg)
	case 30:
		fw.wmLog.Warning(msg)
	case 40:
		fw.wmLog.Error(msg)
	case 90:
		fw.wmLog.System(msg)
	}
}

// StdLogger StdLogger
// type StdLogger struct {
// 	LogWriter   io.Writer
// 	LogReplacer *strings.Replacer
// 	Name        string
// 	LogLevel    int
// }

// func (l *StdLogger) writeLog(msg string, level int) {
// 	if l.LogLevel > 1 && level < l.LogLevel {
// 		return
// 	}
// 	if l.LogReplacer != nil {
// 		msg = l.LogReplacer.Replace(msg)
// 	}
// 	if l.Name != "" {
// 		msg = "[" + l.Name + "] " + msg
// 	}
// 	l.LogWriter.Write(gopsu.Bytes(msg))
// 	logOut(level, msg)
// }

// // Debug Debug
// func (l *StdLogger) Debug(msgs string) {
// 	l.writeLog(msgs, 10)
// }

// // Info Info
// func (l *StdLogger) Info(msgs string) {
// 	l.writeLog(msgs, 20)
// }

// // Warning Warn
// func (l *StdLogger) Warning(msgs string) {
// 	l.writeLog(msgs, 30)
// }

// // Error Error
// func (l *StdLogger) Error(msgs string) {
// 	l.writeLog(msgs, 40)
// }

// // System System
// func (l *StdLogger) System(msgs string) {
// 	l.writeLog(msgs, 90)
// }

// // DefaultWriter 返回默认writer
// func (l *StdLogger) DefaultWriter() io.Writer {
// 	return l.LogWriter
// }

// // DebugFormat Debug
// func (l *StdLogger) DebugFormat(f string, msg ...interface{}) {
// 	if len(msg) == 0 {
// 		l.writeLog(f, 10)
// 		return
// 	}
// 	l.writeLog(fmt.Sprintf(f, msg...), 10)
// }

// // InfoFormat Info
// func (l *StdLogger) InfoFormat(f string, msg ...interface{}) {
// 	if len(msg) == 0 {
// 		l.writeLog(f, 20)
// 		return
// 	}
// 	l.writeLog(fmt.Sprintf(f, msg...), 20)
// }

// // WarningFormat Warn
// func (l *StdLogger) WarningFormat(f string, msg ...interface{}) {
// 	if len(msg) == 0 {
// 		l.writeLog(f, 30)
// 		return
// 	}
// 	l.writeLog(fmt.Sprintf(f, msg...), 30)
// }

// // ErrorFormat Error
// func (l *StdLogger) ErrorFormat(f string, msg ...interface{}) {
// 	if len(msg) == 0 {
// 		l.writeLog(f, 40)
// 		return
// 	}
// 	l.writeLog(fmt.Sprintf(f, msg...), 40)
// }

// // SystemFormat System
// func (l *StdLogger) SystemFormat(f string, msg ...interface{}) {
// 	if f == "" {
// 		l.writeLog(fmt.Sprintf("%v", msg), 90)
// 	} else {
// 		l.writeLog(fmt.Sprintf(f, msg...), 90)
// 	}
// }

// func logOut(level int, msg string) {
// 	if *logLevel > 10 && level >= 40 {
// 		println(time.Now().Format(logger.ShortTimeFormat) + msg)
// 	}
// }
