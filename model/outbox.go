package model
import "time"
// outbox表：成功就删，表里剩pending
type Outbox struct{
	ID uint `gorm:"primaryKey"`
	EventID string `gorm:"uniqueIndex;size:36"`//uuid幂等键
	EventType string `gorm:"index"`//"notification"
	Payload string `gorm:"type:text"`//NotificationEvent的JSON
	CreatedAt time.Time
	UpdatedAt time.Time
}
