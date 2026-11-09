
/*!40101 SET @OLD_CHARACTER_SET_CLIENT=@@CHARACTER_SET_CLIENT */;
/*!40101 SET @OLD_CHARACTER_SET_RESULTS=@@CHARACTER_SET_RESULTS */;
/*!40101 SET @OLD_COLLATION_CONNECTION=@@COLLATION_CONNECTION */;
/*!50503 SET NAMES utf8mb4 */;
/*!40103 SET @OLD_TIME_ZONE=@@TIME_ZONE */;
/*!40103 SET TIME_ZONE='+00:00' */;
/*!40014 SET @OLD_UNIQUE_CHECKS=@@UNIQUE_CHECKS, UNIQUE_CHECKS=0 */;
/*!40014 SET @OLD_FOREIGN_KEY_CHECKS=@@FOREIGN_KEY_CHECKS, FOREIGN_KEY_CHECKS=0 */;
/*!40101 SET @OLD_SQL_MODE=@@SQL_MODE, SQL_MODE='NO_AUTO_VALUE_ON_ZERO' */;
/*!40111 SET @OLD_SQL_NOTES=@@SQL_NOTES, SQL_NOTES=0 */;

/*!40000 DROP DATABASE IF EXISTS `allto`*/;

CREATE DATABASE /*!32312 IF NOT EXISTS*/ `allto` /*!40100 DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci */ /*!80016 DEFAULT ENCRYPTION='N' */;

USE `allto`;
DROP TABLE IF EXISTS `comments`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `comments` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `content` longtext NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `post_id` bigint unsigned NOT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `fk_comments_user` (`user_id`),
  CONSTRAINT `fk_comments_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=14 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

LOCK TABLES `comments` WRITE;
/*!40000 ALTER TABLE `comments` DISABLE KEYS */;
INSERT INTO `comments` VALUES (1,'miaomiao喵！',1,1,'2026-04-28 20:15:31.443'),(2,'可以可以',2,3,'2026-05-06 19:23:20.584'),(3,'喵喵喵',2,3,'2026-05-07 10:21:03.351'),(4,'不是哥们',3,3,'2026-05-07 10:21:52.397'),(5,'wolege',2,5,'2026-08-11 14:58:02.365'),(6,'当恩怨各一半',1,6,'2026-08-19 15:25:34.522'),(7,'哈哈',2,6,'2026-08-19 15:39:43.459'),(8,'喵喵喵',1,6,'2026-08-19 15:39:56.468'),(9,'喵喵喵',4,11,'2026-10-22 20:48:24.212'),(10,'SSE end-to-end test comment',5,12,'2026-10-22 21:07:29.822'),(11,'vtest01 comment to trigger notification',7,13,'2026-10-29 19:34:49.640'),(12,'cross-process check 2',7,13,'2026-10-29 20:28:34.583'),(13,'vm-redis check',7,13,'2026-10-29 20:30:59.305');
/*!40000 ALTER TABLE `comments` ENABLE KEYS */;
UNLOCK TABLES;
DROP TABLE IF EXISTS `follows`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `follows` (
  `follow_id` bigint unsigned NOT NULL,
  `following_id` bigint unsigned NOT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`follow_id`,`following_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

LOCK TABLES `follows` WRITE;
/*!40000 ALTER TABLE `follows` DISABLE KEYS */;
INSERT INTO `follows` VALUES (1,2,'2026-07-29 15:42:20.549'),(2,1,'2026-08-11 15:00:18.978'),(2,3,'2026-07-27 15:36:22.512');
/*!40000 ALTER TABLE `follows` ENABLE KEYS */;
UNLOCK TABLES;
DROP TABLE IF EXISTS `likes`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `likes` (
  `user_id` bigint unsigned NOT NULL,
  `post_id` bigint unsigned NOT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`user_id`,`post_id`),
  KEY `fk_likes_post` (`post_id`),
  CONSTRAINT `fk_likes_post` FOREIGN KEY (`post_id`) REFERENCES `posts` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

