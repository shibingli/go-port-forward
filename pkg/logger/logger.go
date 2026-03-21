// Package logger 提供全局日志功能，供 pkg 包使用
// Package logger provides global logging for pkg packages
package logger

import (
	"go.uber.org/zap"
)

// global 是全局 zap.Logger 实例，默认为 nop（不输出）
// global is the global zap.Logger instance, defaults to nop (no output)
var global *zap.Logger = zap.NewNop()

// SetLogger 设置全局 Logger 实例 | Set the global Logger instance
func SetLogger(l *zap.Logger) {
	if l != nil {
		global = l
	}
}

// Get 获取全局 Logger 实例 | Get the global Logger instance
func Get() *zap.Logger {
	return global
}

// Debug logs a debug message with fields.
func Debug(msg string, fields ...zap.Field) {
	global.Debug(msg, fields...)
}

// Info logs an info message with fields.
func Info(msg string, fields ...zap.Field) {
	global.Info(msg, fields...)
}

// Warn logs a warning message with fields.
func Warn(msg string, fields ...zap.Field) {
	global.Warn(msg, fields...)
}

// Error logs an error message with fields.
func Error(msg string, fields ...zap.Field) {
	global.Error(msg, fields...)
}

// Fatal logs a fatal message with fields, then calls os.Exit(1).
func Fatal(msg string, fields ...zap.Field) {
	global.Fatal(msg, fields...)
}

// Sync flushes any buffered log entries.
func Sync() {
	_ = global.Sync()
}
