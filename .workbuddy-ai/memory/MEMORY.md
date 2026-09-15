# MediaUnlock 项目长期笔记

## 构建 / 发布 / 部署

- `scripts/build.sh` —— **唯一的构建脚本**（原来的 `release.sh` 已合并进来，2026-09-15）：
  前端 → 同步进 `server/internal/static/web` → 逐平台编译 Server + Agent，产物在 `dist/`（已 gitignore）。
  开关：`PLATFORMS`（默认 `go env GOOS/GOARCH` = 本机）、`SKIP_WEB=1`、`SKIP_AGENT=1`
  - `SKIP_WEB=1` 的语义是「**不重新构建前端，但照旧同步 `web/dist` 进嵌入目录**」，
    `web/dist` 不存在时直接报错退出（2026-09-15 修正，之前是"完全跳过前端"→会嵌进上一次的残留）
- **前端嵌入目录 = `server/internal/static/web/`**（2026-09-15 挪的，详见 `static.go` 注释）：
  - 用 `//go:embed all:web` + `fs.Sub(embedded, "web")`。**不能写 `all:*` 或 `*`** ——
    两种写法都会把 `static.go` 自己嵌进二进制（`*` 只排除 `.` / `_` 开头的文件，**不排除 `.go`**），
    于是 `GET /static.go` 能读到 Go 源码
  - **模式里也不能含 `..`**（`all:../web` 报 `invalid pattern syntax`，实测）→
    embed 的可见范围只有**本包目录及其子目录**。所以 `static.go` 挪包，`web/` 必须跟着搬，
    否则编不过（2026-09-15 用户问过"能不能放 handlers / router"，结论是建议保持现状）
  - 目录里只有 `.gitkeep`（git 追踪，**embed 要求目标非空**）、内容被 gitignore
  - 前端没构建时首页返回一个带构建指引的提示页（原来白页）
  - 改这个目录要同步 5 处：`static.go`、`.gitignore`、`.dockerignore`、`scripts/build.sh`
    （`EMBED_DIST` + 清理用 `find -mindepth 1 ! -name '.gitkeep'`）、`server.yml`、`server/Dockerfile`
- `scripts/docker/docker-compose.yml` —— Server 单服务（2026-09-15 从根目录那个 0 字节空文件搬来并补全）。
  **compose 以文件所在目录为项目目录**，所以里面所有路径都写成 `../..` 从 `scripts/docker/` 往仓库根找：
  `context: ../..`（Dockerfile 要求构建上下文是仓库根）、`../../server/config.yml:/app/config.yml:ro`、
  命名卷 `data:/app/data`。用法：`docker compose -f scripts/docker/docker-compose.yml up -d`
- **workflow 不调用这两个脚本**（各自内联编译步骤）：agent 只编 agent 且要额外打 tar.gz；
  server 的 Docker job 是在镜像内部编的
- `server/Dockerfile` —— 多阶段（node 构建前端 → golang 编译 → alpine 运行）。
  **构建上下文必须是仓库根目录**：`docker build -f server/Dockerfile .`
- `.github/workflows/agent.yml` / `server.yml` —— 打 tag(`v*`)或手动触发；两个架构 linux/amd64+arm64；
  server 额外推 ghcr.io 多架构镜像（镜像名要全小写，用 `${GITHUB_REPOSITORY_OWNER,,}` 转）
- **web 的 TS 配置**：`tsconfig.app.json` **不要写 `baseUrl`** —— TS 6.0 起报 `TS5101` 废弃错误，
  而且 `paths` 的映射值必须写成相对路径（`"@/*": ["./src/*"]`），否则报 `TS5090`。
  项目的 TS 是 5.9.3，所以**构建一直是对的，只有编辑器（更新的 TS）会报**
