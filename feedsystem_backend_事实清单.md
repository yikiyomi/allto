# feedsystem_video_go/backend 工程做法事实清单

参考项目根：`D:\iraikosu0\feedsystem_video_go`（下称 R）。Go module 名 `feedsystem_video_go`。

## 1. Outbox 完整实现

**Model** `R\backend\internal\video\video_entity.go:42-48`
```go
type OutboxMsg struct {
	ID         uint      `gorm:"primaryKey"`
	VideoID    uint      `gorm:"index"`
	EventType  string    `gorm:"type:varchar(50)"`
	CreateTime time.Time `gorm:"autoCreateTime"`
	Status     string    `gorm:"type:varchar(50);index"`
}
```
- AutoMigrate 注册：`R\backend\internal\db\db.go:29-33` `db.AutoMigrate(&account.Account{}, &video.Video{}, ..., &video.OutboxMsg{}, ...)`

**写侧（同一事务内写业务表 + outbox 行）** `R\backend\internal\video\video_service.go:48-76`
```go
func (vs *VideoService) Publish(ctx context.Context, video *Video) error {
	err := vs.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(video).Error; err != nil { return err }
		msg := OutboxMsg{VideoID: video.ID, EventType: "video_published",
			Status: "pending", CreateTime: video.CreateTime}
		if err := tx.Create(&msg).Error; err != nil { return err }
		...
	})
```
- 注释原文（:47）`//事务保证视频写入库和消息写入本地消息表的一致性`
- repo 里另有未被调用的写法 `R\backend\internal\video\video_repo.go:25` `func (vr *VideoRepository) CreateMsg(ctx context.Context, Msg *OutboxMsg) error`（grep 全库无调用点，service 直接 `tx.Create(&msg)`）

**Poller** `R\backend\internal\worker\outboxworker.go:17-47`
```go
func StartOutboxPoller(db *gorm.DB, tmq *rabbitmq.TimelineMQ) {
	if db == nil || tmq == nil { log.Printf("Outbox poller disabled: timeline mq is not initialized"); return }
	go func() {
		for {
			var messages []video.OutboxMsg
			err := db.Where("status = ?", "pending").Order("create_time ASC").Limit(100).Find(&messages).Error
			if err != nil || len(messages) == 0 { time.Sleep(1 * time.Second); continue }
			for _, msg := range messages {
				err := tmq.PublishVideo(context.Background(), msg.VideoID, msg.CreateTime)
				if err == nil {
					if err := db.Delete(&msg).Error; err != nil {
						log.Printf("删除 outbox 消息失败: id=%d, err=%v", msg.ID, err) }
				} else {
					log.Printf("投递MQ失败: VideoID: %d, err: %v", msg.VideoID, err)
				}
			}
		}
	}()
}
```
事实：间隔 1 秒；批量 `Limit(100)`；按 `create_time ASC`；成功 = **物理删除该行**（不是改 status）；失败 = 只打日志、**不更新 status、无退避、无重试计数**，下一轮因 status 仍为 pending 会重投。无 `SELECT ... FOR UPDATE`/`SKIP LOCKED`，多实例会重复投递。

**Poller 启动点** `R\backend\internal\http\router.go:201-207`（在 **API 进程** 内，不在 cmd/worker）
```go
timelineMQ, err := rabbitmq.NewTimelineMQ(rmq)
if err != nil { log.Printf("timelineMQ init failed (mq disabled): %v", err); timelineMQ = nil }
worker.StartOutboxPoller(db, timelineMQ)
worker.StartConsumer(timelineMQ, "video.timeline.update.queue", cache, rmq)
```

