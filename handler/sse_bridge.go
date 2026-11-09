package handler

import (
	"allto/database"
	"allto/util"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

// StartNotificationBridge 订阅 Redis "notify:*" → hub.Push。
// worker 是独立进程，通知在那边落库后 Publish，API 进程订阅到再推给本进程的 SSE 连接。
func StartNotificationBridge() {
	go func() {
		rdb := database.GetRedis()
		if rdb == nil {
			util.Logger.Warn("Redis 未就绪，SSE 跨进程桥接未启动")
			return
		}
		pubsub := rdb.PSubscribe(database.GetCtx(), "notify:*")
		defer pubsub.Close()

		util.Logger.Info("SSE 跨进程桥接已启动", zap.String("pattern", "notify:*"))
		for msg := range pubsub.Channel() {
			userID, err := strconv.Atoi(strings.TrimPrefix(msg.Channel, "notify:"))
			if err != nil {
				util.Logger.Error("解析通知频道失败", zap.String("channel", msg.Channel))
				continue
			}
			hub.Push(uint(userID), []byte(msg.Payload))
		}
	}()
}