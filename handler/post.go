package handler

import (
	"allto/database"
	"allto/model"
	"allto/repo"
	"allto/response"
	"allto/util"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// singleflight 分组，防止缓存击穿
var postSingleflight singleflight.Group

// 缓存常量
const (
	postCacheKeyPrefix = "post:"
	postCacheEmpty     = "EMPTY"          // 空值缓存哨兵
	postCacheTTL       = 1 * time.Hour    // 正常缓存 1 小时
	postEmptyTTL       = 1 * time.Minute  // 空值缓存 1 分钟
	postCacheJitter    = 10 * time.Minute // 随机抖动上限，防雪崩
)

// 发帖
func CreatePost(c *gin.Context) {
	userID := c.GetUint("userID")
	postRepo := repo.NewPostRepo(database.GetDB())
	var req struct {
		Title   string `json:"title" binding:"required"`
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	post := model.Post{Title: req.Title, Content: req.Content, UserID: userID}
	postRepo.Create(&post)
	response.Success(c, post)

}

// 帖子列表
func ListPosts(c *gin.Context) {
	//cursor是上一页最后一条id
	postRepo := repo.NewPostRepo(database.GetDB())
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

	posts, err := postRepo.List(uint(cursor), pageSize)
	if err != nil {
		response.InternalError(c, "数据库查询失败")
		return
	}
	//查有没有下一页
	hasMore := len(posts) > pageSize
	if hasMore {
		posts = posts[:pageSize]
	}
	//下一页游标=最后一条的id
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

// 帖子详细
func GetPost(c *gin.Context) {
	id := c.Param("id")
	cacheKey := postCacheKeyPrefix + id
	postRepo := repo.NewPostRepo(database.GetDB())
	//先查 Redis 缓存
	if data, err := database.GetCache(cacheKey); err == nil {
		if data == postCacheEmpty {
			response.NotFound(c, "帖子不存在")
			return
		}

		var post model.Post
		if err := json.Unmarshal([]byte(data), &post); err == nil {
			//阅读数统计（redis歇逼,降级）
			if err = database.SafeIncr("post_view:" + id); err != nil {
				util.Logger.Warn("阅读数统计失败(降级)", zap.Error(err))
			}
			response.Success(c, post)
			return
		}
		database.DelCache(cacheKey)
	}
	//singleflight查sql(合并请求防击穿)
	result, err, _ := postSingleflight.Do(id, func() (interface{}, error) {
		postID, parseErr := strconv.ParseUint(id, 10, 64)
		if parseErr != nil {
			return nil, fmt.Errorf("无效帖子id")
		}
		post, dbErr := postRepo.GetByID(uint(postID))
		if dbErr != nil {
			if errors.Is(dbErr, gorm.ErrRecordNotFound) {
				//查不到就空缓存一分钟
				database.SetCache(cacheKey, postCacheEmpty, postEmptyTTL)
				return nil, fmt.Errorf("帖子不存在")
			}
			return nil, dbErr
		}
		//查到缓存一小时+随机抖动（防雪崩
		data, _ := json.Marshal(post)
		jitter := time.Duration(rand.Int63n(int64(postCacheJitter)))
		database.SetCache(cacheKey, string(data), postCacheTTL+jitter)

		return *post,nil
	})
	if err != nil {
		response.NotFound(c, "帖子不存在")
		return
	}
	post := result.(model.Post)
	//阅读数统计
	if err := database.SafeIncr("post_view:" + id); err != nil {
		util.Logger.Warn("阅读数统计失败（降级）", zap.Error(err))
	}
	response.Success(c, post)
}
