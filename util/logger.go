package util

import (
	"go.uber.org/zap"
)

var (
	Logger *zap.Logger
	Sugar  *zap.SugaredLogger
)

// 初始化日志器:dev为true时好看为false时json难看但性能好
func InitLogger(dev bool) {
	var err error
	if dev {
		Logger, err = zap.NewDevelopment()
	} else {
		Logger, err = zap.NewProduction()
	}
	if err != nil {
		// 日志初始化失败终止程序
		panic("初始化日志器失败: " + err.Error())
	}
	Sugar = Logger.Sugar()
}
func SyncLogger() {
	if Logger != nil {
		_ = Logger.Sync()
	}
}
