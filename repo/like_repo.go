package repo

import (
	"allto/model"

	"gorm.io/gorm"
)

type LikeRepo struct {
	db *gorm.DB
}

func NewLikeRepo(db *gorm.DB) *LikeRepo {
	return &LikeRepo{db: db}
}
func (r *LikeRepo) WithTx(tx *gorm.DB) *LikeRepo {
	return &LikeRepo{db: tx}
}

// 是否已点赞
func (r *LikeRepo) Get(userID, postID uint) (*model.Like, error) {
	var like model.Like
	err := r.db.Where("user_id=? and post_id=?", userID, postID).First(&like).Error
	return &like, err
}

// 创建点赞
func (r *LikeRepo) Create(l *model.Like) error {
	return r.db.Create(l).Error
}

// 取消点赞
func (r *LikeRepo) Delete(userID, postID uint) error {
	return r.db.Where("user_id=? AND post_id=?", userID, postID).Delete(&model.Like{}).Error
}

// 帖子的点赞记录
func (r *LikeRepo) ListByPost(postID uint) ([]model.Like, error) {
	var likes []model.Like
	err := r.db.Where("post_id = ?", postID).Order("created_at desc").Find(&likes).Error
	return likes, err
}

// 帖子的点赞用户
func (r *LikeRepo) ListLikers(postID uint) ([]model.User, error) {
	var likers []model.User
	err := r.db.Joins("JOIN likes ON users.id = likes.user_id").
		Where("likes.post_id = ?", postID).
		Select("users.id, users.username").
		Find(&likers).Error
	return likers, err
}

// 用户点过的赞
func (r *LikeRepo) ListByUser(userID uint) ([]model.Like, error) {
	var likes []model.Like
	err := r.db.Preload("Post").Where("user_id = ?", userID).Order("created_at desc").Find(&likes).Error
	return likes, err
}
