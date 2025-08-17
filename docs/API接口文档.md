# ANIDOG 番剧自动下载管理系统 - API接口文档

## 1. 概述

### 1.1 基础信息
- **基础URL**: `http://localhost:8000/api/v1`
- **数据格式**: JSON
- **字符编码**: UTF-8
- **认证方式**: Bearer Token (JWT)

### 1.2 通用响应格式
```json
{
    "code": 200,
    "message": "success",
    "data": {},
    "timestamp": "2025-01-03T12:00:00Z"
}
```

### 1.3 错误响应格式
```json
{
    "code": 400,
    "message": "错误描述",
    "errors": [
        {
            "field": "字段名",
            "message": "具体错误信息"
        }
    ],
    "timestamp": "2025-01-03T12:00:00Z"
}
```

### 1.4 状态码说明
| 状态码 | 说明 |
|--------|------|
| 200 | 请求成功 |
| 201 | 创建成功 |
| 400 | 请求参数错误 |
| 401 | 未授权 |
| 403 | 禁止访问 |
| 404 | 资源不存在 |
| 409 | 资源冲突 |
| 500 | 服务器错误 |

## 2. 认证接口

### 2.1 用户注册
**POST** `/auth/register`

请求体：
```json
{
    "username": "testuser",
    "email": "test@example.com",
    "password": "password123"
}
```

响应：
```json
{
    "code": 201,
    "message": "注册成功",
    "data": {
        "id": 1,
        "username": "testuser",
        "email": "test@example.com",
        "is_active": true,
        "created_at": "2025-01-03T12:00:00Z"
    }
}
```

### 2.2 用户登录
**POST** `/auth/login`

请求体：
```json
{
    "username": "testuser",
    "password": "password123"
}
```

响应：
```json
{
    "code": 200,
    "message": "登录成功",
    "data": {
        "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
        "token_type": "bearer",
        "expires_in": 86400,
        "user": {
            "id": 1,
            "username": "testuser",
            "email": "test@example.com",
            "is_admin": false
        }
    }
}
```

### 2.3 刷新Token
**POST** `/auth/refresh`

请求头：
```
Authorization: Bearer <refresh_token>
```

响应：
```json
{
    "code": 200,
    "message": "Token刷新成功",
    "data": {
        "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
        "token_type": "bearer",
        "expires_in": 86400
    }
}
```

### 2.4 获取当前用户信息
**GET** `/auth/me`

请求头：
```
Authorization: Bearer <access_token>
```

响应：
```json
{
    "code": 200,
    "message": "success",
    "data": {
        "id": 1,
        "username": "testuser",
        "email": "test@example.com",
        "is_admin": false,
        "is_active": true,
        "created_at": "2025-01-03T12:00:00Z"
    }
}
```

## 3. 番剧管理接口

### 3.1 获取番剧列表
**GET** `/animes`

请求参数：
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| page | int | 否 | 页码，默认1 |
| per_page | int | 否 | 每页数量，默认10 |
| status | string | 否 | 状态筛选: ongoing, finished, upcoming |
| search | string | 否 | 搜索关键词 |

响应：
```json
{
    "code": 200,
    "message": "success",
    "data": {
        "items": [
            {
                "id": 1,
                "title": "某科学的超电磁炮",
                "original_title": "とある科学の超電磁砲",
                "status": "ongoing",
                "season": 3,
                "year": 2025,
                "current_episode": 12,
                "episode_count": 24,
                "cover_url": "https://example.com/cover.jpg",
                "created_at": "2025-01-03T12:00:00Z",
                "updated_at": "2025-01-03T12:00:00Z"
            }
        ],
        "total": 50,
        "page": 1,
        "per_page": 10,
        "pages": 5
    }
}
```

### 3.2 获取单个番剧详情
**GET** `/animes/{anime_id}`

响应：
```json
{
    "code": 200,
    "message": "success",
    "data": {
        "id": 1,
        "title": "某科学的超电磁炮",
        "original_title": "とある科学の超電磁砲",
        "aliases": "Toaru Kagaku no Railgun,A Certain Scientific Railgun",
        "description": "学园都市的故事...",
        "status": "ongoing",
        "season": 3,
        "year": 2025,
        "current_episode": 12,
        "episode_count": 24,
        "cover_url": "https://example.com/cover.jpg",
        "directory": "/media/anime/railgun-s3",
        "created_at": "2025-01-03T12:00:00Z",
        "updated_at": "2025-01-03T12:00:00Z"
    }
}
```

### 3.3 创建番剧
**POST** `/animes`

请求体：
```json
{
    "title": "某科学的超电磁炮",
    "original_title": "とある科学の超電磁砲",
    "description": "学园都市的故事...",
    "status": "ongoing",
    "season": 3,
    "year": 2025,
    "episode_count": 24,
    "cover_url": "https://example.com/cover.jpg"
}
```