LOCK TABLES `likes` WRITE;
/*!40000 ALTER TABLE `likes` DISABLE KEYS */;
INSERT INTO `likes` VALUES (1,6,'2026-08-19 15:39:50.709'),(2,5,'2026-08-19 15:39:30.624'),(2,6,'2026-08-19 15:34:49.931'),(3,3,'2026-05-07 10:21:56.327'),(3,4,'2026-05-07 10:21:36.503');
/*!40000 ALTER TABLE `likes` ENABLE KEYS */;
UNLOCK TABLES;
DROP TABLE IF EXISTS `notifications`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `notifications` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `recipient_id` bigint unsigned DEFAULT NULL,
  `sender_id` bigint unsigned DEFAULT NULL,
  `type` longtext,
  `target_id` bigint unsigned DEFAULT NULL,
  `is_read` tinyint(1) DEFAULT '0',
  `created_at` datetime(3) DEFAULT NULL,
  `dedup_key` varchar(36) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_notifications_dedup_key` (`dedup_key`),
  KEY `idx_notifications_recipient_id` (`recipient_id`)
) ENGINE=InnoDB AUTO_INCREMENT=8 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

LOCK TABLES `notifications` WRITE;
/*!40000 ALTER TABLE `notifications` DISABLE KEYS */;
INSERT INTO `notifications` VALUES (1,1,2,'like',6,1,'2026-08-19 15:34:49.937',NULL),(2,1,2,'comment',6,1,'2026-08-19 15:39:43.465',NULL),(3,1,4,'comment',11,0,'2026-10-22 21:06:33.135','d8039a48-186d-4cad-8bc2-ccf643c71cb0'),(4,6,5,'comment',12,0,'2026-10-22 21:07:30.380','43f343ba-efe6-47e7-8f80-4fd23c2aefe7'),(5,8,7,'comment',13,0,'2026-10-29 19:34:49.787','bd42b906-3db2-4042-8656-255d14f49989'),(6,8,7,'comment',13,0,'2026-10-29 20:28:35.352','fb19d2a6-1fda-4cc0-b9e2-d397d5505177'),(7,8,7,'comment',13,0,'2026-10-29 20:30:59.592','cdc0fea7-5fcc-491b-ac4f-bb88dbeff330');
/*!40000 ALTER TABLE `notifications` ENABLE KEYS */;
UNLOCK TABLES;
DROP TABLE IF EXISTS `outboxes`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `outboxes` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `event_id` varchar(36) DEFAULT NULL,
  `event_type` varchar(191) DEFAULT NULL,
  `payload` text,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_outboxes_event_id` (`event_id`),
  KEY `idx_outboxes_event_type` (`event_type`)
) ENGINE=InnoDB AUTO_INCREMENT=6 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

