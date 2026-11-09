# allto 博客

基于 Go + Gin 的博客系统，含账号、文章、点赞、评论、关注、实时通知，支持双 Token 鉴权、邮箱验证码、Redis 缓存降级、SSE 推送、结构化日志、Docker Compose 部署。

## 功能

| 模块 | 功能 |
|------|------|
| 账号 | 注册、登录、双 Token（access + refresh 滚动更新）、登出、改名、改密、头像上传、简介、邮箱验证码 |
| 文章 | 发布、列表（分页）、详情、Redis 阅读计数（降级）、用户文章 |
| 点赞 | 点赞、取消点赞、已赞列表、事务保证一致性、Redis 缓存（降级） |
| 评论 | 发布、列表 |
| 关注 | 关注、取关、粉丝列表、关注列表 |
| 通知 | 点赞/评论/关注事件通知、SSE 实时推送、未读计数、已读标记 |
| 工程 | Viper 配置、Docker Compose、CI、Redis 降级体系、退避重试、zap 结构化日志、限流、软鉴权、健康检查 |

## 技术栈

- 后端：Go 1.26 + Gin + GORM + go-redis + JWT + bcrypt + Viper + zap
- 前端：原生 JavaScript SPA（单页应用）

## Docker Compose 一键启动

```bash
docker compose up -d --build
```

访问：`http://localhost:8080/static`


## 本地开发

```bash
# 1. 安装依赖
go mod tidy

# 2. 配置（编辑 config.yaml，或设置环境变量）
# 3. 启动
go run main.go

# 访问前端：http://localhost:8080/static
```

## 配置说明

配置文件 `config.yaml`（Docker 环境用 `config.docker.yaml`）：

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `server.port` | `:8080` | 监听端口 |
| `database.dsn` | `root:123456@tcp(127.0.0.1:3306)/allto...` | MySQL 连接串 |
| `redis.addr` | `localhost:6379` | Redis 地址 |
| `redis.password` | 空 | Redis 密码 |
| `jwt.secret` | `my_secret_key` | JWT 密钥，生产须改 |
| `smtp.host` | `smtp.qq.com` | SMTP 服务器 |
| `smtp.port` | `587` | SMTP 端口 |
| `smtp.username` / `smtp.from` | QQ 邮箱 | 发件账号 |
| `smtp.password` | 空 | 授权码，用环境变量 `APP_SMTP_PASSWORD` 注入 |

## 接口清单

### 公开接口（无需登录）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/register` | 注册 |
| POST | `/login` | 登录，返回 access_token + refresh_token |
| POST | `/refresh` | 刷新 token（滚动更新） |
| POST | `/verify-email` | 校验邮箱验证码 |
| GET | `/notifications/stream` | SSE 通知流（token 放 query string） |

### 半公开接口（软鉴权，未登录也能访问）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/posts` | 文章列表（分页） |
| GET | `/api/posts/:id` | 文章详情（阅读数 +1） |
| GET | `/api/posts/:id/comments` | 评论列表 |
| GET | `/api/posts/:id/likes` | 点赞列表 |
| GET | `/api/posts/:id/likers` | 点赞用户列表 |

### 账号 `/api`（需 JWT）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/logout` | 登出（失效双 token） |
| POST | `/api/rename` | 改名 |
| POST | `/api/changePassword` | 改密码（清 token 强制重登） |
| POST | `/api/avatar` | 上传头像 |
| POST | `/api/profile` | 修改简介 |
| POST | `/api/send-verify-code` | 发送验证码（绑定邮箱） |
| GET | `/api/users/:id` | 用户资料（文章/粉丝/关注数） |
| GET | `/api/users/:id/posts` | 用户文章列表 |

### 互动 `/api`（需 JWT）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/posts` | 发布文章 |
| POST | `/api/posts/:id/like` | 点赞 |
| DELETE | `/api/posts/:id/like` | 取消点赞 |
| POST | `/api/posts/:id/comments` | 发表评论 |
| GET | `/api/posts/likes` | 我赞过的文章 |

### 关注 `/api`（需 JWT）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/follow/:id` | 关注用户 |
| DELETE | `/api/follow/:id` | 取消关注 |
| GET | `/api/following` | 我的关注 |
| GET | `/api/followers` | 我的粉丝 |

### 通知 `/api`（需 JWT）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/notifications` | 最近 50 条通知 |
| POST | `/api/notifications/markRead` | 标记已读（传 id 单条，不传全部） |
| GET | `/api/notifications/unreadCount` | 未读计数 |

## 目录结构

```
allto/
├── config/          配置加载（Viper + 环境变量）
├── database/        MySQL + Redis 连接、降级封装（SafeIncr/SAdd 等）
├── frontend/        前端 SPA
├── handler/         接口处理（account/post/interaction/social/email/notification）
├── middleware/      鉴权、CORS、限流、请求日志、SSE Hub
├── model/           GORM 数据模型
├── response/        统一响应格式
├── router/          Gin 路由注册
├── util/            JWT、邮件、日志封装
├── config.yaml      本地配置
├── config.docker.yaml  Docker 配置
└── main.go          入口
```

## 运维与可观测性

- `GET /healthz` 返回健康状态，Docker Compose 依赖它做健康检查。
- Redis 是可选依赖：所有 Redis 操作封装在 `database` 包统一做 nil 检查，挂了降级跳过（限流放行、阅读数跳过、点赞照常），服务不崩。
- 邮件发送带指数退避重试（1s → 2s → 4s，最多 3 次）。
- 日志使用 zap 结构化日志，请求日志中间件记录方法、路径、状态码、耗时。
