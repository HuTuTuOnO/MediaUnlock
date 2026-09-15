#!/usr/bin/env bash
# 单文件构建:前端 → 同步进 Server 嵌入目录 → 编译 Server 二进制。
#
#   ./scripts/build.sh            # 构建 server(内含前端)
#   SKIP_WEB=1 ./scripts/build.sh # 跳过前端构建,复用已有 static
#
# 产物:server/server(嵌入 internal/static,直接托管前端)。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
EMBED_DIST="$ROOT/server/internal/static"

# 1) 构建前端
if [[ "${SKIP_WEB:-}" != "1" ]]; then
  echo "==> 构建前端 (web)"
  (cd "$ROOT/web" && npm run build)
fi

if [[ ! -f "$ROOT/web/dist/index.html" ]]; then
  echo "!! web/dist 不存在,请先在 web/ 下 npm install && npm run build" >&2
  exit 1
fi

# 2) 同步到 Server 嵌入目录(只清空旧静态资源,保留 static 包的 Go 入口)
echo "==> 同步 web/dist 到 $EMBED_DIST"
mkdir -p "$EMBED_DIST"
find "$EMBED_DIST" -mindepth 1 ! -name '*.go' -exec rm -rf {} +
cp -R "$ROOT/web/dist/." "$EMBED_DIST/"

# 3) 编译 Server(此时 //go:embed 会打包最新前端)
echo "==> 编译 Server (server)"
(cd "$ROOT/server" && go build -o server ./cmd/server)

echo "==> 完成:$ROOT/server/server"
