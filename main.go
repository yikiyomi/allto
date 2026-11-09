package main

import (
	"allto/config"
	"allto/database"
	"allto/handler"
	"allto/model"

	"allto/router"
	"allto/util"

	"log"
	"net/http"
	_ "net/http/pprof"

	"go.uber.org/zap"
)

func main() {
	if err := config.Load(); err != nil {
		log.Fatal("配置加载失败:", err)
	}

	//日志:为true时好看为false时json难看但性能好
	util.InitLogger(true)
	defer util.SyncLogger()

	// mysql
	if err := database.InitDB(config.AppConfig.Database.DSN); err != nil {
		log.Fatal("mysql连接失败:", err)
	}
	// redis
	if err := database.InitRedis(
		config.AppConfig.Redis.Addr,
		config.AppConfig.Redis.Password); err != nil {
		util.Logger.Error("Redis 连接失败，缓存功能降级", zap.Error(err))
	}
	// 自动建表迁移（API 与 worker 各执行一次，AutoMigrate 幂等；两端同时启动时可能撞一次 DDL 锁）
	if err := database.GetDB().AutoMigrate(
		&model.User{},
		&model.Post{},
		&model.Comment{},
		&model.Like{},
		&model.Follow{},
		&model.Notification{},
		&model.Outbox{},
	); err != nil {
		util.Logger.Error("表迁移失败", zap.Error(err))
	}

	// SSE 跨进程桥接只依赖 Redis；本进程已不连 MQ，所以无条件启动
	handler.StartNotificationBridge() // 订阅 Redis notify:* → 推本进程 SSE 连接

	//pprof
	go func() {
		util.Logger.Info("pprof 启动", zap.String("addr", ":6060"))
		if err := http.ListenAndServe(":6060", nil); err != nil {
			util.Logger.Error("pprof 启动失败", zap.Error(err))
		}
	}()
	//初始化gin
	r := router.SetupRouter()
	util.Logger.Info("服务启动", zap.String("port", config.AppConfig.Server.Port))
	r.Run(config.AppConfig.Server.Port)
}