**发布消息的 routing key 与 exchange** `R\backend\internal\middleware\rabbitmq\timelineMQ.go:15-20,44-61`
```go
const (
	timelineExchange   = "video.timeline.events"
	timelineQueue      = "video.timeline.update.queue"
	timelineBindingKey = "video.timeline.*"
	timelinePublishRK  = "video.timeline.publish"
)
type TimelineEvent struct {
	EventID string `json:"event_id"`; VideoID uint `json:"video_id"`
	CreateTime int64 `json:"create_time"`; OccurredAt time.Time `json:"occurred_at"`
}
func (t *TimelineMQ) PublishVideo(ctx context.Context, videoID uint, createTime time.Time) error {
	id, err := newEventID(16)   // crypto/rand 16 字节 hex
	timeline := TimelineEvent{EventID: id, VideoID: videoID,
		CreateTime: createTime.UnixMilli(), OccurredAt: time.Now()}
	return PublishJSON(ctx, t.ch, timelineExchange, timelinePublishRK, timeline)
}
```
- `NewTimelineMQ(base *RabbitMQ) (*TimelineMQ, error)` 在构造时 `DeclareTopic(ch, timelineExchange, timelineQueue, timelineBindingKey)`（timelineMQ.go:29-42）
- 消费侧（API 进程内 goroutine）`R\backend\internal\worker\outboxworker.go:49-118`：`func StartConsumer(tmq *rabbitmq.TimelineMQ, queueName string, redisClient *rediscache.Client, rmq *rabbitmq.RabbitMQ)`；每轮 `ch, err := rmq.NewChannel()`；`ch.Qos(10, 0, false)`；`ch.Consume(queueName, "", false, false, false, false, nil)`（autoAck=false, exclusive=false）；成功 `ZAdd feed:global_timeline score=CreateTime member=VideoID` 后 `msg.Ack(false)`；失败 `msg.Nack(false, true)`；`ZRemRangeByRank(ctx, timelineKey, 0, -1001)` 截断保留 1000 条；`msgs` channel 关闭 → `ch.Close()` + `time.Sleep(5 * time.Second)` 重连；反序列化失败直接 `msg.Ack(false)` 丢弃。

## 2. RabbitMQ 连接与拓扑声明

**基座** `R\backend\internal\middleware\rabbitmq\rabbitMQ.go`
```go
// RabbitMQ 只管理 Connection，Channel 由各组件按需创建   (:17)
type RabbitMQ struct { Conn *amqp.Connection }
func NewRabbitMQ(cfg *config.RabbitMQConfig) (*RabbitMQ, error)   // amqp.Dial，无自动重连
func (r *RabbitMQ) Close() error
func (r *RabbitMQ) NewChannel() (*amqp.Channel, error)
func DeclareTopic(ch *amqp.Channel, exchange string, queue string, bindingKey string) error
func PublishJSON(ctx context.Context, ch *amqp.Channel, exchange string, routingKey string, payload any) error
func newEventID(n int) (string, error)   // crypto/rand → hex
```
- 重连事实：`NewRabbitMQ` 只有一次性 `amqp.Dial(url)`，**没有** `NotifyClose`/自动重连循环；重连全部靠调用方（`cmd\worker\main.go:59-88` 的 `runWorkerWithRetry`、`outboxworker.go:59-118`、`router.go:236-249`）。
- **PublishJSON 全文**（:98-115）：`json.Marshal` → `ch.PublishWithContext(ctx, exchange, routingKey, false, false, amqp.Publishing{ContentType:"application/json", DeliveryMode: amqp.Persistent, Timestamp: time.Now(), Body: b})`。
- DeclareTopic（:51-96）：`ch.ExchangeDeclare(exchange, "topic", true, false, false, false, nil)` → `ch.QueueDeclare(queue, true, false, false, false, amqp.Table{"x-dead-letter-exchange": DLXExchange})` → `ch.QueueBind(q.Name, bindingKey, exchange, false, nil)` → `DeclareDLX(ch, queue)`（失败仅 log）。参数含义按 顺序：durable=true、autoDelete=false、exclusive=false、noWait=false。

**DLX** `R\backend\internal\middleware\rabbitmq\dlx.go:9-53`
```go
const ( DLXExchange = "dlx.events"; MaxRetryCount = 3 )
func DeclareDLX(ch *amqp.Channel, queueName string) error {
	ch.ExchangeDeclare(DLXExchange, "topic", true, false, false, false, nil)
	dlxQueue := queueName + ".dlx"
	ch.QueueDeclare(dlxQueue, true, false, false, false, nil)   // 死信队列本身无 DLX 参数
	ch.QueueBind(dlxQueue, "#", DLXExchange, false, nil)
	log.Printf("DLX ready: exchange=%s queue=%s", DLXExchange, dlxQueue)
}
func GetRetryCount(d amqp.Delivery) int {
	deaths, ok := d.Headers["x-death"].([]interface{})
	if !ok || len(deaths) == 0 { return 0 }
	death, ok := deaths[0].(amqp.Table)
	if !ok { return 0 }
	count, ok := death["count"].(int64)
	if !ok { return 0 }
	return int(count)
}
```
事实：`GetRetryCount` / `MaxRetryCount` 全库除定义处**无任何调用点**；各 worker 的实际重试是进程内 sleep 重试（见下），未使用 x-death。