响应：
```json
{
    "code": 201,
    "message": "创建成功",
    "data": {
        "id": 1,
        "title": "某科学的超电磁炮",
        // ... 完整番剧信息
    }
}
```

### 3.4 更新番剧信息
**PUT** `/animes/{anime_id}`

请求体（部分更新）：
```json
{
    "current_episode": 13,
    "status": "finished"
}
```

### 3.5 删除番剧
**DELETE** `/animes/{anime_id}`

响应：
```json
{
    "code": 200,
    "message": "删除成功",
    "data": {
        "id": 1,
        "title": "某科学的超电磁炮"
    }
}
```

### 3.6 获取番剧剧集列表
**GET** `/animes/{anime_id}/episodes`

响应：
```json
{
    "code": 200,
    "message": "success",
    "data": [
        {
            "id": 1,
            "anime_id": 1,
            "episode_number": 1,
            "title": "第一集标题",
            "file_path": "/media/anime/railgun-s3/ep01.mkv",
            "file_size": 536870912,
            "downloaded": true,
            "download_id": "abc123",
            "created_at": "2025-01-03T12:00:00Z"
        }
    ]
}
```

### 3.7 搜索番剧
**GET** `/animes/search`

请求参数：
```
keyword=超电磁炮
```

响应：
```json
{
    "code": 200,
    "message": "success",
    "data": {
        "items": [
            {
                "id": 1,
                "title": "某科学的超电磁炮",
                "match_field": "title",
                "score": 0.95
            }
        ]
    }
}
```

### 3.8 解析番剧标题
**POST** `/animes/parse_title`

请求体：
```json
{
    "title": "[Lilith-Raws] 某科学的超电磁炮 S03E12 [1080p][WEB-DL][AAC]"
}
```

响应：
```json
{
    "code": 200,
    "message": "success",
    "data": {
        "title": "某科学的超电磁炮",
        "season": 3,
        "episode": 12,
        "resolution": "1080p",
        "source": "WEB-DL",
        "subgroup": "Lilith-Raws"
    }
}
```

## 4. RSS订阅接口

### 4.1 获取RSS源列表
**GET** `/rss`

响应：
```json
{
    "code": 200,
    "message": "success",
    "data": [
        {
            "id": 1,
            "name": "Mikan RSS",
            "url": "https://mikanani.me/RSS/Classic",
            "enabled": true,
            "interval": 30,
            "last_check": "2025-01-03T12:00:00Z",
            "filter_rules": {
                "include": ["1080p", "简体"],
                "exclude": ["生肉"],
                "regex": "S\\d+E\\d+"
            },
            "created_at": "2025-01-03T12:00:00Z"
        }
    ]
}
```

### 4.2 添加RSS源
**POST** `/rss`

请求体：
```json
{
    "name": "Mikan RSS",
    "url": "https://mikanani.me/RSS/Classic",
    "enabled": true,
    "interval": 30,
    "filter_rules": {
        "include": ["1080p", "简体"],
        "exclude": ["生肉"]
    }
}
```

### 4.3 更新RSS源
**PUT** `/rss/{rss_id}`

### 4.4 删除RSS源
**DELETE** `/rss/{rss_id}`

### 4.5 手动检查RSS更新
**POST** `/rss/{rss_id}/check`

响应：
```json
{
    "code": 200,
    "message": "检查完成",
    "data": {
        "new_items": 5,
        "matched_items": 3,
        "downloads_created": 3
    }
}
```

### 4.6 测试RSS源
**POST** `/rss/test`

请求体：
```json
{
    "url": "https://mikanani.me/RSS/Classic"
}
```

响应：
```json
{
    "code": 200,
    "message": "RSS源有效",
    "data": {
        "valid": true,
        "title": "Mikan Project",
        "item_count": 50,
        "latest_item": {
            "title": "[Lilith-Raws] 某科学的超电磁炮...",
            "pub_date": "2025-01-03T12:00:00Z"
        }
    }
}
```

## 5. 下载管理接口

### 5.1 获取下载列表
**GET** `/downloads`

请求参数：
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| status | string | 否 | 状态筛选: downloading, completed, failed, paused |
| page | int | 否 | 页码 |
| per_page | int | 否 | 每页数量 |

响应：
```json
{
    "code": 200,
    "message": "success",
    "data": {
        "items": [
            {
                "id": 1,
                "name": "[Lilith-Raws] 某科学的超电磁炮 S03E12",
                "torrent_id": "abc123def456",
                "status": "downloading",
                "progress": 65.5,
                "download_speed": 2097152,
                "upload_speed": 524288,
                "eta": 300,
                "size": 536870912,
                "downloaded": 351272960,
                "save_path": "/downloads/anime",
                "category": "anime",
                "tags": ["railgun", "s3"],
                "created_at": "2025-01-03T12:00:00Z"
            }
        ],
        "total": 20
    }
}
```