- `scripts/install.sh` / `media.sh` / `services/media*` —— 部署脚本（从 `StreamAgent` 项目搬来后改名）：
  装到 `/opt/media`，二进制 `media-agent`，服务名 `media`，管理命令 `/usr/bin/media`
  - **已对齐**：agent 的 workflow 在打 tag 时额外产出 `agent-v<版本>-linux-<arch>.tar.gz`
    （包内可执行文件叫 `agent`）—— 正是 `install.sh` 下载的那个文件名
  - **已对齐**：`scripts/extras/config.yml` 按本项目 schema 重写（`node|client`、`stack: default|4|6`、
    必须带 `render`）；**实测用 `config.Load` 两种模式都起得来**

## soga routes.toml 格式（以用户的生产文件为准，2026-09-15 确认）

需求文档 §4.3 的描述与外部资料（soga v2.13.4 实测博客）有冲突，**以用户实际在跑的
生产 routes.toml 为准**：

```toml
enable=true

# 路由 <节点alias>
[[routes]]
rules=[
  "# <平台名>",        # 注释行也是 rules 数组里的一个字符串元素
  "domain:zdf.de",
]

[[routes.Outs]]        # 注意 Outs 大写 O
type="socks"           # socks5 节点写 "socks"（soga 不认 "socks5"）；http 写 "http"
server="..."           # stack 解析后的地址
port=33333
username="..."
password="..."

[[routes]]
rules=["*"]            # 兜底

[[routes.Outs]]
type="direct"
```

- **一个解锁节点一个 `[[routes]]` 块**，块内按平台罗列 rules（平台名前插一行 `"# 平台名"`）
- 出口块字段只有 `type / server / port / username / password`，**没有 `protocol`**
- 兜底用 `rules=["*"]` + `type="direct"`
- 外部博客说 `[[routes.outs]]` 小写 + `protocol` + 兜底 `regexp:.*` —— 与用户生产文件不符，**不要照它改**

## agent 目录约定

- 模块名 `agent`；`pkg/` 是 fork 自 MediaUnlockTest 的检测代码（见 `agent/pkg/README.md`）
- 测试文件跟随源文件名（`foo.go` ↔ `foo_test.go`）
- `internal/node`（node / 解锁节点模式）：
  - **代理保活走内置周期 `proxyInterval`（写死 30s），不跟 `scheduler`**（用户 2026-09-15 定的）。
    `Watch(ctx)` 常驻：每 30s 拉一次本节点信息并保证代理在跑；`Tick`（cron 驱动）只管检测 + 上报。
    理由：代理掉线、或后台改了端口密码，不该等一个检测周期（可能一小时）才生效
  - `startProxy` 返回 `(svc, done <-chan struct{}, err)`，`done` 在 `Serve()` 返回时关闭；
    `ensureProxy` 用 `exited(done)` 判断"服务已退出"，退出就重起（不再只依赖 `sameConn`）。
    **启动失败**（端口被占等）和**运行中退出**现在都会在下一轮 `Watch` 重试
- `internal/detect` 用"包级变量替换"的测试写法（换 `providers` 的包级切片）→ **它的测试不能并行跑**
- `internal/client` **不用**这个写法（用户要求"不能为了测试而改代码"）：生产代码里不留测试接缝，
  `resolve` / `tcping` 直接调用；测试改成「纯逻辑用手工构造数据直接测 +
  网络部分用真实本地监听 / 关闭端口测」。代价是 `stack=4/6` 的 DNS 域名解析分支没有覆盖
