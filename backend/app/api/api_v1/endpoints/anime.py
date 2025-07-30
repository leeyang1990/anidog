from fastapi import APIRouter, Depends, HTTPException, status, BackgroundTasks, Query
from sqlmodel import Session, select, col, or_
from typing import List, Optional
import re
from datetime import datetime
from loguru import logger

from app.core.database import get_db
from app.core.auth import get_current_user
from app.models.user import User
from app.models.anime import (
    Anime, AnimeEpisode, AnimeStatus,
    AnimeCreate, AnimeUpdate, AnimeResponse
)
from app.models.download import Download
from app.services.agno.service import agno_service

router = APIRouter()

@router.get("")
def get_animes(
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user),
    page: int = 1,
    per_page: int = 10,
    status: Optional[AnimeStatus] = None
):
    """获取番剧列表"""
    # 计算跳过的记录数
    skip = (page - 1) * per_page
    
    # 构建查询
    query = select(Anime)
    if status:
        query = query.where(Anime.status == status)
    
    # 获取总记录数
    total = len(db.exec(query).all())
    
    # 获取分页数据
    query = query.offset(skip).limit(per_page).order_by(col(Anime.updated_at).desc())
    animes = db.exec(query).all()
    
    return {
        "items": animes,
        "total": total
    }

@router.get("/{anime_id}", response_model=AnimeResponse)
def get_anime(
    anime_id: int,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user)
):
    """获取指定番剧信息"""
    anime = db.get(Anime, anime_id)
    if not anime:
        raise HTTPException(status_code=404, detail="番剧不存在")
    return anime

@router.post("/", response_model=AnimeResponse)
def create_anime(
    anime_data: AnimeCreate,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user)
):
    """创建新番剧"""
    # 检查是否已存在相同标题的番剧
    existing_anime = db.exec(select(Anime).where(Anime.title == anime_data.title)).first()
    if existing_anime:
        raise HTTPException(status_code=400, detail="该番剧标题已存在")
    
    anime = Anime(**anime_data.dict())
    
    db.add(anime)
    db.commit()
    db.refresh(anime)
    
    logger.info(f"创建番剧: {anime.title}")
    return anime

@router.put("/{anime_id}", response_model=AnimeResponse)
def update_anime(
    anime_id: int,
    anime_data: AnimeUpdate,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user)
):
    """更新番剧信息"""
    anime = db.get(Anime, anime_id)
    if not anime:
        raise HTTPException(status_code=404, detail="番剧不存在")
    
    anime_data_dict = anime_data.dict(exclude_unset=True)
    for key, value in anime_data_dict.items():
        setattr(anime, key, value)
    
    anime.updated_at = datetime.utcnow()
    db.add(anime)
    db.commit()
    db.refresh(anime)
    
    logger.info(f"更新番剧: {anime.title}")
    return anime

@router.delete("/{anime_id}", response_model=AnimeResponse)
def delete_anime(
    anime_id: int,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user)
):
    """删除番剧"""
    anime = db.get(Anime, anime_id)
    if not anime:
        raise HTTPException(status_code=404, detail="番剧不存在")
    
    # 删除相关剧集
    episodes = db.exec(select(AnimeEpisode).where(AnimeEpisode.anime_id == anime_id)).all()
    for episode in episodes:
        db.delete(episode)
    
    db.delete(anime)
    db.commit()
    
    logger.info(f"删除番剧: {anime.title}")
    return anime

@router.get("/{anime_id}/episodes", response_model=List[AnimeEpisode])
def get_anime_episodes(
    anime_id: int,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user),
    skip: int = 0,
    limit: int = 100
):
    """获取番剧的所有剧集"""
    anime = db.get(Anime, anime_id)
    if not anime:
        raise HTTPException(status_code=404, detail="番剧不存在")
    
    episodes = db.exec(
        select(AnimeEpisode)
        .where(AnimeEpisode.anime_id == anime_id)
        .order_by(AnimeEpisode.episode_number)
        .offset(skip)
        .limit(limit)
    ).all()
    
    return episodes

@router.post("/{anime_id}/episodes", response_model=AnimeEpisode)
def create_anime_episode(
    anime_id: int,
    episode_number: int,  # 改为 int 类型
    title: Optional[str] = None,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user)
):
    """创建番剧剧集"""
    anime = db.get(Anime, anime_id)
    if not anime:
        raise HTTPException(status_code=404, detail="番剧不存在")
    
    # 检查剧集是否已存在
    existing_episode = db.exec(
        select(AnimeEpisode)
        .where(AnimeEpisode.anime_id == anime_id)
        .where(AnimeEpisode.episode_number == episode_number)
    ).first()
    
    if existing_episode:
        raise HTTPException(status_code=400, detail=f"第{episode_number}集已存在")
    
    # 创建新剧集
    episode = AnimeEpisode(
        anime_id=anime_id,
        episode_number=episode_number,
        title=title
    )
    
    db.add(episode)
    db.commit()
    db.refresh(episode)
    
    # 更新番剧当前集数信息
    anime.update_current_episode(episode_number)  # 使用新添加的方法
    db.add(anime)
    db.commit()
    
    logger.info(f"创建剧集: {anime.title} 第{episode_number}集")
    return episode

