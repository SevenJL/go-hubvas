# Hubvas

Hubvas 是一个面向多人实时协作场景的在线画布平台。用户可以使用 tldraw 进行绘制，通过 Yjs 与 WebSocket 实时同步画布状态，并将作品发布到社区进行浏览、点赞、评论、Fork 与二次创作。

> 当前项目处于持续开发阶段。核心画布、协作、权限、成员管理、社区与容器化部署链路已经可用；导出、版本历史、聊天等扩展能力仍在规划中。

## 功能概览

### 画布与协作

- tldraw 画布编辑、缩放、平移和多页面状态同步
- Yjs CRDT 增量同步、完整快照恢复和断线重连
- 在线成员、远程光标、选区与当前页面感知
- Redis 分布式 Presence 和协作对象锁
- NATS 跨实例房间消息扇出
- MinIO / S3 快照持久化与冷房间按需加载
- 服务端校验 WebSocket JWT、可信用户身份和画布编辑权限
- 只读成员默认使用“手型”工具，只允许浏览、平移和缩放
- 客户端节流、服务端 Token Bucket、慢客户端隔离和心跳检测

### 画布权限与成员

- 私有、共享和已发布画布访问控制
- Owner / Editor / Viewer 成员角色
- 成员邀请、角色调整和成员移除
- “我的画布”与“共享给我”列表
- 私有画布详情、快照读取和 Fork 权限检查

### 社区与账户

- 注册、登录、JWT 刷新和个人资料维护
- 社区作品分页、标签筛选、最新 / 热门 / 趋势排序
- 发布、点赞、取消点赞、评论和 Fork
- 作者信息、当前用户点赞状态与 Fork 计数
- Dashboard、Community、Canvas Detail、Editor、Profile 等页面
- 中文 / English 切换并使用 localStorage 保存语言偏好
- Toast、Modal、Skeleton / Loading 等统一交互反馈

## 近期更新

以下内容根据 **2026-07-11 至 2026-07-12** 的 Git 提交整理：

| 日期 | 更新 |
|------|------|
| 2026-07-12 | 修复社区趋势查询在 PostgreSQL 中因 `SELECT DISTINCT` 与计算排序表达式冲突导致的 `SQLSTATE 42P10`，并补充 SQL 构建测试与稳定分页排序 |
| 2026-07-12 | 完成前端中英文切换，覆盖认证、Dashboard、社区、详情、编辑器、成员管理、评论和通用反馈组件 |
| 2026-07-12 | 只读画布默认切换为“手型”工具，避免用户误以为可以选择或编辑对象 |
| 2026-07-12 | 增加 Modal、Toast、Loading 等统一组件，改善页面加载、错误提示和操作确认体验 |
| 2026-07-12 | 接入协作对象锁并修复 Dashboard 共享画布路由匹配问题 |
| 2026-07-12 | 完成成员邀请、角色管理、共享画布列表、社区数据一致性和统一配置加载基线 |
| 2026-07-12 | 优化 tldraw / Yjs 实时同步、远程光标页面感知、增量更新和快照编码，降低协作延迟 |
| 2026-07-11 | 完成协作权限校验、冷房间快照恢复、Redis Presence / Lock、Throttle 与 NATS 多实例协作生命周期 |

## 架构

项目采用 DDD（领域驱动设计）四层架构：

```text
interfaces/       → HTTP（Gin）与 WebSocket（coder/websocket）
application/      → 用例编排、DTO 与应用服务
domain/           → 聚合根、实体、值对象、领域事件与仓储接口
infrastructure/   → PostgreSQL、Redis、NATS、MinIO、JWT 等实现
```

### 限界上下文

| 上下文 | 类型 | 主要职责 |
|--------|------|----------|
| `identity` | 通用子域 | 用户、注册登录与身份认证 |
| `canvas` | 核心子域 | 画布、成员、角色、可见性与权限 |
| `collaboration` | 核心域 | Room、在线成员、CRDT 更新、锁与快照 |
| `community` | 支撑子域 | 发布、标签、点赞、评论与 Fork |
| `export` | 支撑子域 | 导出能力，待实现 |

## 技术栈

| 层 | 选型 |
|----|------|
| 后端语言 | Go 1.25.1 |
| HTTP | Gin |
| WebSocket | coder/websocket |
| 数据库 | PostgreSQL 15+、pgx/v5 |
| 分布式状态 | Redis 7 |
| 消息系统 | NATS 2 |
| 对象存储 | MinIO / S3 |
| 认证 | JWT HS256、bcrypt |
| 前端 | React 19、TypeScript 6、Vite 8、Tailwind CSS 4 |
| 画布与协作 | tldraw 5、Yjs 13 |
| 部署 | Docker Compose、Nginx |

