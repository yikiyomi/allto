package repo

import (
	"allto/model"

	"gorm.io/gorm"
)

type FollowRepo struct {
	db *gorm.DB
}

func NewFollowRepo(db *gorm.DB) *FollowRepo {
	return &FollowRepo{db: db}
}
func (r *FollowRepo) WithTx(tx *gorm.DB) *FollowRepo {
	return &FollowRepo{db: tx}
}

// 是否已关注
func (r *FollowRepo) Get(followID, followingID uint) (*model.Follow, error) {
	var follow model.Follow
	err := r.db.Where("follow_id = ? AND following_id = ?", followID, followingID).First(&follow).Error
	return &follow, err
}

// 创建关注
func (r *FollowRepo) Create(f *model.Follow) error {
	return r.db.Create(f).Error
}

// 取消关注，返回 RowsAffected
func (r *FollowRepo) Delete(followID, followingID uint) (int64, error) {
	result := r.db.Where("follow_id = ? AND following_id = ?", followID, followingID).Delete(&model.Follow{})
	return result.RowsAffected, result.Error
}

// 关注的人 ID
func (r *FollowRepo) ListFollowingIDs(userID uint) ([]uint, error) {
	var ids []uint
	err := r.db.Model(&model.Follow{}).Where("follow_id = ?", userID).Pluck("following_id", &ids).Error
	return ids, err
}

// 粉丝 ID
func (r *FollowRepo) ListFollowerIDs(userID uint) ([]uint, error) {
	var ids []uint
	err := r.db.Model(&model.Follow{}).Where("following_id = ?", userID).Pluck("follow_id", &ids).Error
	return ids, err
}

// 粉丝数
func (r *FollowRepo) CountFollowers(userID uint) (int64, error) {
	var count int64
	err := r.db.Model(&model.Follow{}).Where("following_id = ?", userID).Count(&count).Error
	return count, err
}

// 关注数
func (r *FollowRepo) CountFollowing(userID uint) (int64, error) {
	var count int64
	err := r.db.Model(&model.Follow{}).Where("follow_id = ?", userID).Count(&count).Error
	return count, err
}
