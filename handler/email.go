package handler

import (
	"allto/database"
	"allto/repo"
	"allto/response"
	"allto/util"
	"fmt"
	"math/rand"
	"net/mail"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// 验证码校验
func VerifyEmailCode(c *gin.Context) {
	userRepo:=repo.NewUserRepo(database.GetDB())
	var req struct {
		Email string `json:"email" binding:"required,email"`
		Code  string `json:"code" binding:"required"`
	}
	if err := c.ShouldBind(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	//redis中获取验证
	storedCode, err := database.GetVerifyCode(req.Email)
	if err != nil {
		if err == redis.Nil {
			response.BadRequest(c, "验证码过期请重试")
		} else {
			response.InternalError(c, "服务端出错")
		}
		return
	}
	key := "verify_code:" + req.Email

	//对比
	if storedCode != req.Code {
		response.BadRequest(c, "验证码错误")
		return
	}
	// 验证成功：更新用户邮箱验证状态
	user,err:=userRepo.GetByEmail(req.Email)
	if  err != nil {
		response.NotFound(c, "用户不存在")
		return
	}

	if err=userRepo.UpdateColumn(user.ID,"email_verified",true);err != nil {
		response.InternalError(c, "更新验证状态失败")
		return
	}

	// 删除 Redis 中的验证码，防止重复使用 
	if err := database.DelCache(key); err != nil {
		response.BadRequest(c, "验证码清除失败")
		return
	}

	response.Success(c, gin.H{
		"email_verified": true,
		"user_id":        user.ID,
	})
}

// 发验证码
func SendVerifyCode(c *gin.Context) {
	userID := c.GetUint("userID")
	userRepo:=repo.NewUserRepo(database.GetDB())
	var req struct {
		Email string `json:"email" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if _, err := mail.ParseAddress(req.Email); err != nil {
		response.BadRequest(c, "邮箱格式错误")
		return
	}

	_,err:=userRepo.GetByEmail(req.Email)
	if err == nil {
		response.NotFound(c, "邮箱被占用")
		return
	} else if err != gorm.ErrRecordNotFound {
		response.InternalError(c, "发送验证码:系统繁忙，请稍后重试")
		return
	}
	// 把邮箱绑定到当前用户（未验证状态）
	err=userRepo.UpdateColumn(userID,"email",req.Email)
	if err != nil {
		response.InternalError(c, "更新邮箱失败")
		return
	}

	// 生成验证 码
	code := fmt.Sprintf("%06d", rand.Intn(1000000))

	// 存入redis
	if err := database.SetVerifyCode(req.Email, code, 5*time.Minute); err != nil {
		util.Logger.Error("验证码存储失败", zap.Error(err))
		response.InternalError(c, "验证码发送失败")
		return
	}

	// 异步发送验证邮件，链接从请求 Host 推断（本地与部署都适用）
	go func() {
		if err := util.SendVerifyEmail(req.Email, "验证码是"+code); err != nil {
			util.Logger.Error("发送验证邮件失败", zap.Uint("user_id", userID), zap.String("email", req.Email), zap.Error(err))
		}
	}()

	response.Success(c, "验证邮件已发送，请查收")
}