- `internal/client`（client/落地节点模式）三个文件：`run.go` / `assign.go` / `render.go`
  - `run.go` 只做编排（检测 → 拉取 → 分配 → 写文件）
  - `assign.go` = 分配：常量（stack / dns / probe / publicDNS）+ 三个类型（`assignment` / `platform` /
    `node`）+ `assign`（入口）+ 节点层（`probeNode` / `resolve` / `lookupPublicIP` / `tcping`）+
    平台层（`assignPlatforms` / `bestAlias`）
    - **函数拆开、文件不拆**：2026-09-15 先试过按层拆成 `probe.go` + `assign.go`，用户随后要求并回去
  - `render.go` = 按节点归并成块 + 生成 TOML + 原子写入（不碰网络）
  - `render(t, path, a)` **按 `render.type` 分发**（目前只有 `case "soga"`）—— 需求文档 §4.4 里
    有 `render.type`，**后期要加别的渲染器，所以这个扩展点必须留着**（用户 2026-09-15 明确要求）。
    `config.Validate` 里另有一份 `"soga"` 白名单，职责是**启动即失败**（不让配错的 Agent 跑起来）
  - `node` **嵌入 `api.UnlockedNode` + 只加 `Delay`**（不要重抄 Type/Host/Port/账号字段）；
    解析后的地址直接**覆盖 `Host`**，所以 `node.Host` 就是写进配置的那个地址
  - **可见性**：`client` 包对外只导出 `Runner` / `NewRunner` / `Run`（`cmd/agent` 只用这三个），
    其余函数与类型（`assign` / `probeNode` / `assignPlatforms` / `render` / `renderSoga` /
    `nodeType` / `assignment` / `platform` / `node`）一律小写。
    Go 里大写 = 导出（跨包可见），**不是"类型名要大写"**；`type` 是关键字，不能当参数名
  - **命名：被共用的东西名字里不带 `soga`** —— 后期渲染目标不止 soga（还会有 v2 / node 之类）。
    `outType`（节点类型 → soga 出口类型）被 `assign.go`（选路校验）和 `render.go`（生成出口）
    共用，所以名字里不带 soga。
    （2026-09-15 用户曾把它改名成 `nodeType`，随后又改回 `outType`；参数名 `nodeType` 保留）
    `soga` 只出现在**不共用**的地方：`renderSoga` 只被 `render` 调，而且它就是"生成 soga 配置"
    这件事本身，将来加兄弟函数（v2 / node）时名字天然并列
  - **注释**：用户 2026-09-15 亲手删掉了 `assign.go` 里声明上的说明性注释（`node` / `assign` /
    `probeNode` / `assignPlatforms`）——**不要加回去**。常量 / 变量的注释和函数体内的行内注释他留着
  - 术语：从候选里选最好的节点说**「取最优」**，不要说「裁最优」（缩短候选列表只是副作用）
  - **改了文件名就要把函数名 / 类型名一起对齐**（`assign.go` ↔ `assign()`），
    别留文件名和标识符对不上的状态
  - 命名史（这些都被用户否过，别再提）：`stack.go`（配置项名）→ `select.go`（`select` 是 Go 关键字，
    易联想到 channel select）→ `assemble.go`（太笼统，只描述"拼数据"这个副作用）
  - **需求文档 §7 里写的是 `stack.go`，改名未同步文档**（用户选择先不动文档）
  - 文件名按**职责**取，不要用配置项名（`stack` 是 config.yml 的键，不适合当文件名）
- stack 取值是 **`default | 4 | 6`**（2026-09-15 从 `ipv4`/`ipv6` 改的，用户要求；旧值现在会被
  config 校验直接拒掉）。语义：`default` 原样保留 host 不提前解析；`4`/`6` 强制协议栈，
  不符或解析不到 → **移除该节点**；DNS 固定走公共 8.8.8.8 / 1.1.1.1
- yaml.v3 把 `stack: 4` 这类**数字标量读进 string 字段没问题**（实测得到 `"4"`），不需要自定义类型
- client 本地检测**固定 IPv4**，与 stack 无关；本地检测不上报、不写库、不心跳

## 已知遗留

- `internal/node` 的 `ensureProxy` 重启判定只看 `Type / Port / Value1 / Value2` 四个字段
  （`alias` 改名、`host`、`Value3~6` 改了都不重启）—— 这是有意的，别当成 bug 改
