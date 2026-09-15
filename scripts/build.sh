#!/usr/bin/env bash
# 构建:前端 → Server(内嵌前端) + Agent。不传 PLATFORMS 就编本机平台。
#
#   ./scripts/build.sh                                      # 本机平台
#   PLATFORMS="linux/amd64 linux/arm64" ./scripts/build.sh   # 交叉编译(发布用)
#   SKIP_WEB=1 ./scripts/build.sh                           # 复用已有 web/dist
#   SKIP_AGENT=1 ./scripts/build.sh                         # 只编 Server
#
# 产物在 dist/:server_<os>_<arch>、agent_<os>_<arch>
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
EMBED_DIST="$ROOT/server/internal/static/web"
DIST="$ROOT/dist"

# 版本号注入 agent 的 -version;有 git 就取 tag / 短 SHA,否则 dev
VERSION="$(git -C "$ROOT" describe --tags --always --dirty 2>/dev/null || echo dev)"
LDFLAGS="-s -w -X main.version=$VERSION"

if [[ -z "${PLATFORMS:-}" ]]; then
  PLATFORMS="$(go env GOOS)/$(go env GOARCH)"
fi

# 1) 前端:Server 要把它嵌进去。
#    SKIP_WEB=1 只是"别构建,dist 我自己准备好了",同步这一步不能省 ——
#    否则编出来的二进制里嵌的还是上一次的静态资源。
if [[ "${SKIP_WEB:-}" != "1" && ! -f "$ROOT/web/dist/index.html" ]]; then
  echo "==> 构建前端 (web)"
  [[ -d "$ROOT/web/node_modules" ]] || (cd "$ROOT/web" && npm ci)
  (cd "$ROOT/web" && npm run build)
fi

if [[ ! -f "$ROOT/web/dist/index.html" ]]; then
  echo "错误:web/dist 不存在;先跑一次前端构建,或去掉 SKIP_WEB" >&2
  exit 1
fi

# 清掉上次同步进来的;留下 .gitkeep,它被 git 追踪,删了工作区会变脏
echo "==> 同步 web/dist 到 $EMBED_DIST"
find "$EMBED_DIST" -mindepth 1 ! -name '.gitkeep' -exec rm -rf {} +
cp -R "$ROOT/web/dist/." "$EMBED_DIST/"

# 2) 逐平台编译
mkdir -p "$DIST"
for p in $PLATFORMS; do
  os="${p%%/*}"
  arch="${p##*/}"
  echo "==> $os/$arch"
  (cd "$ROOT/server" && GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 \
    go build -trimpath -ldflags="$LDFLAGS" -o "$DIST/server_${os}_${arch}" ./cmd/server)
  if [[ "${SKIP_AGENT:-}" != "1" ]]; then
    (cd "$ROOT/agent" && GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 \
      go build -trimpath -ldflags="$LDFLAGS" -o "$DIST/agent_${os}_${arch}" ./cmd/agent)
  fi
done

echo "==> 完成:"
ls -lh "$DIST"
