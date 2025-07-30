from typing import Optional
from datetime import datetime
from sqlmodel import Field, SQLModel
from enum import Enum

class DownloadStatus(str, Enum):
    """下载状态枚举"""
    PENDING = "pending"  # 等待中
    DOWNLOADING = "downloading"  # 下载中
    COMPLETED = "completed"  # 已完成
    FAILED = "failed"  # 失败
    PAUSED = "paused"  # 已暂停

class Download(SQLModel, table=True):
    """下载任务模型"""
    id: Optional[int] = Field(default=None, primary_key=True)
    torrent_id: str = Field(index=True)  # 下载器中的任务ID
    name: str  # 任务名称
    url: str  # 种子链接或磁力链接
    save_path: Optional[str] = None  # 保存路径
    status: DownloadStatus = Field(default=DownloadStatus.PENDING)
    progress: float = Field(default=0.0)  # 下载进度，0-100
    downloaded_bytes: Optional[int] = None  # 已下载字节数
    total_bytes: Optional[int] = None  # 总字节数
    download_speed: Optional[int] = None  # 下载速度（字节/秒）
    eta: Optional[int] = None  # 预计完成时间（秒）
    created_at: datetime = Field(default_factory=datetime.utcnow)
    updated_at: datetime = Field(default_factory=datetime.utcnow)
    completed_at: Optional[datetime] = None  # 完成时间

class DownloadCreate(SQLModel):
    """创建下载任务的输入模型"""
    name: str
    url: str
    save_path: Optional[str] = None

class DownloadUpdate(SQLModel):
    """更新下载任务的输入模型"""
    status: Optional[DownloadStatus] = None
    progress: Optional[float] = None
    downloaded_bytes: Optional[int] = None
    total_bytes: Optional[int] = None
    download_speed: Optional[int] = None
    eta: Optional[int] = None
    completed_at: Optional[datetime] = None 