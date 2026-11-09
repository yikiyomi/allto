package repo

import (
	"allto/model"
	"gorm.io/gorm"
)

type UserRepo struct {
	db *gorm.DB
}

func NewUserRepo(db *gorm.DB) *UserRepo {
	return &UserRepo{db: db}
}
func (r *UserRepo) WithTx(tx *gorm.DB) *UserRepo {
	return &UserRepo{db: tx}
}
func (r *UserRepo) Create(u *model.User) error {
	return r.db.Create(u).Error
}
func (r *UserRepo) GetByUsername(username string) (*model.User, error) {
	var user model.User
	err := r.db.Where("username = ?", username).First(&user).Error
	return &user, err
}
func (r *UserRepo) GetByID(id uint) (*model.User, error) {
	var user model.User
	err := r.db.First(&user, id).Error
	return &user, err
}
func (r *UserRepo) GetByEmail(email string) (*model.User, error) {
	var user model.User
	err := r.db.Where("email = ?", email).First(&user).Error
	return &user, err
}
func (r *UserRepo) GetByRefreshToken(refreshToken string) (*model.User, error) {
	var user model.User
	err := r.db.Where("refresh_token = ?", refreshToken).First(&user).Error
	return &user, err
}

// 批量查多个id
func (r *UserRepo) GetByIDs(ids []uint) ([]model.User, error) {
	var users []model.User
	err := r.db.Where("id IN ?", ids).Find(&users).Error
	return users, err
}

// 查重名
func (r *UserRepo) GetByUsernameExcept(username string, excludeID uint) (*model.User, error) {
	var user model.User
	err := r.db.Where("username = ? AND id != ?", username, excludeID).First(&user).Error
	return &user, err
}
func (r *UserRepo) UpdateColumn(id uint, column string, value interface{}) error {
	return r.db.Model(&model.User{}).Where("id = ?", id).Update(column, value).Error
}
func (r *UserRepo) Update(id uint, fields map[string]interface{}) error {
	return r.db.Model(&model.User{}).Where("id = ?", id).Updates(fields).Error
}
