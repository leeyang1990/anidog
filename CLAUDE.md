# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

使用中文回答

## 项目概述

**anidog**（御宅追番）是一个番剧自动下载管理系统。技术栈：

- **后端**：Go + Gin + GORM + PostgreSQL
- **前端**：Vue 3 + Naive UI + Tailwind CSS + Vite
- **下载器**：qBittorrent（Docker 容器）
- **流媒体抓取**：go-rod 浏览器自动化 + ffmpeg

核心能力：
- **多源下载编排**：Orchestrator 定时扫描订阅番剧，按用户偏好从 BT Indexer / RSS feed / 流媒体中挑最优源自动下载
- **AutoBangumi 式 RSS 自动发现**：订阅 Mikan MyBangumi 等聚合 feed，自动解析标题 → 调 Bangumi API 补全元数据 → 创建追番条目 → 下载
- **智能标题解析**：自写 titleparse 模块提取字幕组/番名/集数/分辨率/语言
- **Plex 式目录组织**：下载文件按 `<番剧名 (年份)>/Season NN` 归档

## 开发

使用 Docker Compose 统一开发环境（PostgreSQL + qBittorrent + Backend + Frontend）：

```bash
# 启动全部服务（air 热重载 + Vite HMR）
docker compose -f docker-compose.dev.yml up -d

# 查看日志
docker compose -f docker-compose.dev.yml logs -f backend

# 访问
open http://localhost:3002         # 前端
open http://localhost:8080         # qBittorrent WebUI (admin/adminadmin)
```

本地调试 Go 代码或运行单测：

```bash
cd backend
go test ./...
go build ./...
```

前端本地开发（如果你不想用容器里的 frontend 服务）：

```bash
cd frontend
npm install
npm run dev   # Vite 默认端口 3033，代理到 http://localhost:8088
npm run build
```

## 架构

### 后端（`backend/`）

```
cmd/anidog/         main.go — 依赖注入 + 注册路由 + 启动调度器
internal/
  config/           配置加载（viper，支持 env + yaml）
  database/         GORM 初始化 + AutoMigrate
  handler/          HTTP handler（REST + WebSocket）
  middleware/       JWT 认证
  model/            数据模型（Anime / Download / RSSFeed / StreamRule ...）
  service/
    anime/          番剧 CRUD
    auth/           登录 + token
    bangumi.go      Bangumi API 客户端（搜索 / 详情 / 日历）
    bangumi/        自动下载 + 源健康检测
    download/       下载编排（Task / Executor / QBit provider 同步）
    indexer/        BT 聚合搜索（Mikan / Dmhy / BangumiMoe / Nyaa）
    orchestrator/   剧集驱动的多源调度器（核心）
    rss/            RSS 解析 + 自动发现 + 匹配
    scheduler/      定时任务调度器
    setting/        全局设置（Key-Value）
    stream/         流媒体规则执行 + ffmpeg 下载
    streamrule/     流媒体规则 CRUD
    titleparse/     种子标题解析
  downloader/       下载 provider 抽象（qBit / mock）
  ws/               WebSocket Hub（下载进度推送）
```

### 前端（`frontend/`）

```
src/
  views/            页面
    Anime/          番剧列表 + 详情 + 库浏览
    Downloads/      下载管理
    RSS/            RSS feed 管理
    Settings/       系统设置（含下载偏好）
    StreamRules/    流媒体规则管理
  components/
    Anime/
      EpisodeGrid.vue            剧集进度网格（源无关）
      EpisodeDetailDrawer.vue    单集详情/诊断
      ManualSearchDialog.vue     手动 BT 选种
      StreamSetupCard.vue        流媒体源选择（高级模式）
  stores/auth.js    Pinia 认证状态
  utils/api.js      fetch 封装（token + 自动刷新 + 跳登录）
  router/           Vue Router
```

### 数据流核心

```
追番添加（三种入口）：
  Bangumi 详情页 "追番"   → source_origin="bangumi"
  BT 搜索结果 "追这部番" → source_origin="bt_search"
  RSS 自动发现          → source_origin="rss_auto"
                              ↓
                      anime 表（is_subscribed=true）
                              ↓
Orchestrator 定时扫描（30 min）：
  对缺失集按 priority 跑 BT/Stream/RSS
  rss_auto 只走 RSS（避免与 RSS job 冲突）
                              ↓
                     download 表（source=bt/rss/stream）
                              ↓
                        qBittorrent / ffmpeg
                              ↓
                  /downloads/<番剧名 (年份)>/Season NN/
```

## 关键文件

- `backend/dev.yaml` — 开发配置（含开发默认密码，**生产请用 env 覆盖**）
- `docker-compose.dev.yml` — 开发环境编排（源码挂载 + 热重载）
- `docker-compose.yml` — 生产环境编排（镜像构建）
- `backend/internal/service/orchestrator/orchestrator.go` — 调度核心
- `backend/internal/service/rss/engine.go` — RSS 解析 + 自动发现
- `backend/internal/service/indexer/` — BT 聚合搜索
- `frontend/src/components/Anime/EpisodeGrid.vue` — 剧集网格
- `frontend/src/views/Settings/DownloadPrefs.vue` — 下载偏好

## 常用操作

### 默认测试账号

首次启动没有用户，注册后可以登录。开发时示例账号：
- username: `admin`
- password: `admin123`
- email: `admin@admin.com`

### 数据库直连

```bash
docker exec -it anidog-postgres-dev psql -U anidog -d anidog_dev
```

### 清空所有数据重来

```bash
docker exec anidog-postgres-dev psql -U anidog -d anidog_dev -c "
  DELETE FROM download; DELETE FROM orchestrator_diagnosis;
  DELETE FROM rssentry; DELETE FROM anime;
"
```

### 触发 RSS / Orchestrator

```bash
TOKEN=$(curl -s -X POST http://localhost:3002/api/v1/auth/login \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d 'username=admin&password=admin123' | jq -r .access_token)
curl -X POST http://localhost:3002/api/v1/rss/1/refresh -H "Authorization: Bearer $TOKEN"
curl -X POST http://localhost:3002/api/v1/orchestrator/run-all -H "Authorization: Bearer $TOKEN"
```

## 注意事项

1. **生产部署必改**：`SECRET_KEY`、`POSTGRES_PASSWORD`、`DOWNLOADER_PASSWORD` 都通过 docker-compose 的 env 传入，务必覆盖默认值
2. **下载目录共享**：`./downloads` bind mount 同时挂到 backend 和 qBit 容器，确保一致
3. **qBit 首次登录**：使用临时密码（容器日志里），需要通过 Web UI 或 API 改成 compose 里设置的 `adminadmin`，否则 backend 登录失败
