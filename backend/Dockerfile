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
# 阶段二：运行时镜像
# ========================================
FROM alpine:latest

# 安装运行时依赖：ffmpeg + chromium (rod 浏览器) + 字体
RUN apk --no-cache add \
    ca-certificates \
    ffmpeg \
    chromium \
    chromium-swiftshader \
    nss \
    freetype \
    freetype-dev \
    harfbuzz \
    ttf-freefont \
    font-noto-cjk \
    wqy-zenhei

# 告诉 rod 使用系统 chromium，不去下载
ENV ROD_BROWSER_BIN=/usr/bin/chromium-browser

WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /build/anidog .

# 创建下载目录
RUN mkdir -p /downloads

# 暴露端口
EXPOSE 8088

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=40s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8088/healthcheck || exit 1

# 运行应用
CMD ["./anidog"]
