# repo 重构：进度 & 剩余任务

> 状态日期：2026-09-10
> 目标：model 拆分 + 把 handler 里的裸 `database.GetDB()` 收进 `repo` 层（阶段二 RabbitMQ 的前置地基）。
> 配套文档：`MODEL_REPO_PLAN.md`（设计/方法清单）、`RABBITMQ_PLAN.md`（阶段二）。

## 一、当前进度（`go build ./...` 能过 ✅）

### 已完成

| 模块 | 状态 |
|------|------|
| model 拆分 | ✅ 7 文件：user / post / comment / like / follow / notification / outbox |
| `repo/user_repo.go` | ✅ 11 个方法齐全 |
| `repo/post_repo.go` | ✅ 8 个方法齐全（含 `WithTx`、游标分页、`IncrementLikeCount`） |
| handler `Register` | ✅ 用 `switch` 做了严谨错误处理 |
| handler `Login` | ✅ 已接 repo（但见 §四 的小提醒） |
| handler `CreatePost` | ✅ |
| handler `ListPosts` | ✅ |

### 剩余

| 模块 | 状态 |
|------|------|
| `repo/like_repo.go` / `comment_repo.go` / `follow_repo.go` / `notification_repo.go` | ⬜ 空壳（只有 `package repo`） |
| `handler/account.go` 剩下 8 个 handler | ⬜ 还裸 `GetDB()` |
| `handler/interaction.go` / `social.go` / `notification.go` / `email.go` | ⬜ 全部没接 |
| `handler/post.go` 的 `GetPost` | ⬜ 故意留最后（有 Redis 缓存 + singleflight） |

---

## 二、4 个空壳 repo 

### like_repo.go

```go
type LikeRepo struct { db *gorm.DB }
func NewLikeRepo(db *gorm.DB) *LikeRepo
func (r *LikeRepo) WithTx(tx *gorm.DB) *LikeRepo
// 
func (r *LikeRepo) Get(userID, postID uint) (*model.Like, error)   // 是否已点赞
func (r *LikeRepo) Create(l *model.Like) error
func (r *LikeRepo) Delete(userID, postID uint) error
func (r *LikeRepo) ListByPost(postID uint) ([]model.Like, error)
func (r *LikeRepo) ListLikers(postID uint) ([]model.User, error)   // JOIN 查点赞用户
func (r *LikeRepo) ListByUser(userID uint) ([]model.Like, error)   // Preload Post
```

### comment_repo.go

```go
type CommentRepo struct { db *gorm.DB }
func NewCommentRepo(db *gorm.DB) *CommentRepo
func (r *CommentRepo) WithTx(tx *gorm.DB) *CommentRepo

func (r *CommentRepo) Create(c *model.Comment) error
func (r *CommentRepo) ListByPost(postID uint) ([]model.Comment, error) // Preload User，时间倒序
```

### follow_repo.go

```go
type FollowRepo struct { db *gorm.DB }
func NewFollowRepo(db *gorm.DB) *FollowRepo
func (r *FollowRepo) WithTx(tx *gorm.DB) *FollowRepo

func (r *FollowRepo) Get(followID, followingID uint) (*model.Follow, error)
func (r *FollowRepo) Create(f *model.Follow) error
func (r *FollowRepo) Delete(followID, followingID uint) (int64, error) // 返回 RowsAffected
func (r *FollowRepo) ListFollowingIDs(userID uint) ([]uint, error)
func (r *FollowRepo) ListFollowerIDs(userID uint) ([]uint, error)
func (r *FollowRepo) CountFollowers(userID uint) (int64, error)
func (r *FollowRepo) CountFollowing(userID uint) (int64, error)
```

### notification_repo.go

```go
type NotificationRepo struct { db *gorm.DB }
func NewNotificationRepo(db *gorm.DB) *NotificationRepo
func (r *NotificationRepo) WithTx(tx *gorm.DB) *NotificationRepo

func (r *NotificationRepo) Create(n *model.Notification) error              // 阶段二移到 worker
func (r *NotificationRepo) ListByRecipient(userID uint, limit int) ([]model.Notification, error)
func (r *NotificationRepo) CountUnread(userID uint) (int64, error)
func (r *NotificationRepo) MarkRead(userID uint, id uint) error             // id>0 单条，否则全部
```

---

## 三、剩余 handler 改法对照表（每个 handler 用哪些 repo 方法）

### account.go（剩下 8 个，都是单条 SQL）

