from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from fastapi.staticfiles import StaticFiles
import uvicorn
from loguru import logger

from app.core.config import settings
from app.api.api_v1.api import api_router
from app.core.websocket import websocket_router
from app.core.database import init_db

app = FastAPI(
    title=settings.PROJECT_NAME,
    description="御宅追番 - 番剧自动下载管理系统",
    version="1.0.0",
)

# 配置CORS
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# 注册API路由
app.include_router(api_router, prefix="/api/v1")
# 注册WebSocket路由
app.include_router(websocket_router)

# 配置静态文件（前端构建后的文件）
try:
    app.mount("/", StaticFiles(directory="../frontend/dist", html=True), name="static")
    logger.info("已挂载前端静态文件")
except Exception as e:
    logger.warning(f"未能挂载前端静态文件: {e}")

@app.get("/healthcheck", include_in_schema=False)
def healthcheck():
    """健康检查接口"""
    return {"status": "ok"}

# 初始化数据库
@app.on_event("startup")
async def startup_db():
    logger.info("正在初始化数据库...")
    init_db()
    logger.info("数据库初始化完成")

if __name__ == "__main__":
    logger.info("启动御宅追番系统...")
    uvicorn.run("main:app", host="0.0.0.0", port=8000, reload=True) 