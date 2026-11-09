# allto 博客 — 项目状态交接文档

> 这份文档记录了项目当前进度，供新会话快速了解全貌。日期：2026-08-20。

## 项目概览

基于 Go + Gin 的博客系统，前后端一体（后端直接服务前端静态文件）。已完成账号、文章、互动、社交、实时通知、邮箱验证，以及完整的工程化（配置/Docker/CI/测试/降级/重试/日志/pprof）。

- 后端：`D:\iraikosu0\allto`（Go 1.26，module 名 `allto`）
- 前端：`frontend/index.html`（单文件原生 JS SPA，无框架）
- 对比项目：`D:\iraikosu0\feedsystem_video_go`（video 项目，更完整的参考）

## 技术栈

Gin + GORM + go-redis + JWT + bcrypt + Viper + zap + singleflight + miniredis（测试）

## 已完成功能

| 模块 | 内容 |
|------|------|
| 账号 | 注册、登录、登出、双 Token（access 1h + refresh 滚动更新）、改名、改密、头像上传、简介 |
| 邮箱 | 验证码验证（Redis 存验证码 5 分钟过期）、绑定邮箱（登录后可绑）、退避重试发邮件 |
| 文章 | 发布、列表（游标分页）、详情（缓存 + 阅读数）、用户文章 |
| 互动 | 点赞、取消点赞、评论、各种列表 |
| 社交 | 关注、取关、粉丝/关注列表 |
| 通知 | SSE 实时推送、通知列表、未读计数、已读标记 |
| 工程 | Viper 配置、Docker Compose（虚拟机跑通）、CI、Redis 降级、zap 日志、pprof、限流、软鉴权 |

## 目录结构

```
allto/
├── config/          Viper 配置加载 + 环境变量
├── database/        MySQL/Redis 连接 + 降级封装（SafeIncr/SAdd/SRem/IncrWithExpire/SetVerifyCode/GetVerifyCode）
├── frontend/        index.html（单文件 SPA）
├── handler/         account / post / interaction / social / email / notification
├── middleware/      auth / cors / logger(zap) / ratelimit(Redis) / ssehub
├── model/           User/Post/Comment/Like/Follow/Notification
├── response/        统一响应 {code, msg, data}
├── router/          路由注册
├── util/            jwt / email(重试) / logger(zap)
├── config.yaml      本地配置
├── config.docker.yaml  Docker 配置
└── main.go          入口
```

## 关键技术点（面试能讲）

1. **双 Token + 滚动更新**：access 1h，refresh 随机串存 DB，每次 refresh 都换新（旧 token 立即失效）
2. **邮箱验证码**：Redis 存验证码（`verify_code:` 前缀 + 5 分钟 TTL），发邮件带指数退避重试（1s→2s→4s）
3. **SSE 通知**：`middleware/ssehub.go`（内存 hub，`用户ID→channel`），点赞/评论/关注触发 `go Notify(...)`，token 放 query string（EventSource 不能带 header）
4. **Redis 降级**：database 包统一封装 nil 检查，Redis 挂了降级跳过（限流放行、阅读数跳过、缓存走 DB）
5. **缓存防护**：文章详情三级缓存思路——空值缓存（防穿透）、singleflight（防击穿）、TTL 随机抖动（防雪崩）
6. **游标分页**：`WHERE id < cursor ORDER BY id DESC`，替代 offset 深分页
7. **zap 日志**：`util/logger.go`（Logger + Sugar + dev 模式），请求日志中间件记方法/路径/状态码/耗时
8. **pprof**：main.go 开 goroutine 监听 `:6060`

## 接口约定（前后端）

公开（无鉴权）：`/register` `/login` `/refresh` `/verify-email` `/notifications/stream`
软鉴权（optional 组）：`/api/posts` `/api/posts/:id` `/api/posts/:id/comments` 等读接口
硬鉴权（auth 组）：写接口 + `/api/send-verify-code` `/api/users/:id` `/api/notifications` 等

