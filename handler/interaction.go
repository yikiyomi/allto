package handler

import (
	"allto/database"
	"allto/model"
	"allto/repo"
	"allto/response"
	"allto/util"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// 帖子的点赞用户列表
func ListPostLikers(c *gin.Context) {
	postID := c.Param("id")
	likeRepo := repo.NewLikeRepo(database.GetDB())
	postIDInt, err := strconv.Atoi(postID)
	if err != nil {
		response.BadRequest(c, "无效帖子id")
		return
	}
	likers, err := likeRepo.ListLikers(uint(postIDInt))
	if err != nil {
		response.InternalError(c, "查询点赞的用户列表失败")
		return
	}
	response.Success(c, likers)
}

// 点赞
func LikePost(c *gin.Context) {
	userID := c.GetUint("userID")
	postID, _ := strconv.Atoi(c.Param("id"))
	postRepo := repo.NewPostRepo(database.GetDB())
	likeRepo := repo.NewLikeRepo(database.GetDB())
	// 检查帖子是否存在
	post, err := postRepo.GetByID(uint(postID))
	if err != nil {
		response.NotFound(c, "帖子不存在")
		return
	}
	//是否已点赞
	if _, err := likeRepo.Get(uint(userID), uint(postID)); err == nil {
		response.BadRequest(c, "已经点赞了")
		return
	}
	// 事务;创建点赞记录+增加帖子的like_count
	outboxRepo:=repo.NewOutboxRepo(database.GetDB())
	err = database.GetDB().Transaction(func(tx *gorm.DB) error {
		if err := likeRepo.WithTx(tx).Create(&model.Like{UserID: userID, PostID: uint(postID)}); err != nil {
			return err
		}
		if err:= postRepo.WithTx(tx).IncrementLikeCount(uint(postID), 1);err!=nil{
			return err
		}
		return enqueueNotification(tx,outboxRepo,post.UserID,userID,"like",uint(postID))
	})
	if err != nil {
		response.InternalError(c, "点赞失败")
		return
	}
	err = database.SAdd("post_like:"+c.Param("id"), userID) //reids歇逼放行
	if err != nil {
		util.Logger.Error("点赞缓存写入失败(降级)", zap.Error(err))
	}
	// go Notify(post.UserID, userID, "like", uint(postID))
	response.Success(c, "点赞成功")
}

// 取消点赞
func UnlikePost(c *gin.Context) {
	userID := c.GetUint("userID")
	postID, err := strconv.Atoi(c.Param("id"))
	likeRepo := repo.NewLikeRepo(database.GetDB())
	postRepo := repo.NewPostRepo(database.GetDB())
	if err != nil {
		response.BadRequest(c, "无效帖子id")
		return
	}
	// 事务
	err = database.GetDB().Transaction(func(tx *gorm.DB) error {
		// 删除点赞记录
		if err := likeRepo.WithTx(tx).Delete(uint(userID), uint(postID));err != nil {
			return err
		}
		// 减少like_count(保证不小于0)
		return postRepo.WithTx(tx).IncrementLikeCount(uint(postID), -1)
	})
	if err != nil {
		response.InternalError(c, "取消失败")
		return
	}
	if err = database.SRem("post_like:"+c.Param("id"), userID); err != nil {
		util.Logger.Error("取消点赞缓存删除失败(降级)", zap.Error(err))
	}
	response.Success(c, "取消成功")
}

// 评论列表
func ListComments(c *gin.Context) {
	postID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效帖子id")
		return
	}
	commentRepo := repo.NewCommentRepo(database.GetDB())
	comments, err := commentRepo.ListByPost(uint(postID))
	if err != nil {
		response.InternalError(c, "提取评论列表失败")
		return
	}
	response.Success(c, comments)
}

// 发表评论
func CreateComment(c *gin.Context) {
	userID := c.GetUint("userID")
	postID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid post id")
		return
	}
	postRepo := repo.NewPostRepo(database.GetDB())
	commentRepo := repo.NewCommentRepo(database.GetDB())
	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	// 检查帖子是否存在、
	
	post, err := postRepo.GetByID(uint(postID))
	if err != nil {
		response.NotFound(c, "帖子不存在")
		return
	}
	comment := model.Comment{
		Content: req.Content,
		UserID:  userID,
		PostID:  uint(postID),
	}
	outboxRepo:=repo.NewOutboxRepo(database.GetDB())
	err=database.GetDB().Transaction(func (tx *gorm.DB) error {
		if err:=commentRepo.WithTx(tx).Create(&comment);err!=nil{
			return err
		}
		return enqueueNotification(tx,outboxRepo,post.UserID,userID,"comment",uint(postID))
	})
	if err!=nil{
		response.InternalError(c,"评论失败：数据库写入失败")
		return
	}
	// go Notify(post.UserID, userID, "comment", uint(postID))
	response.Success(c, comment)
}

// 当前用户点赞列表
func ListMyLikes(c *gin.Context) {
	userID := c.GetUint("userID")
	likeRepo := repo.NewLikeRepo(database.GetDB())
	likes, err := likeRepo.ListByUser(userID)
	if err != nil {
		response.InternalError(c, "查询失败")
		return
	}
	response.Success(c, likes)
}
