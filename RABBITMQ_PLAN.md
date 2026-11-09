# allto 阶段二：RabbitMQ 异步 Worker + Outbox 方案

> 目标：把「点赞/评论/关注」触发的**通知**副作用异步化，API 进程与 Worker 进程分离，用 Outbox 保证消息不丢。
> 本文档是后端实现落地指南（代码骨架为示意，具体由后端自己写）。

## 0. 异步化边界（面试必讲）

**核心业务同步、副作用异步。**

- 点赞记录、评论内容、关注关系 → **保持同步落库**（用户点完要立刻看到自己操作生效）。
- 「给对方发通知」这个副作用 → 走 outbox + MQ + worker 异步。

所以这次改的是「点赞/评论/关注触发的**通知**」，不是「点赞/评论本身」。

## 1. 已确认的三个决策

| 决策点 | 选择 |
|--------|------|
| SSE 跨进程 | **Redis Pub/Sub**（worker 生成通知后 `Publish`，API 进程订阅后 `hub.Push`） |
| Outbox 投递成功后 | **Delete**（直接删记录），模型加 `updated_at` 字段 |
| MQ 拓扑 | **单队列** `notify.events`（topic exchange + binding key 区分类型，便于后续拆分） |

## 2. 整体架构

```
                    ┌───────────────────── API 进程 (main.go) ─────────────────────┐
  用户点赞          │  handler.LikePost                                        SSE  │
     │             │    └─ 事务内: 写 like + like_count + 写 outbox 记录          │
     │             │                                                     ▲         │
     ▼             │                                    Redis Pub/Sub   │  hub.Push│
   MySQL  ◄────────┤   (outbox 表持久化，事务原子)      notify:{userID} ─┘         │
     ▲             │                                        │                     │
     │             └────────────────────────────────────────┼─────────────────────┘
     │                          ┌───────────────────────────┼──── Worker 进程 (cmd/worker) ──┐
     │  outbox poller(1s轮询)    │                           │                                 │
     │   读 outbox → 投 MQ       ▼                           │   notification worker            │
     └──────►  RabbitMQ  notify.events 队列 ─────────────────┘     消费 → 写 Notification 表
                                                                    → Redis.Publish("notify:{id}")
```

三个关键件：

- **outbox 表**：业务事务里落库，保证「业务数据 + 消息」原子，进程挂了消息不丢。
- **RabbitMQ**：削峰 + 解耦 + 可靠投递（持久化队列）。
- **Redis Pub/Sub**：worker 跨进程把「通知已生成」广播给 API 进程，API 转推 SSE。

## 3. 依赖 & 配置改动

### 3.1 依赖

```bash
go get github.com/rabbitmq/amqp091-go
```

### 3.2 config/config.go 加 RabbitMQ 配置

```go
type Config struct {
    Server   ServerConfig
    Database DatabaseConfig
    Redis    RedisConfig
    RabbitMQ RabbitMQConfig   // 新增
    JWT      JWtConfig
    SMTP     SmtpConfig
}

type RabbitMQConfig struct {
    Host     string
    Port     int
    Username string
    Password string
}
```

`Load()` 里补 `bindEnv`：

```go
bindEnv("rabbitmq.host")
bindEnv("rabbitmq.port")
bindEnv("rabbitmq.username")
bindEnv("rabbitmq.password")
```

### 3.3 config.yaml / config.docker.yaml 加一段

```yaml
rabbitmq:
  host: "localhost"
  port: 5672
  username: "guest"
  password: "guest"
```

## 4. 数据模型（model/model.go）

```go
// 通知事件消息体（outbox payload 和 MQ 消息共用）
type NotificationEvent struct {
    EventID     string `json:"event_id"`     // 幂等键 = outbox 记录的 UUID
    RecipientID uint   `json:"recipient_id"` // 收通知的人（帖子作者/被关注者）
    SenderID    uint   `json:"sender_id"`
    Type        string `json:"type"`         // "like" / "comment" / "follow"
    TargetID    uint   `json:"target_id"`
}

// Outbox 表（投递成功即删除，表里只剩 pending）
type Outbox struct {
    ID        uint      `gorm:"primaryKey"`
    EventID   string    `gorm:"uniqueIndex;size:36"` // UUID，幂等键
    EventType string    `gorm:"index"`               // "notification"
    Payload   string    `gorm:"type:text"`           // NotificationEvent 的 JSON
    CreatedAt time.Time
    UpdatedAt time.Time
}

// Notification 加一列，worker 落库时防重
type Notification struct {
    // ...现有字段不变...
    DedupKey string `gorm:"uniqueIndex"` // 存 EventID
}
```

> `main.go` 的 AutoMigrate 里补 `&model.Outbox{}`。

## 5. MQ 拓扑（单队列）

| 项 | 值 |
|----|----|
| exchange | `notify.events`（topic，durable） |
| queue | `notify.events`（durable） |
| binding key | `notify.*` |
| 发布 routing key | `notify.like` / `notify.comment` / `notify.follow` |

单队列 + topic exchange：类型通过 routing key 和 payload 双重携带，后续要拆成多队列时只改 binding 和消费端，不动发布端。

