package model

import "time"

type Like struct {
	UserID    uint      `gorm:"primaryKey" json:"user_id"`
	PostID    uint      `gorm:"primaryKey" json:"post_id"`
	Post      Post      `gorm:"foreignKey:PostID" json:"post,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
