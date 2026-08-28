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

- 注册、登录、短期 Access JWT、HttpOnly 轮换刷新会话、登出和个人资料维护
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
│   ├── migrate/                          # 带锁、版本和校验和的数据库迁移器
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

开发模式固定读取 `deployments/docker/.env.dev`。生产部署应复制 `deployments/docker/.env.example` 为受保护的 `.env`，填写不可变 `IMAGE_TAG`、数据库/Redis/NATS/对象存储凭据以及至少 32 字符的 `JWT_ACCESS_SECRET`、`METRICS_TOKEN`。生产配置缺失或仍使用开发凭据时，API 与 WS 会直接拒绝启动。

### 方式一.1：GitHub Actions 自动部署到服务器

`.github/workflows/ci.yml` 已配置为：只有在 GitHub Release 状态变为 `published` 后才运行，代码普通 `push` 不会触发部署。工作流会检出该 Release 的 tag，先执行测试，再构建并推送 API、WS、Web 三个不可变 SHA 标签镜像到 GHCR，然后通过 SSH 将生产 Compose 文件上传到服务器，执行 `docker compose pull`、数据库迁移和 `docker compose up -d --wait`。生产数据库、Redis、NATS、MinIO 数据卷会保留，不会因发布新镜像而删除。

服务器首次部署（以 `/opt/go-hubvas` 为例）：

```bash
sudo mkdir -p /opt/go-hubvas
sudo chown -R "$USER":"$USER" /opt/go-hubvas
cd /opt/go-hubvas
git clone https://github.com/SevenJL/go-hubvas.git .
cp deployments/docker/.env.example deployments/docker/.env
chmod 600 deployments/docker/.env
# 编辑 .env，替换所有生产凭据和 IMAGE_TAG
```

服务器需要安装 Docker Engine 和 Docker Compose v2，并确保 SSH 用户能够执行 Docker（加入 `docker` 用户组或使用 root）。如果 GHCR 镜像为私有包，还需要在 GitHub Actions Secrets 中添加一个具备 `read:packages` 权限的 Token。

在 GitHub 仓库的 `Settings → Secrets and variables → Actions` 添加：

| Secret | 说明 |
|---|---|
| `DEPLOY_HOST` | 服务器 IP 或域名 |
| `DEPLOY_USER` | SSH 用户名 |
| `DEPLOY_PORT` | SSH 端口，通常为 `22` |
| `DEPLOY_SSH_KEY` | 对应服务器 `authorized_keys` 的私钥 |
| `DEPLOY_KNOWN_HOSTS` | 服务器 SSH 主机指纹，可在可信环境执行 `ssh-keyscan -H <host>` 后核对 |
| `DEPLOY_PATH` | 项目目录，默认 `/opt/go-hubvas` |
| `GHCR_DEPLOY_USERNAME` | 仅私有 GHCR 包需要 |
| `GHCR_DEPLOY_TOKEN` | 仅私有 GHCR 包需要，至少具备 `read:packages` |

不要把生产 `.env`、数据库密码、JWT 密钥或 SSH 私钥提交到 Git。每次 GitHub Release 发布后，工作流会将服务器 `.env` 中的 `IMAGE_TAG` 更新为该 Release tag 对应提交的 SHA，并等待 Web、API、WS 服务健康后才报告部署成功。

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

# API 与 WS 必须使用相同的 Access JWT 配置
export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=postgres
export DB_PASSWORD=postgres
export DB_NAME=hubvas
export JWT_ACCESS_SECRET=dev-access-secret

# 使用版本化迁移器；可重复运行，并通过 advisory lock 防止多实例并发迁移
go run ./cmd/migrate
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

连接地址只包含非敏感画布参数：

```text
/ws?canvas=<canvas_id>
```

浏览器通过 `Sec-WebSocket-Protocol` 提交 Access JWT，当前协议为固定的 `hubvas` 加 `hubvas.access.<jwt>` 凭据项；服务端只回显固定协议名，不在 URL、访问日志或选中的子协议中回显令牌。服务端在升级连接前会验证 JWT、用户身份、画布访问权限及编辑角色。Viewer 可以接收同步和 Presence，但服务端会拒绝其编辑、锁定等写操作。

消息信封：

