package util

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// 日志工具及封装

type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
)

func (l LogLevel) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "INFO"
	}
}

func ParseLogLevel(s string) LogLevel {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "debug":
		return LevelDebug
	case "info", "":
		return LevelInfo
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

type DailyFileLoggerOptions struct {
	BaseDir    string   // 通常为 settings.toml 所在目录
	Dir        string   // 默认为 "logs"
	Extension  string   // 默认为 "log"
	Prefix     string   // 可为空
	MinLevel   LogLevel // 最低日志级别
	Stdout     bool     // 是否同时输出到标准输出
	TimeFormat string   // 为空则 RFC3339
}

// 实现 Logger 接口，按天写入新文件
type DailyFileLogger struct {
	mu      sync.Mutex
	opts    DailyFileLoggerOptions
	dateKey string
	file    *os.File
	logger  *log.Logger
}

func NewDailyFileLogger(opts DailyFileLoggerOptions) (*DailyFileLogger, error) {
	if opts.Dir == "" {
		opts.Dir = "logs"
	}
	if opts.Extension == "" {
		opts.Extension = "log"
	}
	if opts.TimeFormat == "" {
		opts.TimeFormat = time.RFC3339
	}

	l := &DailyFileLogger{opts: opts}
	if err := l.rotateIfNeeded(time.Now()); err != nil {
		return nil, err
	}
	return l, nil
}

func (l *DailyFileLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return nil
	}
	err := l.file.Close()
	l.file = nil
	l.logger = nil
	l.dateKey = ""
	return err
}

func (l *DailyFileLogger) Infof(format string, args ...any)  { l.logf(LevelInfo, format, args...) }
func (l *DailyFileLogger) Debugf(format string, args ...any) { l.logf(LevelDebug, format, args...) }
func (l *DailyFileLogger) Warnf(format string, args ...any)  { l.logf(LevelWarn, format, args...) }
func (l *DailyFileLogger) Errorf(format string, args ...any) { l.logf(LevelError, format, args...) }

func (l *DailyFileLogger) logf(level LogLevel, format string, args ...any) {
	if level < l.opts.MinLevel {
		return
	}

	now := time.Now()
	line := fmt.Sprintf("%s [%s] %s", now.Format(l.opts.TimeFormat), level.String(), fmt.Sprintf(format, args...))

	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.rotateIfNeeded(now); err != nil {
		// 文件打开失败时降级到标准库 log
		log.Printf("logger rotate failed: %v", err)
		log.Print(line)
		return
	}

	l.logger.Print(line)
}

func (l *DailyFileLogger) rotateIfNeeded(now time.Time) error {
	dateKey := now.Format("2006-01-02")
	if l.file != nil && l.logger != nil && l.dateKey == dateKey {
		return nil
	}

	logsDir := filepath.Join(l.opts.BaseDir, l.opts.Dir)
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return err
	}

	ext := strings.TrimSpace(l.opts.Extension)
	ext = strings.TrimPrefix(ext, ".")
	if ext == "" {
		ext = "log"
	}

	fileName := fmt.Sprintf("%s.%s", dateKey, ext)
	if strings.TrimSpace(l.opts.Prefix) != "" {
		fileName = fmt.Sprintf("%s-%s.%s", l.opts.Prefix, dateKey, ext)
	}
	path := filepath.Join(logsDir, fileName)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}

	if l.file != nil {
		_ = l.file.Close()
	}

	var out io.Writer = f
	if l.opts.Stdout {
		out = io.MultiWriter(f, os.Stdout)
	}

	l.file = f
	l.dateKey = dateKey
	l.logger = log.New(out, "", 0)
	return nil
}

type nopLogger struct{}

func (nopLogger) Infof(string, ...any)  {}
func (nopLogger) Debugf(string, ...any) {}
func (nopLogger) Warnf(string, ...any)  {}
func (nopLogger) Errorf(string, ...any) {}

var (
	globalLoggerMu sync.RWMutex
	globalLogger   Logger = nopLogger{}
)

// SetGlobalLogger 设置全局 Logger（用于进程内统一事件记录）。
func SetGlobalLogger(l Logger) {
	if l == nil {
		l = nopLogger{}
	}
	globalLoggerMu.Lock()
	defer globalLoggerMu.Unlock()
	globalLogger = l
}

// L 返回全局 Logger。
func L() Logger {
	globalLoggerMu.RLock()
	defer globalLoggerMu.RUnlock()
	return globalLogger
}
