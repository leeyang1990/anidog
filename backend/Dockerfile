# 多阶段构建优化镜像大小

# ========================================
# 阶段一：构建器
# ========================================
FROM golang:1.26.2-alpine AS builder

WORKDIR /build

# 安装必要的工具
RUN apk add --no-cache git ca-certificates

# 复制依赖文件
COPY go.mod go.sum .

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 版本号由 CI 通过 --build-arg VERSION=<git tag> 注入，缺省 dev
ARG VERSION=dev

# 构建应用（禁用 CGO 以减小镜像大小；ldflags 注入版本号 + 裁剪符号表）
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags "-s -w -X 'github.com/anidog/anidog-go/internal/config.Version=${VERSION}'" \
    -o anidog ./cmd/anidog

# ========================================
# 阶段二：稳定运行时镜像
# ========================================
# Chromium + ffmpeg + 字体约 914MB，固定在单独的基础镜像中。这样普通发版
# 不会每次生成并重新下载一个 400MB+ 的依赖层，只需拉取新的 Go 二进制层。
FROM leeyang1990/anidog-backend-runtime:alpine3.24-v1

# 从构建阶段复制二进制文件
COPY --from=builder /build/anidog .

# 暴露端口
EXPOSE 8088

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=40s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8088/healthcheck || exit 1

# 运行应用
CMD ["./anidog"]
