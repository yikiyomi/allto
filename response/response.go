package response

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "ok",
		"data": data,
	})
}

func Error(c *gin.Context, code int, msg string) {
	c.JSON(code, gin.H{
		"code": code,
		"msg":  msg,
		"data": nil,
	})
}

func BadRequest(c *gin.Context, msg string)    { Error(c, http.StatusBadRequest, msg) }
func Unauthorized(c *gin.Context, msg string)  { Error(c, http.StatusUnauthorized, msg) }
func NotFound(c *gin.Context, msg string)      { Error(c, http.StatusNotFound, msg) }
func Conflict(c *gin.Context, msg string)      { Error(c, http.StatusConflict, msg) }
func InternalError(c *gin.Context, msg string) { Error(c, http.StatusInternalServerError, msg) }
