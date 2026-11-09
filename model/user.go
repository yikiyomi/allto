package model

import "time"

type User struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	Username      string    `gorm:"unique;not null" json:"username"`
	Password      string    `gorm:"not null" json:"-"`
	CreatedAt     time.Time `json:"created_at"`
	Email         string    `json:"email"`
	EmailVerified bool      `gorm:"default:false" json:"email_verified"`
	AvatarUrl     string    `json:"avatar_url"`
	Token         string    `json:"-"`
	RefreshToken  string    `json:"-"`
	Bio           string    `json:"bio"`
}
