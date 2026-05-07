package common

import (
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/hertz-contrib/logger/zap"
	"gopkg.in/natefinch/lumberjack.v2"
	"io"
	"os"
	"path/filepath"
)

var Logger *zap.Logger

func NewLogger() {
	Logger = zap.NewLogger()
	var lumberjackLogger *lumberjack.Logger
	if !Config.Debug {
		accessLog := Config.AccessLog
		if err := os.MkdirAll(filepath.Dir(accessLog), 0755); err != nil {
			panic(err)
		}
		// Set filename to date
		if _, err := os.Stat(accessLog); err != nil {
			if _, err := os.Create(accessLog); err != nil {
				panic(err)
			}
		}
		lumberjackLogger = &lumberjack.Logger{
			Filename:   accessLog,
			MaxSize:    20,   // 单个文件最大 20MB
			MaxBackups: 5,    // 最多保留 5 个备份
			MaxAge:     10,   // 日志文件最大保留 10 天
			Compress:   true, // 压缩旧日志
			LocalTime:  true, // 备份文件用本地时间命名（默认 UTC）
		}
	}
	writer := io.Writer(os.Stdout)
	if lumberjackLogger != nil {
		writer = lumberjackLogger
	}
	Logger.SetOutput(writer)
	Logger.SetLevel(hlog.Level(Config.LogLevel))
	hlog.SetLogger(Logger)
}