LOCK TABLES `outboxes` WRITE;
/*!40000 ALTER TABLE `outboxes` DISABLE KEYS */;
/*!40000 ALTER TABLE `outboxes` ENABLE KEYS */;
UNLOCK TABLES;
DROP TABLE IF EXISTS `posts`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `posts` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `title` longtext NOT NULL,
  `content` text NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  `like_count` bigint DEFAULT '0',
  PRIMARY KEY (`id`),
  KEY `fk_posts_user` (`user_id`),
  CONSTRAINT `fk_posts_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=14 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

LOCK TABLES `posts` WRITE;
/*!40000 ALTER TABLE `posts` DISABLE KEYS */;
INSERT INTO `posts` VALUES (1,'我的第一篇博客','Hello from Apifox!',1,'2026-04-28 20:09:59.382',0),(2,'我的第二篇博客','Hello from Apifox!',1,'2026-04-28 20:12:13.059',0),(3,'woshi帅哥','无需多言',2,'2026-05-06 19:07:16.063',1),(4,'a','潇洒',2,'2026-05-06 19:26:07.985',1),(5,'az','喵喵喵',2,'2026-08-11 14:11:24.458',1),(6,'关于爱丽丝','自卑敏感玻璃心',1,'2026-08-12 14:50:58.847',2),(7,'试试','http.Status OK = 200 =  成功\nhttp.status created=201=创建成功\nhttp.Status BadRequest = 400 =  参数错了\nhttp.Status Unauthorized = 401 = 未登录\nhttp.status forbidden=403=没有权限\nhttp.Status Conflict = 409 = 数据重复\nhttp.status notfound=404=资源不存在\nhttp.status internalservererror=500=服务器错误，挂了，数据库，代码panic\nhttp.status BadGateway=502=网关错误=连接不到后端',1,'2026-08-20 17:59:12.618',0),(8,'再试试','ctx	Context	上下文（超时/取消信号）\nreq / r	Request	请求\nres / resp	Response	响应\nerr	Error	错误（必看）\ndb	Database	数据库\ncache / rdb	Redis Cache	缓存\ncfg / conf	Configuration	配置\nsvc	Service	业务服务层\nrepo	Repository	数据仓库层（存/取数据）\nhdl / ctrl	Handler / Controller	控制器（处理路由）\nmid / mw	Middleware	中间件（鉴权/日志）\nwg	WaitGroup	等待组（并发控制）\nmu	Mutex	互斥锁\nch	Channel	管道\ncl	Client	客户端\npk	Private Key	私钥\npub	Public Key	公钥\ndto	Data Transfer Object	数据传输对象（接口参数）\ndao	Data Access Object	数据访问对象（表映射）\nuid	User ID	用户ID\n\nhttp.Status OK = 200 =  成功\nhttp.status created=201=创建成功\nhttp.Status BadRequest = 400 =  参数错了\nhttp.Status Unauthorized = 401 = 未登录\nhttp.status forbidden=403=没有权限\nhttp.Status Conflict = 409 = 数据重复\nhttp.status notfound=404=资源不存在\nhttp.status internalservererror=500=服务器错误，挂了，数据库，代码panic\nhttp.status BadGateway=502=网关错误=连接不到后端',1,'2026-08-20 17:59:45.695',0),(9,'凑数','请求一个根本不存在的 ID（比如 GET /api/posts/999999）。因为没有缓存，每次都查 MySQL，MySQL\n  返回\"查不到\"，这个\"空结果\"也没缓存，下次又查。攻击者用脚本刷大量不存在的 ID，就能把 DB 打爆\n穿透就是查不存在的数据，用空值缓存（或布隆过滤器）拦截，避免每次都打 DB\n\n一篇爆款文章（热点）缓存过期了。过期那一瞬间，1000 个请求同时发现缓存没了，1000 个请求同时打 DB，DB\n  扛不住。这和穿透的区别：穿透是\"数据本来就不存在\"，击穿是\"数据存在但缓存刚好过期\n击穿就是热点 key 过期瞬间大量请求打 DB，用\n  singleflight（或互斥锁）让同一时刻只有一个请求回源，其他请求共享结果',1,'2026-08-20 18:00:30.227',0),(10,'到哪里了e','要凑到翻页说是',1,'2026-08-20 18:00:56.366',0),(11,'来罢！！','冲刺',1,'2026-08-20 18:01:12.546',0),(12,'SSE ?????','???? outbox ? MQ ? consumer ? redis ? sse ???',6,'2026-10-22 21:07:21.881',0),(13,'Step5 ?????','???? API ? worker ??????????',8,'2026-10-29 19:34:40.304',0);
/*!40000 ALTER TABLE `posts` ENABLE KEYS */;
UNLOCK TABLES;
DROP TABLE IF EXISTS `users`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `users` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `username` varchar(191) NOT NULL,
  `password` longtext NOT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  `bio` longtext,
  `avatar_url` longtext,
  `token` longtext,
  `refresh_token` longtext,
  `email` longtext,
  `email_verified` tinyint(1) DEFAULT '0',
  `verify_token` longtext,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_users_username` (`username`)
) ENGINE=InnoDB AUTO_INCREMENT=9 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