**消费者失败重试（以 `R\backend\internal\worker\socialworker.go:60-83` 为例，likeworker/commentworker/popularityworker/notificationworker 同构，分别位于 `likeworker.go:61`、`commentworker.go:62`、`popularityworker.go:61`、`notificationworker.go:69`）**
```go
func (w *SocialWorker) handleDelivery(ctx context.Context, d amqp.Delivery) {
	const maxRetries = 3
	for i := 0; i <= maxRetries; i++ {
		select { case <-ctx.Done(): _ = d.Nack(false, true); return; default: }
		if err := w.process(ctx, d.Body); err != nil {
			if i >= maxRetries {
				log.Printf("social worker: 重试 %d 次后仍失败, 丢弃: %v", maxRetries, err)
				_ = d.Ack(false); return
			}
			wait := time.Duration(1<<uint(i)) * time.Second
			log.Printf("social worker: 处理失败, %v 后重试 (%d/%d): %v", wait, i+1, maxRetries, err)
			time.Sleep(wait); continue
		}
		_ = d.Ack(false); return
	}
}
```
即：最多 4 次尝试、退避 1s/2s/4s（同进程内 sleep，阻塞该消费者）、耗尽后 **Ack 丢弃**（不投 DLX）。`process` 对 JSON 解析失败 `return nil`（当成功 Ack 丢弃）、对 MySQL 1062 重复键也 `return nil`（幂等处理，socialworker.go:104-107）。

**各 MQ 封装**（同包，模式统一：`XxxMQ{ch *amqp.Channel}` + `NewXxxMQ(base *RabbitMQ) (*XxxMQ, error)` 内 `DeclareTopic` + publish 私有方法）
- `likeMQ.go:15-22` `likeExchange="like.events"` `likeQueue="like.events"` `likeBindingKey="like.*"`，`likeLikeRK="like.like"` `likeUnlikeRK="like.unlike"`；`func (l *LikeMQ) Like(ctx context.Context, userID, videoID uint) error` / `Unlike(...)`；事件含 `EventID`(16字节hex) + `OccurredAt`。
- `commentMQ.go` / `socialMQ.go` / `popularityMQ.go` 同结构（exchange 名分别 comment.events / social.events / video.popularity.events，见 `cmd\worker\main.go:34-40`）。

## 3. worker 进程入口 `R\backend\cmd\worker\main.go`（348 行）

初始化顺序（:90-209）：
1. `godotenv.Load()`（失败仅 log "`.env not found; continuing`"）
2. `configPath := os.Getenv("CONFIG_PATH")`，默认 `configs/config.yaml`；`cfg, usedDefault, err := config.LoadLocalDev(configPath)`
3. `connectWithRetry("MySQL", 10, fn)`（:43-56，退避 `1<<i` 秒，上限 30s，超限 `log.Fatalf`）→ `db.NewDB(cfg.Database)`；`defer db.CloseDB(sqlDB)`；**worker 进程不调用 AutoMigrate**
4. Redis：`rediscache.NewFromEnv(&cfg.Redis)` → `cache.Ping(300ms ctx)`；任一失败则 `cache = nil`（仅禁用 popularity worker，不致命）
5. RabbitMQ：拼 URL `amqp://user:pass@host:port/` → `connectWithRetry("RabbitMQ", 10, ...)` → `defer conn.Close()`
6. 用**临时 Channel** `topoCh` 依次声明 4 套拓扑（social/like/comment/popularity，各自 `declareXxxTopology(ch)`，:211-348），然后 `topoCh.Close()`；注释原文（:146）`// 用临时 Channel 声明拓扑（持久化队列，声明一次即可）`。每套：`ExchangeDeclare(name,"topic",true,false,false,false,nil)` + `QueueDeclare(name,true,false,false,false,amqp.Table{"x-dead-letter-exchange": mqrabbit.DLXExchange})` + `QueueBind(q.Name, bindingKey, exchange,false,nil)`；bindingKey 为 `social.*`/`like.*`/`comment.*`/`video.popularity.*`
7. 构造 repo：`social.NewSocialRepository(sqlDB)`、`video.NewVideoRepository/NewLikeRepository/NewCommentRepository`
8. `ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)`；`defer stop()`
9. pprof：`observability.NewPprofServer("Worker", cfg.ObservabilityConfig.Pprof.Enabled, cfg.ObservabilityConfig.Pprof.WorkerAddr)`（API 进程用 `"API"` + `Pprof.ApiAddr`，见 `cmd\main.go:80-90`）；失败仅 log
10. 启动 4 个 worker，每个都包在 `go runWorkerWithRetry(ctx, "<Name>", conn, func(ch *amqp.Channel) error { return worker.NewXxxWorker(ch, repo, queue).Run(ctx) })` 内；`cache != nil` 时才启动 PopularityWorker
11. `<-ctx.Done()` → `log.Printf("Worker shutting down...")` → `time.Sleep(2 * time.Second)`（注释 `// 等待正在处理的消息完成`）→ `log.Printf("Worker stopped")`

