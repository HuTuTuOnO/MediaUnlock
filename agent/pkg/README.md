# agent/pkg —— fork 自 MediaUnlockTest

本目录是 [MediaUnlockTest](https://github.com/HsukqiLee/MediaUnlockTest) 的 `pkg/` fork,
import 前缀改为本项目 module(`agent/pkg`),作为自有代码维护(含修复其失效的解锁项)。
详见 需求文档.md 4.5 MediaUnlockTest 接入方式。

## fork 步骤(在你自己的终端跑,需联网)

```bash
cd /Users/hututu/Desktop/MediaUnlock/agent

# 1) 取上游 pkg/ 到临时目录(只要 pkg,core+providers 就是全部检测逻辑)
TMP=$(mktemp -d)
git clone --depth 1 https://github.com/HsukqiLee/MediaUnlockTest "$TMP"

# 2) 拷 core / providers 覆盖到本目录(保留本 README 与 .gitkeep)
rm -f pkg/core/.gitkeep pkg/providers/.gitkeep
cp -R "$TMP"/pkg/core/. pkg/core/
cp -R "$TMP"/pkg/providers/. pkg/providers/
# 这一步上游没有对应文件:实测 HsukqiLee/MediaUnlockTest 根目录没有 LICENSE
# (LICENSE / LICENSE.md / LICENSE.txt / COPYING 全部 404),README 的 License 段只挂了
# 一个 FOSSA 徽章。fork 时不要 `cp "$TMP"/LICENSE`,许可证归属待定,见文末说明。

# 3) 探测上游 module 名,把 import 前缀 <上游module>/pkg → agent/pkg
UP=$(head -1 "$TMP"/go.mod | awk '{print $2}')
grep -rl "$UP/" pkg/ | xargs sed -i '' "s#$UP/#agent/#g"

# 4) 合并依赖:把上游 go.mod 的 require 并进来,再 tidy
#    建议先把上游 go.sum 里用到的版本抄进 agent/go.sum 避免拉到不兼容新版
go mod tidy

rm -rf "$TMP"
go build ./...   # 应能编过
```

> 上游 module 名若确实就是 `MediaUnlockTest`,第 3 步等价于 `s#agent/#agent/#g`。
> 用 `$UP` 自动探测更稳,无论上游 module 是 `MediaUnlockTest` 还是 `github.com/HsukqiLee/MediaUnlockTest`。

## 接真检测:已落地(契约备查)

检测已在 `agent/internal/detect` 接好,**没有 `Stub`/`RealDetector`/`Detector` 接口** ——
只有一个 `All(ctx) []Result` 函数,`cmd/agent` 不做检测器注入。
下面是接入时用到的契约,改动 `detect` 时对照:

- **数据类型**:`core.Result{ Status int; Region string; Info string; Err error }`
  → 归一到 `detect.Result{ Name, Status, Region, Info, Err(string) }`(err 取 `Err.Error()`)。
- **检测项来源**:`providers` 以导出变量暴露各地区切片(12 个):
  `GlobeTests / TaiwanTests / HongKongTests / JapanTests / KoreaTests / NorthAmericaTests /
   SouthAmericaTests / EuropeTests / AfricaTests / SouthEastAsiaTests / OceaniaTests / AITests`。
  元素为 `TestItem{ Name string; Func func(client) core.Result; SupportsV6 bool }`。
- **地区组装**:上游把"跑哪些地区"的逻辑放在未导出的 `cli/main`,我们在 `detect` 里自行组装:
  合并全部地区切片,按 `item.Name` 去重(保留首次出现),跳过 `Func == nil` 的地区占位项
  (如 `GB`/`IN`)。201 条有效条目 → 177 个唯一名 → **163 项可检测**。
- **client 构造**:**每项各建一个 client**。`pkg/providers` 里有 6 个 provider 会改写 client 自身状态
  (`Bing`/`BahamutAnime`/`Mora`/`DirecTVStream` 调 `SetCookieJar`,`PrimeVideo`/`J_COM_ON_DEMAND`
  调 `SetFollowRedirect(true)`),共享给并发 worker 会数据竞争,还会让 cookie/redirect 跨平台串味。
  上游 `pkg/` 内部零并发、是顺序跑的,所以上游可以共享,我们并发就**必须**每项各建。
  固定 IPv4(`core.NewHttpClient(4)`)。
- **平台名匹配**:`item.Name` 即 server 端 `platforms.name` 的匹配键(report 用它对齐)。
- **`recover()` 已加**:`detect.All` 的每个 worker 都兜住 panic,转成 `StatusFailed` + `Err`,
  否则 163 项里任何一个 provider panic 都会带走整个 Agent 进程(Agent 是常驻的,
  和上游一次性 CLI 不同)。上游 `cli/execute.go` 也是每个测试函数套一层 `recover()`。
  注意 `recover` 的 `defer` 必须注册在 `defer wg.Done()` **之后**(LIFO 才能保证先写回结果再 Done)。

## 说明

- MediaUnlockTest 要求 **Go 1.26.4**,本项目 `agent/go.mod` 已对齐。
- 依赖较重(`tls-client` 模拟 TLS 指纹等 40+ 间接依赖),会增大 Agent 体积——可接受。
- fork 后与上游脱钩:上游新增/修复的平台不会自动同步,需要时手动比对合并。

### 许可证归属(未决)

本项目 `pkg/` 下**没有 LICENSE 文件**,且这不是漏拷 —— 上游 `HsukqiLee/MediaUnlockTest`
本身就没有许可证文件,README 的 License 段只有一个 FOSSA 徽章。再往上的链条:

| 仓库 | 许可 |
|---|---|
| `lmc999/RegionRestrictionCheck` | **AGPL-3.0** |
| `nkeonkeo/MediaUnlockTest` | 无 LICENSE 文件 |
| `HsukqiLee/MediaUnlockTest`(直接上游) | 无 LICENSE 文件 |

AGPL-3.0 带网络使用条款:若以网络服务形式运行衍生版本,需向服务使用者提供源码。
本项目是 panel + agent 的服务形态,因此是否继承该义务需要判断。
上游 README 自称是按 lmc 脚本的**思路**用 Golang 重写,但中间两层都去掉了许可证,属灰色地带。
**未擅自补 LICENSE 文件** —— 从哪一层继承、继承什么,是使用者的决定。
