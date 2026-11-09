package util

import (
	"allto/config"
	"testing"
)

func init() {
	config.AppConfig = &config.Config{
		JWT: config.JWtConfig{Secret: "test-secret-key"},
	}
}
func TestGenerateAndParseToken(t *testing.T) {
	//生成
	token, err := GenerateToken(10086, "testuser")
	if err != nil {
		t.Fatalf("生成token失败: %v", err)
	}
	//解析
	claims, err := ParseToken(token)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if claims.UserID != 10086 || claims.Username != "testuser" {
		t.Errorf("解析不对应： %+v", claims)
	}
}

func TestParseInvalidToken(t *testing.T) {
	_, err := ParseToken("错误token")
	if err == nil {
		t.Error("解析失败：错误的应该解析失败")
	}
}
