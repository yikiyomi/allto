package handler

import (
	"allto/database"
	"allto/middleware"
	"allto/model"
	"allto/repo"
	"allto/response"
	"allto/util"
	"encoding/json"
	"fmt"

	"github.com/gin-gonic/gin"
)

var hub = middleware.NewSSEHub()

// Notify 写通知 + 推 SSE。recipientID=接收者，senderID=触发者，typ=类型，targetID=目标
func Notify(recipientID, senderID uint, typ string, targetID uint) {
	notificationRepo := repo.NewNotificationRepo(database.GetDB())
	if recipientID == senderID {
		return // 不给自己发通知
	}
	n := model.Notification{
		RecipientID: recipientID,
		SenderID:    senderID,
		Type:        typ,
		TargetID:    targetID,
	}
	notificationRepo.Create(&n)

	// 推 SSE（失败也不影响，通知已存 DB）
	msg, _ := json.Marshal(gin.H{
		"type":      typ,
		"sender_id": senderID,
		"target_id": targetID,
	})
	hub.Push(recipientID, msg)
}

// NotificationStream SSE 长连接（token 放 query string，因为 EventSource 不能带 header）
func NotificationStream(c *gin.Context) {
	token := c.Query("token")
	claims, err := util.ParseToken(token)
	if err != nil {
		response.Unauthorized(c, "无效token")
		return
	}
	userID := claims.UserID

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	ch := hub.Register(userID)
	defer hub.Unregister(userID)
	c.Writer.Flush()

	for {
		select {
		case msg := <-ch:
			fmt.Fprintf(c.Writer, "data: %s\n\n", msg)
			c.Writer.Flush()
		case <-c.Request.Context().Done():
			return
		}
	}
}

// 通知列表（50条）
func ListNotifications(c *gin.Context) {
	notificationRepo := repo.NewNotificationRepo(database.GetDB())
	userID := c.GetUint("userID")
	list, err := notificationRepo.ListByRecipient(userID, 50)
	if err != nil {
		response.InternalError(c, "获取通知失败")
		return
	}
	response.Success(c, list)
}

// UnreadCount 未读计数
func UnreadCount(c *gin.Context) {
	userID := c.GetUint("userID")
	notificationRepo := repo.NewNotificationRepo(database.GetDB())
	count, err := notificationRepo.CountUnread(userID)
	if err != nil {
		response.InternalError(c, "未读计数失败")
		return
	}
	response.Success(c, gin.H{"count": count})
}

// 标记已读（看id单条
func MarkRead(c *gin.Context) {
	userID := c.GetUint("userID")
	notificationRepo := repo.NewNotificationRepo(database.GetDB())
	var req struct {
		ID uint `json:"id"`
	}
	c.ShouldBind(&req)
	err := notificationRepo.MarkRead(userID, req.ID)
	if err != nil {
		response.InternalError(c, "标记已读失败")
		return
	}
	response.Success(c, "已读")
}