## 项目结构

```text
go-hubvas/
├── cmd/
│   ├── api-server/                       # REST API，默认 :8080
│   └── ws-server/                        # WebSocket，默认 :8081
├── frontend/                             # React + TypeScript + tldraw + Yjs
├── internal/
│   ├── domain/                           # 领域模型与仓储接口
│   ├── application/                      # 应用服务与 DTO
│   ├── infrastructure/
│   │   ├── auth/                         # JWT、bcrypt、PermissionService
│   │   ├── messaging/nats/               # 跨节点消息扇出
│   │   ├── persistence/postgres/         # PostgreSQL 仓储与迁移
│   │   ├── persistence/redis/            # Presence 与对象锁
│   │   ├── storage/minio/                # 协作快照存储
│   │   └── throttle/                     # Token Bucket 限流
│   └── interfaces/
│       ├── http/                         # Gin Router、Middleware、Handler
│       └── ws/                           # Gateway、Hub、Room、Client、协议
├── pkg/
│   ├── config/                           # YAML + 环境变量统一配置
│   ├── idgen/                            # Snowflake ID
│   └── logger/                           # 结构化日志
├── configs/config.yaml                   # 默认配置
├── deployments/docker/                   # Dockerfile、Compose、Nginx
├── docs/                                 # 设计与开发文档
└── Makefile
```

## 快速开始

### 方式一：Docker Compose（推荐）

前置条件：Docker 与 Docker Compose。

```bash
git clone https://github.com/SevenJL/go-hubvas.git
cd go-hubvas

# 开发模式：从当前源码构建，并启动 Web、API、WS、PostgreSQL、
# Redis、NATS 和 MinIO；数据库首次启动时自动执行全部迁移。
make docker-up
```

启动后访问：

- Web 入口：`http://localhost:6161`
- API：`http://localhost:8080`
- WebSocket：`ws://localhost:8081/ws`
- MinIO Console：`http://localhost:9001`

```bash
# 查看服务状态或日志
docker compose -f deployments/docker/docker-compose.yaml \
  -f deployments/docker/docker-compose.override.yml ps
docker compose -f deployments/docker/docker-compose.yaml \
  -f deployments/docker/docker-compose.override.yml logs -f

# 停止服务
make docker-down
```

可通过 `WEB_HOST_PORT`、`DB_PASSWORD`、`JWT_ACCESS_SECRET`、`JWT_REFRESH_SECRET` 等环境变量覆盖默认值。生产环境必须替换 JWT 与存储密钥。

### 方式二：本地开发

前置条件：Go 1.25.1、Node.js、npm、PostgreSQL 15+。Redis、NATS 和 MinIO 在单实例开发时可以暂时不启用，但对应的分布式 Presence、对象锁、跨节点同步和持久快照能力会降级。

```bash
git clone https://github.com/SevenJL/go-hubvas.git
cd go-hubvas

# Go 依赖
make tidy

# 前端依赖
cd frontend && npm install && cd ..

# 创建数据库
psql -U postgres -c "CREATE DATABASE hubvas"

# 执行全部迁移
for migration in internal/infrastructure/persistence/postgres/migrations/*.sql; do
  psql -U postgres -d hubvas -f "$migration"
done

# API 与 WS 必须使用相同的 JWT secrets
export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=postgres
export DB_PASSWORD=postgres
export DB_NAME=hubvas
export JWT_ACCESS_SECRET=dev-access-secret-change-in-production
export JWT_REFRESH_SECRET=dev-refresh-secret-change-in-production
```

分别启动三个开发进程：

```bash
# Terminal 1 — REST API
make run-api

# Terminal 2 — WebSocket
make run-ws

# Terminal 3 — 前端；Vite 会将 /api 和 /ws 分别代理到 8080 与 8081
cd frontend
npm run dev
```

打开 `http://localhost:5173`。

配置默认读取 `configs/config.yaml`，也可以通过 `HUBVAS_CONFIG` 指定其他配置文件；环境变量会覆盖 YAML 配置。

## 常用命令

```bash
# 后端构建与测试
make build
make test
make test-race
make lint

# 同时启动 API 与 WS
make dev

# 前端检查与构建
cd frontend
npm run lint
npm run build

# 容器构建与运行
make docker-build
make docker-up
make docker-down
```

## API 概览

