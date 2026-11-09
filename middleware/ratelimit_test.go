package middleware

import (
	"allto/database"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRateLimit(t *testing.T) {
	//内存redis
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis启动失败:%v", err)
	}
	defer mr.Close()
	//注入redis全局客户端
	if err := database.InitRedis(mr.Addr(), ""); err != nil {
		t.Fatalf("连接miniredis失败:%v", err)
	}
	gin.SetMode(gin.TestMode)

	limit := 3
	//同ip连续请求
	for i := 1; i <= limit+1; i++ {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/", nil)
		c.Request.RemoteAddr = "127.0.0.1:9999"

		RateLimit(limit)(c)
		if i <= limit {
			if w.Code != http.StatusOK {
				t.Errorf("第 %d 次需放行，实际返回 %d", i, w.Code)
			}
		} else {
			if w.Code != http.StatusTooManyRequests {
				t.Errorf("第 %d 次当被限流，实际返回 %d", i, w.Code)
			}
		}
	}
}
