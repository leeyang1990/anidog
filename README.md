<div align="center">

<img src="docs/assets/banner.svg" alt="AniDog" width="600"/>

# AniDog · 御宅自动追番

**一只替你蹲守新番的边牧** —— 订阅一次，全季自动到碗里。

多源编排 · 智能选种 · Plex 式归档 · 动森风 UI

[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Vue](https://img.shields.io/badge/Vue-3-4FC08D?logo=vuedotjs&logoColor=white)](https://vuejs.org)
[![Docker](https://img.shields.io/badge/Docker-multi--arch-2496ED?logo=docker&logoColor=white)](https://hub.docker.com)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-4169E1?logo=postgresql&logoColor=white)](https://www.postgresql.org)
[![License](https://img.shields.io/badge/license-MIT-5D4037)](#-许可证)

</div>

---

## ✨ 这是什么

AniDog 把「追番」这件事彻底自动化：你只需要在 Bangumi 里点一下「追这部」，剩下的交给它——

它会**每隔一段时间扫一遍你的追番列表**，对每一集缺失的剧集，按你设定的偏好依次尝试 **BT 种子搜索**、**流媒体抓取** 两条主动通道，外加 **RSS 规则订阅** 的被动通道，抢到资源就丢进 qBittorrent / ffmpeg 下载，并按 Plex 规范归档到 `番剧名 (年份)/Season NN/`。

下不到？它会告诉你为什么（种子全死、规则太严、还没播）。下失败了？它会**按退避节奏自动重试**，直到成功或彻底放弃。下完了？**推一条 Telegram / Bark 通知**给你，且保证每集只推一次。

> 就像养了一只边牧：你不用管它怎么找，它自己会把新番叼回来放你脚边。 🐕

---

## 🎬 核心能力

| 能力 | 说明 |
|------|------|
| 🧩 **多源编排** | Orchestrator 以「每集」为粒度调度，BT / Stream 按优先级填坑，先到先得；RSS 作为独立被动通道并行 |
| 🎯 **智能选种** | 自研 titleparse 解析字幕组 / 集数 / 分辨率 / 语言 / 合集，按质量偏好打分排序选最优 |
| 📁 **Plex 归档** | 所有源统一落盘到 `<番剧名 (年份)>/Season NN/`，Emby / Jellyfin 直接刮削 |
| 🔁 **失败自愈** | 区分「临时失败（源过期/死种）」与「永久失败」，前者按 10min→1h→6h 退避自动重试；死种黑名单 14 天 TTL 自动复活 |
| 💀 **死种狙击** | qBit 同步时识别 metaDL 卡死 / stalledDL 0 做种，主动放弃并换下一候选，不再卡在无效种子上 |
| 🔔 **精准通知** | 完成即推 Telegram / Bark / Webhook，按 `(番剧, 集数)` 幂等去重，绝不重复轰炸 |
| 🩺 **诊断面板** | 未命中时展示各源检查明细（候选数 / 被淘汰数 / 原因），一眼看出是偏好太严还是真没资源 |
| 🎨 **双主题皮肤** | 动森风（圆润饱满）与 Classic（清爽商务）一键切换，纯 CSS 变量、零刷新 |
| 📊 **实时系统面板** | CPU / 内存 / 磁盘 / Goroutines / DB 连接池 / qBit 在线状态，5 秒刷新 |

---

## 🖼️ 界面预览

<div align="center">
<img src="docs/assets/screenshot-login.png" alt="登录页" width="720"/>
</div>

> 剧集进度网格 · 单集诊断抽屉 · 下载管理 · 系统设置 —— 全部动森风自绘组件库，无第三方 UI 框架。

---

## 🏗️ 技术栈

<table>
<tr>
<td valign="top" width="50%">

**后端**
- Go 1.26 + Gin + GORM
- PostgreSQL 16
- qBittorrent（LinuxServer 镜像）
- go-rod（流媒体浏览器自动化）
- ffmpeg（m3u8 流下载）

</td>
<td valign="top" width="50%">

**前端**
- Vue 3 + Composition API + Pinia
- 自研 `components/ac` 组件库（24 组件）
- Tailwind CSS（HSL token 皮肤系统）
- Vite

</td>
</tr>
</table>

---

## 🚀 快速开始

### 方式一：生产部署（拉镜像，推荐）

> 镜像由 GitHub Actions 在打 tag 时自动构建，支持 **amd64 / arm64** 多架构。
> 生产 `docker-compose.yml` 的全部服务（前端 / 后端 / PostgreSQL / qBittorrent）都走镜像，无需本地构建。

```bash
# 1. 克隆仓库（或从 Release 下载 docker-compose.yml + .env.example）
git clone https://github.com/leeyang1990/anidog.git && cd anidog

# 2. 配置密码密钥
cp .env.example .env
$EDITOR .env        # 填 SECRET_KEY / POSTGRES_PASSWORD / DOWNLOADER_PASSWORD

# 3. 启动（默认读 docker-compose.yml，全部拉镜像）
docker compose up -d

# 4. 打开
open http://localhost:3002
```

### 方式二：源码开发（热重载）

```bash
# 一键起全栈：PostgreSQL + qBittorrent + air 热重载后端 + Vite HMR 前端
docker compose -f docker-compose.dev.yml up -d
docker compose -f docker-compose.dev.yml logs -f backend

open http://localhost:3002        # 前端
open http://localhost:8080        # qBittorrent WebUI (admin/adminadmin)
```

不想用容器跑 Go / 前端：

```bash
# 后端
cd backend && go mod download && CONFIG_NAME=dev go run ./cmd/anidog

# 前端
cd frontend && npm install && npm run dev
```

> 首次启动没有用户，注册后登录即可。

---

## 🔄 工作流

```
      ┌──────────────┐  追番（Bangumi 详情 / BT 搜索结果）
      │   anime 表    │◀──────────────────────────────────────┐
      └──────┬───────┘                                        │
             │ Orchestrator 每 30min 扫描缺失集                 │
             ▼                                                 │
   ┌─────────────────────┐   按优先级 & 质量偏好选最优源          │
   │  tryDownloadEpisode  │─── BT 种子搜索 ──┐                  │
   └─────────────────────┘─── 流媒体抓取 ────┤                  │
             ▲                                ▼                 │
   RetryFailedJob 退避重试            ┌───────────────┐          │
   （transient 失败 10m/1h/6h）       │  download 表   │          │
             │                       └───────┬───────┘          │
             │                               ▼                  │
             │              qBittorrent（种子）/ ffmpeg（流）     │
             │                               ▼                  │
             └──── 失败分类 ◀── /downloads/<番剧名 (年份)>/Season NN/
                                             ▼
                                完成 → 幂等去重 → Telegram/Bark 通知
```

RSS 是**独立被动通道**：`RSSRefreshJob` 定时刷新已订阅 feed，命中规则即下载，与主动编排并行、互不干扰。

---

## 📂 项目结构

```
anidog/
├── backend/                      # Go 后端
│   ├── cmd/anidog/               # main：依赖注入 + 路由 + 调度器
│   └── internal/
│       ├── handler/              # HTTP / WebSocket 路由
│       ├── service/
│       │   ├── orchestrator/     # ★ 多源剧集编排调度（核心）
│       │   ├── indexer/          # BT 聚合搜索 (Mikan/Dmhy/BangumiMoe/Nyaa)
│       │   ├── titleparse/       # 种子标题解析器
│       │   ├── rss/              # RSS 引擎 + 规则匹配
│       │   ├── stream/           # 流媒体规则执行 + ffmpeg
│       │   ├── download/         # 下载编排 + 失败分类 + 通知去重
│       │   ├── scheduler/        # 定时任务（重试/RSS/健康检测/清理）
│       │   └── bangumi/          # Bangumi API 集成
│       ├── downloader/           # qBittorrent provider 抽象
│       └── model/                # GORM 数据模型
├── frontend/                     # Vue 3 前端
│   └── src/
│       ├── components/ac/        # ★ 自研动森风组件库
│       ├── views/                # 页面
│       └── composables/          # useToast / useConfirm / useSkin ...
├── .github/workflows/            # CI：push tag → 多架构镜像 + Release
├── docker-compose.dev.yml        # 开发（源码挂载 + 热重载，本地构建）
└── docker-compose.yml            # 生产（全部服务拉 Docker Hub 镜像）
```

---

## 📦 发布流程

打一个 `v` 开头的 tag 即触发 CI 自动构建并发布：

```bash
git tag v1.0.0
git push origin v1.0.0
```

CI 会：
1. 在原生 amd64 / arm64 runner 上分别构建 `anidog-backend`、`anidog-frontend` 镜像并推 Docker Hub
2. 合成多架构 manifest（`:v1.0.0` 与 `:latest`）
3. 生成 GitHub Release，附带 `docker-compose.yml` + `.env.example`

> 需在仓库 **Settings → Secrets** 配置 `DOCKERHUB_USERNAME` 与 `DOCKERHUB_TOKEN`。

---

## 🔒 生产部署须知

`backend/dev.yaml` 内的密码 / JWT 密钥均为**开发默认值**，生产部署**必须**通过环境变量覆盖：

```bash
SECRET_KEY           # 32 字节随机串，openssl rand -hex 32
POSTGRES_PASSWORD    # 数据库强密码
DOWNLOADER_PASSWORD  # qBittorrent WebUI 密码
```

`docker-compose.yml` 已把这些接成 `.env` 变量，照 `.env.example` 填即可。

---

## 📖 深入文档

架构细节、数据流、常用运维命令见 **[CLAUDE.md](CLAUDE.md)**。

---

## 📄 许可证

[MIT](LICENSE) © leeyang1990

<div align="center">
<sub>🐾 用 Go 与 Vue 精心喂养 · Made with ❤️ for anime fans</sub>
</div>
