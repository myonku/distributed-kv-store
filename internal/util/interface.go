package util

// 日志工具类接口
type Logger interface {
	Infof(format string, args ...any)
	Debugf(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
}

// 后台任务的基本接口
type BackgroundTask interface {
	Start() error
	Stop() error
}
