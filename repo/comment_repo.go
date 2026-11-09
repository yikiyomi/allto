package repo

import (
	"allto/model"
	"gorm.io/gorm"
)

type CommentRepo struct {
	db *gorm.DB
}

func NewCommentRepo(db *gorm.DB) *CommentRepo {
	return &CommentRepo{db: db}
}
func (r *CommentRepo) WithTx(tx *gorm.DB) *CommentRepo {
	return &CommentRepo{db: tx}
}

// 发评论
func (r *CommentRepo) Create(c *model.Comment) error {
	return r.db.Create(c).Error
}

// 帖子评论列表
func (r *CommentRepo) ListByPost(postID uint) ([]model.Comment, error) {
	var Comments []model.Comment
	err := r.db.Preload("User").Where("post_id = ?", postID).Order("created_at desc").Find(&Comments).Error
	return Comments, err
}
