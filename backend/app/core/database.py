from sqlmodel import Session, SQLModel, create_engine
from typing import Generator
from loguru import logger

from app.core.config import settings

# 创建数据库引擎
engine = create_engine(
    settings.DATABASE_URL, 
    echo=False,  # 设置为True可以查看SQL语句
    connect_args={"check_same_thread": False} if settings.DATABASE_URL.startswith("sqlite") else {}
)

def init_db() -> None:
    """初始化数据库，创建所有表"""
    try:
        logger.info("初始化数据库...")
        # 导入所有表模型以确保SQLModel能够找到它们
        from app.models import anime, rss, user, download
        
        SQLModel.metadata.create_all(engine)
        logger.info("数据库初始化完成")
    except Exception as e:
        logger.error(f"数据库初始化失败: {e}")
        raise

def get_db() -> Generator[Session, None, None]:
    """获取数据库会话的依赖函数"""
    db = Session(engine)
    try:
        yield db
    finally:
        db.close() 