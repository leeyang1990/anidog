from typing import Optional, List
from datetime import datetime
from sqlmodel import Field, SQLModel, Relationship
from enum import Enum

class AnimeStatus(str, Enum):
    """番剧状态枚举"""
    ONGOING = "ongoing"  # 连载中
    FINISHED = "finished"  # 已完结
    UPCOMING = "upcoming"  # 即将播出
    UNKNOWN = "unknown"  # 未知状态

class Anime(SQLModel, table=True):
    """番剧模型"""
    id: Optional[int] = Field(default=None, primary_key=True)
    title: str = Field(index=True)
    original_title: Optional[str] = None  # 原始标题（日文等）
    aliases: Optional[str] = None  # 别名，用逗号分隔
    description: Optional[str] = None
    status: AnimeStatus = Field(default=AnimeStatus.UNKNOWN)
    season: Optional[int] = None  # 季度，如1表示第一季
    year: Optional[int] = None  # 年份
    cover_url: Optional[str] = None  # 封面图片URL
    episode_count: Optional[int] = None  # 总集数
    current_episode: Optional[int] = None  # 当前更新集数
    directory: Optional[str] = None  # 存储目录
    created_at: datetime = Field(default_factory=datetime.utcnow)
    updated_at: datetime = Field(default_factory=datetime.utcnow)
    
    # 关联
    episodes: List["AnimeEpisode"] = Relationship(back_populates="anime")

    def update_current_episode(self, episode_number: int) -> None:
        """更新当前集数"""
        if self.current_episode is None or episode_number > self.current_episode:
            self.current_episode = episode_number

class AnimeEpisode(SQLModel, table=True):
    """番剧集数模型"""
    id: Optional[int] = Field(default=None, primary_key=True)
    episode_number: int = Field(description="集数")  # 改为 int 类型
    title: Optional[str] = None
    file_path: Optional[str] = None  # 文件路径
    file_size: Optional[int] = None  # 文件大小（字节）
    downloaded: bool = Field(default=False)
    download_id: Optional[str] = None  # 对应的下载ID
    created_at: datetime = Field(default_factory=datetime.utcnow)
    updated_at: datetime = Field(default_factory=datetime.utcnow)
    
    # 外键
    anime_id: int = Field(foreign_key="anime.id")
    
    # 关联
    anime: Optional[Anime] = Relationship(back_populates="episodes")

    def __init__(self, **data):
        """初始化时进行类型转换"""
        if "episode_number" in data:
            # 将浮点数转换为整数
            try:
                data["episode_number"] = int(float(data["episode_number"]))
            except (ValueError, TypeError):
                raise ValueError("Invalid episode number")
        super().__init__(**data)

# Pydantic模型，用于API
class AnimeCreate(SQLModel):
    """创建番剧的输入模型"""
    title: str
    original_title: Optional[str] = None
    aliases: Optional[str] = None
    description: Optional[str] = None
    status: AnimeStatus = AnimeStatus.UNKNOWN
    season: Optional[int] = None
    year: Optional[int] = None
    cover_url: Optional[str] = None
    episode_count: Optional[int] = None
    directory: Optional[str] = None

class AnimeUpdate(SQLModel):
    """更新番剧的输入模型"""
    title: Optional[str] = None
    original_title: Optional[str] = None
    aliases: Optional[str] = None
    description: Optional[str] = None
    status: Optional[AnimeStatus] = None
    season: Optional[int] = None
    year: Optional[int] = None
    cover_url: Optional[str] = None
    episode_count: Optional[int] = None
    current_episode: Optional[int] = None
    directory: Optional[str] = None

class AnimeResponse(AnimeCreate):
    """番剧响应模型"""
    id: int
    current_episode: Optional[int] = None
    created_at: datetime
    updated_at: datetime 