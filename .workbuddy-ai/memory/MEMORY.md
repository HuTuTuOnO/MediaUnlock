# MediaUnlock 项目长期笔记

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

- `internal/detect/detect_test.go` 调 `items()`，`detect.go` 里叫 `allItems()` → 该测试编译不过，
  导致 `go test ./...` FAIL（`go build ./...` 正常）。2026-09-15 发现时未修。
- `internal/node` 的 `ensureProxy` 重启判定只看 `Type / Port / Value1 / Value2` 四个字段
  （`alias` 改名、`host`、`Value3~6` 改了都不重启）—— 这是有意的，别当成 bug 改
- **ctx 被取消时会产出"垃圾结果"**（2026-09-15 审查发现，未修）：`detect.All(ctx)` 在 ctx 已取消时
  给**每一项**都返回 `StatusNetworkErr`，于是
  ① `node.Tick` 拿"全失败"的结果上报 → 服务端 `handlers/agent.go:108` 整体替换 `node_platforms`
     → **该节点的解锁关联被清空**（外加 163 条 "context canceled" 历史噪音）
  ② `client.Run` 的"本机已解锁"集合为空 → **写出一份把所有平台都分流出去的配置**
  触发条件：SIGTERM 时正好有一轮在跑。修法：检测跑完若 `ctx.Err() != nil` 就放弃本轮（不上报 / 不写文件）
- `agent/go.mod` 里 `golang.org/x/net` 被列为**直接**依赖，但全仓没人 import 它
  （`go mod tidy` 会把它挪到 indirect 块）
- `api.UnlockedNode.UploadAt` 是死字段：服务端下发 `upload_at`，agent 解析进字段但全仓没人读
