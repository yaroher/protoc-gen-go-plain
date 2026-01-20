package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type customLogger struct {
	fd *os.File
}

func (c customLogger) Write(p []byte) (n int, err error) {
	return c.fd.Write(p)
}

func (c customLogger) Sync() error {
	return c.fd.Sync()
}

var logLevel = os.Getenv("LOG_LEVEL")

func getLogLevel() zapcore.Level {
	if logLevel == "" {
		return zapcore.InfoLevel
	}
	level, err := zapcore.ParseLevel(logLevel)
	if err != nil {
		return zapcore.InfoLevel
	}
	return level
}

func getFd() *os.File {
	logPath := os.Getenv("LOG_FILE")
	if logPath != "" {
		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			f, err := os.Create(logPath)
			if err != nil {
				panic(err)
			}
			return f
		}
		f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}
		// flush file
		if err := f.Truncate(0); err != nil {
			panic(err)
		}
		if _, err := f.Seek(0, 0); err != nil {
			panic(err)
		}
		return f
	}
	return os.Stderr
}

var Logger = zap.New(zapcore.NewCore(zapcore.NewConsoleEncoder(
	zapcore.EncoderConfig{
		MessageKey:     "msg",
		LevelKey:       "level",
		NameKey:        "logger",
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
	}), &customLogger{fd: getFd()}, getLogLevel())).Named("protoc-gen-plain")

func Debug(msg string, fields ...zap.Field) {
	Logger.Debug(msg, fields...)
}

func Info(msg string, fields ...zap.Field) {
	Logger.Info(msg, fields...)
}

func Warn(msg string, fields ...zap.Field) {
	Logger.Warn(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
	Logger.Error(msg, fields...)
}