## 6. 分步实施（每步都编译、可跑）

- **Step 1**：加依赖 + config + model（Outbox、Notification.DedupKey），`go build` 过，AutoMigrate 建表。
- **Step 2**：写 `mq/rabbitmq.go`（连接 + 声明拓扑 + Publish/Consume）。本地起 RabbitMQ 验证能连。
- **Step 3**：改 handler，事务内写 outbox，去掉 `go Notify(...)`。此时先保留一个临时兜底（直接写通知）保证功能不断。
- **Step 4**：写 worker（outbox poller + notification worker），先在**同一进程**起 goroutine 跑通「outbox → MQ → 写 Notification → hub.Push」，验证通知正常落库 + SSE 能收到。
- **Step 5**：拆进程。新建 `cmd/worker/main.go`，worker 独立跑；SSE 跨进程用 Redis Pub/Sub 桥接。API、worker 分别启动验证。
- **Step 6**：补 docker-compose（rabbitmq + worker 服务），部署验证。

## 7. 各组件代码骨架

### 7.1 mq/rabbitmq.go（新增）

```go
package mq

const (
    Exchange    = "notify.events"
    Queue       = "notify.events"
    BindingKey  = "notify.*"
)

type RabbitMQ struct { Conn *amqp.Connection }

func New(cfg config.RabbitMQConfig) (*RabbitMQ, error) {
    url := fmt.Sprintf("amqp://%s:%s@%s:%d/", cfg.Username, cfg.Password, cfg.Host, cfg.Port)
    conn, err := amqp.Dial(url)
    if err != nil { return nil, err }
    return &RabbitMQ{Conn: conn}, nil
}

func (r *RabbitMQ) Close() error { return r.Conn.Close() }
func (r *RabbitMQ) NewChannel() (*amqp.Channel, error) { return r.Conn.Channel() }

// Declare 声明 topic exchange + durable queue + binding（幂等，可重复调用）
func Declare(ch *amqp.Channel) error {
    ch.ExchangeDeclare(Exchange, "topic", true, false, false, false, nil)
    ch.QueueDeclare(Queue, true, false, false, false, nil)
    return ch.QueueBind(Queue, BindingKey, Exchange, false, nil)
}

// Publish 发布通知事件，routingKey = "notify.<type>"
func Publish(ch *amqp.Channel, routingKey string, payload []byte) error {
    return ch.Publish(Exchange, routingKey, false, false, amqp.Publishing{
        ContentType:  "application/json",
        DeliveryMode: amqp.Persistent,
        Body:         payload,
    })
}
```

### 7.2 worker/outbox.go（新增，outbox poller）

```go
// 轮询 outbox 表 pending 记录 → 投 MQ → 投递成功 Delete
func StartOutboxPoller(db *gorm.DB, mqConn *mq.RabbitMQ) {
    go func() {
        for {
            ch, err := mqConn.NewChannel()
            if err != nil { time.Sleep(5 * time.Second); continue }

            var msgs []model.Outbox
            db.Order("id").Limit(100).Find(&msgs) // 表里只剩 pending，无需 where

            for _, m := range msgs {
                var ev model.NotificationEvent
                json.Unmarshal([]byte(m.Payload), &ev)
                if err := mq.Publish(ch, "notify."+ev.Type, []byte(m.Payload)); err != nil {
                    util.Logger.Error("投递MQ失败", zap.Error(err))
                    continue // 留在 outbox，下轮重试
                }
                db.Delete(&m) // 投递成功即删
            }
            ch.Close()
            time.Sleep(time.Second)
        }
    }()
}
```

### 7.3 worker/notification.go（新增，消费 + 落库 + 广播）

```go
func StartNotificationWorker(db *gorm.DB, mqConn *mq.RabbitMQ) {
    go func() {
        for {
            ch, err := mqConn.NewChannel()
            if err != nil { time.Sleep(5 * time.Second); continue }
            mq.Declare(ch) // 兜底声明拓扑
            ch.Qos(10, 0, false)

            msgs, err := ch.Consume(mq.Queue, "", false, false, false, false, nil)
            if err != nil { ch.Close(); time.Sleep(5 * time.Second); continue }

            for d := range msgs {
                var ev model.NotificationEvent
                if err := json.Unmarshal(d.Body, &ev); err != nil {
                    d.Ack(false) // 非法消息直接丢弃
                    continue
                }
                n := model.Notification{
                    RecipientID: ev.RecipientID, SenderID: ev.SenderID,
                    Type: ev.Type, TargetID: ev.TargetID, DedupKey: ev.EventID,
                }
                if err := db.Create(&n).Error; err != nil {
                    // 唯一索引冲突 = 重复消息，直接 ack；否则 nack 重投
                    if errors.Is(err, gorm.ErrDuplicatedKey) {
                        d.Ack(false); continue
                    }
                    d.Nack(false, true); continue
                }
                // 跨进程广播给 API 进程的 SSE
                database.GetRedis().Publish(database.GetCtx(),
                    "notify:"+strconv.Itoa(int(ev.RecipientID)), d.Body)
                d.Ack(false)
            }
            ch.Close()
            time.Sleep(5 * time.Second) // channel 断开重连
        }
    }()
}
```