`runWorkerWithRetry`（:59-88）原文要点：
```go
func runWorkerWithRetry(ctx context.Context, name string, conn *amqp.Connection, fn func(*amqp.Channel) error) {
	for {
		select { case <-ctx.Done(): return; default: }
		ch, err := conn.Channel()
		if err != nil { log.Printf("%s: 创建 Channel 失败: %v, 5秒后重试", name, err); time.Sleep(5*time.Second); continue }
		if err := ch.Qos(50, 0, false); err != nil { log.Printf("%s: QoS 设置失败: %v", name, err) }
		log.Printf("%s started, consuming", name)
		if err := fn(ch); err != nil {
			if ctx.Err() != nil { ch.Close(); return }
			log.Printf("%s: %v, 5秒后重连...", name, err)
		}
		ch.Close(); time.Sleep(5 * time.Second)
	}
}
```
事实：每个 worker 独立 Channel，QoS=50（API 侧 timeline consumer 是 QoS=10）。

## 4. SSE 与跨进程推送

实现文件 `R\backend\internal\worker\ssehub.go`（213 行，与 worker 同包）。
```go
type SSEHub struct {
	mu      sync.RWMutex
	clients map[uint][]chan *Notification
	db      *gorm.DB
}
func NewSSEHub(db *gorm.DB) *SSEHub
func (h *SSEHub) Push(userID uint, n *Notification)          // :28 RLock + select/default 非阻塞投递，慢消费者丢消息
func (h *SSEHub) Subscribe(userID uint) chan *Notification   // :43 缓冲 20
func (h *SSEHub) Unsubscribe(userID uint, ch chan *Notification)  // :51 close(c)，空则 delete(clients, userID)
func (h *SSEHub) SSERequireAuth() gin.HandlerFunc            // :78 token 取 ?token= 或 Bearer，auth.ParseToken → c.Set("accountID", claims.AccountID)
func (h *SSEHub) SSEHandler(c *gin.Context)                  // :101
func (h *SSEHub) ListHandler / MarkReadHandler / UnreadCountHandler  // :141/:163/:191
func (h *SSEHub) RegisterRoutes(r *gin.Engine, group *gin.RouterGroup)  // :206 GET /stream, POST /list, /markRead, /unreadCount
var _ NotificationHub = (*SSEHub)(nil)                       // :213 编译期接口断言
```
- SSEHandler 响应头：`Content-Type: text/event-stream`、`Cache-Control: no-cache`、`Connection: keep-alive`；`fmt.Fprintf(c.Writer, "data: %s\n\n", b)` + `flusher.Flush()`；`case <-time.After(30*time.Second)` 发 `": keepalive\n\n"`；`ctx.Done()` 退出并 Unsubscribe。
- `type NotificationHub interface { Push(userID uint, n *Notification) }` 定义在 `R\backend\internal\worker\notificationworker.go:33-35`，由 `NewNotificationWorker(ch, db, queue, hub)` 注入，`process` 末尾 `w.hub.Push(notif.RecipientID, notif)`（:160-162）。
- **跨进程机制事实：不是 Redis Pub/Sub，是每进程本地内存 hub + RabbitMQ 扇出。** 全库无 Redis 订阅代码；`clients map[uint][]chan` 是纯进程内 map。消费者 `NewNotificationWorker` 在 **API 进程** 内被调起：`R\backend\internal\http\router.go:210-255`，先声明 3 个通知队列 `DeclareTopic(notifCh, "like.events", "notification.like", "like.like")`、`("comment.events","notification.comment","comment.publish")`、`("social.events","notification.social","social.follow")`，再对每个 queue 起 `go func(queue string){ for { ch, err := rmq.NewChannel(); ... w := worker.NewNotificationWorker(ch, db, queue, hub); w.Run(ctx); ch.Close(); time.Sleep(5*time.Second) } }(q)`。`cmd\worker\main.go` **不**创建 SSEHub。
- 推送内容先落库再推：`w.db.WithContext(ctx).Create(notif)`，通知按 routing key 分派（`like.like` → 收件人=视频作者，`comment.publish`，`social.follow`），内容为中文字符串如 `"点赞了你的视频"`（notificationworker.go:121/141/151）。
- `Notification` 表：`ID/RecipientID(index,not null)/SenderID/Type(varchar50)/TargetID/Content(varchar255)/IsRead(default false)/CreatedAt(autoCreateTime)`（notificationworker.go:15-24）；`NotificationWorker.Run` 内会 `w.db.AutoMigrate(&Notification{})`（:48）。

