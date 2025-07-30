import qbittorrentapi
from abc import ABC, abstractmethod
from loguru import logger
from app.core.config import settings
import asyncio

class DownloaderException(Exception):
    """下载器异常"""
    pass

class BaseDownloader(ABC):
    """下载器基类"""
    
    @abstractmethod
    def add_torrent(self, torrent_url: str, save_path: str = None) -> str:
        """添加种子任务，返回任务ID"""
        pass
    
    @abstractmethod
    def pause_torrent(self, torrent_id: str) -> None:
        """暂停种子任务"""
        pass
    
    @abstractmethod
    def resume_torrent(self, torrent_id: str) -> None:
        """恢复种子任务"""
        pass
    
    @abstractmethod
    def remove_torrent(self, torrent_id: str, remove_files: bool = False) -> None:
        """删除种子任务"""
        pass
    
    @abstractmethod
    def get_torrent_info(self, torrent_id: str) -> dict:
        """获取种子任务信息"""
        pass

class QBittorrentDownloader(BaseDownloader):
    """qBittorrent下载器实现"""
    
    def __init__(self):
        """初始化qBittorrent客户端"""
        try:
            self.client = qbittorrentapi.Client(
                host=settings.DOWNLOADER_HOST,
                username=settings.DOWNLOADER_USERNAME,
                password=settings.DOWNLOADER_PASSWORD
            )
            self.client.auth_log_in()
            logger.info(f"qBittorrent连接成功: {settings.DOWNLOADER_HOST}")
        except Exception as e:

            # logger.error(f"qBittorrent连接失败: {str(e)}")
            raise DownloaderException(f"qBittorrent连接失败: {str(e)}")
    
    async def add_torrent(self, torrent_url: str, save_path: str = None) -> str:
        """添加种子任务"""
        try:
            kwargs = {}
            if save_path:
                kwargs["savepath"] = save_path
            
            self.client.torrents_add(urls=torrent_url, **kwargs)
            
            # 等待片刻，让qBittorrent有时间添加种子
            await asyncio.sleep(2)
            
            # 获取新添加的种子信息以返回ID
            torrents = self.client.torrents_info(sort='added_on', reverse=True, limit=1)
            if torrents:
                return torrents[0].hash
            else:
                raise DownloaderException("无法获取新添加的种子信息")
            
        except Exception as e:
            logger.error(f"添加种子失败: {str(e)}")
            raise DownloaderException(f"添加种子失败: {str(e)}")
    
    def pause_torrent(self, torrent_id: str) -> None:
        """暂停种子任务"""
        try:
            self.client.torrents_pause(torrent_hashes=torrent_id)
        except Exception as e:
            logger.error(f"暂停种子失败: {str(e)}")
            raise DownloaderException(f"暂停种子失败: {str(e)}")
    
    def resume_torrent(self, torrent_id: str) -> None:
        """恢复种子任务"""
        try:
            self.client.torrents_resume(torrent_hashes=torrent_id)
        except Exception as e:
            logger.error(f"恢复种子失败: {str(e)}")
            raise DownloaderException(f"恢复种子失败: {str(e)}")
    
    def remove_torrent(self, torrent_id: str, remove_files: bool = False) -> None:
        """删除种子任务"""
        try:
            self.client.torrents_delete(delete_files=remove_files, torrent_hashes=torrent_id)
        except Exception as e:
            logger.error(f"删除种子失败: {str(e)}")
            raise DownloaderException(f"删除种子失败: {str(e)}")
    
    def get_torrent_info(self, torrent_id: str) -> dict:
        """获取种子任务信息"""
        try:
            torrent = self.client.torrents_info(torrent_hashes=torrent_id)
            
            if not torrent:
                raise DownloaderException(f"未找到种子任务: {torrent_id}")
            
            torrent = torrent[0]
            
            # 计算进度和状态
            progress = torrent.progress
            if torrent.state == "downloading" or torrent.state == "stalledDL":
                status = "downloading"
            elif torrent.state == "pausedDL":
                status = "paused"
            elif torrent.state == "uploading" or torrent.state == "stalledUP" or torrent.state == "pausedUP":
                status = "completed"
            elif torrent.state == "error" or torrent.state == "missingFiles":
                status = "error"
            else:
                status = "downloading"
            
            return {
                "torrent_id": torrent.hash,
                "name": torrent.name,
                "progress": progress,
                "downloaded_bytes": torrent.downloaded,
                "total_bytes": torrent.size,
                "download_speed": torrent.dlspeed,
                "eta": torrent.eta if hasattr(torrent, "eta") else 0,
                "status": status
            }
            
        except Exception as e:
            logger.error(f"获取种子信息失败: {str(e)}")
            raise DownloaderException(f"获取种子信息失败: {str(e)}")

class TransmissionDownloader(BaseDownloader):
    """Transmission下载器实现"""
    
    def __init__(self):
        """初始化Transmission客户端"""
        # 如有需要可以扩展实现Transmission下载器
        raise NotImplementedError("Transmission下载器未实现")
        
    def add_torrent(self, torrent_url: str, save_path: str = None) -> str:
        pass
    
    def pause_torrent(self, torrent_id: str) -> None:
        pass
    
    def resume_torrent(self, torrent_id: str) -> None:
        pass
    
    def remove_torrent(self, torrent_id: str, remove_files: bool = False) -> None:
        pass
    
    def get_torrent_info(self, torrent_id: str) -> dict:
        pass

def get_downloader() -> BaseDownloader:
    """获取下载器实例"""
    downloader_type = settings.DOWNLOADER_TYPE.lower()
    
    if downloader_type == "qbittorrent":
        return QBittorrentDownloader()
    elif downloader_type == "transmission":
        return TransmissionDownloader()
    else:
        logger.error(f"不支持的下载器类型: {downloader_type}")
        raise DownloaderException(f"不支持的下载器类型: {downloader_type}") 