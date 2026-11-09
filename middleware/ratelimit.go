package middleware

import (
	"allto/database"
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

func RateLimit(limit int) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := "rate_limit:" + c.ClientIP()
		count, err := database.IncrWithExpire(key, time.Minute) //reids歇逼放行
		if err != nil {
			c.Next()
			return
		}
		if count > int64(limit) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code": 429,
				"msg":  "请求过于频繁",
			})
			return
		}
		c.Next()
	}
}
