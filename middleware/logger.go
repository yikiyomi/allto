package middleware

import (
	"allto/util"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		if util.Logger != nil {
			util.Logger.Info("http_request",
				zap.String("method", c.Request.Method),
				zap.String("path", c.Request.URL.Path),
				zap.Int("status", c.Writer.Status()),
				zap.Duration("duration", time.Since(start)),
				zap.String("ip", c.ClientIP()),
			)
		}
	}
}
