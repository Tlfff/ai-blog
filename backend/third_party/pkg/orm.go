package log

import (
	"strings"

	"codeup.aliyun.com/qimao/leo/leo/log"
	core "xorm.io/xorm/log"
)

var _ core.Logger = &ORMLogger{}

type ORMLogger struct {
	log.Logger
}

// 定义此orm log 目的是 用项目中log格式体来规范orm的log输出
func NewORMLogger(lHelper log.Logger) *ORMLogger {

	return &ORMLogger{lHelper}
}

func (l ORMLogger) Debug(v ...interface{}) {
	l.Logger.Debug(v...)
	return
}

func (l ORMLogger) Debugf(format string, v ...interface{}) {
	l.Logger.Debugf(format, v...)
	return
}

func (l ORMLogger) Error(v ...interface{}) {
	l.Logger.Error(v...)
	return
}

func (l ORMLogger) Errorf(format string, v ...interface{}) {
	l.Logger.Errorf(format, v...)
	return
}

func (l ORMLogger) Info(v ...interface{}) {
	l.Logger.Info(v...)
	return
}

func (l ORMLogger) Infof(format string, v ...interface{}) {
	l.Logger.Infof(format, v...)
	return
}

func (l ORMLogger) Warn(v ...interface{}) {
	l.Logger.Warn(v...)

	return
}

func (l ORMLogger) Warnf(format string, v ...interface{}) {
	l.Logger.Warnf(format, v...)
	return
}

func (l ORMLogger) Level() core.LogLevel {
	ls := l.PasteLevel(l.Logger.GetLevel().Name())
	return ls
}

func (l ORMLogger) SetLevel(lx core.LogLevel) {
	l.Logger.SetLevel(log.LevelWarn)
}

func (l ORMLogger) PasteLevel(text string) core.LogLevel {
	switch strings.ToLower(string(text)) {
	case "debug":
		return core.LOG_DEBUG
	case "info", "": // make the zero value useful
		return core.LOG_DEBUG
	case "warn":
		return core.LOG_WARNING
	case "error":
		return core.LOG_ERR
	case "panic":
		return core.LOG_ERR
	case "fatal":
		return core.LOG_OFF
	default:
		return core.LOG_UNKNOWN
	}
}

func (l ORMLogger) ShowSQL(show ...bool) {
	return
}

func (l ORMLogger) IsShowSQL() bool {
	return true
}
