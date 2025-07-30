from fastapi import APIRouter, Depends, HTTPException, status
from sqlmodel import Session, select
from typing import List
from loguru import logger
from pydantic import BaseModel

from app.core.database import get_db
from app.core.auth import get_current_user, get_password_hash, verify_password
from app.models.user import User, UserCreate, UserUpdate, UserResponse

router = APIRouter()

# 密码修改模型
class PasswordChange(BaseModel):
    old_password: str
    new_password: str

@router.get("/me", response_model=UserResponse)
def read_users_me(current_user: User = Depends(get_current_user)):
    """获取当前用户信息"""
    return current_user

@router.put("/password", status_code=status.HTTP_200_OK)
def change_password(
    password_data: PasswordChange,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user)
):
    """修改用户密码"""
    # 验证旧密码
    if not verify_password(password_data.old_password, current_user.password_hash):
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="旧密码不正确"
        )
    
    # 更新密码
    current_user.password_hash = get_password_hash(password_data.new_password)
    db.add(current_user)
    db.commit()
    logger.info(f"用户 {current_user.username} 修改了密码")
    
    return {"message": "密码修改成功"}

@router.get("/", response_model=List[UserResponse])
def get_users(
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user),
    skip: int = 0,
    limit: int = 100
):
    """获取所有用户信息（需要管理员权限）"""
    if not current_user.is_admin:
        raise HTTPException(status_code=403, detail="无权限操作")
    
    users = db.exec(select(User).offset(skip).limit(limit)).all()
    return users

@router.post("/", response_model=UserResponse)
def create_user(
    user_data: UserCreate,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user)
):
    """创建新用户（需要管理员权限）"""
    if not current_user.is_admin:
        raise HTTPException(status_code=403, detail="无权限操作")
    
    # 检查用户名是否重复
    user_exists = db.exec(select(User).where(User.username == user_data.username)).first()
    if user_exists:
        raise HTTPException(status_code=400, detail="用户名已存在")
    
    user = User(
        username=user_data.username,
        email=user_data.email,
        is_admin=user_data.is_admin,
        is_active=user_data.is_active,
        password_hash=get_password_hash(user_data.password)
    )
    
    db.add(user)
    db.commit()
    db.refresh(user)
    logger.info(f"管理员创建用户: {user.username}")
    
    return user

@router.put("/{user_id}", response_model=UserResponse)
def update_user(
    user_id: int,
    user_data: UserUpdate,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user)
):
    """更新用户信息（需要管理员权限或自己的账户）"""
    if not current_user.is_admin and current_user.id != user_id:
        raise HTTPException(status_code=403, detail="无权限操作")
    
    user = db.get(User, user_id)
    if not user:
        raise HTTPException(status_code=404, detail="用户不存在")
    
    # 更新用户数据
    user_data_dict = user_data.dict(exclude_unset=True)
    if "password" in user_data_dict and user_data_dict["password"]:
        user_data_dict["password_hash"] = get_password_hash(user_data_dict.pop("password"))
    
    for key, value in user_data_dict.items():
        setattr(user, key, value)
    
    db.add(user)
    db.commit()
    db.refresh(user)
    logger.info(f"更新用户信息: {user.username}")
    
    return user

@router.delete("/{user_id}", response_model=UserResponse)
def delete_user(
    user_id: int,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user)
):
    """删除用户（需要管理员权限且不能删除自己）"""
    if not current_user.is_admin:
        raise HTTPException(status_code=403, detail="无权限操作")
    
    if current_user.id == user_id:
        raise HTTPException(status_code=400, detail="不能删除当前登录用户")
    
    user = db.get(User, user_id)
    if not user:
        raise HTTPException(status_code=404, detail="用户不存在")
    
    db.delete(user)
    db.commit()
    logger.info(f"删除用户: {user.username}")
    
    return user 