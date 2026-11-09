# allto 重构：Model 拆分 + Repo 层封装 方案

> 目标：把挤在 `model/model.go` 的 6 个实体拆成独立文件；把散落在 handler 里的裸 `database.GetDB()` 调用收进 `repo` 层。
> 本文档是后端实现落地指南（只给方法签名和边界，具体实现后端自己写）。

## 0. 为什么现在做（和 RabbitMQ 的衔接）

先重构地基、后加功能：把 SQL 收进 repo 后，阶段二 RabbitMQ 的 handler 改动会变成「调 repo + 事务里写 outbox」，而不是「裸 SQL + 事务写 outbox」纠缠在一起。repo 的 `WithTx` 能力直接决定 outbox 事务能不能写干净（见 §3.3）。

## 1. Model 拆分

保持**单一 `model` 包**，按实体拆文件（不拆成多 package，allto 体量用不上）：

```
model/
├── user.go            // User
├── post.go            // Post
├── comment.go         // Comment
├── like.go            // Like
├── follow.go          // Follow
├── notification.go    // Notification
├── outbox.go          // Outbox（阶段二再加，现在可留空）
└── event.go           // NotificationEvent 消息体（阶段二加）
```

每个文件 `package model`。纯移动，不改字段、不改 tag，`go build` 必须立刻过。

## 2. Repo 层设计原则

### 2.1 结构体 + 构造注入 db（不用包级函数）

```go
type UserRepo struct { db *gorm.DB }
func NewUserRepo(db *gorm.DB) *UserRepo { return &UserRepo{db} }
```

理由：① 方法签名干净；② 为阶段四依赖注入铺路（结构体能收任意 `*gorm.DB`，测试塞 sqlmock）；③ 支持事务（§2.3）。

### 2.2 错误处理统一（顺手完成阶段一第 4 项）

repo 返回 `error`，**不在 repo 里写 `response.XXX`**；handler 用 `errors.Is(err, gorm.ErrRecordNotFound)` 判断。把现在散落的这种：

```go
// 现在（坏味道）
if err := db.Where("username=?").First(&exist).Error; err == nil {
    response.Conflict(c, "用户名已存在"); return
}
```

统一成：

```go
// 抽到 repo 后
_, err := userRepo.GetByUsername(req.Username)
switch {
case err == nil:
    response.Conflict(c, "用户名已存在"); return
case errors.Is(err, gorm.ErrRecordNotFound):
    // 不存在，继续
default:
    response.InternalError(c, "查询失败"); return
}
```

### 2.3 事务支持（最关键，决定 outbox 能不能写进去）

每个 repo 加 `WithTx`，返回一个绑定事务的新 repo：

```go
func (r *UserRepo) WithTx(tx *gorm.DB) *UserRepo { return &UserRepo{db: tx} }
```

handler 里事务就干净了（阶段二 LikePost 直接照搬）：

```go
database.GetDB().Transaction(func(tx *gorm.DB) error {
    likeRepo := repo.NewLikeRepo(tx)   // 事务内新建 repo（或 repo.WithTx(tx)）
    if err := likeRepo.Create(&like); err != nil { return err }
    return postRepo.WithTx(tx).IncrementLikeCount(postID, 1)
})
```

> 两种写法 `NewXxxRepo(tx)` 和 `repo.WithTx(tx)` 二选一，全局统一别混用。推荐 `WithTx`（语义更明确）。

### 2.4 不定义 interface、不建 service 层（YAGNI）

- repo 先**不定义 interface**，等要 mock 测试时再 `go:generate mockgen` 生成。
- **不做 service 层**：allto 业务逻辑薄，handler = controller，repo = 数据访问，够了。等出现「点赞 + 发通知 + 改计数」这种跨实体编排时再抽。

### 2.5 命名约定

| 语义 | 前缀 | 例子 |
|------|------|------|
| 查单条 | `GetByXxx` | `GetByID` / `GetByUsername` / `GetByEmail` |
| 查列表 | `ListXxx` | `ListByUser` / `ListLikers` |
| 查数量 | `CountXxx` | `CountUnread` / `CountByUser` |
| 写 | `Create` / `Update` / `UpdateColumn` / `Delete` | — |

## 3. 各 Repo 方法清单（从现有 handler 逆推）

### 3.1 UserRepo（account.go + email.go）

```go
type UserRepo struct { db *gorm.DB }

func (r *UserRepo) GetByID(id uint) (*model.User, error)
func (r *UserRepo) GetByIDs(ids []uint) ([]model.User, error)        // ListFollowing/Followers 批量查
func (r *UserRepo) GetByUsername(username string) (*model.User, error)
func (r *UserRepo) GetByUsernameExcept(username string, excludeID uint) (*model.User, error) // Rename 查重名
func (r *UserRepo) GetByEmail(email string) (*model.User, error)
func (r *UserRepo) GetByRefreshToken(token string) (*model.User, error)
func (r *UserRepo) Create(u *model.User) error
func (r *UserRepo) Update(id uint, fields map[string]interface{}) error            // 登录/登出/改密/refresh 批量更新
func (r *UserRepo) UpdateColumn(id uint, column string, value interface{}) error   // 头像/昵称/简介/邮箱验证 单列更新
```

### 3.2 PostRepo（post.go + account.go 的 UserPosts）

