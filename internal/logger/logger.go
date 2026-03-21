package logger

import (
	"go-port-forward/internal/config"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var L *zap.Logger
var S *zap.SugaredLogger

// Init initialises the global logger from config.
func Init(cfg config.LogConfig) error {
	level := parseLevel(cfg.Level)

	// Rotating file writer
	rotator := &lumberjack.Logger{
		Filename:   cfg.Path,
		MaxSize:    cfg.MaxSizeMB,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAgeDays,
		Compress:   cfg.Compress,
	}

	jsonEnc := zap.NewProductionEncoderConfig()
	jsonEnc.TimeKey = "ts"
	jsonEnc.EncodeTime = zapcore.ISO8601TimeEncoder

	fileCore := zapcore.NewCore(zapcore.NewJSONEncoder(jsonEnc), zapcore.AddSync(rotator), level)
	consoleCore := zapcore.NewCore(zapcore.NewJSONEncoder(jsonEnc), zapcore.AddSync(os.Stdout), level)

	core := zapcore.NewTee(fileCore, consoleCore)
	L = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	S = L.Sugar()
	return nil
}

// Sync flushes any buffered log entries.
func Sync() { _ = L.Sync() }

func parseLevel(s string) zapcore.Level {
	switch s {
	case "debug":
		return zapcore.DebugLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}
