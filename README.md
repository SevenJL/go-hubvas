# Hubvas

一个支持百人级实时协作的在线画布产品：多人可同时在同一画布上绘制，并能将作品发布到社区进行分享、点赞、Fork 与二次创作。

## 架构

采用 **DDD（领域驱动设计）四层架构**：

```
interfaces/   →  HTTP (Gin) + WebSocket (coder/websocket)
application/  →  用例编排，DTO，应用服务
domain/       →  聚合根、实体、值对象、领域事件、仓储接口（零外部依赖）
infrastructure/ →  PostgreSQL (pgx)、Redis、NATS、MinIO、JWT
```

### 限界上下文

| 上下文 | 类型 | 聚合根 |
|--------|------|--------|
| **identity** | 通用子域 | User |
| **canvas** | 核心子域 | Canvas + CanvasMember |
| **collaboration** | 核心域 | Room + RoomMember（运行时聚合，内存中） |
| **community** | 支撑子域 | PublishedCanvas, Like, Comment, Fork |
| **export** | 支撑子域 | （待实现） |

## 技术栈

| 层 | 选型 |
|----|------|
| 语言 | Go 1.22+ |
| HTTP 框架 | Gin |
| WebSocket | coder/websocket |
| 数据库 | PostgreSQL 15+ (pgx/v5) |
| 缓存 | Redis |
| 消息 | NATS |
| 对象存储 | MinIO / S3 |
| 认证 | JWT (HS256) |
| 前端 | React + TypeScript + Yjs + tldraw |
| 部署 | Docker + Kubernetes |

## 项目结构

```
go-hubvas/
├── cmd/
│   ├── api-server/main.go       # REST API (:8080) ✅ 完整注入可运行
│   └── ws-server/main.go        # WebSocket (:8081)
├── internal/
│   ├── domain/                  # 领域层
│   │   ├── shared/              #   AggregateRoot, Entity, DomainEvent
│   │   ├── identity/            #   User 聚合
│   │   ├── canvas/              #   Canvas 聚合 + 成员
│   │   ├── collaboration/       #   Room 聚合（核心）
│   │   └── community/           #   发布/点赞/评论/Fork
│   ├── application/             # 应用层
│   │   ├── auth/                #   注册/登录
│   │   ├── canvas/              #   画布 CRUD/发布/Fork
│   │   ├── collaboration/       #   房间加入/离开/快照
│   │   └── community/           #   浏览/点赞/评论
│   ├── infrastructure/          # 基础设施层
│   │   ├── persistence/postgres/#   ✅ UserRepo, CanvasRepo, CommunityRepo
│   │   ├── persistence/redis/   #   ✅ PresenceRepo, LockRepository
│   │   ├── messaging/nats/      #   ✅ PubSub (跨节点扇出) + EventBus
│   │   ├── storage/minio/       #   ✅ SnapshotRepo (S3)
│   │   └── auth/                #   ✅ JWT, bcrypt, PermissionService
│   └── interfaces/              # 接口层
│       ├── http/                #   路由 + 中间件 + Handler
│       └── ws/                  #   Hub/Room/Client/Gateway
├── pkg/
│   ├── config/                  # 配置结构体
│   ├── idgen/                   # Snowflake ID 生成器
│   └── logger/                  # 结构化日志
├── configs/config.yaml          # 默认配置
├── deployments/docker/          # Dockerfile
├── docs/                        # 开发文档
└── Makefile
```

## 快速开始

### 前置条件

- Go 1.22+
- PostgreSQL 15+
- Redis（可选，开发阶段可跳过）
- NATS（可选，单机部署可跳过）

### 安装 & 运行

```bash
# 克隆仓库
git clone https://github.com/hubvas/hubvas.git
cd go-hubvas

# 安装依赖
make tidy

# 创建数据库并运行迁移
psql -U postgres -c "CREATE DATABASE hubvas"
psql -U postgres -d hubvas -f internal/infrastructure/persistence/postgres/migrations/001_create_users.sql
psql -U postgres -d hubvas -f internal/infrastructure/persistence/postgres/migrations/002_create_canvases.sql

# 配置环境变量（或编辑 configs/config.yaml）
export DB_HOST=localhost DB_PORT=5432 DB_USER=postgres DB_PASSWORD=postgres DB_NAME=hubvas
export JWT_ACCESS_SECRET=your-secret-key

# 启动 API 服务
go run ./cmd/api-server

# 另一个终端启动 WS 服务
go run ./cmd/ws-server
```

