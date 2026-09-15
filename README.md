
# MediaUnlock

流媒体解锁检测与节点分流管理系统，包含 Go Server、Go Agent 和 React Web 管理端。

## 快速开始

**1. 启动 Server**

首次启动会自动建库、创建管理员 `admin`，并把**随机密码打印到启动日志**（不硬编码弱口令）。

```bash
cd server
cp config.example.yml config.yml        # 按需改端口 / DB 路径 / JWT 密钥
go run ./cmd/server -config config.yml
```

启动日志里会有：

```
WARN 初始管理员已创建,请尽快登录并修改密码 username=admin password=<随机密码>
```

用它登录后请在「系统配置」里改密。忘记密码时删掉 `users` 表里的记录再重启，会重新生成一个。

**2. 前端开发**（可选；`vite` 已把 `/api` 代理到 `localhost:8080`）

```bash
cd web
npm install
npm run dev
```

## 构建

```bash
./scripts/build.sh
```

脚本会构建前端 `web/dist` → 同步进 `server/internal/static/web` → 编译 **Server（内嵌前端）与 Agent**。
产物在 `dist/`：

```
dist/server_<os>_<arch>   # 单文件，直接托管前端，不需要单独部署
dist/agent_<os>_<arch>    # 部署到各节点
```

常用参数：

```bash
PLATFORMS="linux/amd64 linux/arm64" ./scripts/build.sh  # 交叉编译（默认编本机平台）
SKIP_WEB=1 ./scripts/build.sh                           # 前端不重新构建，直接用现有 web/dist
SKIP_AGENT=1 ./scripts/build.sh                         # 只编 Server
```

`server/internal/static/web/` 里的内容是构建产物，被 git 忽略（只保留一个 `.gitkeep` 占位）。
新 clone 后直接 `go run ./cmd/server`，前端还没构建，首页会返回一个带构建指引的提示页。

发布也可以直接走 GitHub Actions：打 `v*` tag 会自动编两个架构并附到 Release，
Server 还会推一份多架构镜像到 ghcr.io（`.github/workflows/`）。

## Docker

`scripts/docker/docker-compose.yml` 直接用 ghcr 上发布的镜像（公开包，不用登录）：

```bash
docker compose -f scripts/docker/docker-compose.yml up -d
```

compose 把宿主机上的配置和数据目录挂进容器：

| 容器内 | 放什么 |
|---|---|
| `/app/config.yml` | 配置文件，照 `server/config.example.yml` 写一份（首次记得改 `jwt.secret`） |
| `/app/data` | SQLite 数据库，`database.path` 保持默认的 `./data/database.db` 就能对上 |

挂载的宿主机路径在 compose 里指定，按自己的目录结构改。初始管理员的随机密码在容器日志里：

```bash
docker compose -f scripts/docker/docker-compose.yml logs server
```

镜像由 `.github/workflows/server.yml` 在打 `v*` tag 时构建推送（多架构 amd64 + arm64）：
`ghcr.io/hututuono/mediaunlock-server`，tag 名同 git tag，`latest` 只在打 tag 时更新、
始终指向最近一次发布 —— 想固定版本就把 compose 里的 `:latest` 换成 `:v0.1.0`。

想改代码后立刻验证，就直接构建本地镜像（CI 用的是同一个 Dockerfile）：

```bash
docker build -f server/Dockerfile -t mediaunlock-server .   # 上下文必须是仓库根目录
```

运行参数见 `server/Dockerfile` 顶部注释。

## 测试

```bash
cd server && go test ./...
cd agent  && go test ./...
cd web    && npm run lint
```

约定：测试文件跟随源文件名（`foo.go` ↔ `foo_test.go`）；路由级测试用真实 Gin 引擎 + `t.TempDir()` 临时库，不 mock、不碰开发库。

## 关联模型

节点与平台使用 GORM 原生多对多关系：

```go
Platforms []Platform `gorm:"many2many:node_platforms;"`
Nodes     []Node     `gorm:"many2many:node_platforms;"`
```

`node_platforms` 是 GORM 自动维护的关联表，不定义独立的 `NodePlatform` 结构体，也不需要 `SetupJoinTable`。

这张表**只有 Agent 上报会写**：每次 `POST /api/agent/report`，用本次 `status == 1` 的平台**整体替换**该节点的关联。也就是说它的语义是"该节点**最近一次**检测中成功解锁的平台"，而不是历史累积——历史另存在 `unlocks` 表里（每次检测追加一条）。目前没有独立的管理员绑定接口。
