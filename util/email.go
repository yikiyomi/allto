package util

import (
	"allto/config"
	"fmt"
	"net/smtp"
	"time"

	"go.uber.org/zap"
)

// 发邮件
func SendVerifyEmail(to, verifyLink string) error {
	//读config配置
	host := config.AppConfig.SMTP.Host
	port := config.AppConfig.SMTP.Port
	username := config.AppConfig.SMTP.Username
	password := config.AppConfig.SMTP.Password
	from := config.AppConfig.SMTP.From
	//写邮件
	subject := "AllTo 博客 - 邮箱验证"
	body := fmt.Sprintf("验证码：\n\n%s\n\n如果这不是你的操作请忽略此邮件。", verifyLink)
	msg := "From: " + from + "\r\n" +
		"To: " + to + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"\r\n" +
		body

	addr := fmt.Sprintf("%s:%d", host, port)
	auth := smtp.PlainAuth("", username, password, host)

	// 587 端口需要 STARTTLS
	return smtp.SendMail(addr, auth, from, []string{to}, []byte(msg))
}

// 重试函数
func SendVerifyEmailWithRetry(to, content string) {
	for attempt := 0; attempt < 3; attempt++ {
		if err := SendVerifyEmail(to, content); err == nil {
			return
		}
		time.Sleep(time.Duration(1<<attempt) * time.Second)
	}
	Logger.Error("发送失败，已重试三次",
		zap.String("to", to),
	)
}
