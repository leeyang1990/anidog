# AniDog (anidog)

一个基于 Go + Vue 3 的番剧自动下载管理系统。通过 RSS 订阅、BT Indexer 搜索、流媒体抓取三种手段自动为追番列表填坑。

## 功能特点

- **三路并行下载**：Orchestrator 统一调度 RSS / BT Indexer / 流媒体三种源，按剧集粒度自动选最优源
- **智能标题解析**：中文优化的正则解析器，识别字幕组 / 集数 / 分辨率 / 语言 / 批量包
- **Plex 友好目录**：下载统一落到 `<名称 (年份)>/Season NN/` 目录结构
- **剧集网格**：源无关的剧集进度 UI，badge 标注每集实际来源（BT / Str / RSS）
- **诊断面板**：未命中时展示各源检查结果，辅助用户判断是偏好太严还是源无资源

## 技术栈

### 后端
- Go + Gin + GORM
- PostgreSQL
- qBittorrent（Docker LinuxServer 镜像）
- go-rod（流媒体浏览器自动化）

### 前端
- Vue 3 + Pinia + Vue Router
- Naive UI + Tailwind CSS
- Vite

## 快速启动

### Docker Compose（推荐）

```bash
# 开发环境（挂载源码 + 热重载）
docker compose -f docker-compose.dev.yml up -d

# 生产环境
docker compose up -d

# 查看日志
docker compose logs -f backend

# 停止
docker compose down
```

访问：
- 前端：http://localhost:3002
- 后端：http://localhost:8088/api/v1
- qBittorrent WebUI：容器内 8080，开发环境不暴露

默认测试账户：`admin / admin123`（首次启动需自行注册）

### 本地开发（非 Docker）

后端：
```bash
cd backend
go mod download
CONFIG_NAME=dev go run ./cmd/anidog
```

前端：
```bash
cd frontend
npm install
npm run dev
```

## 项目结构

```
anidog/
├── backend/                  # Go 后端
│   ├── cmd/anidog/           # 主入口
│   ├── internal/
│   │   ├── handler/          # HTTP 路由
│   │   ├── service/          # 业务逻辑
│   │   │   ├── orchestrator/ # 多源编排调度
│   │   │   ├── indexer/      # BT Indexer (Mikan/Dmhy/BangumiMoe/Nyaa)
│   │   │   ├── titleparse/   # 标题解析器
│   │   │   ├── rss/          # RSS 引擎
│   │   │   ├── stream/       # 流媒体抓取
│   │   │   ├── bangumi/      # Bangumi API 集成
│   │   │   └── download/     # 下载任务管理
│   │   ├── downloader/       # qBittorrent provider
│   │   └── model/            # GORM 数据模型
│   └── dev.yaml              # 开发配置
├── frontend/                 # Vue 前端
│   └── src/
│       ├── components/
│       ├── views/
│       ├── stores/
│       └── utils/
├── docker-compose.yml        # 生产
└── docker-compose.dev.yml    # 开发
```

## 生产部署注意事项

`backend/dev.yaml` 中的密码、JWT 密钥都是开发默认值，生产部署必须通过环境变量覆盖：

```yaml
# docker-compose.yml 中 backend 服务的 environment
SECRET_KEY: "<32字节随机字符串>"
DOWNLOADER_PASSWORD: "<qBittorrent WebUI 密码>"
DATABASE_URL: "postgres://anidog:<强密码>@postgres:5432/anidog?sslmode=disable"
```

同样地，生产环境应把 `docker-compose.yml` 中 PostgreSQL 和 qBittorrent 的默认密码改掉。

## 开发

详细架构、数据流、常用运维命令见 [CLAUDE.md](CLAUDE.md)。
