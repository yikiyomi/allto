package main

import (
	"allto/config"
	"allto/database"
	"allto/model"
	"allto/mq"
	"allto/util"
	"allto/worker"
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
)

func main() {
	if err := config.Load(); err != nil {
		log.Fatal("worker:配置加载失败:", err)
	}
	// 日志
	util.InitLogger(true)
	defer util.SyncLogger()

	// mysql
	if err := database.InitDB(config.AppConfig.Database.DSN); err != nil {
		log.Fatal("worker:mysql连接失败:", err)
	}
	// redis
	if err := database.InitRedis(
		config.AppConfig.Redis.Addr,
		config.AppConfig.Redis.Password); err != nil {
		log.Fatal("worker:redis连接失败:", err)
	}

	// 表迁移
	if err := database.GetDB().AutoMigrate(
		&model.User{},
		&model.Post{},
		&model.Comment{},
		&model.Like{},
		&model.Follow{},
		&model.Notification{},
		&model.Outbox{},
	); err != nil {
		util.Logger.Error("表迁移失败:", zap.Error(err))
	}

	// RabbitMQ：worker 的全部职责都依赖它，连不上直接退出，交给外部重启
	mqConn, err := mq.New(config.AppConfig.RabbitMQ)
	if err != nil {
		log.Fatal("worker:RabbitMQ 连接失败:", err)
	}

	// 初始化
	database.InitMQ(mqConn)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 开始
	worker.StartOutboxPoller()         // 扫 outbox 表 → 投 MQ
	worker.StartNotificationConsumer() // 消费 MQ → 落库 + Redis Publish
	<-ctx.Done()
	// 睡两秒退出
	time.Sleep(2 * time.Second)
}
