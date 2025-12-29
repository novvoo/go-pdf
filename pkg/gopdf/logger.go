package gopdf

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"
)

// LogLevel 日志级别
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
	LogLevelNone
)

// Logger 结构化日志记录器
type Logger struct {
	mu      sync.RWMutex
	level   LogLevel
	output  io.Writer
	prefix  string
	logger  *log.Logger
	enabled bool
}

var (
	defaultLogger *Logger
	loggerOnce    sync.Once
)

// GetLogger 获取默认日志记录器（单例）
func GetLogger() *Logger {
	loggerOnce.Do(func() {
		defaultLogger = NewLogger(LogLevelWarn, os.Stderr, "[gopdf] ")
	})
	return defaultLogger
}

// NewLogger 创建新的日志记录器
func NewLogger(level LogLevel, output io.Writer, prefix string) *Logger {
	return &Logger{
		level:   level,
		output:  output,
		prefix:  prefix,
		logger:  log.New(output, prefix, log.LstdFlags),
		enabled: true,
	}
}

// SetLevel 设置日志级别
func (l *Logger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// SetEnabled 启用或禁用日志
func (l *Logger) SetEnabled(enabled bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.enabled = enabled
}

// Debug 记录调试信息
func (l *Logger) Debug(format string, v ...interface{}) {
	l.log(LogLevelDebug, "DEBUG", format, v...)
}

// Info 记录信息
func (l *Logger) Info(format string, v ...interface{}) {
	l.log(LogLevelInfo, "INFO", format, v...)
}

// Warn 记录警告
func (l *Logger) Warn(format string, v ...interface{}) {
	l.log(LogLevelWarn, "WARN", format, v...)
}

// Error 记录错误
func (l *Logger) Error(format string, v ...interface{}) {
	l.log(LogLevelError, "ERROR", format, v...)
}

// log 内部日志记录方法
func (l *Logger) log(level LogLevel, levelStr, format string, v ...interface{}) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if !l.enabled || level < l.level {
		return
	}

	msg := fmt.Sprintf(format, v...)
	l.logger.Printf("[%s] %s", levelStr, msg)
}

// 全局便捷函数
func Debug(format string, v ...interface{}) {
	GetLogger().Debug(format, v...)
}

func Info(format string, v ...interface{}) {
	GetLogger().Info(format, v...)
}

func Warn(format string, v ...interface{}) {
	GetLogger().Warn(format, v...)
}

func LogError(format string, v ...interface{}) {
	GetLogger().Error(format, v...)
}

// SetLogLevel 设置全局日志级别
func SetLogLevel(level LogLevel) {
	GetLogger().SetLevel(level)
}

// EnableLogging 启用或禁用全局日志
func EnableLogging(enabled bool) {
	GetLogger().SetEnabled(enabled)
}