```json
{
  "type": "sync|awareness|presence|lock|unlock|lock_state|chat|ack|error",
  "seq": 12345,
  "payload": {}
}
```

主要同步流程：

1. 客户端通过 WebSocket 子协议携带 JWT，并使用画布 ID 建立连接。
2. 服务端校验身份和权限，将用户加入 Room。
3. 冷房间从持久化快照恢复；客户端接收完整状态及在线成员。
4. 绘制过程中仅广播 Yjs 增量更新、Awareness 与锁状态。
5. NATS 将消息同步到其他 WS 实例，Redis 维护跨实例 Presence 和对象锁。
6. 客户端断线后自动重连，通过完整快照和后续增量恢复一致状态。

## 测试与质量基线

当前后端已覆盖领域模型、Canvas / Community / Social / Media 应用服务、HTTP 路由、WebSocket Client / Hub / Room、快照编解码、MinIO 仓储和 Token Bucket 等测试；前端已接入 Vitest 组件测试和 Playwright 社交流程测试。

```bash
go test ./...
go vet ./...
cd frontend && npm run test && npm run test:e2e && npm run lint && npm run build
```

尚待补充：100 人协作压力测试。完成正式压测前，“百人级”仍表示架构目标，而不是已完成容量认证。

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
| WebSocket 通知集成与社交 E2E | ✅ 已完成 |
| 100 人协作压力测试 | ⏳ 待完善 |
| 导出与版本历史 | ⏳ 待实现 |
| 协作聊天 | ⏳ 协议已预留，产品功能待实现 |
| 关注、拉黑与站内通知 | ✅ 已完成 |
| 收藏 | ⏳ 待实现 |
| 模板市场 | ⏳ 待实现 |

## 开发约定

每个领域模块遵循统一分层：

1. `domain/`：定义聚合根、值对象、领域事件和仓储接口。
2. `application/`：定义 DTO、应用服务与用例编排。
3. `infrastructure/`：实现 PostgreSQL、Redis、NATS、MinIO 等适配器。
4. `interfaces/`：提供 HTTP Handler、Middleware 与 WebSocket Gateway。

更多协作设计说明见 `docs/协作画布开发文档.md`。

## 生产级社交基础能力

当前版本已经将原有演示级社区功能扩展为可持久化、可审核、可恢复投递的社交基础版：

- 个人资料：不可变 `username`、可编辑展示昵称/简介/网站、真实头像上传与删除。
- 头像媒体：预签名 PUT 与 multipart 中转双链路；服务端按实际解码结果校验 JPEG/PNG/WebP、5 MB、4000 万像素上限和正方形裁剪，统一输出去元数据的 512×512 WebP。
- 社交图谱：关注/取消关注、粉丝与关注列表、关注内容流、双向拉黑隔离。
- 讨论：一级评论线程、回复、作者软删除、被删除/隐藏占位和拉黑过滤。
- 治理：用户/画布/评论举报去重，管理员审核队列、暂停/恢复用户、隐藏/恢复评论和取消/恢复发布画布。
- 通知：关注、点赞、评论、回复和 Fork 的持久化站内通知；数据库事务 outbox、NATS 重试分发和 `/ws/notifications` 用户级实时推送。客户端重连后仍以 REST 列表和未读数为准补偿。
- 可靠性：领域错误统一映射到 400/401/403/404/409/429；登录、头像、关注、评论、举报和管理员操作使用独立限流器。

### 头像对象存储配置

快照 bucket 与媒体 bucket 相互独立。MinIO 和兼容 S3 的部署至少需要配置：

| 环境变量 | 默认值 | 说明 |
|---|---:|---|
| `STORAGE_ENDPOINT` | `localhost:9000` | MinIO/S3 endpoint |
| `STORAGE_ACCESS_KEY` | 空 | 对象存储访问密钥 |
| `STORAGE_SECRET_KEY` | 空 | 对象存储秘密密钥 |
| `STORAGE_USE_SSL` | `false` | 是否使用 HTTPS |
| `STORAGE_BUCKET` | `hubvas-snapshots` | 画布快照私有 bucket |
| `STORAGE_MEDIA_BUCKET` | `hubvas-media` | 头像临时对象和成品 bucket |
| `STORAGE_PUBLIC_BASE_URL` | 空 | 头像公开/CDN 基础地址；Compose 使用 `/media` |
| `STORAGE_PRESIGN_TTL` | `15m` | 临时上传和预签名地址有效期 |
| `STORAGE_AVATAR_MAX_BYTES` | `5242880` | 头像最大字节数 |

