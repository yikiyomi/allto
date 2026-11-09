package repo

import (
	"allto/model"
	"gorm.io/gorm"
)

type NotificationRepo struct {
	db *gorm.DB
}

func NewNotificationRepo(db *gorm.DB) *NotificationRepo {
	return &NotificationRepo{db: db}
}
func (r *NotificationRepo) WithTx(tx *gorm.DB) *NotificationRepo {
	return &NotificationRepo{db: tx}
}

// 创建通知（后到worker
func (r *NotificationRepo) Create(n *model.Notification) error {
	return r.db.Create(n).Error
}

// 通知列表
func (r *NotificationRepo) ListByRecipient(userID uint, limit int) ([]model.Notification, error) {
	var list []model.Notification
	err := r.db.Where("recipient_id=?", userID).Order("created_at desc").Limit(50).Find(&list).Error
	return list, err
}

// 未读计数
func (r *NotificationRepo) CountUnread(userID uint) (int64, error) {
	var count int64
	err := r.db.Model(&model.Notification{}).Where("recipient_id = ? AND is_read = ?", userID, false).Count(&count).Error
	return count, err
}

// 标记已读
func (r *NotificationRepo) MarkRead(userID uint, id uint) error {
	q := r.db.Model(&model.Notification{}).Where("recipient_id = ?", userID)
	if id > 0 {
		q = q.Where("id = ?", id)
	}
	return q.Update("is_read", true).Error
}