@router.delete("/{anime_id}/episodes/{episode_id}", response_model=AnimeEpisode)
def delete_anime_episode(
    anime_id: int,
    episode_id: int,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user)
):
    """删除番剧剧集"""
    # 先获取番剧信息
    anime = db.get(Anime, anime_id)
    if not anime:
        raise HTTPException(status_code=404, detail="番剧不存在")

    # 获取要删除的剧集
    episode = db.get(AnimeEpisode, episode_id)
    if not episode or episode.anime_id != anime_id:
        raise HTTPException(status_code=404, detail="剧集不存在")
    
    # 保存剧集信息用于返回
    episode_info = AnimeEpisode(
        id=episode.id,
        anime_id=episode.anime_id,
        episode_number=episode.episode_number,
        title=episode.title or "",
        file_path=episode.file_path,
        file_size=episode.file_size,
        downloaded=episode.downloaded,
        download_id=episode.download_id,
        created_at=episode.created_at,
        updated_at=episode.updated_at
    )
    
    # 删除剧集
    db.delete(episode)
    db.commit()
    
    # 更新番剧当前集数
    latest_episode = db.exec(
        select(AnimeEpisode)
        .where(AnimeEpisode.anime_id == anime_id)
        .order_by(col(AnimeEpisode.episode_number).desc())  # 使用 col() 函数
    ).first()
    
    if latest_episode:
        anime.update_current_episode(latest_episode.episode_number)
    else:
        anime.current_episode = None
    
    anime.updated_at = datetime.utcnow()
    db.add(anime)
    db.commit()
    
    logger.info(f"删除剧集: {anime.title} 第{episode_info.episode_number}集")
    return episode_info

@router.post("/parse_title", response_model=dict)
def parse_anime_title(
    title: str, 
    current_user: User = Depends(get_current_user)
):
    """解析番剧标题，提取信息"""
    result = {
        "title": "",  # 使用空字符串而不是 None
        "season": 0,  # 使用 0 表示未知季度
        "episode": 0,  # 使用 0 表示未知集数
        "resolution": "",  # 使用空字符串而不是 None
        "source": "",  # 使用空字符串而不是 None
        "subgroup": ""  # 使用空字符串而不是 None
    }
    
    # 提取季度信息
    season_match = re.search(r'S(\d+)', title, re.IGNORECASE)
    if season_match:
        result["season"] = int(season_match.group(1))
    elif "第二季" in title:
        result["season"] = 2
    elif "第三季" in title:
        result["season"] = 3
    elif "第四季" in title:
        result["season"] = 4
    
    # 提取集数信息
    episode_match = re.search(r'E(\d+)', title, re.IGNORECASE)
    if episode_match:
        result["episode"] = int(episode_match.group(1))
    
    # 提取分辨率信息
    resolution_match = re.search(r'(1080p|720p|2160p|4K)', title, re.IGNORECASE)
    if resolution_match:
        result["resolution"] = resolution_match.group(1)
    
    # 提取视频源信息
    source_match = re.search(r'(BD|WEB-DL|BDRIP)', title, re.IGNORECASE)
    if source_match:
        result["source"] = source_match.group(1)
    
    # 提取字幕组信息（假设在方括号内）
    subgroup_match = re.search(r'\[(.*?)\]', title)
    if subgroup_match:
        result["subgroup"] = subgroup_match.group(1)
    
    # 提取主标题（移除季度、集数等信息后的部分）
    clean_title = title
    clean_title = re.sub(r'\[.*?\]', '', clean_title)  # 移除方括号内容
    clean_title = re.sub(r'S\d+|E\d+', '', clean_title, flags=re.IGNORECASE)  # 移除季度和集数标记
    clean_title = re.sub(r'1080p|720p|2160p|4K', '', clean_title, flags=re.IGNORECASE)  # 移除分辨率信息
    clean_title = re.sub(r'BD|WEB-DL|BDRIP', '', clean_title, flags=re.IGNORECASE)  # 移除视频源信息
    clean_title = re.sub(r'第[一二三四]季', '', clean_title)  # 移除中文季度信息
    clean_title = clean_title.strip()  # 移除首尾空白
    
    if clean_title:
        result["title"] = clean_title
    
    return result

@router.get("/search")
async def search_anime(
    keyword: str = Query(..., description="搜索关键词"),
    db: Session = Depends(get_db)
) -> dict:
    """搜索番剧"""
    # 使用 SQL LIKE 操作符进行模糊搜索
    query = select(Anime).where(
        or_(
            col(Anime.title).like(f"%{keyword}%"),
            col(Anime.original_title).like(f"%{keyword}%"),
            col(Anime.aliases).like(f"%{keyword}%")
        )
    )
    
    animes = db.exec(query).all()
    return {"items": animes}

@router.get("/crawl")
async def crawl_anime_info(url: str = Query(..., description="动漫网站URL")) -> dict:
    """爬取动漫网站信息"""
    result = await agno_service.crawl_anime_info(url)
    if not result["success"]:
        raise HTTPException(status_code=500, detail=result["error"])
    return result

@router.get("/{anime_id}/downloads")
def get_anime_downloads(
    anime_id: int,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user),
    page: int = 1,
    per_page: int = 10
):
    """获取指定动漫的下载记录"""
    # 检查动漫是否存在
    anime = db.get(Anime, anime_id)
    if not anime:
        raise HTTPException(status_code=404, detail="番剧不存在")
    
    # 计算跳过的记录数
    skip = (page - 1) * per_page
    
    # 查询与该动漫相关的下载记录（通过名称匹配）
    query = select(Download).where(
        col(Download.name).like(f"%{anime.title}%")
    )
    
    # 获取总记录数
    total = len(db.exec(query).all())
    
    # 获取分页数据
    query = query.offset(skip).limit(per_page).order_by(col(Download.created_at).desc())
    downloads = db.exec(query).all()
    
    return {
        "items": downloads,
        "total": total
    } 