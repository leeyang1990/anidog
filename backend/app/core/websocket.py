from fastapi import APIRouter, WebSocket, WebSocketDisconnect
from typing import Dict, List
import json
from loguru import logger

# WebSocket路由
websocket_router = APIRouter()

# 活跃连接管理
class ConnectionManager:
    def __init__(self):
        # 存储所有活跃连接的字典
        self.active_connections: Dict[str, List[WebSocket]] = {}
    
    async def connect(self, websocket: WebSocket, client_id: str):
        """处理新的WebSocket连接"""
        await websocket.accept()
        if client_id not in self.active_connections:
            self.active_connections[client_id] = []
        self.active_connections[client_id].append(websocket)
        logger.info(f"客户端 {client_id} 已连接，当前连接数: {len(self.active_connections)}")
    
    def disconnect(self, websocket: WebSocket, client_id: str):
        """处理连接断开"""
        if client_id in self.active_connections:
            try:
                self.active_connections[client_id].remove(websocket)
                if not self.active_connections[client_id]:
                    del self.active_connections[client_id]
                logger.info(f"客户端 {client_id} 已断开连接，当前连接数: {len(self.active_connections)}")
            except ValueError:
                pass
    
    async def send_message(self, message: dict, client_id: str = None):
        """发送消息到指定客户端或所有客户端"""
        if client_id:
            # 发送到特定客户端
            if client_id in self.active_connections:
                for connection in self.active_connections[client_id]:
                    try:
                        await connection.send_text(json.dumps(message))
                    except Exception as e:
                        logger.error(f"发送消息到客户端 {client_id} 失败: {e}")
        else:
            # 广播到所有客户端
            for client_connections in self.active_connections.values():
                for connection in client_connections:
                    try:
                        await connection.send_text(json.dumps(message))
                    except Exception as e:
                        logger.error(f"广播消息失败: {e}")

# 创建连接管理器实例
manager = ConnectionManager()

@websocket_router.websocket("/ws/{client_id}")
async def websocket_endpoint(websocket: WebSocket, client_id: str):
    """WebSocket端点"""
    await manager.connect(websocket, client_id)
    try:
        while True:
            # 等待接收消息
            data = await websocket.receive_text()
            try:
                # 解析客户端发送的消息
                message = json.loads(data)
                logger.debug(f"收到来自 {client_id} 的消息: {message}")
                
                # 这里可以添加处理接收到消息的逻辑
                # 示例：简单回复
                await manager.send_message(
                    {"type": "response", "content": "服务器已收到消息"}, 
                    client_id=client_id
                )
            except json.JSONDecodeError:
                logger.error(f"无效的JSON格式: {data}")
                
    except WebSocketDisconnect:
        manager.disconnect(websocket, client_id)
    except Exception as e:
        logger.error(f"WebSocket错误: {e}")
        manager.disconnect(websocket, client_id)

# 用于其他模块调用的广播函数
async def broadcast_download_progress(torrent_id: str, file_name: str, progress: float):
    """广播下载进度更新"""
    await manager.send_message({
        "type": "download_progress",
        "data": {
            "id": torrent_id,
            "file_name": file_name,
            "progress": progress
        }
    })

async def broadcast_download_complete(torrent_id: str, file_name: str):
    """广播下载完成消息"""
    await manager.send_message({
        "type": "download_complete",
        "data": {
            "id": torrent_id,
            "file_name": file_name
        }
    })

async def broadcast_new_anime_found(anime_info: dict):
    """广播发现新番剧消息"""
    await manager.send_message({
        "type": "new_anime",
        "data": anime_info
    }) 