from agno.agent import Agent
from agno.models.openrouter import OpenRouter
from agno.tools.duckduckgo import DuckDuckGoTools
from agno.tools.crawl4ai import Crawl4aiTools
from loguru import logger

from app.core.config import settings

# 初始化 OpenRouter 模型
try:
    model = OpenRouter(id="openrouter/optimus-alpha", api_key=settings.OPENROUTER_API_KEY)
    logger.info("成功初始化 OpenRouter 模型")
except Exception as e:
    logger.error(f"初始化 OpenRouter 模型失败: {e}")
    raise

# 创建搜索代理
search_agent = Agent(
    name="AnimeSearcher",
    model=model,
    tools=[DuckDuckGoTools(search=True, news=False)],
    add_history_to_messages=True,
    num_history_responses=3,
    description="专门用于搜索动漫相关信息的代理",
    instructions=[
        "使用 duckduckgo_search 进行动漫相关的网络搜索",
        "优先搜索日本动画、新番信息、动漫资讯等内容",
        "返回结果时需要包含信息来源的 URL",
        "重点关注最新的动漫信息"
    ],
    add_datetime_to_instructions=True,
    markdown=True,
    exponential_backoff=True
)

# 创建网站爬虫代理
crawler_agent = Agent(
    name="AnimeCrawler",
    model=model,
    tools=[Crawl4aiTools(max_length=None)],
    add_history_to_messages=True,
    num_history_responses=3,
    description="专门用于爬取动漫网站内容的代理",
    instructions=[
        "使用 web_crawler 提取动漫网站的内容",
        "重点关注新番列表、动漫介绍、播放时间等信息",
        "提取内容时需要包含页面 URL",
        "对提取的内容进行结构化整理"
    ],
    markdown=True,
    exponential_backoff=True
)

# 导出代理实例
__all__ = ["search_agent", "crawler_agent"] 