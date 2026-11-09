package router

import (
	"allto/handler"
	"allto/middleware"
	"github.com/gin-gonic/gin"
)

func SetupRouter() *gin.Engine {

	//注册路由
	r := gin.New()

	//检测是否存活
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	//加载html
	r.Static("/static", "./frontend")
	r.Static("/uploads", "./uploads")

	//中间件
	r.Use(middleware.CORS())
	r.Use(middleware.Recovery())
	r.Use(middleware.Logger())
	r.Use(middleware.RateLimit(60))

	r.SetTrustedProxies(nil)
	
	//公开接口
	r.POST("/register", handler.Register) //注册
	r.POST("/login", handler.Login)       //登录
	r.POST("/refresh", handler.Refresh)
	r.POST("/verify-email", handler.VerifyEmailCode) //验收验证码
	r.GET("/notifications/stream", handler.NotificationStream)

	//可选登录接口（登录可选，未登录也能访问）
	optional := r.Group("/api")
	optional.Use(middleware.SoftAuthMiddleware())
	{
		optional.GET("/posts", handler.ListPosts)                 //文章列表
		optional.GET("/posts/:id", handler.GetPost)               //文章详细
		optional.GET("/posts/:id/comments", handler.ListComments) //评论区
		optional.GET("/posts/:id/likers", handler.ListPostLikers) //点赞用户列表
	}

	//必须登录接口
	auth := r.Group("/api")
	auth.Use(middleware.AuthMiddleware())
	{
		auth.GET("/notifications", handler.ListNotifications)       //最近50条信息
		auth.POST("/notifications/markRead", handler.MarkRead)      //标记已读
		auth.GET("/notifications/unreadCount", handler.UnreadCount) //未读数
		auth.POST("/send-verify-code", handler.SendVerifyCode)      //发送验证码

		auth.POST("/logout", handler.Logout)                    //登出
		auth.POST("/rename", handler.Rename)                    //改名
		auth.POST("/changePassword", handler.ChangePassword)    //改密码
		auth.POST("/avatar", handler.UploadAvatar)              //上传头像
		auth.POST("/profile", handler.UpdateProfile)            //修改简介
		auth.GET("/users/:id", handler.UserProfile)             //用户资料
		auth.GET("/users/:id/posts", handler.UserPosts)         //用户文章列表
		auth.POST("/posts", handler.CreatePost)                 //创建帖子
		auth.POST("/posts/:id/like", handler.LikePost)          //点赞帖子
		auth.DELETE("/posts/:id/like", handler.UnlikePost)      //取消点赞
		auth.POST("/posts/:id/comments", handler.CreateComment) //评论
		auth.GET("/posts/likes", handler.ListMyLikes)           //当前用户点赞的帖子列表
		auth.POST("/follow/:id", handler.FollowUser)            //关注用户
		auth.DELETE("/follow/:id", handler.UnfollowUser)        //取消关注
		auth.GET("/following", handler.Listfollowing)           //查看关注列表
		auth.GET("/followers", handler.GetFollowers)            //查看粉丝列表
	}

	return r
}
