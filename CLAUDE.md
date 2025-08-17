# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

使用中文回答

## 项目概述

御宅追番 (Mikanani-Dog) 是一个基于 FastAPI 和 Vue 3 的番剧自动下载管理系统。主要功能包括RSS订阅管理、番剧资源自动下载、文件智能处理和通知系统。

## 开发命令

### 后端开发
```bash
cd backend
# 安装依赖
pip install -r requirements.txt
# 启动开发服务器
python main.py
```

### 前端开发
```bash
cd frontend
# 安装依赖
npm install
# 启动开发服务器
npm run dev
# 构建生产版本
npm run build
# 代码检查
npm run lint
```

## 架构说明

### 后端架构 (FastAPI)
- **API层** (`backend/app/api/`): RESTful API端点，统一在`api_v1`版本下
  - 认证模块 (`auth.py`): 用户注册、登录、JWT认证
  - 动漫模块 (`anime.py`): 番剧的CRUD操作
  - RSS模块 (`rss.py`): RSS订阅源管理
  - 下载模块 (`downloads.py`): 下载任务管理
  
- **核心模块** (`backend/app/core/`):
  - `config.py`: 环境配置管理，支持.env文件
  - `database.py`: SQLModel数据库配置
  - `websocket.py`: WebSocket实时通信
  - `auth.py`: JWT认证逻辑

- **服务层** (`backend/app/services/`):
  - `downloader.py`: 下载器接口（支持qBittorrent等）
  - `agno/`: AI代理服务（用于番剧识别等）

### 前端架构 (Vue 3)
- **UI框架**: Naive UI + Tailwind CSS 双UI方案
- **状态管理**: Pinia (stores/auth.js)
- **路由**: Vue Router (router/index.js)
- **构建工具**: Vite，开发服务器端口3000，代理后端API到8000端口

### 数据模型
- User: 用户管理
- Anime: 番剧信息
- RSSFeed: RSS订阅源
- DownloadTask: 下载任务

## 环境变量配置

后端支持通过.env文件配置：
- `DATABASE_URL`: 数据库连接（默认SQLite）
- `SECRET_KEY`: JWT密钥
- `DOWNLOADER_*`: 下载器配置
- `OPENROUTER_API_KEY`: AI服务密钥

## 开发注意事项

1. 前端开发时使用Vite代理自动转发API请求到后端
2. WebSocket连接用于实时更新下载进度等信息
3. 支持多种下载器，通过配置切换
4. UI组件同时提供Naive UI和原生实现两种版本