## 5. 可观测性

`R\backend\internal\observability\` 只有 `pprof.go` 与 `pprof_test.go`。签名：
```go
type PprofServer struct { name string; server *http.Server; shutdownTimeout time.Duration }   // :14
func NewPprofMux() *http.ServeMux                                                            // :20
func NewPprofServer(name string, enabled bool, addr string) (*PprofServer, error)             // :31
func Shutdown(ctx context.Context, srv *http.Server) error                                   // :57
func (s *PprofServer) Close() error                                                          // :64
```
- `NewPprofMux`：`/debug/pprof/`→`pprof.Index`、`/cmdline`→`pprof.Cmdline`、`/profile`→`pprof.Profile`、`/symbol`→`pprof.Symbol`、`/trace`→`pprof.Trace`。
- `NewPprofServer`：`if !enabled || addr == "" { return nil, nil }`；`net.Listen("tcp", addr)` 失败即返回 error（**先 Listen 再 Serve，端口占用能立即发现**）；`shutdownTimeout: 3 * time.Second`；`http.Server{Addr, Handler, ReadHeaderTimeout: 5 * time.Second}`；`go func(){ log.Printf("%s pprof listening on %s", name, addr); server.Serve(ln) }()`。
- 路由注册：`R\backend\internal\http\router.go:28-30` 在 `SetRouter(db, cache, rmq)` 内 `r.GET("/healthz", func(c *gin.Context){ c.JSON(200, gin.H{"status": "ok"}) })`；**不存在** `/readyz`、`/metrics`、Prometheus、OpenTelemetry、结构化日志（全项目用标准库 `log.Printf`）。
- 调用点：`cmd\main.go:80`（"API" + `Pprof.ApiAddr`）、`cmd\worker\main.go:176`（"Worker" + `Pprof.WorkerAddr`），两者都是 `if pprofServer != nil { defer pprofServer.Close() }`。
- `router.go:24-26` `r.SetTrustedProxies(nil)`（失败仅 log）；`router.go:32` `r.Static("/static", "./.run/uploads")`。

## 6. 分层约定（以 `R\backend\internal\account\` 为例，共 4 文件）

- `entity.go`（79 行）：`Account` GORM 模型（`Username gorm:"unique"`、`Password/Token/RefreshToken` 均 `json:"-"`）+ 全部请求/响应 DTO：`CreateAccountRequest`、`RenameRequest`、`FindByIDRequest/FindByIDResponse`、`FindByUsernameRequest/Response`、`ChangePasswordRequest`、`LoginRequest/LoginResponse`、`UpdateProfileRequest`、`RefreshRequest`、`GetProfileRequest/GetProfileResponse`。
- `repo.go`（106 行）：`AccountRepository{db *gorm.DB}` + `NewAccountRepository(db *gorm.DB) *AccountRepository`；方法全带 `ctx context.Context` 并 `db.WithContext(ctx)`；`RenameWithToken(ctx, id, newUsername, token)` 用 `db.Transaction(func(tx *gorm.DB) error {...})`；`Update` 后判 `result.RowsAffected == 0` 返回 `gorm.ErrRecordNotFound`（:29-31）。
- `service.go`（244 行）：`AccountService{accountRepository *AccountRepository; cache *rediscache.Client}` + `NewAccountService(accountRepository *AccountRepository, cache *rediscache.Client) *AccountService`；包级哨兵错误 `ErrUsernameTaken = errors.New("username already exists")`、`ErrNewUsernameRequired = errors.New("new_username is required")`（:24-27）；把 MySQL 错误映射成领域错误：`errors.As(err, &mysqlErr) && mysqlErr.Number == 1062` → `ErrUsernameTaken`（:56-59）；bcrypt `bcrypt.GenerateFromPassword(pw, bcrypt.DefaultCost)`；缓存写用 `context.WithTimeout(ctx, 50*time.Millisecond)` 且失败只 log（:65-70）。
- `handler.go`（257 行）：`AccountHandler{accountService *AccountService}` + `NewAccountHandler(accountService *AccountService) *AccountHandler`；方法签名统一 `func (h *AccountHandler) Xxx(c *gin.Context)`；文件内含 3 个包私有辅助函数 `randHex(n int) (string, error)`（:228）、`buildAbsoluteURL(c *gin.Context, p string) string`（:236，读 `X-Forwarded-Proto`）、`getAccountID(c *gin.Context) (uint, error)`（:247）。

**切分方式事实**：`internal\<domain>\` 目录固定 4 文件（entity/repo/service/handler）；**不使用 interface**（repo/service/handler 之间全是具体指针类型 `*AccountRepository`、`*AccountService`），仅 `worker.NotificationHub` 一处用接口；依赖注入全为**显式构造函数注入**（参数即依赖，无 DI 框架、无全局单例 getter）；构造函数命名固定 `New<Type>`，handlers 只在唯一入口 `R\backend\internal\http\router.go:22` 的 `SetRouter(db, cache, rmq) *gin.Engine` 内手工装配（`accountRepository := account.NewAccountRepository(db)` → `accountService := ...` → `accountHandler := ...`，:43-45）。video 包是唯一按领域横向拆多文件按关注点切的包（`video_entity.go`/`video_repo.go`/`video_service.go`/`video_handler.go` + `like_*`/`comment_*`/`chunk_*`/`tag_entity.go`/`popularity_cache.go`）。

## 7. 错误处理

`R\backend\internal\apierror\errors.go`（全文 28 行）：
```go
var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrValidation   = errors.New("validation error")
)
func ClassifyHTTPStatus(err error) int {
	switch {
	case err == nil:                                return http.StatusOK
	case errors.Is(err, ErrUnauthorized):           return http.StatusUnauthorized
	case errors.Is(err, ErrValidation):             return http.StatusBadRequest
	case errors.Is(err, gorm.ErrRecordNotFound):    return http.StatusNotFound
	default:                                        return http.StatusInternalServerError
	}
}
```
handler 用法 2 例：
- `R\backend\internal\account\handler.go:28-42`（CreateAccount）：绑定失败 `c.JSON(apierror.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})`；而 service 错误写死 `c.JSON(500, ...)`。
- `R\backend\internal\account\handler.go:55-72`（Rename）：`errors.Is(err, ErrNewUsernameRequired)` → `ClassifyHTTPStatus`；`ErrUsernameTaken` → `409`；`gorm.ErrRecordNotFound` → `404`；其余 → `500`。即 ClassifyHTTPStatus 多用于请求绑定错误，业务错误仍手写状态码。
- service 侧引用：`R\backend\internal\video\video_service.go:89` `return apierror.ErrUnauthorized`（鉴权失败用哨兵错误向上传）。

## 8. 工程脚本

`R\start.sh`（233 行，bash，`set -euo pipefail`）：
- 目录变量：`BACKEND_DIR=$ROOT_DIR/backend`、`FRONTEND_DIR=$ROOT_DIR/frontend`、`RUN_DIR=$BACKEND_DIR/.run`（:7-9）。
- 开关变量（全部 `${VAR:-默认}` 可覆盖）：`START_REDIS=1`、`START_RABBITMQ=1`、`START_BACKEND=1`、`START_WORKER=1`、`START_FRONTEND=1`（:11-15）；`COMPOSE_FILE=$ROOT_DIR/docker-compose.yml`、`STOP_DOCKER=0`（:21-22）；`FRONTEND_INSTALL=auto|1|0`、`FRONTEND_SCRIPT=dev|preview|build`（:27-28）；`REDIS_HOST=127.0.0.1`、`REDIS_PORT=6379`、`REDIS_CONF=`（:41-43）。
- 若 `CONFIG_PATH` 未设且 `START_RABBITMQ=1`，则导出 `CONFIG_PATH="configs/config.compose-local.yaml"`（注释说明该配置 MySQL 用宿主 3307 端口，:34-37）。
- 守卫：`require_dir` / `require_cmd go` / `require_cmd npm`；`trap cleanup INT TERM EXIT`，cleanup 对 FRONTEND/WORKER/BACKEND 三个 PID `kill`，并回退 `taskkill //PID <pid> //T //F`（注释 `# On Windows Git Bash, child processes sometimes don't receive signals reliably.`），`STOP_DOCKER=1` 时 `$COMPOSE_CMD -f "$COMPOSE_FILE" stop`（:45-95）。
- 执行的命令：`detect_compose` 探测 `docker compose version` 或 `docker-compose`；`start_rabbitmq_compose` → `$COMPOSE_CMD -f "$COMPOSE_FILE" up -d rabbitmq` 后最多 30×1s 轮询 `docker exec <cid> rabbitmq-diagnostics -q ping`；`start_redis` → 先 `redis-cli ping` 探测已运行则跳过，否则 `nohup redis-server --bind $REDIS_HOST --port $REDIS_PORT` 写 `$RUN_DIR/redis.log` 与 `redis.pid`；`start_backend_bg` → `(cd "$BACKEND_DIR" && go run ./cmd) &` → `backend.pid`；`start_worker_bg` → `(cd "$BACKEND_DIR" && go run ./cmd/worker) &` → `worker.pid`；`start_frontend_bg` → 条件 `npm install` + `(cd "$FRONTEND_DIR" && npm run "$FRONTEND_SCRIPT") &` → `frontend.pid`；最后 `wait` 直到 Ctrl+C。

