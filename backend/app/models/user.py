from typing import Optional
from datetime import datetime
from sqlmodel import Field, SQLModel

class UserBase(SQLModel):
    """用户基本信息模型"""
    username: str = Field(index=True, unique=True)
    email: Optional[str] = Field(default=None, index=True)
    is_admin: bool = Field(default=False)
    is_active: bool = Field(default=True)

class User(UserBase, table=True):
    """用户数据库模型"""
    id: Optional[int] = Field(default=None, primary_key=True)
    password_hash: str
    created_at: datetime = Field(default_factory=datetime.utcnow)
    updated_at: datetime = Field(default_factory=datetime.utcnow)

class UserCreate(UserBase):
    """创建用户的输入模型"""
    password: str

class UserUpdate(SQLModel):
    """更新用户的输入模型"""
    username: Optional[str] = None
    email: Optional[str] = None
    password: Optional[str] = None
    is_admin: Optional[bool] = None
    is_active: Optional[bool] = None

class UserResponse(UserBase):
    """用户信息响应模型"""
    id: int
    created_at: datetime 