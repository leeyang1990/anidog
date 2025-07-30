from fastapi import APIRouter, Depends
from sqlmodel import Session, select, func, col
from datetime import datetime, timedelta
from typing import List, Dict, Any
from pydantic import BaseModel

from app.core.database import get_db
from app.core.auth import get_current_user
from app.models.user import User
from app.models.anime import Anime
from app.models.download import Download, DownloadStatus
from app.models.rss import RSSFeed

router = APIRouter()

class DashboardResponse(BaseModel):
    stats: Dict[str, int]
    downloadStats: Dict[str, List]
    recentDownloads: List[Dict[str, Any]]

@router.get("", response_model=DashboardResponse)
async def get_dashboard(
    current_user: User = Depends(get_current_user),
    db: Session = Depends(get_db)
):
    """获取仪表盘数据"""
    # 获取基本统计数据
    anime_count = db.exec(select(func.count()).select_from(Anime)).one()
    rss_count = db.exec(select(func.count()).select_from(RSSFeed)).one()
    
    # 获取下载统计
    waiting_count = db.exec(select(func.count()).select_from(Download).where(Download.status == DownloadStatus.PENDING)).one()
    downloading_count = db.exec(select(func.count()).select_from(Download).where(Download.status == DownloadStatus.DOWNLOADING)).one()
    completed_count = db.exec(select(func.count()).select_from(Download).where(Download.status == DownloadStatus.COMPLETED)).one()
    failed_count = db.exec(select(func.count()).select_from(Download).where(Download.status == DownloadStatus.FAILED)).one()
    
    # 获取最近7天的下载统计
    today = datetime.now().date()
    dates = []
    counts = []
    
    for i in range(6, -1, -1):
        date = today - timedelta(days=i)
        dates.append(date.strftime("%m-%d"))
        
        start_datetime = datetime.combine(date, datetime.min.time())
        end_datetime = datetime.combine(date, datetime.max.time())
        
        count = db.exec(
            select(func.count())
            .select_from(Download)
            .where(Download.created_at >= start_datetime)
            .where(Download.created_at <= end_datetime)
        ).one()
        
        counts.append(count)
    
    # 获取最近的下载
    recent_downloads = db.exec(
        select(Download)
        .order_by(col(Download.updated_at).desc())
        .limit(10)
    ).all()
    
    # 格式化最近下载数据
    recent_downloads_data = []
    for download in recent_downloads:
        recent_downloads_data.append({
            "id": download.id,
            "filename": download.name,
            "size": f"{download.total_bytes / 1024 / 1024:.2f} MB" if download.total_bytes else "未知",
            "status": download.status.value,
            "progress": download.progress,
            "updated_at": download.updated_at.strftime("%Y-%m-%d %H:%M:%S") if download.updated_at else None
        })
    
    return {
        "stats": {
            "animeCount": anime_count,
            "rssCount": rss_count,
            "waitingCount": waiting_count,
            "downloadingCount": downloading_count,
            "completedCount": completed_count,
            "failedCount": failed_count
        },
        "downloadStats": {
            "dates": dates,
            "counts": counts
        },
        "recentDownloads": recent_downloads_data
    } 