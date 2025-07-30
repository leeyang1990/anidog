# 御宅追番 (Mikanani-Dog)

一个基于 FastAPI 和 Vue 3 的番剧自动下载管理系统。

## 功能特点

- 番剧订阅：管理RSS订阅源，设置过滤规则
- 资源管理：自动解析番剧标题、集数信息
- 下载管理：对接下载器（如qBittorrent），监控下载进度
- 文件处理：智能重命名文件，按番剧整理
- 通知系统：支持多平台通知

## 技术栈

### 后端

- Web框架：FastAPI
- 数据库：SQLModel + SQLite
- 任务处理：内置线程池
- 实时通信：WebSocket

### 前端

- 框架：Vue 3
- 状态管理：Pinia
- UI库：Naive UI + Tailwind CSS
- 构建工具：Vite
- 实时通信：Socket.IO

## 项目结构

```
mikanani-dog/
  ├── backend/             # 后端代码
  │   ├── app/             # 应用代码
  │   │   ├── api/         # API路由
  │   │   ├── core/        # 核心模块
  │   │   ├── models/      # 数据模型
  │   │   └── services/    # 服务模块
  │   ├── main.py          # 主入口
  │   └── requirements.txt # 依赖
  └── frontend/            # 前端代码
      ├── src/             # 源码
      ├── package.json     # 依赖配置
      ├── vite.config.js   # Vite配置
      └── tailwind.config.js # Tailwind配置
```

## 安装与运行

### 后端

```bash
cd backend
pip install -r requirements.txt
python main.py
```

### 前端

```bash
cd frontend
npm install
npm run dev
```

## 快速开始

1. 启动后端服务
2. 启动前端开发服务器
3. 访问 http://localhost:3000
4. 首次使用时注册管理员账户
5. 添加RSS订阅源和过滤规则
6. 配置下载器连接信息

## 开发者

本项目基于 FastAPI 和 Vue 3 技术栈，欢迎贡献代码。 