关键约定：
- 响应统一 `{code:0, msg:"ok", data:...}`（response.Success 封装）
- 游标分页返回 `{data, next_cursor, has_more}`，前端传 `cursor` 参数
- SSE 的 token 走 query string：`/notifications/stream?token=xxx`
- 登录返回 `{token, refresh_token, user:{id, username, email, email_verified}}`

## 当前进度

✅ 全部功能 + 工程化已完成，项目可完整运行（本地 `go run main.go`，Docker `docker compose up -d --build`）

## 待办事项（按优先级）

1. **拼写清理**（低级错误，面试前必须做）：
2. **RabbitMQ 异步 Worker**（最大的架构升级，video 项目有，allto 没有）：点赞/评论/通知改异步落库
3. **三级缓存**：加 L1 本地内存缓存（现在是 Redis + MySQL 两级）
4. **更多测试**：handler 业务测试（需 mock DB）
5. **统一 Redis 封装**：GetPost 里的 `Get/Set/Del` 还是裸调，可加 `GetCache/SetCache/DelCache` 统一降级

## 注意事项

- **前端只能 Claude 改，后端用户自己写**（这是用户明确要求的边界，虽然过程中有时会让 Claude 改后端）
- 后端函数都用 `database.GetDB()/GetRedis()` 全局单例，没做依赖注入
- 数据库用 GORM AutoMigrate 自动建表（main.go 里列了 6 个 model）
- 测试：`util/jwt_test.go` + `middleware/ratelimit_test.go`（miniredis），`go test ./...` 可跑
- Docker 已配国内代理 `goproxy.cn`，虚拟机能正常 build

## 未来 Roadmap（五阶段）

### 阶段一：面试前打磨（近期，性价比最高）

1. 拼写清理（上文待办清单里那批）
2. `go vet ./...` + `gofmt -w .` 代码规范
3. 补 handler 业务测试（sqlmock 或 SQLite mock 测注册/登录）
4. 错误处理统一：残留的 `c.JSON(404,...)` 换成 `response.XXX()`
5. 统一 Redis 封装：GetPost 的裸 `Get/Set/Del` 收进 database 包

### 阶段二：技术深度（面试亮点）

1. **RabbitMQ 异步 Worker**：点赞/评论/通知改异步落库，API + Worker 进程分离（video 项目最大差距）
2. **Outbox 模式**：事务内写 outbox 表，后台投递 MQ，保证消息不丢
3. **三级缓存**：文章缓存加 L1 本地内存层
4. **分布式锁**：SETNX + Lua 释放，防并发重复操作
5. **幂等设计**：点赞/评论防重复提交（唯一索引 + 幂等键）
6. **复合索引**：likes/follows 表加联合索引支撑查询

### 阶段三：功能扩展（体量）

1. 文章标签/分类（`#话题`，可复用 video 项目 tag 逻辑）
2. Markdown 渲染（前端 marked.js）
3. 文章草稿/发布状态（Post 加 `status` 字段）
4. 全文搜索（MySQL FULLTEXT 或 Meilisearch）
5. 私信（点对点聊天，参考 video 的 message 模块）
6. @提及通知（评论里 @username 触发通知）

### 阶段四：架构升级（长期）

1. service/repo 分层（现在 handler 直接操作 DB，抽业务层和仓储层）
2. 依赖注入（替代 `database.GetDB()` 全局单例）
3. 数据库迁移（golang-migrate 替代 AutoMigrate，版本化）

### 阶段五：运维（长期，真实项目）

1. 指标监控（Prometheus + /metrics）
2. 日志采集（结构化日志接入 ELK/Loki）
3. HTTPS（TLS 证书 + Nginx 反代）

### 建议路线

```
现在 → 阶段一（拼写 + gofmt + 测试）      ← 面试前必须
然后 → 阶段二 RabbitMQ + Outbox        ← 最大亮点
可选 → 阶段三挑 1-2 个（标签/Markdown）
长期 → 阶段四/五
```

**核心判断**：功能已经够多，不要再堆功能。接下来最有价值的是「阶段一打磨」+「阶段二 RabbitMQ」——RabbitMQ 是唯一还能拉开质的差距的东西，证明你懂异步架构而不只是 CRUD。
