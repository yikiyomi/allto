package model

import "time"

type Notification struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	RecipientID uint      `gorm:"index" json:"recipient_id"` //收消息人
	SenderID    uint      `json:"sender_id"`                 //发消息人
	Type        string    `json:"type"`                      //"like" / "comment" / "follow"
	TargetID    uint      `json:"target_id"`                 // 文章ID 或 用户ID
	DedupKey    string    `gorm:"uniqueIndex;size:36" json:"-"`       //存eventID，消费幂等
	IsRead      bool      `gorm:"default:false" json:"is_read"`
	CreatedAt   time.Time `json:"created_at"`
}