`R\.github\workflows\ci.yml`（64 行，完整）：
```yaml
name: CI
on:
  pull_request:
  push:
    branches: [main, master]
permissions:
  contents: read
concurrency:
  group: ci-${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true
jobs:
  backend:
    name: Backend
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: backend
    steps:
      - name: Checkout
        uses: actions/checkout@v4
      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: "1.24.x"
          cache-dependency-path: backend/go.sum
      - name: Download dependencies
        run: go mod download
      - name: Vet
        run: go vet ./...
      - name: Test
        run: go test -race -count=1 ./...
  frontend:
    name: Frontend
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: frontend
    steps:
      - name: Checkout
        uses: actions/checkout@v4
      - name: Setup Node
        uses: actions/setup-node@v4
        with:
          node-version: "22"
          cache: npm
          cache-dependency-path: frontend/package-lock.json
      - name: Install dependencies
        run: npm ci
      - name: Build
        run: npm run build
```
（无 lint/golangci-lint、无 Docker 构建、无 DB/Redis service 容器、无覆盖率上传。）

**其他事实**：`R\backend\go.mod` 的 module 名为 `feedsystem_video_go`，含 `cmd\main.go` 与 `cmd\worker\main.go` 两个 main；`internal\db\db.go` 的 `NewDB` 只返回 `*gorm.DB`，AutoMigrate 是独立函数且只在 API 进程调用。