临时对象位于 `tmp/avatars/{userID}/{uploadID}`，必须保持私有；成品位于 `avatars/{userID}/{version}.webp`，可通过 CDN 或只读代理公开。Compose 的 `minio-init` 已创建两个 bucket，仅对媒体 bucket 的 `avatars` 前缀开放下载，Nginx 为版本化成品返回 immutable 缓存头。

### 管理员初始化

所有迁移后的账号默认都是普通用户。部署人员需要在数据库中显式提升管理员：

```sql
UPDATE users
SET account_role = 'admin', updated_at = now()
WHERE username = 'admin_username';
```

不要通过前端请求或注册参数授予管理员角色。管理员接口位于 `/api/admin/*`，每次调用都会从数据库重新校验角色。

### 主要新增接口

- `PATCH /api/auth/profile`、`DELETE /api/auth/avatar`
- `POST /api/media/avatars/presign`、`POST /api/media/avatars/complete`、`POST /api/media/avatars`
- `GET /api/users/:username`、`GET /api/users/:username/canvases`
- `POST|DELETE /api/users/:id/follow`、`GET /api/users/:id/followers|following`
- `GET /api/community/following`
- `GET /api/notifications`、`GET /api/notifications/unread-count`、通知已读接口
- `POST|DELETE /api/users/:id/block`、`GET /api/blocks`
- `POST /api/reports` 与 `/api/admin/reports`、用户/评论/画布审核接口
- `GET /ws/notifications`（JWT 通过 WebSocket 子协议传输）

### 测试与验收

```bash
# 后端单元/集成与静态检查
go test ./...
go vet ./...

# 前端组件、浏览器流程、代码质量和生产构建
cd frontend
npm run test
npm run test:e2e
npm run lint
npm run build
```

数据库迁移统一通过 `go run ./cmd/migrate` 或 Compose 的 `migrate` 服务执行；迁移器使用 PostgreSQL advisory lock、事务、版本记录和 SHA-256 校验和，并可安全重复运行。对象存储验收需要同时验证预签名 PUT 和 multipart 两条链路最终都生成 WebP，并确认替换/删除头像后旧对象被清理。

> 生产环境的 HTTP 全局与细粒度限流使用 Redis/Lua 原子 Token Bucket，可在多个 API 副本间共享额度；开发环境仅在 Redis 不可用时回退到带空闲 key 回收的进程内限流。生产启动会要求 PostgreSQL、Redis、NATS 和对象存储均可用。

## 生产部署安全基线

- Access JWT 包含 issuer、audience、subject、`jti`、`iat`、`nbf`、`exp`、`sv`（security version）和 `typ=at+jwt`，并严格限制 HS256；修改密码、全部退出和管理员变更账号状态会立即撤销旧 Access Token。
- Refresh Token 是 32 字节随机不透明凭据，仅将 SHA-256 哈希保存在 PostgreSQL；每次刷新都会轮换，检测到旧令牌复用时撤销整个会话族。
- Refresh Token 只写入 `HttpOnly` Cookie；前端不再将认证令牌保存到 localStorage，并对并发 401 只发起一次刷新请求。
- `/health` 仅表示进程存活，`/ready` 检查 PostgreSQL、Redis、NATS 和对象存储；容器编排仅把 readiness 成功的实例加入服务。
- `/metrics` 在生产环境必须使用至少 32 字符的 Bearer Token 保护：`curl -H "Authorization: Bearer $METRICS_TOKEN" http://localhost:8080/metrics`。
- API/WS 使用读头、读写、空闲和优雅关闭超时；运行镜像使用非 root 用户。
- `TRUSTED_PROXIES` 必须与实际反向代理网段一致，否则客户端 IP、审计信息和按 IP 限流可能不准确。
- 生产 Compose 强制指定不可变 `IMAGE_TAG`，禁止默默回退到 `latest`。
- PostgreSQL 与对象存储备份、校验和验证及恢复步骤见 `deployments/backup/README.md`。

## License

MIT