### Docker

```bash
make docker-build
make docker-up
```

## API 端点

| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| POST | `/api/auth/register` | 注册 | 无 |
| POST | `/api/auth/login` | 登录，返回 JWT | 无 |
| GET | `/api/auth/me` | 当前用户信息 | Bearer |
| POST | `/api/canvases` | 创建画布 | Bearer |
| GET | `/api/canvases` | 我的画布列表 | Bearer |
| GET | `/api/canvases/:id` | 画布详情 | Bearer |
| POST | `/api/canvases/:id/publish` | 发布到社区 | Bearer |
| POST | `/api/canvases/:id/fork` | Fork 画布 | Bearer |
| DELETE | `/api/canvases/:id` | 删除画布 | Bearer |
| GET | `/api/community` | 社区作品流 | Bearer |
| POST | `/api/canvases/:id/like` | 点赞 | Bearer |
| DELETE | `/api/canvases/:id/like` | 取消点赞 | Bearer |
| POST | `/api/canvases/:id/comments` | 评论 | Bearer |
| GET | `/api/canvases/:id/comments` | 评论列表 | Bearer |
| GET | `/ws?canvas=<id>&token=<jwt>` | WebSocket 连接 | Query |

## WebSocket 协议

```jsonc
// 消息信封
{
  "type": "sync|awareness|presence|chat|ack|error",
  "seq": 12345,
  "payload": { /* ... */ }
}
```

连接流程：
1. 客户端连接 `/ws?canvas=<id>&token=<jwt>`
2. 服务端校验 → 推送 `presence`（在线列表）+ `sync`（全量快照）
3. 双方仅交换增量 `sync` / `awareness`

## 核心设计

### 单房间单 goroutine 串行

每个 Room 由一个独立 goroutine 管理，所有操作经 `inbound` channel 进入，处理后广播。画布状态无需加锁。

### 慢客户端隔离

- 每连接独立带缓冲 `send` channel（256 条消息）
- Channel 满 → 踢线，客户端自动重连并增量补齐
- 读/写超时 + Ping/Pong 心跳检测僵尸连接

### 冷热房间分离

- 房间空闲 5 分钟 → 快照落盘 → 从内存卸载
- 新用户加入时懒加载，从 MinIO 拉取最新快照

## 开发进度

| 阶段 | 内容 | 状态 |
|------|------|------|
| M1 | 项目骨架、DDD 分层 | ✅ |
| M1 | User 聚合 + JWT + bcrypt | ✅ |
| M1 | Canvas 聚合 + CRUD | ✅ |
| M2 | WebSocket Hub/Room/Client | ✅ |
| M2 | PostgreSQL UserRepo | ✅ |
| M2 | PostgreSQL CanvasRepo | ✅ |
| M2 | PostgreSQL CommunityRepo | ✅ |
| M2 | Redis PresenceRepo + LockRepository | ✅ |
| M2 | MinIO SnapshotRepo (S3) | ✅ |
| M2 | NATS PubSub 跨节点扇出 | ✅ |
| M2 | Snowflake ID 生成器 | ✅ |
| M2 | PermissionService | ✅ |
| M2 | `api-server` 完整依赖注入（可运行） | ✅ |
| M3 | `ws-server` 完整依赖注入 | ⏳ |
| M3 | Yjs CRDT 中继集成 | ⏳ |
| M4 | 前端 React + tldraw | ⏳ |
| M5 | 导出、限流、监控 | ⏳ |

## 开发指南

```bash
# 构建
make build

# 测试
make test
make test-race

# 代码检查
make lint

# 同时启动两个服务
make dev
```

每个领域模块遵循统一的开发模式：

1. **domain/** — 定义聚合根、值对象、领域事件、仓储接口
2. **application/** — 定义 DTO 和用例编排服务
3. **infrastructure/** — 实现 domain 中定义的接口
4. **interfaces/** — HTTP Handler / WebSocket Gateway

## License

MIT