### 公开接口

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/health` | API 健康检查 |
| `GET` | `/ready` | API 就绪检查 |
| `POST` | `/api/auth/register` | 注册 |
| `POST` | `/api/auth/login` | 登录 |
| `POST` | `/api/auth/refresh` | 刷新 Token |
| `GET` | `/api/community` | 浏览社区作品 |
| `GET` | `/api/community/:id` | 社区作品详情 |
| `GET` | `/api/canvases/:id` | 获取有权访问的画布 |
| `GET` | `/api/canvases/:id/comments` | 评论列表 |
| `GET` | `/api/canvases/:id/like-status` | 当前点赞状态（可选认证） |
| `GET` | `/api/canvases/:id/snapshot` | 获取有权访问的画布快照 |

### 认证接口

以下接口要求 `Authorization: Bearer <access_token>`：

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/auth/me` | 当前用户 |
| `PUT` | `/api/auth/profile` | 更新个人资料 |
| `POST` | `/api/canvases` | 创建画布 |
| `GET` | `/api/canvases` | 我的画布 |
| `GET` | `/api/canvases/shared` | 共享给我的画布 |
| `GET/POST` | `/api/canvases/:id/members` | 查看或邀请成员 |
| `PUT/DELETE` | `/api/canvases/:id/members/:userId` | 调整角色或移除成员 |
| `POST` | `/api/canvases/:id/publish` | 发布画布 |
| `POST` | `/api/canvases/:id/fork` | Fork 画布 |
| `DELETE` | `/api/canvases/:id` | 删除画布 |
| `POST/DELETE` | `/api/canvases/:id/like` | 点赞或取消点赞 |
| `POST` | `/api/canvases/:id/comments` | 发表评论 |
| `PUT` | `/api/canvases/:id/snapshot` | 保存画布快照 |

## WebSocket 协作

连接地址：

```text
/ws?canvas=<canvas_id>&token=<jwt>
```

服务端在升级连接前会验证 JWT、用户身份、画布访问权限及编辑角色。Viewer 可以接收同步和 Presence，但服务端会拒绝其编辑、锁定等写操作。

消息信封：

```json
{
  "type": "sync|awareness|presence|lock|unlock|lock_state|chat|ack|error",
  "seq": 12345,
  "payload": {}
}
```

主要同步流程：

1. 客户端携带 JWT 与画布 ID 建立连接。
2. 服务端校验身份和权限，将用户加入 Room。
3. 冷房间从持久化快照恢复；客户端接收完整状态及在线成员。
4. 绘制过程中仅广播 Yjs 增量更新、Awareness 与锁状态。
5. NATS 将消息同步到其他 WS 实例，Redis 维护跨实例 Presence 和对象锁。
6. 客户端断线后自动重连，通过完整快照和后续增量恢复一致状态。

## 测试与质量基线

当前后端已覆盖领域模型、Canvas / Community 应用服务、配置加载、HTTP 路由、WebSocket Client / Hub / Room、快照编解码、MinIO 仓储和 Token Bucket 等测试。

```bash
go test ./...
cd frontend && npm run lint && npm run build
```

尚待补充：完整 WebSocket 集成测试、浏览器 E2E 测试和 100 人协作压力测试。上述项目完成前，“百人级”表示架构目标，而不是已经完成正式压测认证。

## 开发进度

| 能力 | 状态 |
|------|------|
| 账户、JWT、个人资料 | ✅ 已完成 |
| 画布 CRUD、访问权限与成员角色 | ✅ 已完成 |
| tldraw + Yjs 实时协作基线 | ✅ 已完成 |
| 冷房间加载、完整快照与重连恢复 | ✅ 已完成 |
| Redis Presence、对象锁与 Throttle | ✅ 已完成 |
| NATS 多实例消息互通 | ✅ 已完成 |
| 社区发布、点赞、评论、Fork 与趋势排序 | ✅ 已完成 |
| 前端中英文切换与统一交互反馈 | ✅ 已完成 |
| Docker Compose 完整部署 | ✅ 已完成 |
| WebSocket 集成测试、E2E、100 人压力测试 | ⏳ 待完善 |
| 导出与版本历史 | ⏳ 待实现 |
| 协作聊天 | ⏳ 协议已预留，产品功能待实现 |
| 收藏、关注与通知 | ⏳ 待实现 |
| 模板市场 | ⏳ 待实现 |

## 开发约定

每个领域模块遵循统一分层：

1. `domain/`：定义聚合根、值对象、领域事件和仓储接口。
2. `application/`：定义 DTO、应用服务与用例编排。
3. `infrastructure/`：实现 PostgreSQL、Redis、NATS、MinIO 等适配器。
4. `interfaces/`：提供 HTTP Handler、Middleware 与 WebSocket Gateway。

更多协作设计说明见 `docs/协作画布开发文档.md`。

## License

MIT
