import asyncio

from fastapi import APIRouter, Depends, HTTPException, status, BackgroundTasks
from sqlmodel import Session, select, col
from typing import List, Optional
import os
import shutil
from datetime import datetime
from loguru import logger

from app.core.database import get_db
from app.core.auth import get_current_user
from app.models.user import User
from app.models.download import Download, DownloadStatus, DownloadCreate, DownloadUpdate
from app.core.config import settings
from app.core.websocket import broadcast_download_progress, broadcast_download_complete
from app.services.downloader import get_downloader, DownloaderException

router = APIRouter()

@router.get("")
def get_downloads(
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user),
    status: Optional[DownloadStatus] = None,
    page: int = 1,
    per_page: int = 10
):
    """获取下载任务列表"""
    # 计算跳过的记录数
    skip = (page - 1) * per_page
    
    # 构建查询
    query = select(Download)
    if status:
        query = query.where(Download.status == status)
    
    # 获取总记录数
    total_query = select(Download)
    if status:
        total_query = total_query.where(Download.status == status)
    total = len(db.exec(total_query).all())
    
    # 获取分页数据
    query = query.offset(skip).limit(per_page).order_by(col(Download.created_at).desc())
    downloads = db.exec(query).all()
    
    # 返回带有items和total的数据结构
    return {
        "items": downloads,
        "total": total
    }

@router.get("/{download_id}", response_model=Download)
def get_download(
    download_id: int,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user)
):
    """获取下载任务详情"""
    download = db.get(Download, download_id)
    if not download:
        raise HTTPException(status_code=404, detail="下载任务不存在")
    return download

@router.post("/", response_model=Download)
def create_download(
    download_data: DownloadCreate,
    background_tasks: BackgroundTasks,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user)
):
    """创建下载任务"""
    try:
        # 获取下载器实例
        downloader = get_downloader()
        
        # 添加下载任务
        torrent_id = downloader.add_torrent(download_data.url, download_data.save_path)
        
        # 保存下载任务
        download = Download(
            torrent_id=torrent_id,
            name=download_data.name,
            url=download_data.url,
            save_path=download_data.save_path,
            status=DownloadStatus.DOWNLOADING
        )
        
        db.add(download)
        db.commit()
        db.refresh(download)
        
        logger.info(f"创建下载任务: {download.name}")
        
        # 启动后台任务监控下载进度
        background_tasks.add_task(update_download_status, download.id, db)
        
        return download
    except DownloaderException as e:
        raise HTTPException(status_code=400, detail=f"下载任务创建失败: {str(e)}")

@router.put("/{download_id}/pause", response_model=Download)
def pause_download(
    download_id: int,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user)
):
    """暂停下载任务"""
    download = db.get(Download, download_id)
    if not download:
        raise HTTPException(status_code=404, detail="下载任务不存在")
    
    try:
        downloader = get_downloader()
        downloader.pause_torrent(download.torrent_id)
        
        download.status = DownloadStatus.PAUSED
        download.updated_at = datetime.utcnow()
        db.add(download)
        db.commit()
        db.refresh(download)
        
        logger.info(f"暂停下载任务: {download.name}")
        return download
    except DownloaderException as e:
        raise HTTPException(status_code=400, detail=f"暂停任务失败: {str(e)}")

@router.put("/{download_id}/resume", response_model=Download)
def resume_download(
    download_id: int,
    background_tasks: BackgroundTasks,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user)
):
    """恢复下载任务"""
    download = db.get(Download, download_id)
    if not download:
        raise HTTPException(status_code=404, detail="下载任务不存在")
    
    try:
        downloader = get_downloader()
        downloader.resume_torrent(download.torrent_id)
        
        download.status = DownloadStatus.DOWNLOADING
        download.updated_at = datetime.utcnow()
        db.add(download)
        db.commit()
        db.refresh(download)
        
        # 重新启动监控
        background_tasks.add_task(update_download_status, download.id, db)
        
        logger.info(f"恢复下载任务: {download.name}")
        return download
    except DownloaderException as e:
        raise HTTPException(status_code=400, detail=f"恢复任务失败: {str(e)}")

@router.delete("/{download_id}", response_model=Download)
def delete_download(
    download_id: int,
    remove_files: bool = False,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user)
):
    """删除下载任务"""
    download = db.get(Download, download_id)
    if not download:
        raise HTTPException(status_code=404, detail="下载任务不存在")
    
    try:
        downloader = get_downloader()
        downloader.remove_torrent(download.torrent_id, remove_files)
        
        db.delete(download)
        db.commit()
        
        logger.info(f"删除下载任务: {download.name}")
        return download
    except DownloaderException as e:
        raise HTTPException(status_code=400, detail=f"删除任务失败: {str(e)}")

@router.put("/{download_id}/refresh", response_model=Download)
def refresh_download_status(
    download_id: int,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user)
):
    """手动刷新下载状态"""
    download = db.get(Download, download_id)
    if not download:
        raise HTTPException(status_code=404, detail="下载任务不存在")
    
    try:
        update_single_download(download, db)
        db.refresh(download)
        return download
    except DownloaderException as e:
        raise HTTPException(status_code=400, detail=f"刷新任务状态失败: {str(e)}")

async def update_download_status(download_id: int, db: Session):
    """后台任务：更新下载状态"""
    with Session(db.engine) as session:
        download = session.get(Download, download_id)
        if not download:
            logger.error(f"下载任务不存在: ID={download_id}")
            return
        
        try:
            while download.status == DownloadStatus.DOWNLOADING:
                # 更新下载状态
                update_single_download(download, session)
                session.refresh(download)
                
                # 如果已完成或失败，则退出循环
                if download.status in [DownloadStatus.COMPLETED, DownloadStatus.FAILED]:
                    break
                
                # 等待几秒后再次检查
                await asyncio.sleep(5)
                
        except Exception as e:
            logger.error(f"更新下载状态失败: {download.name}, {str(e)}")

def update_single_download(download: Download, db: Session):
    """更新单个下载任务状态"""
    try:
        downloader = get_downloader()
        info = downloader.get_torrent_info(download.torrent_id)
        
        # 更新下载信息
        download.progress = info["progress"] * 100  # 转为百分比
        download.downloaded_bytes = info["downloaded_bytes"]
        download.total_bytes = info["total_bytes"]
        download.download_speed = info["download_speed"]
        download.eta = info["eta"]
        download.updated_at = datetime.utcnow()
        
        # 更新状态
        if info["status"] == "downloading":
            download.status = DownloadStatus.DOWNLOADING
        elif info["status"] == "paused":
            download.status = DownloadStatus.PAUSED
        elif info["status"] == "completed":
            download.status = DownloadStatus.COMPLETED
            download.completed_at = datetime.utcnow()
        elif info["status"] == "error":
            download.status = DownloadStatus.FAILED
        
        db.add(download)
        db.commit()
        
        # 广播下载进度更新
        broadcast_download_progress(download.torrent_id, download.name, download.progress)
        
        # 如果下载完成，广播完成消息
        if download.status == DownloadStatus.COMPLETED:
            broadcast_download_complete(download.torrent_id, download.name)
            
    except DownloaderException as e:
        logger.error(f"获取下载信息失败: {download.name}, {str(e)}")
        download.status = DownloadStatus.FAILED
        db.add(download)
        db.commit() 