package model

import "time"

type Follow struct {
	FollowID    uint      `gorm:"primaryKey" json:"follow_id"`
	FollowingID uint      `gorm:"primaryKey" json:"following_id"`
	CreatedAt   time.Time `json:"created_at"`
}
