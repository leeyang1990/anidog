from fastapi import APIRouter, Depends, HTTPException, status
from sqlmodel import Session
from pydantic import BaseModel
from loguru import logger
import psutil
import platform
from datetime import datetime
import os

from app.core.database import get_db
from app.core.auth import get_current_user
from app.models.user import User

router = APIRouter()

# 系统设置模型
class SystemSettings(BaseModel):
    downloadDir: str
    maxConcurrent: int

# 全局设置变量
SETTINGS = {
    "downloadDir": os.path.expanduser("~/Downloads"),
    "maxConcurrent": 3
}

@router.get("/", response_model=SystemSettings)
def get_settings(current_user: User = Depends(get_current_user)):
    """获取系统设置"""
    return SETTINGS

@router.put("/", status_code=status.HTTP_200_OK)
def update_settings(
    settings: SystemSettings,
    current_user: User = Depends(get_current_user)
):
    """更新系统设置（需要管理员权限）"""
    if not current_user.is_admin:
        raise HTTPException(status_code=403, detail="无权限操作")
    
    # 验证下载目录是否存在
    if not os.path.exists(settings.downloadDir):
        try:
            os.makedirs(settings.downloadDir)
        except:
            raise HTTPException(status_code=400, detail="下载目录创建失败")
    
    # 更新设置
    SETTINGS["downloadDir"] = settings.downloadDir
    SETTINGS["maxConcurrent"] = settings.maxConcurrent
    
    logger.info(f"用户 {current_user.username} 更新了系统设置")
    return {"message": "设置更新成功"}

# 系统信息模型
class SystemInfo(BaseModel):
    version: str
    uptime: str
    cpuUsage: float
    memoryUsage: float
    diskUsage: float

@router.get("/info", response_model=SystemInfo)
def get_system_info(current_user: User = Depends(get_current_user)):
    """获取系统信息"""
    # 获取系统启动时间
    boot_time = datetime.fromtimestamp(psutil.boot_time())
    uptime = str(datetime.now() - boot_time).split('.')[0]
    
    # 获取CPU使用率
    cpu_usage = psutil.cpu_percent(interval=1)
    
    # 获取内存使用率
    memory = psutil.virtual_memory()
    memory_usage = memory.percent
    
    # 获取磁盘使用率
    disk = psutil.disk_usage('/')
    disk_usage = disk.percent
    
    return {
        "version": "1.0.0",
        "uptime": uptime,
        "cpuUsage": cpu_usage,
        "memoryUsage": memory_usage,
        "diskUsage": disk_usage
    } 