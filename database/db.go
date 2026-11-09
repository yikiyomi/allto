package database

import (
	"allto/mq"
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// 全局变量

var (
	db  *gorm.DB
	rdb *redis.Client
	ctx = context.Background()
	mqConn *mq.Rabbitmq
)

func InitDB(dsn string) error {
	var err error
	db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{TranslateError: true})
	return err
}

func InitRedis(addr, password string) error {
	rdb = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       0,
	})
	return rdb.Ping(ctx).Err()
}
func GetDB() *gorm.DB         { return db }
func GetRedis() *redis.Client { return rdb }
func GetCtx() context.Context { return ctx }

// Lua 脚本：原子 INCR + 条件 PEXPIRE（限流固定窗口）
var incrWithExpireScript = redis.NewScript(`
  local count = redis.call("INCR", KEYS[1])
  if count == 1 then
    redis.call("PEXPIRE", KEYS[1], ARGV[1])
  end
  return count
  `)

// IncrWithExpire 限流计数：自增并在首次设置过期
func IncrWithExpire(key string, ttl time.Duration) (int64, error) {
	if rdb == nil {
		return 0, errors.New("redis 未初始化")
	}
	return incrWithExpireScript.Run(ctx, rdb, []string{key}, ttl.Milliseconds()).Int64()
}

// SafeIncr 普通自增（阅读数等）
func SafeIncr(key string) error {
	if rdb == nil {
		return errors.New("redis 未初始化")
	}
	return rdb.Incr(ctx, key).Err()
}

// SAdd 集合添加（点赞记录等）
func SAdd(key string, member interface{}) error {
	if rdb == nil {
		return errors.New("redis 未初始化")
	}
	return rdb.SAdd(ctx, key, member).Err()
}

// SRem 集合删除（取消点赞等）
func SRem(key string, member interface{}) error {
	if rdb == nil {
		return errors.New("redis 未初始化")
	}
	return rdb.SRem(ctx, key, member).Err()
}

// 验证
func SetVerifyCode(email, code string, ttl time.Duration) error {
	if rdb == nil {
		return errors.New("redis 未初始化")
	}
	return rdb.Set(ctx, "verify_code:"+email, code, ttl).Err()
}

func GetVerifyCode(email string) (string, error) {
	if rdb == nil {
		return "", errors.New("redis 未初始化")
	}
	return rdb.Get(ctx, "verify_code:"+email).Result()
}

// GetCache 读缓存，Redis 挂了返回错误由调用方降级
func GetCache(key string) (string, error) {
	if rdb == nil {
		return "", errors.New("redis 未初始化")
	}
	return rdb.Get(ctx, key).Result()
}

// SetCache 写缓存
func SetCache(key, value string, ttl time.Duration) error {
	if rdb == nil {
		return errors.New("redis 未初始化")
	}
	return rdb.Set(ctx, key, value, ttl).Err()
}

// DelCache 删缓存
func DelCache(key string) error {
	if rdb == nil {
		return errors.New("redis 未初始化")
	}
	return rdb.Del(ctx, key).Err()
}

// 加一个初始化函数
func InitMQ(conn *mq.Rabbitmq) { mqConn = conn }

// 加一个 getter
func GetMQ() *mq.Rabbitmq { return mqConn }