### 5.2 添加下载任务
**POST** `/downloads`

请求体：
```json
{
    "url": "magnet:?xt=urn:btih:...",
    "save_path": "/downloads/anime/railgun",
    "category": "anime",
    "tags": ["railgun", "s3"],
    "paused": false
}
```

### 5.3 暂停/恢复下载
**PUT** `/downloads/{download_id}/pause`
**PUT** `/downloads/{download_id}/resume`

### 5.4 删除下载任务
**DELETE** `/downloads/{download_id}`

请求参数：
```
delete_files=true  # 是否同时删除文件
```

### 5.5 获取下载统计
**GET** `/downloads/stats`

响应：
```json
{
    "code": 200,
    "message": "success",
    "data": {
        "total_downloads": 150,
        "active_downloads": 3,
        "completed_downloads": 140,
        "failed_downloads": 7,
        "total_size": 107374182400,
        "download_speed": 5242880,
        "upload_speed": 1048576
    }
}
```

## 6. 设置接口

### 6.1 获取系统设置
**GET** `/settings`

响应：
```json
{
    "code": 200,
    "message": "success",
    "data": {
        "general": {
            "language": "zh-CN",
            "theme": "dark",
            "timezone": "Asia/Shanghai"
        },
        "download": {
            "default_path": "/downloads",
            "auto_start": true,
            "max_concurrent": 3,
            "speed_limit": {
                "download": 0,
                "upload": 1048576
            }
        },
        "notification": {
            "enabled": true,
            "channels": {
                "email": {
                    "enabled": false
                },
                "telegram": {
                    "enabled": true,
                    "bot_token": "***",
                    "chat_id": "123456"
                }
            }
        },
        "rss": {
            "check_interval": 30,
            "auto_download": true
        }
    }
}
```

### 6.2 更新系统设置
**PUT** `/settings`

请求体（部分更新）：
```json
{
    "download": {
        "max_concurrent": 5
    },
    "notification": {
        "enabled": false
    }
}
```

### 6.3 测试通知设置
**POST** `/settings/notification/test`

请求体：
```json
{
    "channel": "telegram",
    "message": "测试通知消息"
}
```

## 7. 仪表板接口

### 7.1 获取仪表板数据
**GET** `/dashboard`

响应：
```json
{
    "code": 200,
    "message": "success",
    "data": {
        "stats": {
            "total_animes": 50,
            "ongoing_animes": 15,
            "total_episodes": 600,
            "downloaded_episodes": 550,
            "total_size": 322122547200
        },
        "recent_downloads": [
            {
                "id": 1,
                "anime_title": "某科学的超电磁炮",
                "episode": 12,
                "completed_at": "2025-01-03T12:00:00Z"
            }
        ],
        "upcoming_episodes": [
            {
                "anime_id": 1,
                "anime_title": "某科学的超电磁炮",
                "next_episode": 13,
                "air_date": "2025-01-10T00:00:00Z"
            }
        ],
        "system_status": {
            "cpu_usage": 15.5,
            "memory_usage": 45.2,
            "disk_usage": 68.7,
            "qbittorrent_status": "connected"
        }
    }
}
```

### 7.2 获取活动日志
**GET** `/dashboard/activities`

请求参数：
```
limit=20
offset=0
```

响应：
```json
{
    "code": 200,
    "message": "success",
    "data": [
        {
            "id": 1,
            "type": "download_completed",
            "message": "下载完成: 某科学的超电磁炮 第12集",
            "metadata": {
                "anime_id": 1,
                "episode": 12,
                "file_size": 536870912
            },
            "created_at": "2025-01-03T12:00:00Z"
        }
    ]
}
```

## 8. WebSocket接口

### 8.1 连接WebSocket
**WS** `/ws`

连接时需要在URL参数中携带token：
```
ws://localhost:8000/ws?token=<access_token>
```

### 8.2 事件类型

#### 8.2.1 下载进度更新
```json
{
    "event": "download_progress",
    "data": {
        "download_id": 1,
        "progress": 75.5,
        "speed": 2097152,
        "eta": 120
    }
}
```

#### 8.2.2 下载完成通知
```json
{
    "event": "download_completed",
    "data": {
        "download_id": 1,
        "anime_id": 1,
        "episode": 12,
        "file_path": "/downloads/anime/railgun/ep12.mkv"
    }
}
```

