from typing import Optional, List, Dict, Any
from loguru import logger

from .config import search_agent, crawler_agent

class AgnoService:
    """Agno AI代理服务类"""
    
    @staticmethod
    async def search_anime(query: str) -> Dict[str, Any]:
        """使用搜索代理查找动漫信息"""
        try:
            response = await search_agent.arun(query)
            return {
                "success": True,
                "data": response,
                "error": None
            }
        except Exception as e:
            logger.error(f"搜索动漫信息失败: {e}")
            return {
                "success": False,
                "data": None,
                "error": str(e)
            }
    
    @staticmethod
    async def crawl_anime_info(url: str) -> Dict[str, Any]:
        """使用爬虫代理提取动漫网站信息"""
        try:
            response = await crawler_agent.arun(f"请提取该页面的动漫信息: {url}")
            return {
                "success": True,
                "data": response,
                "error": None
            }
        except Exception as e:
            logger.error(f"提取动漫网站信息失败: {e}")
            return {
                "success": False,
                "data": None,
                "error": str(e)
            }

# 创建服务实例
agno_service = AgnoService() 