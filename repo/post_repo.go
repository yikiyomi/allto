package repo

import (
	"allto/model"

	"gorm.io/gorm"
)

type PostRepo struct {
	db *gorm.DB
}

func NewPostRepo(db *gorm.DB) *PostRepo {
	return &PostRepo{db: db}
}

// create发帖
func (r *PostRepo) Create(p *model.Post) error {
	return r.db.Create(p).Error
}

// getbyid查帖子详细（带作者....检查帖子存在
func (r *PostRepo) GetByID(id uint) (*model.Post, error) {
	var post model.Post
	err := r.db.Preload("User").First(&post, id).Error
	return &post, err
}

// list 帖子列表（游标分页，返回limit+1条，给handler判断has_more
func (r *PostRepo) List(cursor uint, limit int) ([]model.Post, error) {
	var posts []model.Post
	query := r.db.Preload("User").Order("id desc").Limit(limit + 1)
	if cursor > 0 {
		query = query.Where("id < ?", cursor)
	}
	err := query.Find(&posts).Error
	return posts, err
}

// ListByUser某用户帖子（游标分页
func (r *PostRepo) ListByUser(userID, cursor uint, limit int) ([]model.Post, error) {
	var posts []model.Post
	query := r.db.Preload("User").Where("user_id = ?", userID).Order("id desc").Limit(limit + 1)
	if cursor > 0 {
		query = query.Where("id < ?", cursor)
	}
	err := query.Find(&posts).Error
	return posts, err
}

// CountByUser 用户帖子数
func (r *PostRepo) CountByUser(userID uint) (int64, error) {
	var count int64
	err := r.db.Model(&model.Post{}).Where("user_id = ?", userID).Count(&count).Error
	return count, err
}

// IncrementLikeCount 点赞数 +delta（delta 可为负数，取消点赞时保证不小于0）
func (r *PostRepo) IncrementLikeCount(postID uint, delta int) error {
	expr := "like_count + ?"
	if delta < 0 {
		expr = "greatest(like_count + ?, 0)"
	}
	return r.db.Model(&model.Post{}).Where("id = ?", postID).
		Update("like_count", gorm.Expr(expr, delta)).Error
}
func (r *PostRepo) WithTx(tx *gorm.DB) *PostRepo {
	return &PostRepo{db: tx}
}