#### 8.2.3 新番剧更新
```json
{
    "event": "anime_updated",
    "data": {
        "anime_id": 1,
        "new_episode": 13,
        "rss_source": "Mikan RSS"
    }
}
```

#### 8.2.4 系统通知
```json
{
    "event": "system_notification",
    "data": {
        "level": "info",
        "message": "RSS源检查完成，发现3个新项目",
        "timestamp": "2025-01-03T12:00:00Z"
    }
}
```

## 9. 错误码参考

| 错误码 | 说明 | 处理建议 |
|--------|------|----------|
| 1001 | 用户名已存在 | 更换用户名 |
| 1002 | 邮箱已注册 | 使用其他邮箱 |
| 1003 | 用户名或密码错误 | 检查登录信息 |
| 1004 | Token已过期 | 重新登录 |
| 1005 | Token无效 | 重新登录 |
| 2001 | 番剧不存在 | 检查番剧ID |
| 2002 | 番剧标题重复 | 修改标题 |
| 3001 | RSS源无效 | 检查URL格式 |
| 3002 | RSS解析失败 | 检查RSS内容 |
| 4001 | 下载器连接失败 | 检查qBittorrent配置 |
| 4002 | 磁力链接无效 | 检查链接格式 |
| 4003 | 存储空间不足 | 清理磁盘空间 |
| 5001 | 参数验证失败 | 检查请求参数 |
| 5002 | 权限不足 | 检查用户权限 |

## 10. SDK示例

### 10.1 Python SDK示例
```python
import requests
from typing import Dict, List, Optional

class AnidogClient:
    def __init__(self, base_url: str, token: Optional[str] = None):
        self.base_url = base_url.rstrip('/')
        self.session = requests.Session()
        if token:
            self.session.headers['Authorization'] = f'Bearer {token}'
    
    def login(self, username: str, password: str) -> Dict:
        """用户登录"""
        resp = self.session.post(
            f'{self.base_url}/auth/login',
            json={'username': username, 'password': password}
        )
        resp.raise_for_status()
        data = resp.json()['data']
        self.session.headers['Authorization'] = f'Bearer {data["access_token"]}'
        return data
    
    def get_animes(self, page: int = 1, per_page: int = 10) -> Dict:
        """获取番剧列表"""
        resp = self.session.get(
            f'{self.base_url}/animes',
            params={'page': page, 'per_page': per_page}
        )
        resp.raise_for_status()
        return resp.json()['data']
    
    def create_anime(self, anime_data: Dict) -> Dict:
        """创建番剧"""
        resp = self.session.post(
            f'{self.base_url}/animes',
            json=anime_data
        )
        resp.raise_for_status()
        return resp.json()['data']

# 使用示例
client = AnidogClient('http://localhost:8000/api/v1')
client.login('testuser', 'password123')
animes = client.get_animes(page=1, per_page=20)
```

### 10.2 JavaScript/TypeScript SDK示例
```typescript
interface AnimeData {
  id?: number;
  title: string;
  originalTitle?: string;
  status: 'ongoing' | 'finished' | 'upcoming';
  // ... 其他字段
}

class AnidogAPI {
  private baseURL: string;
  private token: string | null = null;

  constructor(baseURL: string) {
    this.baseURL = baseURL.replace(/\/$/, '');
  }

  async login(username: string, password: string): Promise<void> {
    const response = await fetch(`${this.baseURL}/auth/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password })
    });
    
    if (!response.ok) throw new Error('Login failed');
    
    const data = await response.json();
    this.token = data.data.access_token;
  }

  async getAnimes(page = 1, perPage = 10): Promise<AnimeData[]> {
    const response = await fetch(
      `${this.baseURL}/animes?page=${page}&per_page=${perPage}`,
      {
        headers: this.getHeaders()
      }
    );
    
    if (!response.ok) throw new Error('Failed to fetch animes');
    
    const data = await response.json();
    return data.data.items;
  }

  async createAnime(animeData: AnimeData): Promise<AnimeData> {
    const response = await fetch(`${this.baseURL}/animes`, {
      method: 'POST',
      headers: this.getHeaders(),
      body: JSON.stringify(animeData)
    });
    
    if (!response.ok) throw new Error('Failed to create anime');
    
    const data = await response.json();
    return data.data;
  }

  private getHeaders(): HeadersInit {
    const headers: HeadersInit = {
      'Content-Type': 'application/json'
    };
    
    if (this.token) {
      headers['Authorization'] = `Bearer ${this.token}`;
    }
    
    return headers;
  }
}

// 使用示例
const api = new AnidogAPI('http://localhost:8000/api/v1');
await api.login('testuser', 'password123');
const animes = await api.getAnimes(1, 20);
```

---

文档版本：1.0  
创建日期：2025-01-03  
作者：API设计团队