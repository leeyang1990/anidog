from fastapi import APIRouter, Depends, HTTPException, status, BackgroundTasks
from sqlmodel import Session, select
from typing import List
import feedparser
from datetime import datetime
import re
from loguru import logger

from app.core.database import get_db
from app.core.auth import get_current_user
from app.models.user import User
from app.models.rss import (
    RSSFeed, RSSRule, RSSEntry, 
    RSSFeedCreate, RSSFeedUpdate,
    RSSRuleCreate, RSSRuleUpdate
)

router = APIRouter()

@router.get("/")
def get_feeds(
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user),
    page: int = 1,
    per_page: int = 10
):
    """获取所有RSS订阅源"""
    # 计算跳过的记录数
    skip = (page - 1) * per_page
    
    # 获取总记录数
    total = len(db.exec(select(RSSFeed)).all())
    
    # 获取分页数据
    feeds = db.exec(select(RSSFeed).offset(skip).limit(per_page)).all()
    
    return {
        "items": feeds,
        "total": total
    }

@router.get("/feeds/{feed_id}", response_model=RSSFeed)
def get_feed(
    feed_id: int,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user)
):
    """获取指定RSS订阅源信息"""
    feed = db.get(RSSFeed, feed_id)
    if not feed:
        raise HTTPException(status_code=404, detail="订阅源不存在")
    return feed

@router.post("/feeds", response_model=RSSFeed)
def create_feed(
    feed_data: RSSFeedCreate,
    background_tasks: BackgroundTasks,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user)
):
    """创建新的RSS订阅源"""
    # 检查URL是否已存在
    feed_exists = db.exec(select(RSSFeed).where(RSSFeed.url == feed_data.url)).first()
    if feed_exists:
        raise HTTPException(status_code=400, detail="该RSS URL已存在")
    
    # 验证RSS源是否可访问
    try:
        parsed_feed = feedparser.parse(feed_data.url)
        if hasattr(parsed_feed, 'bozo_exception') and parsed_feed.bozo_exception:
            raise HTTPException(status_code=400, detail=f"无效的RSS源: {parsed_feed.bozo_exception}")
    except Exception as e:
        raise HTTPException(status_code=400, detail=f"无法访问RSS源: {str(e)}")
    
    # 创建RSS源
    feed = RSSFeed(**feed_data.dict())
    
    db.add(feed)
    db.commit()
    db.refresh(feed)
    
    # 后台任务，检查RSS条目
    background_tasks.add_task(check_rss_feed, feed.id, db)
    
    return feed

@router.put("/feeds/{feed_id}", response_model=RSSFeed)
def update_feed(
    feed_id: int,
    feed_data: RSSFeedUpdate,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user)
):
    """更新RSS订阅源"""
    feed = db.get(RSSFeed, feed_id)
    if not feed:
        raise HTTPException(status_code=404, detail="订阅源不存在")
    
    # 更新数据
    feed_data_dict = feed_data.dict(exclude_unset=True)
    for key, value in feed_data_dict.items():
        setattr(feed, key, value)
    
    feed.updated_at = datetime.utcnow()
    db.add(feed)
    db.commit()
    db.refresh(feed)
    
    return feed

@router.delete("/feeds/{feed_id}", response_model=RSSFeed)
def delete_feed(
    feed_id: int,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user)
):
    """删除RSS订阅源"""
    feed = db.get(RSSFeed, feed_id)
    if not feed:
        raise HTTPException(status_code=404, detail="订阅源不存在")
    
    # 删除相关的规则和条目
    db.exec(select(RSSRule).where(RSSRule.rss_feed_id == feed_id))
    db.exec(select(RSSEntry).where(RSSEntry.rss_feed_id == feed_id))
    
    db.delete(feed)
    db.commit()
    
    return feed

@router.get("/feeds/{feed_id}/rules", response_model=List[RSSRule])
def get_feed_rules(
    feed_id: int,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user)
):
    """获取指定订阅源的所有规则"""
    feed = db.get(RSSFeed, feed_id)
    if not feed:
        raise HTTPException(status_code=404, detail="订阅源不存在")
    
    rules = db.exec(select(RSSRule).where(RSSRule.rss_feed_id == feed_id)).all()
    return rules