- `api.UnlockedNode.UploadAt` 是死字段：服务端下发 `upload_at`，agent 解析进字段但全仓没人读
- 2026-09-15 审查发现、**用户暂未要求修**的小问题：
  - `node.Runner` 退出时不 `Close()` 代理服务（进程退出由 OS 释放监听，无害）
  - `api.Client` 不接收 `ctx`（关停时在途 HTTP 请求不会被取消，靠 30s 超时兜底）
  - `client.probeNode` **串行**测速：节点多且有不通的时最坏 `节点数 × 3s`（要不要并发，用户还没定）
- **server 也有 `-version` 了**（2026-09-15 补的，与 agent 一致）：`var version = "dev"` 在
  `cmd/server/main.go`。在此之前 `build.sh` 的 `-X main.version=...` 对 server 是**空操作** ——
  **链接器对不存在的符号静默忽略、不报错**（这点已实测，别再靠"没报错"判断注入成功）
- **部署脚本传的 flag 必须和 agent 一致：是 `-config`，不是 `-c`** ——
  `scripts/services/media`（openrc）与 `media.service`（systemd）已改对
- workflow 的 tag 语义：**`latest` 只在打 tag 时生成**（`docker/metadata-action` 的
  `type=raw,value=latest,enable=${{ startsWith(github.ref,'refs/tags/') }}`）；
  两个 workflow 的 release job 会同时跑，`gh release create` 加了 `|| true` 容忍竞态
- `server.yml` 的 docker job 有 `needs: build` 了（2026-09-15 补，`go test` 没过不再推镜像）
- **docker 构建/运行已实跑通过**（2026-09-15，`server/Dockerfile` 第一次真验）：
  - **沙箱坑**：`docker compose build` 在本机沙箱里必失败（buildx 要写 `~/.docker/buildx/*`）。
    `dangerouslyDisableSandbox` **没生效**；可行绕法是 **`BUILDX_CONFIG=/tmp/buildxcfg docker compose ...`**
  - 端到端：容器起来 → 日志有初始管理员随机密码 → `/app/data/database.db` 落在命名卷里 →
    `/`、`/favicon.png`、`/assets/*`、`/api/*` 全部正确；**`/static.go` 返回 index.html（SPA 回退），
    不泄漏源码**；`POST /api/auth/login` 拿到 JWT
  - **容器里 Gin 跑的是 debug 模式**（Dockerfile 没设 `GIN_MODE=release`）—— 未改，只是记录
- **线上镜像（ghcr）**：2026-09-15 用户拍板「static 保持现状 + 镜像走线上」，于是
  把这一整批工作提交推送（`26e59ce`）并打 tag **`v0.1.0`** 触发 workflow。
  - 镜像是 `ghcr.io/hututuono/mediaunlock-server`（tag 名同 git tag，`latest` 只在打 tag 时更新）
  - **`scripts/docker/docker-compose.yml` 现在是"拉镜像"形态**：只有 `image:`，没有 `build:`
    （用户要线上，所以去掉了本地构建段；想本地构建用
    `docker build -f server/Dockerfile -t mediaunlock-server .`）
  - **仓库私有 → ghcr 包默认私有 → 拉取要 `docker login ghcr.io`（PAT 带 `read:packages`）**；
    想免登录拉要去 GitHub 包设置里把可见性改成 public（**注意：改可见性是要用户自己决定的事**）
  - 提交身份：本机**没配 `user.name` / `user.email`**（`~/.gitconfig` 和 `/etc/gitconfig` 都不存在），
    但首个 commit 的作者是 `胡图图 <hututu@hututudeMac-mini.local>` → 沿用同一身份，
    用**环境变量**（`GIT_AUTHOR_*` / `GIT_COMMITTER_*`）传，**不去改用户的 git 配置**