## 9. 补充：`R\backend\Dockerfile` 全文（56 行，api / worker 两个 target）

```dockerfile
# syntax=docker/dockerfile:1.7
#
# Multi-stage build for backend API and worker.
#
# Build:
#   docker build -f backend/Dockerfile -t feedsystem-backend:api --target api .
#   docker build -f backend/Dockerfile -t feedsystem-backend:worker --target worker .
#

ARG GO_VERSION=1.24.5
ARG GOPROXY=https://goproxy.cn,direct
ARG GOSUMDB=sum.golang.google.cn

FROM golang:${GO_VERSION} AS deps
ARG GOPROXY
ARG GOSUMDB
ENV GOPROXY=${GOPROXY} \
    GOSUMDB=${GOSUMDB}

WORKDIR /src/backend

COPY backend/go.mod backend/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

FROM deps AS source
COPY backend/ ./

ENV CGO_ENABLED=0

FROM source AS api-build
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w" -o /out/api ./cmd

FROM source AS worker-build
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker

FROM alpine:3.21 AS base
RUN apk add --no-cache ca-certificates tzdata && adduser -D -H -s /sbin/nologin app
WORKDIR /app
COPY --from=source /src/backend/configs ./configs
RUN mkdir -p ./.run/uploads && chown -R app:app /app
USER app

FROM base AS api
COPY --from=api-build /out/api /app/api
EXPOSE 8080
ENTRYPOINT ["/app/api"]

FROM base AS worker
COPY --from=worker-build /out/worker /app/worker
ENTRYPOINT ["/app/worker"]
```
事实：`deps`（仅 go.mod/go.sum + 模块缓存挂载）→ `source`（COPY backend/ 并设 `CGO_ENABLED=0`）→ `api-build`（`-o /out/api ./cmd`）与 `worker-build`（`-o /out/worker ./cmd/worker`）→ `base`（alpine:3.21 + ca-certificates/tzdata + 非 root 用户 app + 拷贝 configs + 预建 `./.run/uploads`）→ `api`（EXPOSE 8080）/ `worker`（无 EXPOSE）。两个 target 均无 healthcheck、无 `/app/.env`。

## 10. 补充：`R\backend\internal\worker\` 全部启动函数签名与轮询/批量参数

