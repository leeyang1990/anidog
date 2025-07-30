from datetime import timedelta
from fastapi import APIRouter, Depends, HTTPException, status
from fastapi.security import OAuth2PasswordRequestForm
from sqlmodel import Session, select
from loguru import logger

from app.core.database import get_db
from app.core.auth import authenticate_user, create_access_token, get_password_hash
from app.core.config import settings
from app.models.user import User, UserCreate, UserResponse

router = APIRouter()

@router.post("/login", response_model=dict)
def login(form_data: OAuth2PasswordRequestForm = Depends(), db: Session = Depends(get_db)):
    """用户登录接口"""
    user = authenticate_user(db, form_data.username, form_data.password)
    if not user:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="用户名或密码错误",
            headers={"WWW-Authenticate": "Bearer"},
        )
    if not user.is_active:
        raise HTTPException(status_code=400, detail="账户已被禁用")
    
    # 创建访问令牌
    access_token_expires = timedelta(minutes=settings.ACCESS_TOKEN_EXPIRE_MINUTES)
    access_token = create_access_token(
        data={"sub": user.username}, expires_delta=access_token_expires
    )
    return {"access_token": access_token, "token_type": "bearer"}

@router.post("/register", response_model=UserResponse)
def register(user_data: UserCreate, db: Session = Depends(get_db)):
    """注册新用户（初始安装时使用）"""
    # 检查是否已有用户存在
    user_exists = db.exec(select(User)).first()
    if user_exists:
        # 如果已有用户存在，则后续注册必须是管理员
        admin_exists = db.exec(select(User).where(User.is_admin == True)).first()
        if admin_exists:
            raise HTTPException(
                status_code=status.HTTP_403_FORBIDDEN,
                detail="管理员账户已存在，仅允许管理员创建新用户"
            )
    
    # 检查用户名是否重复
    user_check = db.exec(select(User).where(User.username == user_data.username)).first()
    if user_check:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="用户名已存在"
        )
    
    # 创建新用户
    user = User(
        username=user_data.username,
        email=user_data.email,
        password_hash=get_password_hash(user_data.password),
        is_admin=True if not user_exists else False,  # 第一个用户自动设为管理员
    )
    
    db.add(user)
    db.commit()
    db.refresh(user)
    logger.info(f"创建用户: {user.username}")
    
    return user 