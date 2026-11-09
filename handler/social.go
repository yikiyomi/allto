package handler

import (
	"allto/database"
	"allto/model"
	"allto/repo"
	"allto/response"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// 关注用户
func FollowUser(c *gin.Context) {
	followID := c.GetUint("userID")
	followingID, err := strconv.Atoi(c.Param("id"))
	followRepo := repo.NewFollowRepo(database.GetDB())
	userRepo := repo.NewUserRepo(database.GetDB())
	if err != nil {
		response.BadRequest(c, "无效用户id")
		return
	}
	//不能关注自己
	if followID == uint(followingID) {
		response.BadRequest(c, "不能关注自己")
		return
	}
	// 检查关注的用户是否存在
	_, err = userRepo.GetByID(uint(followingID))
	if err != nil {
		response.NotFound(c, "关注的用户不存在")
		return
	}
	//是否已关注
	_, err = followRepo.Get(followID, uint(followingID))
	if err == nil {
		response.BadRequest(c, "已关注")
		return
	}
	// 事务;创建关注记录
	nfollow := model.Follow{FollowID: followID, FollowingID: uint(followingID)}
	outboxRepo:=repo.NewOutboxRepo(database.GetDB())
	err=database.GetDB().Transaction(func(tx *gorm.DB)error{
		if err:=followRepo.WithTx(tx).Create(&nfollow);err!=nil{
			return err
		}
		return enqueueNotification(tx,outboxRepo,uint(followingID),followID,"follow",uint(followingID))
	})
	if err != nil {
		response.InternalError(c, "创建关注记录失败")
		return
	}
	// go Notify(uint(followingID), followID, "follow", uint(followingID))
	response.Success(c, gin.H{"message": "关注成功"})
}

// 取消关注
func UnfollowUser(c *gin.Context) {
	followID := c.GetUint("userID")
	followingID, _ := strconv.Atoi(c.Param("id"))
	followRepo := repo.NewFollowRepo(database.GetDB())

	result, err := followRepo.Delete(followID, uint(followingID))
	if err != nil {
		response.InternalError(c, "删除关注记录出错")
		return
	}
	if result == 0 {
		response.BadRequest(c, "未关注该用户")
		return
	}
	response.Success(c, "取消成功")
}

// 查看关注列表
func Listfollowing(c *gin.Context) {
	userID := c.GetUint("userID")
	followRepo := repo.NewFollowRepo(database.GetDB())
	userRepo := repo.NewUserRepo(database.GetDB())

	// 获取关注的用户ID列表
	ids, err := followRepo.ListFollowingIDs(userID)
	if err != nil {
		response.InternalError(c, "查询关注列表失败")
		return
	}

	// 批量查询用户信息
	users, err := userRepo.GetByIDs(ids)
	if err != nil {
		response.InternalError(c, "查询用户信息失败")
		return
	}
	response.Success(c, users)
}

// 查看粉丝列表
func GetFollowers(c *gin.Context) {
	userID := c.GetUint("userID")
	followRepo := repo.NewFollowRepo(database.GetDB())
	userRepo := repo.NewUserRepo(database.GetDB())

	// 获取粉丝ID列表
	ids, err := followRepo.ListFollowerIDs(userID)
	if err != nil {
		response.InternalError(c, "查询粉丝列表失败")
		return
	}
	
	// 批量查询用户信息
	users, err := userRepo.GetByIDs(ids)
	if err != nil {
		response.InternalError(c, "查询用户信息失败")
		return
	}
	response.Success(c, users)
}