**唯一的轮询循环是 outbox（outboxworker.go，启动于 API 进程）**：
```go
func StartOutboxPoller(db *gorm.DB, tmq *rabbitmq.TimelineMQ)                       // :17
func StartConsumer(tmq *rabbitmq.TimelineMQ, queueName string, redisClient *rediscache.Client, rmq *rabbitmq.RabbitMQ)  // :49
```
- 轮询间隔 `time.Sleep(1 * time.Second)`（:30），且 `err != nil || len(messages) == 0` 时也 sleep 1s（同时充当 DB 错误退避）。
- 批量 `Limit(100)`、`Where("status = ?", "pending")`、`Order("create_time ASC")`（:27）。
- 一批内**串行逐条** `tmq.PublishVideo(context.Background(), msg.VideoID, msg.CreateTime)`，无并发/无批量确认。
- 成功：`db.Delete(&msg)`（物理删除）；删除失败仅 `log.Printf("删除 outbox 消息失败: id=%d, err=%v", ...)`，该行下轮会重投。
- 失败：仅 `log.Printf("投递MQ失败: VideoID: %d, err: %v", ...)`，status 保持 pending、无重试计数、无退避；一轮结束后立刻进入下一轮 sleep。
- `StartConsumer` 侧参数：`ch.Qos(10, 0, false)`、`ch.Consume(queueName, "", false, false, false, false, nil)`、Redis 写 `ZAdd` 超时 `500*time.Millisecond`、保留最近 1000 条（`ZRemRangeByRank(ctx, key, 0, -1001)`）、`msgs` 关闭后 `time.Sleep(5 * time.Second)` 重连。

**其余 worker 都是 MQ 长连接消费（无轮询、无批量、无定时器）**，统一形状：`for { select { case <-ctx.Done(): return ctx.Err(); case d, ok := <-deliveries: ... } }`，`Consume(queue, "", false, false, false, false, nil)`（autoAck=false）：

| 文件 | 构造函数签名 | Run | 队列常量来源 |
|---|---|---|---|
| `socialworker.go:22` | `func NewSocialWorker(ch *amqp.Channel, repo *social.SocialRepository, queue string) *SocialWorker` | `func (w *SocialWorker) Run(ctx context.Context) error` (:26) | `socialQueue="social.events"` (cmd\worker\main.go:27) |
| `likeworker.go:22` | `func NewLikeWorker(ch *amqp.Channel, likes *video.LikeRepository, videos *video.VideoRepository, queue string) *LikeWorker` | `Run(ctx context.Context) error` (:26) | `likeQueue="like.events"` (main.go:31) |
| `commentworker.go:23` | `func NewCommentWorker(ch *amqp.Channel, comments *video.CommentRepository, videos *video.VideoRepository, queue string) *CommentWorker` | `Run(ctx context.Context) error` (:27) | `commentQueue="comment.events"` (main.go:35) |
| `popularityworker.go:22` | `func NewPopularityWorker(ch *amqp.Channel, cache *rediscache.Client, queue string) *PopularityWorker` | `Run(ctx context.Context) error` (:26) | `popularityQueue="video.popularity.events"` (main.go:39) |
| `notificationworker.go:37` | `func NewNotificationWorker(ch *amqp.Channel, db *gorm.DB, queue string, hub NotificationHub) *NotificationWorker` | `Run(ctx context.Context) error` (:41) | `notification.like` / `notification.comment` / `notification.social`（router.go:234） |

- 各 `Run` 前置校验同构：`if w == nil || w.ch == nil || w.repo == nil { return errors.New("<domain> worker is not initialized") }`、`if w.queue == "" { return errors.New("queue is required") }`；`NotificationWorker.Run` 额外先 `w.db.WithContext(ctx).AutoMigrate(&Notification{})`（notificationworker.go:48），其余 worker 不做 AutoMigrate。
- 重试（所有 MQ worker 一致）：`const maxRetries = 3`；`for i := 0; i <= maxRetries; i++`；失败退避 `wait := time.Duration(1<<uint(i)) * time.Second` → 1s/2s/4s，`time.Sleep(wait)` **阻塞该消费者**；第 4 次仍失败则 `log.Printf("<domain> worker: 重试 %d 次后仍失败, 丢弃: %v", maxRetries, err)` + `_ = d.Ack(false)`；`ctx.Done()` 时 `_ = d.Nack(false, true)` 并 return。
- QoS 由外层统一设置：`cmd\worker\main.go:73` `ch.Qos(50, 0, false)`（每个 worker 独立 Channel）；`router.go` 里的 notification worker 走 `rmq.NewChannel()` 后**未设 QoS**（默认无限预取）。
- 无 `time.Ticker`、无 cron、无分布式锁、无 `SELECT FOR UPDATE`（grep `Ticker|Interval` 在 worker 包无命中）。
