# Server 运行镜像:多阶段构建 —— 先构建前端,再编译内嵌前端的 Server。
#
# 构建上下文必须是**仓库根目录**(要同时拿到 web/ 与 server/):
#
#   docker build -f server/Dockerfile -t mediaunlock-server .
#
# 运行(挂一份 config.yml;数据库默认写 ./data,建议挂卷):
#
#   docker run -d --name mediaunlock -p 8080:8080 \
#     -v "$PWD/server/config.yml:/app/config.yml:ro" \
#     -v mediaunlock-data:/app/data \
#     mediaunlock-server
#
# config.yml 里 database.path 用默认的 ./data/database.db 就能对上挂载点。

# 前两个阶段都在 $BUILDPLATFORM 上原生跑,不进 qemu:它们只产出平台无关的东西
# (前端资源、静态链接的二进制)。目标架构靠 GOARCH 交叉编译拿到 ——
# 在 qemu 里模拟 arm64 跑 go build 会慢几十倍。
FROM --platform=$BUILDPLATFORM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
WORKDIR /src
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
# 前端产物填进嵌入目录(仓库里这里只有 .gitkeep)
COPY --from=web /src/web/dist/ ./internal/static/web/
# TARGETOS/TARGETARCH 由 buildx 注入;CGO 关着,不需要目标平台的工具链
ARG TARGETOS TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
      go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server \
 && echo "built for ${TARGETOS}/${TARGETARCH}"

FROM alpine:3
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
# 否则 Gin 走 debug 模式,启动时把全部路由和 handler 包路径打进日志(本地 go run 不受影响)
ENV GIN_MODE=release
COPY --from=build /out/server ./server
COPY server/config.example.yml ./config.example.yml
EXPOSE 8080
VOLUME ["/app/data"]
ENTRYPOINT ["./server", "-config", "config.yml"]
