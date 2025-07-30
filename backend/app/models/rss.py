from typing import Optional, List
from datetime import datetime
from sqlmodel import Field, SQLModel, Relationship

class RSSFeed(SQLModel, table=True):
    """RSS订阅源模型"""
    id: Optional[int] = Field(default=None, primary_key=True)
    name: str = Field(index=True)
    url: str = Field(unique=True)
    description: Optional[str] = None
    enabled: bool = Field(default=True)
    last_check: Optional[datetime] = None
    check_interval: int = Field(default=30)  # 检查间隔（分钟）
    created_at: datetime = Field(default_factory=datetime.utcnow)
    updated_at: datetime = Field(default_factory=datetime.utcnow)
    
    # 关联
    rules: List["RSSRule"] = Relationship(back_populates="rss_feed")

class RSSRule(SQLModel, table=True):
    """RSS过滤规则模型"""
    id: Optional[int] = Field(default=None, primary_key=True)
    name: str
    keyword: str  # 关键词或正则表达式
    is_regex: bool = Field(default=False)  # 是否是正则表达式
    include: bool = Field(default=True)  # True表示包含，False表示排除
    enabled: bool = Field(default=True)
    created_at: datetime = Field(default_factory=datetime.utcnow)
    
    # 外键
    rss_feed_id: int = Field(foreign_key="rssfeed.id")
    
    # 关联
    rss_feed: RSSFeed = Relationship(back_populates="rules")

class RSSEntry(SQLModel, table=True):
    """RSS条目记录模型，用于记录已处理的条目"""
    id: Optional[int] = Field(default=None, primary_key=True)
    entry_id: str = Field(index=True)  # 条目唯一标识（如guid）
    title: str
    link: str
    published: Optional[datetime] = None
    processed_at: datetime = Field(default_factory=datetime.utcnow)
    downloaded: bool = Field(default=False)
    
    # 外键
    rss_feed_id: int = Field(foreign_key="rssfeed.id")

# Pydantic模型，用于API
class RSSFeedCreate(SQLModel):
    """创建RSS订阅源的输入模型"""
    name: str
    url: str
    description: Optional[str] = None
    enabled: bool = True
    check_interval: int = 30

class RSSFeedUpdate(SQLModel):
    """更新RSS订阅源的输入模型"""
    name: Optional[str] = None
    url: Optional[str] = None
    description: Optional[str] = None
    enabled: Optional[bool] = None
    check_interval: Optional[int] = None

class RSSRuleCreate(SQLModel):
    """创建RSS规则的输入模型"""
    name: str
    keyword: str
    is_regex: bool = False
    include: bool = True
    enabled: bool = True
    
class RSSRuleUpdate(SQLModel):
    """更新RSS规则的输入模型"""
    name: Optional[str] = None
    keyword: Optional[str] = None
    is_regex: Optional[bool] = None
    include: Optional[bool] = None
    enabled: Optional[bool] = None 