package model
// NotificationEvent outbox payload 和 MQ 消息共用
  type NotificationEvent struct {
    EventID     string `json:"event_id"`
    RecipientID uint   `json:"recipient_id"`
    SenderID    uint   `json:"sender_id"`
    Type        string `json:"type"` // "like" / "comment" / "follow"
    TargetID    uint   `json:"target_id"`
}