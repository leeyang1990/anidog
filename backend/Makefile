.PHONY: build run clean tidy lint

# 构建
build:
	go build -o bin/anidog ./cmd/anidog

# 运行
run:
	go run ./cmd/anidog

# 清理
clean:
	rm -rf bin/

# 整理依赖
tidy:
	go mod tidy

# 代码检查
lint:
	golangci-lint run ./...

# 开发模式 (热重载需要 air)
dev:
	air || go run ./cmd/anidog

# Docker 构建
docker-build:
	docker build -t anidog-backend:latest .

# Docker 启动
docker-up:
	docker-compose -f docker-compose.yml up -d

# Docker 停止
docker-down:
	docker-compose down

# Docker 日志
docker-logs:
	docker-compose logs -f backend

# Docker 重启
docker-restart:
	docker-compose restart backend