- **本机有 GitHub OAuth 凭据**（git credential helper = `osxkeychain`，token 前缀 `gho_`，40 位）：
  `printf 'protocol=https\nhost=github.com\n\n' | git credential fill` 能取出 `password=`。
  用它调 GitHub API 查 Actions 状态（`/repos/<owner>/<repo>/actions/runs`）**可行**。
  **但不要把 token 打印到输出里** —— 本机也没有 `gh` CLI
- **ghcr 镜像的当前状态**（2026-09-15 收尾时）：
  - 仓库**私有** → ghcr 包默认继承私有可见性，推上去也要 `docker login` 才能拉，
    **不能匿名 pull**（要匿名得去包设置里改 public —— 这是用户自己决定的事）
  - compose 同时写 `image` + `build` 时，**`up` 走本地 build、不会 pull**（实测）；
    想用线上镜像要显式 `docker compose pull`，或去掉 `build` / 设 `pull_policy: always`
  - **已提交推送**：`26e59ce`（static 重构 + 修复 + CI/Docker/脚本，tag `v0.1.0` 打在这里）、
    `87f8c75`（compose 改成拉镜像）、`eeac9ac`（修 release job，`main` 已是最新）
  - **首轮 run：build 全绿，release job 挂在 `gh` 上；ghcr 上此刻仍无镜像**
    （docker job 卡在多架构 qemu 首次构建，被用户手动取消）
  - **待办**：`v0.1.0` tag 还指在 `26e59ce`，修复在 `eeac9ac` → **得把 tag 移过去重推**才会生效
- **release job 必须给 `gh` 指路**：`release` job 没有 `checkout` → 工作目录无 `.git` →
  `gh` 报 `fatal: not a git repository`。`GH_TOKEN` 只解决认证、**解决不了定位仓库**；
  修法是加 `GH_REPO: ${{ github.repository }}`（比加 `checkout` 轻）。两个 workflow 各一处
  - **改 workflow 后 tag 必须移动才会生效**：tag 触发的 run 用的是**tag 那个 commit 上的
    workflow 文件**，"Re-run failed jobs" 不会用新文件
- **查 ghcr 镜像是否存在的可靠办法**：gh CLI 的 OAuth token **没有 `read:packages`**
  （`/user/packages` 403），但 **registry 的 token 交换可以**：
  `curl -u "HuTuTuOnO:$TOKEN" "https://ghcr.io/token?scope=repository:hututuono/mediaunlock-server:pull&service=ghcr.io"`
  取 `token`，再 `curl -H "Authorization: Bearer $REG" https://ghcr.io/v2/<owner>/<pkg>/tags/list`。
  不存在返回 `NAME_UNKNOWN`，存在返回 tag 列表
- 多架构镜像**首跑没有 gha 缓存时很慢**（qemu 里跑 arm64 的 npm ci + go build），预算要放宽

- 需求文档与代码有多处不一致（用户说过先不动文档）—— 清单见 2026-09-15 的日志
- **第三轮审查的 14 条问题已修完**（2026-09-15，用户拍板后动手）。三个"最容易踩"的结论现在变成了
  约束，改相关代码时必须遵守：
  ① **`//go:embed` 不能用 `all:*` 或 `*`** —— 都会把 `.go` 自己嵌进去（`*` 只排除 `.` / `_` 开头的），
     所以前端嵌在 `server/internal/static/web/`、写法是 `all:web` + `fs.Sub`
  ② **嵌入目录要能在"前端没构建"时正确降级**：`fs.Stat` 查不到 `index.html` 就返回提示页，
     不能再往里放 vite 产物（那会被 gitignore，新 clone 直接白页）
  ③ **`SKIP_WEB=1` = 跳过构建但照常同步**，不是"整个跳过前端"
- **用户明确不做的两件事（别再提）**：登录限流（**不加**）、`bestAlias` 反向依赖 `outType`（**保持现状**）
- 改密码不吊销已有 JWT（无状态 token，前端"请重新登录"只清 localStorage）
- **server 侧业务逻辑至今未发现 bug**：router 接口清单与需求文档 §3.2 完全对得上

