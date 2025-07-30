import os
from pathlib import Path
from typing import Optional
from pydantic import BaseModel
from dotenv import load_dotenv

# 加载.env文件
load_dotenv()

class Settings(BaseModel):
    """应用配置项"""
    PROJECT_NAME: str = "御宅追番"
    PROJECT_VERSION: str = "1.0.0"
    
    # 数据库配置
    DATABASE_URL: str = os.getenv("DATABASE_URL", "sqlite:///./mikanani.db")
    
    # JWT配置
    SECRET_KEY: str = os.getenv("SECRET_KEY", "supersecretkey")
    ACCESS_TOKEN_EXPIRE_MINUTES: int = int(os.getenv("ACCESS_TOKEN_EXPIRE_MINUTES", "1440"))
    
    # 下载器配置
    DOWNLOADER_TYPE: str = os.getenv("DOWNLOADER_TYPE", "qbittorrent")  # 支持：qbittorrent, transmission
    DOWNLOADER_HOST: str = os.getenv("DOWNLOADER_HOST", "http://localhost:8080")
    DOWNLOADER_USERNAME: str = os.getenv("DOWNLOADER_USERNAME", "admin")
    DOWNLOADER_PASSWORD: str = os.getenv("DOWNLOADER_PASSWORD", "adminadmin")
    
    # 媒体目录配置
    MEDIA_ROOT: Optional[str] = os.getenv("MEDIA_ROOT")
    
    # 任务配置
    RSS_CHECK_INTERVAL: int = int(os.getenv("RSS_CHECK_INTERVAL", "30"))  # 分钟
    
    # 通知配置
    ENABLE_NOTIFICATIONS: bool = os.getenv("ENABLE_NOTIFICATIONS", "false").lower() == "true"
    TELEGRAM_BOT_TOKEN: Optional[str] = os.getenv("TELEGRAM_BOT_TOKEN")
    TELEGRAM_CHAT_ID: Optional[str] = os.getenv("TELEGRAM_CHAT_ID")
    
    # 日志配置
    LOG_LEVEL: str = os.getenv("LOG_LEVEL", "INFO")
    LOG_DIR: Path = Path(os.getenv("LOG_DIR", "./logs"))
    
    # AI代理配置
    OPENROUTER_API_KEY: Optional[str] = os.getenv("OPENROUTER_API_KEY")
    
    class Config:
        from_attributes = True

# 创建全局配置实例
settings = Settings() 