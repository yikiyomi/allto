package handler

import (
	"allto/model"
	"allto/repo"
	"encoding/json"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// enqueueNotification 在业务事务内写 outbox（业务数据 + 消息同事务原子）。
// recipientID == senderID 直接跳过（不给自己发通知），返回 nil 不影响业务提交。
func enqueueNotification(tx *gorm.DB, outboxRepo *repo.OutboxRepo,
	recipientID, senderID uint, typ string, targetID uint) error {

	if recipientID == senderID {
		return nil
	}
	ev := model.NotificationEvent{
		EventID:     uuid.NewString(),
		RecipientID: recipientID,
		SenderID:    senderID,
		Type:        typ,
		TargetID:    targetID,
	}
	b, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	return outboxRepo.WithTx(tx).Create(&model.Outbox{
		EventID:   ev.EventID,
		EventType: "notification",
		Payload:   string(b),
	})
}