> 消费幂等靠 `Notification.DedupKey` 唯一索引：outbox 投递成功但 Delete 失败 → 重复消息 → 第二次 `db.Create` 撞唯一索引 → 跳过。这就是 **at-least-once + 消费幂等**。

### 7.4 cmd/worker/main.go（新增，worker 进程入口）

```go
func main() {
    config.Load()
    util.InitLogger(true)
    database.InitDB(config.AppConfig.Database.DSN)
    database.InitRedis(config.AppConfig.Redis.Addr, config.AppConfig.Redis.Password)
    database.GetDB().AutoMigrate(&model.Notification{}, &model.Outbox{})

    mqConn, err := mq.New(config.AppConfig.RabbitMQ)
    if err != nil { log.Fatal("RabbitMQ 连接失败:", err) }
    defer mqConn.Close()

    worker.StartOutboxPoller(database.GetDB(), mqConn)
    worker.StartNotificationWorker(database.GetDB(), mqConn)

    // 优雅退出：监听 SIGINT/SIGTERM
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()
    <-ctx.Done()
    log.Println("worker shutting down")
}
```

### 7.5 main.go 改造（API 进程）

加一个「订阅 Redis → hub.Push」桥接。因为 `hub` 是 `handler` 包里的包级变量，建议在 handler 包加一个函数，main.go 只负责调用：

```go
// handler/notification.go 或新文件 handler/sse_bridge.go
func StartNotificationBridge() {
    go func() {
        pubsub := database.GetRedis().PSubscribe(database.GetCtx(), "notify:*")
        for msg := range pubsub.Channel() {
            // channel 形如 "notify:123"
            userID, _ := strconv.Atoi(strings.TrimPrefix(msg.Channel, "notify:"))
            hub.Push(uint(userID), []byte(msg.Payload))
        }
    }()
}
```

`main.go` 在 Redis 初始化成功后调用：

```go
handler.StartNotificationBridge()
```

### 7.6 handler 改动（interaction.go / social.go）

点赞（示意，评论/关注同理，关键是把 `go Notify(...)` 换成事务内写 outbox）：

```go
err = database.GetDB().Transaction(func(tx *gorm.DB) error {
    tx.Create(&model.Like{UserID: userID, PostID: uint(postID)})
    tx.Model(&post).Update("like_count", gorm.Expr("like_count + ?", 1))

    if post.UserID == userID { return nil } // 不给自己发通知

    ev := model.NotificationEvent{
        EventID: uuid.NewString(), RecipientID: post.UserID,
        SenderID: userID, Type: "like", TargetID: uint(postID),
    }
    b, _ := json.Marshal(ev)
    return tx.Create(&model.Outbox{EventID: ev.EventID, EventType: "notification", Payload: string(b)}).Error
})
```

对应改动：

- `handler/interaction.go`：`LikePost`、`CreateComment` 写 outbox。
- `handler/social.go`：`FollowUser` 写 outbox（recipient = followingID）。
- `handler/notification.go`：删除 `Notify()` 里的 `database.GetDB().Create(&n)`（改由 worker 落库）；`ListNotifications`/`UnreadCount`/`MarkRead` 不变；`NotificationStream` 不变。

## 8. 关键设计点（面试讲点）

1. **Outbox 为什么保证不丢**：业务写库 + 写 outbox 在**同一事务**里，要么都成功要么都回滚；事务提交后消息已持久化在 outbox 表，poller 投递失败下轮重试，进程崩溃重启后还能继续。对比 `go Notify(...)`（goroutine 随进程崩溃丢消息）。
2. **at-least-once + 消费幂等**：outbox 投递成功但 Delete 失败 → 下轮重复投递 → 重复消息，靠 `DedupKey` 唯一索引在消费端去重，保证最终一致。
3. **SSE 跨进程 = Redis Pub/Sub**：SSE 长连接在 API 进程、通知生成在 worker 进程，中间靠 Redis 频道广播，是「多进程实时推送」的标准答案。
4. **持久化队列 + 手动 Ack + Nack 重投**：队列 `durable`、消息 `Persistent`，处理成功才 `Ack`，失败 `Nack` 重投。
5. **Channel 断线重连**：每个 consumer 独立 Channel，断开后重建（worker 里 `for` 循环 + `ch.Close()` + sleep 重连）。
6. **异步化边界**：只异步「通知」这类非关键路径副作用，核心业务保持同步，避免「点完赞刷新看不到」的体验问题。

## 9. 可选的后续增强（先不做，面试提一嘴即可）

- **DLX 死信队列**：毒消息（反复失败）转死信队列，避免无限重投热循环（video 项目有 `dlx.go`，可参照）。
- **SSE hub 多连接**：现在 `map[uint]chan` 一个用户只能一个连接，可改成 `map[uint][]chan` 支持多设备。
- **Worker 多实例**：队列天然支持多消费者并发，`Qos` 控制预取，横向扩容 worker。