@router.post("/feeds/{feed_id}/rules", response_model=RSSRule)
def create_feed_rule(
    feed_id: int,
    rule_data: RSSRuleCreate,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user)
):
    """创建订阅源过滤规则"""
    feed = db.get(RSSFeed, feed_id)
    if not feed:
        raise HTTPException(status_code=404, detail="订阅源不存在")
    
    # 如果是正则表达式，验证其有效性
    if rule_data.is_regex:
        try:
            re.compile(rule_data.keyword)
        except re.error:
            raise HTTPException(status_code=400, detail="无效的正则表达式")
    
    rule = RSSRule(**rule_data.dict(), rss_feed_id=feed_id)
    
    db.add(rule)
    db.commit()
    db.refresh(rule)
    
    return rule

@router.delete("/rules/{rule_id}", response_model=RSSRule)
def delete_rule(
    rule_id: int,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user)
):
    """删除订阅源规则"""
    rule = db.get(RSSRule, rule_id)
    if not rule:
        raise HTTPException(status_code=404, detail="规则不存在")
    
    db.delete(rule)
    db.commit()
    
    return rule

@router.post("/feeds/{feed_id}/check", response_model=dict)
def manual_check_feed(
    feed_id: int,
    background_tasks: BackgroundTasks,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user)
):
    """手动检查RSS订阅源更新"""
    feed = db.get(RSSFeed, feed_id)
    if not feed:
        raise HTTPException(status_code=404, detail="订阅源不存在")
    
    # 后台任务检查RSS
    background_tasks.add_task(check_rss_feed, feed_id, db)
    
    return {"status": "success", "message": "RSS检查任务已启动"}

def check_rss_feed(feed_id: int, db: Session):
    """后台任务：检查RSS源更新"""
    with Session(db.engine) as session:
        feed = session.get(RSSFeed, feed_id)
        if not feed:
            logger.error(f"RSS订阅源不存在: ID={feed_id}")
            return
        
        logger.info(f"检查RSS订阅源: {feed.name} ({feed.url})")
        
        try:
            # 解析RSS源
            parsed_feed = feedparser.parse(feed.url)
            
            # 获取该源的所有规则
            rules = session.exec(select(RSSRule).where(RSSRule.rss_feed_id == feed_id)).all()
            
            # 获取已处理的条目ID
            processed_entries = session.exec(
                select(RSSEntry.entry_id).where(RSSEntry.rss_feed_id == feed_id)
            ).all()
            processed_ids = [entry[0] for entry in processed_entries]
            
            # 处理新条目
            new_entries_count = 0
            for entry in parsed_feed.entries:
                # 提取条目ID
                entry_id = getattr(entry, 'id', entry.link)
                
                # 检查是否已处理过
                if entry_id in processed_ids:
                    continue
                
                # 检查标题是否匹配规则
                title = entry.title
                if match_rules(title, rules):
                    logger.info(f"发现新匹配条目: {title}")
                    # TODO: 处理匹配的条目（添加到下载队列等）
                    new_entries_count += 1
                
                # 记录已处理条目
                db_entry = RSSEntry(
                    entry_id=entry_id,
                    title=title,
                    link=entry.link,
                    published=datetime(*entry.published_parsed[:6]) if hasattr(entry, 'published_parsed') else None,
                    rss_feed_id=feed_id
                )
                session.add(db_entry)
            
            # 更新RSS源最后检查时间
            feed.last_check = datetime.utcnow()
            session.add(feed)
            session.commit()
            
            logger.info(f"RSS检查完成: {feed.name}, 发现{new_entries_count}个新匹配条目")
            
        except Exception as e:
            logger.error(f"检查RSS源时出错: {feed.name}, {str(e)}")

def match_rules(title: str, rules: List[RSSRule]) -> bool:
    """检查标题是否匹配规则"""
    if not rules:
        return True  # 如果没有规则，默认匹配所有
    
    matched = False
    
    # 首先检查包含规则
    include_rules = [r for r in rules if r.include and r.enabled]
    exclude_rules = [r for r in rules if not r.include and r.enabled]
    
    # 如果有包含规则，则必须匹配至少一个
    if include_rules:
        for rule in include_rules:
            if rule.is_regex:
                pattern = re.compile(rule.keyword)
                if pattern.search(title):
                    matched = True
                    break
            else:
                if rule.keyword in title:
                    matched = True
                    break
    else:
        # 如果没有包含规则，默认匹配
        matched = True
    
    # 如果有排除规则且已经匹配了包含规则，则检查是否应该排除
    if matched and exclude_rules:
        for rule in exclude_rules:
            if rule.is_regex:
                pattern = re.compile(rule.keyword)
                if pattern.search(title):
                    matched = False
                    break
            else:
                if rule.keyword in title:
                    matched = False
                    break
    
    return matched 