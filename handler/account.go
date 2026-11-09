package handler

import (
	"allto/database"
	"allto/model"
	"allto/repo"
	"allto/response"
	"allto/util"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// 注册
func Register(c *gin.Context) {
	userRepo := repo.NewUserRepo(database.GetDB())
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	// 检查重名
	_, err := userRepo.GetByUsername(req.Username)
	switch {
	case err == nil:
		response.Conflict(c, "用户名已存在")
		return
	case errors.Is(err, gorm.ErrRecordNotFound):
	default:
		response.InternalError(c, "查询失败")
		return
	}

	//加密密码
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		util.Logger.Error("密码加密失败")
		response.InternalError(c, "注册失败,请稍后再试")
		return
	}
	user := model.User{Username: req.Username, Password: string(hashed)}
	userRepo.Create(&user)
	response.Success(c, gin.H{"message": "注册成功", "user_id": user.ID})
}

// 登录
func Login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	userRepo := repo.NewUserRepo(database.GetDB())
	user, err := userRepo.GetByUsername(req.Username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Unauthorized(c, "账号不存在")
		} else {
			response.InternalError(c, "查询失败")
		}
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		response.Unauthorized(c, "密码错误")
		return
	}
	//生成双token并存入数据库
	token, err := util.GenerateToken(user.ID, user.Username)
	if err != nil {
		response.InternalError(c, "token生成失败")
		return
	}
	refreshToken, err := util.GenerateRefreshToken(32)
	if err != nil {
		response.InternalError(c, "refresh token生成失败")
		return
	}
	if err := userRepo.Update(user.ID, map[string]interface{}{
		"token":         token,        // 当前 access_token
		"refresh_token": refreshToken, // 当前 refresh_token
	}); err != nil {
		util.Logger.Error("登录时的双token存入数据库失败")
		response.InternalError(c, "token存入数据库失败")
		return
	}

	response.Success(c, gin.H{
		"token":         token,
		"refresh_token": refreshToken,
		"user": gin.H{
			"id":             user.ID,
			"username":       user.Username,
			"email":          user.Email,
			"email_verified": user.EmailVerified}})
}

// 登出
func Logout(c *gin.Context) {
	userID := c.GetUint("userID")
	userRepo := repo.NewUserRepo(database.GetDB())
	if err := userRepo.Update(userID, map[string]interface{}{
		"token":         "",
		"refresh_token": "",
	}); err != nil {
		util.Logger.Error("登出时的双token存入数据库失败")
		response.InternalError(c, "token存入数据库失败")
		return
	}
	response.Success(c, "已登出")
}

// 头像
func UploadAvatar(c *gin.Context) {
	userID := c.GetUint("userID")
	userRepo := repo.NewUserRepo(database.GetDB())
	//获取文件
	file, header, err := c.Request.FormFile("avatar")
	if err != nil {
		return
	}
	defer file.Close()

	ext := filepath.Ext(header.Filename)
	allowedExts := map[string]bool{".jpg": true, ".png": true}
	if !allowedExts[ext] {
		response.BadRequest(c, "头像仅支持jpg,png")
		return
	}
	if header.Size > 5*1024*1024 {
		response.BadRequest(c, "头像大小不超过5MB")
		return
	}

	filename := fmt.Sprintf("%s%s", uuid.NewString(), ext)
	uploadDir := "./uploads/avatars"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		response.InternalError(c, "服务器文件系统错误")
		return
	}

	destPath := filepath.Join(uploadDir, filename)
	dst, err := os.Create(destPath)
	if err != nil {
		response.InternalError(c, "保存头像失败")
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		response.InternalError(c, "写入头像失败")
		return
	}

	avatarURL := fmt.Sprintf("/uploads/avatars/%s", filename)
	if err := userRepo.Update(userID, map[string]interface{}{"avatar_url": avatarURL}); err != nil {
		os.Remove(destPath)
		response.InternalError(c, "更新头像记录失败")
		return
	}
	response.Success(c, gin.H{"avatar_url": avatarURL})
}

// 改密码
func ChangePassword(c *gin.Context) {
	userID := c.GetUint("userID")
	userRepo := repo.NewUserRepo(database.GetDB())
	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误"+err.Error())
		return
	}
	user, err := userRepo.GetByID(userID)
	if err != nil {
		response.BadRequest(c, "用户不存在")
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)); err != nil {
		response.Unauthorized(c, "旧密码错误")
		return
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		response.InternalError(c, "密码加密失败")
		return
	}
	err = userRepo.Update(userID, map[string]interface{}{
		"password":      string(hashed),
		"token":         "",
		"refresh_token": "",
	})
	if err != nil {
		response.InternalError(c, "密码更新失败")
		return
	}
	response.Success(c, "修改成功，请重新登陆")
}