```go
type PostRepo struct { db *gorm.DB }

func (r *PostRepo) Create(p *model.Post) error
func (r *PostRepo) GetByID(id uint) (*model.Post, error)                  // 带 Preload("User")
func (r *PostRepo) List(cursor uint, limit int) ([]model.Post, error)     // 游标分页，Preload User，取 limit+1
func (r *PostRepo) ListByUser(userID, cursor uint, limit int) ([]model.Post, error)
func (r *PostRepo) CountByUser(userID uint) (int64, error)                // UserProfile 帖子数
func (r *PostRepo) IncrementLikeCount(postID uint, delta int) error       // 点赞/取消点赞改计数（事务内用）
```

> **游标分页契约**：`List` / `ListByUser` 内部取 `limit+1` 条，返回后 handler 判断 `len > limit` 得 `has_more`、取最后一条 `ID` 得 `next_cursor`（保留现有约定，别改成 offset）。

### 3.3 LikeRepo（interaction.go）

```go
type LikeRepo struct { db *gorm.DB }

func (r *LikeRepo) Get(userID, postID uint) (*model.Like, error)   // 是否已点赞
func (r *LikeRepo) Create(l *model.Like) error
func (r *LikeRepo) Delete(userID, postID uint) error
func (r *LikeRepo) ListByPost(postID uint) ([]model.Like, error)
func (r *LikeRepo) ListLikers(postID uint) ([]model.User, error)   // JOIN 查询点赞用户
func (r *LikeRepo) ListByUser(userID uint) ([]model.Like, error)   // 我点赞的列表，Preload Post
```

### 3.4 CommentRepo（interaction.go）

```go
type CommentRepo struct { db *gorm.DB }

func (r *CommentRepo) Create(c *model.Comment) error
func (r *CommentRepo) ListByPost(postID uint) ([]model.Comment, error)   // Preload User，按时间倒序
```

### 3.5 FollowRepo（social.go）

```go
type FollowRepo struct { db *gorm.DB }

func (r *FollowRepo) Get(followID, followingID uint) (*model.Follow, error)  // 是否已关注
func (r *FollowRepo) Create(f *model.Follow) error
func (r *FollowRepo) Delete(followID, followingID uint) (int64, error)       // 返回 RowsAffected
func (r *FollowRepo) ListFollowingIDs(userID uint) ([]uint, error)           // 关注的人 ID
func (r *FollowRepo) ListFollowerIDs(userID uint) ([]uint, error)            // 粉丝 ID
func (r *FollowRepo) CountFollowers(userID uint) (int64, error)
func (r *FollowRepo) CountFollowing(userID uint) (int64, error)
```

> `ListFollowing` / `GetFollowers` 现在「查 follow → 提 ids → 查 users」两步。抽完后：`FollowRepo.ListFollowingIDs` 拿 ids，`UserRepo.GetByIDs` 查用户，handler 组合。别让 FollowRepo 直接返回 `[]model.User`（职责混了）。

### 3.6 NotificationRepo（notification.go）

```go
type NotificationRepo struct { db *gorm.DB }

func (r *NotificationRepo) Create(n *model.Notification) error                  // 阶段二后移到 worker
func (r *NotificationRepo) ListByRecipient(userID uint, limit int) ([]model.Notification, error)
func (r *NotificationRepo) CountUnread(userID uint) (int64, error)
func (r *NotificationRepo) MarkRead(userID uint, id uint) error                 // id>0 单条，否则全部
```

### 3.7 OutboxRepo（阶段二预留，现在可先不建）

```go
type OutboxRepo struct { db *gorm.DB }

func (r *OutboxRepo) Create(o *model.Outbox) error
func (r *OutboxRepo) ListPending(limit int) ([]model.Outbox, error)
func (r *OutboxRepo) Delete(id uint) error
```

## 4. 分步实施（每步都编译、可跑）

- **Step 1 拆 model**：纯移动 6 个实体到独立文件，`go build` 过。
- **Step 2 建 repo 骨架 + 先抽 User/Post**：写 `repo/user_repo.go`、`repo/post_repo.go`，改 `account.go`、`post.go` 改用 repo。跑通「登录/发帖/列表」验证。
- **Step 3 铺开剩余 repo**：Like/Comment/Follow/Notification，改 `interaction.go`、`social.go`、`notification.go`。
- **Step 4 统一错误处理**：全局清 `c.JSON` 残留（其实阶段一已清）和 `if err == nil {...} else if err != gorm.ErrRecordNotFound` 散落判断，统一 `errors.Is`。
- **Step 5 加 WithTx + 验证事务**：给每个 repo 加 `WithTx`，把 `LikePost` 的 `Transaction` 改成 repo 写法，确认编译 + 行为一致。
- **Step 6（衔接阶段二）**：此时 handler 只调 repo，再按 `RABBITMQ_PLAN.md` 的 Step 3 做「事务里写 outbox」就非常顺。

## 5. 常见坑

1. **Preload 放哪**：`GetByID` / `List` 需要 `Preload("User")`，统一放在 **repo 内部**固定（handler 不关心关联查询细节）。但像 `ListByUser`（点赞列表 Preload Post）这种也要在 repo 内部做。
2. **别把 `response` 写进 repo**：repo 只返回 `(data, error)`，HTTP 响应是 handler 的事。否则 repo 层和 HTTP 层耦合，测试也难写。
3. **`Update` 用 `map[string]interface{}`**：现在登录/登出/改密都是 `Updates(map)`，保留这个签名，别改成结构体（空值不更新的坑）。
4. **游标分页别破坏契约**：repo 里 `Limit(limit+1)` 别漏 `+1`，否则 `has_more` 永远 false。
5. **事务里用 `WithTx` 后的 repo**：`Transaction` 闭包里千万别再 `database.GetDB()`（那会绕过事务、用全局连接），必须用 `tx` 构造的 repo。
