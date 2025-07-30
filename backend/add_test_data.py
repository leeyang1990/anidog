#!/usr/bin/env python3
"""
添加测试动漫数据的脚本
"""

from sqlmodel import Session, select
from app.core.database import engine, init_db
from app.models.anime import Anime, AnimeStatus
from loguru import logger

def add_test_animes():
    """添加测试动漫数据"""
    # 先初始化数据库
    init_db()
    
    test_animes = [
        {
            "title": "进击的巨人 最终季",
            "original_title": "Shingeki no Kyojin: The Final Season",
            "description": "人类与巨人的最终决战",
            "status": AnimeStatus.FINISHED,
            "season": 4,
            "year": 2023,
            "cover_url": "https://cdn.myanimelist.net/images/anime/1948/120625.jpg",
            "episode_count": 16,
            "current_episode": 16
        },
        {
            "title": "鬼灭之刃 刀匠村篇",
            "original_title": "Kimetsu no Yaiba: Katanakaji no Sato-hen",
            "description": "炭治郎前往刀匠村的新冒险",
            "status": AnimeStatus.FINISHED,
            "season": 3,
            "year": 2023,
            "cover_url": "https://cdn.myanimelist.net/images/anime/1765/135099.jpg",
            "episode_count": 11,
            "current_episode": 11
        },
        {
            "title": "间谍过家家",
            "original_title": "Spy x Family",
            "description": "伪装家庭的温馨喜剧",
            "status": AnimeStatus.FINISHED,
            "season": 1,
            "year": 2022,
            "cover_url": "https://cdn.myanimelist.net/images/anime/1441/122795.jpg",
            "episode_count": 12,
            "current_episode": 12
        },
        {
            "title": "咒术回战 第二季",
            "original_title": "Jujutsu Kaisen Season 2",
            "description": "五条悟的过去与涉谷事变",
            "status": AnimeStatus.FINISHED,
            "season": 2,
            "year": 2023,
            "cover_url": "https://cdn.myanimelist.net/images/anime/1792/138022.jpg",
            "episode_count": 23,
            "current_episode": 23
        },
        {
            "title": "葬送的芙莉莲",
            "original_title": "Sousou no Frieren",
            "description": "精灵法师的千年之旅",
            "status": AnimeStatus.ONGOING,
            "season": 1,
            "year": 2023,
            "cover_url": "https://cdn.myanimelist.net/images/anime/1015/138006.jpg",
            "episode_count": 28,
            "current_episode": 28
        },
        {
            "title": "药师少女的独白",
            "original_title": "Kusuriya no Hitorigoto",
            "description": "宫廷药师的推理故事",
            "status": AnimeStatus.ONGOING,
            "season": 1,
            "year": 2023,
            "cover_url": "https://cdn.myanimelist.net/images/anime/1708/138033.jpg",
            "episode_count": 24,
            "current_episode": 24
        }
    ]
    
    with Session(engine) as session:
        # 检查是否已有数据
        existing_count = len(session.exec(select(Anime)).all())
        if existing_count > 0:
            logger.info(f"数据库中已有 {existing_count} 条动漫数据，跳过添加")
            return
        
        # 添加测试数据
        for anime_data in test_animes:
            anime = Anime(**anime_data)
            session.add(anime)
        
        session.commit()
        logger.info(f"成功添加 {len(test_animes)} 条测试动漫数据")

if __name__ == "__main__":
    add_test_animes() 