LOCK TABLES `users` WRITE;
/*!40000 ALTER TABLE `users` DISABLE KEYS */;
INSERT INTO `users` VALUES (1,'alice','$2a$10$dxLn.IxJAmk.Jevi.f9PZeAMXuq4eNiKUExJLVavx4jv3Fcw3B/cO','2026-04-28 19:33:45.112','不要攻击我','/uploads/avatars/754f89df-d900-44b0-9e05-7e0fd1141a2c.jpg','eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoxLCJ1c2VybmFtZSI6ImFsaWNlIiwiZXhwIjoxNzkzNjIyMzI2fQ.oqG3oz5Wtip0JmK-fW0J76dab8fuSX-pe4FT6mBh-Us','de2022adb35831dbbbc7b8301707f46fbb94c7ae812ef6a3775917a942572f67','hao583349@gmail.com',1,''),(2,'123456','$2a$10$QllfR26.NcJKpPb7prlzXOSy7dN0lnz7aW9VQLkuwjVu3D40HWcNi','2026-05-06 19:06:49.220','喵','/uploads/avatars/604e990e-764c-448d-a4b2-49f8787ff19e.jpg','','','2463301806@qq.com',0,NULL),(3,'hao','$2a$10$nH7/ODfMCqx/myAazuLPX.H4HFmQ4s3HCnocP/IUo83wmIaW4T1N.','2026-05-07 10:21:12.846',NULL,NULL,NULL,NULL,NULL,0,NULL),(4,'456','$2a$10$0pU1cgUh2vBhDQoy8Hwcg.LeS4fykF1jOm1JniwdZA6lg34rp2I4.','2026-09-15 10:32:55.558','','','','','827278063@qq.com',0,NULL),(5,'ssetest01','$2a$10$TyQ.nXMU5dVx8tAsZbGLAe/qu0u25.CZwG6JAp/.MZAQfSHtm5YNW','2026-10-22 21:07:16.721','','','eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjo1LCJ1c2VybmFtZSI6InNzZXRlc3QwMSIsImV4cCI6MTc5MjY3ODA0MX0.1ZW5f_xrENQRI-2Ca1n2OnylOdhHsb-75Kix64X77ks','0e7afd743446843bb9fd4970668d109d45f6acd9c9fada1364b2649aab945bd8','',0,NULL),(6,'ssetest02','$2a$10$mDYm/lr01T99eZEgF97hHeU0e/Gqg84BTu4aIV461TiktFVBeYgaW','2026-10-22 21:07:21.752','','','eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjo2LCJ1c2VybmFtZSI6InNzZXRlc3QwMiIsImV4cCI6MTc5MjY3ODA0MX0.R3mqnJQ_eIM5itHc4uc5wwQ-XaTxwVhe6j4oIlmPPPw','3b1ce66de67c3aa4ec09c02a758542c1166c8e3b47357b34d9bdc1400f34eb37','',0,NULL),(7,'vtest01','$2a$10$lJpQNn3lpq6apQZg/R5OKO3ovQv/flylF0xod7WrPFzQ74.jrgU9q','2026-10-29 19:34:35.500','','','eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjo3LCJ1c2VybmFtZSI6InZ0ZXN0MDEiLCJleHAiOjE3OTMyODA2NTB9.uFZ8KZ4bDtcH9QFJ36qzmxaXLoBGjJaVGRFrIKC4ji8','f9446dcf3ae6351a5c252deefc73cbd8b749bdbff734ecd8a0974c1aa36a01e3','',0,NULL),(8,'vtest02','$2a$10$V9spnOyHK5AFdSysVFoABOXbf1g5Lf/QFvX5k/lFeKa93UiYjYXki','2026-10-29 19:34:35.576','','','eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjo4LCJ1c2VybmFtZSI6InZ0ZXN0MDIiLCJleHAiOjE3OTMyODA2NTB9.zDvPg6jyaClJYG5YaCPzoesMWBVmRx40xS35F1HMcZs','24943d5329b9eff6b7be1dbbaa5c3fcff624c714a9a62d5360e75a3e7a86865e','',0,NULL);
/*!40000 ALTER TABLE `users` ENABLE KEYS */;
UNLOCK TABLES;
/*!40103 SET TIME_ZONE=@OLD_TIME_ZONE */;

/*!40101 SET SQL_MODE=@OLD_SQL_MODE */;
/*!40014 SET FOREIGN_KEY_CHECKS=@OLD_FOREIGN_KEY_CHECKS */;
/*!40014 SET UNIQUE_CHECKS=@OLD_UNIQUE_CHECKS */;
/*!40101 SET CHARACTER_SET_CLIENT=@OLD_CHARACTER_SET_CLIENT */;
/*!40101 SET CHARACTER_SET_RESULTS=@OLD_CHARACTER_SET_RESULTS */;
/*!40101 SET COLLATION_CONNECTION=@OLD_COLLATION_CONNECTION */;
/*!40111 SET SQL_NOTES=@OLD_SQL_NOTES */;

