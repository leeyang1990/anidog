from fastapi import APIRouter
from app.api.api_v1.endpoints import auth, users, rss, anime, downloads, settings, dashboard

# 创建主路由
api_router = APIRouter()

# 注册各模块的路由
api_router.include_router(auth.router, prefix="/auth", tags=["认证"])
api_router.include_router(users.router, prefix="/users", tags=["用户管理"])
api_router.include_router(rss.router, prefix="/rss", tags=["RSS订阅"])
api_router.include_router(anime.router, prefix="/anime", tags=["番剧管理"])
api_router.include_router(downloads.router, prefix="/downloads", tags=["下载管理"]) 
api_router.include_router(settings.router, prefix="/settings", tags=["系统设置"])
api_router.include_router(dashboard.router, prefix="/dashboard", tags=["仪表盘"])
 