// 改名
func Rename(c *gin.Context) {
	userID := c.GetUint("userID")
	userRepo := repo.NewUserRepo(database.GetDB())
	var req struct {
		NewUsername string `json:"new_username" binding:"required"`
	}
	user, err := userRepo.GetByUsernameExcept(req.NewUsername, userID)
	if err == nil {
		response.Conflict(c, "用户名已被占用")
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		response.InternalError(c, "查询用户失败")
		return
	}
	if user, err = userRepo.GetByID(userID); err != nil {
		response.Unauthorized(c, "用户不存在")
		return
	}

	if err := userRepo.UpdateColumn(userID, "username", req.NewUsername); err != nil {
		response.InternalError(c, "更新用户名失败")
		return
	}
	newToken, err := util.GenerateToken(user.ID, req.NewUsername)
	if err != nil {
		response.InternalError(c, "token生成失败")
		return
	}

	if err := userRepo.UpdateColumn(userID, "token", newToken); err != nil {
		response.InternalError(c, "更新 token 失败")
		return
	}
	response.Success(c, gin.H{
		"token":    newToken,
		"username": req.NewUsername,
	})
}

// 简介
func UpdateProfile(c *gin.Context) {
	userID := c.GetUint("userID")
	userRepo := repo.NewUserRepo(database.GetDB())
	var req struct {
		Bio string `json:"bio"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := userRepo.UpdateColumn(userID, "bio", req.Bio); err != nil {
		response.InternalError(c, "简介更新失败")
		return
	}

	response.Success(c, "修改成功")
}

// 用户详细
func UserProfile(c *gin.Context) {
	userID, err := strconv.Atoi(c.Param("id"))
	userRepo := repo.NewUserRepo(database.GetDB())
	postRepo := repo.NewPostRepo(database.GetDB())
	followRepo := repo.NewFollowRepo(database.GetDB())
	if err != nil {
		response.BadRequest(c, "无效用户id")
		return
	}
	user, err := userRepo.GetByID(uint(userID))
	if err != nil {
		response.NotFound(c, "找不到该用户")
		return
	}

	var postCount int64
	var followerCount int64
	var followingCount int64

	postCount, err = postRepo.CountByUser(uint(userID))
	if err != nil {
		util.Logger.Error("查询帖子数失败", zap.Error(err))
	}
	followerCount, err = followRepo.CountFollowers(uint(userID))
	if err != nil {
		util.Logger.Error("查询粉丝数失败", zap.Error(err))
	}
	followingCount, err = followRepo.CountFollowing(uint(userID))
	if err != nil {
		util.Logger.Error("查询关注数失败", zap.Error(err))
	}

	// database.GetDB().Model(&model.Post{}).Where("user_id = ?", userID).Count(&postCount)            //贴子数
	// database.GetDB().Model(&model.Follow{}).Where("following_id = ?", userID).Count(&followerCount) // 粉丝
	// database.GetDB().Model(&model.Follow{}).Where("follow_id = ?", userID).Count(&followingCount)   // 关注

	//打包返回
	response.Success(c, gin.H{
		"id":              user.ID,
		"username":        user.Username,
		"created_at":      user.CreatedAt,
		"post_count":      postCount,
		"bio":             user.Bio,
		"follower_count":  followerCount,
		"following_count": followingCount,
		"avatar_url":      user.AvatarUrl,
	})
}

// 用户帖子
func UserPosts(c *gin.Context) {
	userID, err := strconv.Atoi(c.Param("id"))
	postRepo := repo.NewPostRepo(database.GetDB())
	if err != nil {
		response.BadRequest(c, "无效用户id")
		return
	}
	cursor, err := strconv.Atoi(c.DefaultQuery("cursor", "0"))
	if err != nil {
		response.BadRequest(c, "cursor 页码解析错误")
		return
	}
	pageSize, err := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	if err != nil {
		response.BadRequest(c, "pagesize 页码解析错误")
		return
	}
	if pageSize > 50 {
		pageSize = 50
	}

	posts, err := postRepo.ListByUser(uint(userID), uint(cursor), pageSize)
	if err != nil {
		response.InternalError(c, "数据库查询失败")
		return
	}
	// 判断是否有下一页
	hasMore := len(posts) > pageSize
	if hasMore {
		posts = posts[:pageSize]
	}

	// 计算下一页游标
	var nextCursor uint
	if len(posts) > 0 {
		nextCursor = posts[len(posts)-1].ID
	}
	response.Success(c, gin.H{
		"data":        posts,
		"next_cursor": nextCursor,
		"has_more":    hasMore,
	})
}

// refresh token
func Refresh(c *gin.Context) {
	userRepo := repo.NewUserRepo(database.GetDB())
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	user, err := userRepo.GetByRefreshToken(req.RefreshToken)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Unauthorized(c, "refresh token无效或过期")
		} else {
			response.InternalError(c, "服务端错误")
		}
		return
	}
	newAccessToken, err := util.GenerateToken(user.ID, user.Username)
	if err != nil {
		response.InternalError(c, "生成 access token 失败")
		return
	}

	// 3. 生成新的 refresh_token（滚动更新）
	newRefreshToken, err := util.GenerateRefreshToken(16)
	if err != nil {
		response.InternalError(c, "生成 refresh token 失败")
		return
	}
	updates := map[string]interface{}{
		"token":         newAccessToken,
		"refresh_token": newRefreshToken,
	}

	if err := userRepo.Update(user.ID, updates); err != nil {
		response.InternalError(c, "更新token失败")
		return
	}
	response.Success(c, gin.H{
		"token":         newAccessToken,
		"refresh_token": newRefreshToken,
	})
}
