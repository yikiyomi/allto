package config

import (
	"fmt"
	"github.com/spf13/viper"
	"strings"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	RabbitMQ RabbitMQConfig
	JWT      JWtConfig
	SMTP     SmtpConfig
}

type RabbitMQConfig struct{
	Host  string
	Port  int
	Username string
	Password string
}

type ServerConfig struct {
	Port string
}
type DatabaseConfig struct {
	DSN string
}
type RedisConfig struct {
	Addr     string
	Password string
}
type JWtConfig struct {
	Secret string
}
type SmtpConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

var AppConfig *Config

func Load() error {
	AppConfig = &Config{}
	viper.Unmarshal(AppConfig)
	viper.SetConfigName("config") // 文件名 config.yaml
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")        // 默认当前目录
	viper.AddConfigPath("./config") // 也可以放在 config/ 目录下

	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}
	//环境变量覆盖
	viper.SetEnvPrefix("APP")
	//允许viper自动读取变脸
	viper.AutomaticEnv()
	//将viper的.换成_
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	// ---------- 绑定特定的环境变量到配置键 ----------
	// 虽然 AutomaticEnv 会自动绑定所有键，但显式绑定可以确保只绑定我们需要的
	// 这里列出所有配置键，方便维护
	bindEnv("server.port")
	bindEnv("database.dsn")
	bindEnv("redis.addr")
	bindEnv("redis.password")
	bindEnv("jwt.secret")
	bindEnv("smtp.host")
	bindEnv("smtp.port")
	bindEnv("smtp.username")
	bindEnv("smtp.password")
	bindEnv("smtp.from")
	bindEnv("rabbitmq.host")
	bindEnv("rabbitmq.port")
	bindEnv("rabbitmq.username")
	bindEnv("rabbitmq.password")
	if err := viper.Unmarshal(&AppConfig); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}
	return nil
}

// bindEnv 将配置键映射到环境变量，格式为 APP_xxx_yyy
func bindEnv(key string) {
	// viper 会自动把 key 的 "." 替换为 "_" 并转为大写，加上前缀
	// 例如 jwt.secret -> APP_JWT_SECRET
	viper.BindEnv(key)
}