| handler | 用到的 repo 方法 |
|---------|------------------|
| `Logout` | `userRepo.Update(userID, {"token":"","refresh_token":""})` |
| `ChangePassword` | `userRepo.GetByID(userID)` + `userRepo.Update(userID, {"password","token","refresh_token"})` |
| `Rename` | `userRepo.GetByUsernameExcept(newName, userID)` 查重名 → `userRepo.GetByID` → `userRepo.UpdateColumn("username")` → `userRepo.UpdateColumn("token")` |
| `UpdateProfile` | `userRepo.UpdateColumn(userID, "bio", bio)` |
| `UploadAvatar` | `userRepo.UpdateColumn(userID, "avatar_url", url)` |
| `UserProfile` | `userRepo.GetByID` + `postRepo.CountByUser` + `followRepo.CountFollowers` + `followRepo.CountFollowing` |
| `UserPosts` | `postRepo.ListByUser(userID, cursor, pageSize)`（游标记得 `uint(cursor)`） |
| `Refresh` | `userRepo.GetByRefreshToken(token)` + `userRepo.Update(userID, {"token","refresh_token"})` |

### interaction.go（⭐ 这里第一次用事务 `WithTx`）

| handler | 用到的 repo 方法 |
|---------|------------------|
| `LikePost` | 事务内：`likeRepo.Create` + `postRepo.IncrementLikeCount(postID, +1)` |
| `UnlikePost` | 事务内：`likeRepo.Delete` + `postRepo.IncrementLikeCount(postID, -1)` |
| `CreateComment` | `commentRepo.Create` |
| `ListComments` | `commentRepo.ListByPost(postID)` |
| `ListPostLikes` | `likeRepo.ListByPost(postID)` |
| `ListPostLikers` | `likeRepo.ListLikers(postID)` |
| `ListMyLikes` | `likeRepo.ListByUser(userID)` |

**LikePost 事务写法（重点，务必用 `tx` 构造的 repo，别用 `GetDB()`）：**

```go
err := database.GetDB().Transaction(func(tx *gorm.DB) error {
    if err := repo.NewLikeRepo(tx).Create(&like); err != nil {
        return err
    }
    return repo.NewPostRepo(tx).IncrementLikeCount(postID, 1)
})
```

### social.go

| handler | 用到的 repo 方法 |
|---------|------------------|
| `FollowUser` | `followRepo.Get`（查是否已关注）→ `followRepo.Create` |
| `UnfollowUser` | `followRepo.Delete`（用返回的 RowsAffected 判断是否已关注） |
| `ListFollowing` | `followRepo.ListFollowingIDs(userID)` → `userRepo.GetByIDs(ids)` |
| `GetFollowers` | `followRepo.ListFollowerIDs(userID)` → `userRepo.GetByIDs(ids)` |

### notification.go

| handler | 用到的 repo 方法 |
|---------|------------------|
| `ListNotifications` | `notificationRepo.ListByRecipient(userID, 50)` |
| `UnreadCount` | `notificationRepo.CountUnread(userID)` |
| `MarkRead` | `notificationRepo.MarkRead(userID, id)` |
| `Notify`（阶段二移走） | `notificationRepo.Create`（暂时留） |

### email.go

| handler | 用到的 repo 方法 |
|---------|------------------|
| `VerifyEmailCode` | `userRepo.GetByEmail` + `userRepo.UpdateColumn("email_verified", true)` |
| `SendVerifyCode` | `userRepo.GetByEmail`（查占用）→ `userRepo.UpdateColumn("email")` |

---

## 四、两个待修的小问题（Login）

1. `Login` 第 69-72 行错误处理没区分：`if err != nil` 一律报「账号不存在」，应像 `Register` 那样用 `errors.Is(err, gorm.ErrRecordNotFound)` 区分（查不到 401 / 真出错 500）。
2. 第 93 行 `"登陆时的双token..."` → 「登陆」改「登录」。

## 五、关键知识点速查

| 点 | 说明 |
|----|------|
| `WithTx(tx)` | 事务闭包里用 `NewXxxRepo(tx)` 或 `repo.WithTx(tx)`，**绝不能再用 `database.GetDB()`**（会绕过事务） |
| `Preload("User")` | 查帖子/评论/点赞列表带作者时用，放 repo 内部 |
| 游标分页 | `List`/`ListByUser` 取 `limit+1` 条，handler 判断 `has_more` |
| `gorm.Expr` | `Update("like_count", gorm.Expr("like_count + ?", delta))` 交给 DB 算，并发安全 |
| 错误处理 | repo 返回 `error`，handler 用 `errors.Is(err, gorm.ErrRecordNotFound)` 判断 |

## 六、建议顺序

1. `go fmt ./...`（先清理格式）
2. 接完 `account.go` 剩下 8 个 handler（都是单条 SQL，最快）
3. 写 `like_repo.go` + `comment_repo.go`，接 `interaction.go`（**重点：事务 `WithTx`**）
4. 写 `follow_repo.go` + `notification_repo.go`，接 `social.go` / `notification.go` / `email.go`
5. 最后啃 `GetPost`（Redis 缓存 + singleflight）
6. 全部接完 → 开始阶段二 `RABBITMQ_PLAN.md`
