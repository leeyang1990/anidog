package handler

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/anidog/anidog-go/internal/model"
	authsvc "github.com/anidog/anidog-go/internal/service/auth"
	usersvc "github.com/anidog/anidog-go/internal/service/user"
	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	userSvc *usersvc.Service
	authSvc *authsvc.Service
}

func NewUserHandler(userSvc *usersvc.Service, authSvc *authsvc.Service) *UserHandler {
	return &UserHandler{userSvc: userSvc, authSvc: authSvc}
}

func (h *UserHandler) RegisterRoutes(rg *gin.RouterGroup) {
	users := rg.Group("/users")
	users.GET("/me", h.GetMe)
	users.POST("/change-password", h.ChangePassword)
	users.PUT("/password", h.ChangePassword)
	users.GET("/", h.ListUsers)
	users.POST("/", h.CreateUser)
	users.PUT("/:id", h.UpdateUser)
	users.DELETE("/:id", h.DeleteUser)
}

// GetMe 获取当前用户信息
func (h *UserHandler) GetMe(c *gin.Context) {
	username, _ := c.Get("username")
	usernameStr, ok := username.(string)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"detail": "无效的认证凭据"})
		return
	}

	user, err := h.userSvc.GetByUsername(c.Request.Context(), usernameStr)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, toUserResponse(*user))
}

// ChangePassword 修改用户密码
func (h *UserHandler) ChangePassword(c *gin.Context) {
	username, _ := c.Get("username")
	usernameStr, ok := username.(string)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"detail": "无效的认证凭据"})
		return
	}

	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"detail": "请求参数错误: " + err.Error()})
		return
	}

	user, err := h.authSvc.Authenticate(c.Request.Context(), usernameStr, req.OldPassword)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"detail": "旧密码不正确"})
		return
	}

	hashedNew, err := h.authSvc.HashPassword(req.NewPassword)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}

	if err := h.userSvc.ChangePassword(c.Request.Context(), user.ID, req.OldPassword, hashedNew); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"detail": "更新密码失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "密码修改成功"})
}

// ListUsers 获取所有用户列表（管理员权限）
func (h *UserHandler) ListUsers(c *gin.Context) {
	currentUser, err := h.getCurrentUser(c)
	if err != nil || !currentUser.IsAdmin {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"detail": "无权限操作"})
		return
	}

	skip, _ := strconv.Atoi(c.DefaultQuery("skip", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	if limit <= 0 {
		limit = 100
	}

	users, err := h.userSvc.List(c.Request.Context(), skip, limit)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}

	responses := make([]gin.H, 0, len(users))
	for _, u := range users {
		responses = append(responses, toUserResponse(u))
	}
	c.JSON(http.StatusOK, responses)
}

// CreateUser 创建新用户（管理员权限）
func (h *UserHandler) CreateUser(c *gin.Context) {
	currentUser, err := h.getCurrentUser(c)
	if err != nil || !currentUser.IsAdmin {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"detail": "无权限操作"})
		return
	}

	var req struct {
		Username string  `json:"username" binding:"required"`
		Email    *string `json:"email"`
		Password string  `json:"password" binding:"required,min=6"`
		IsAdmin  bool    `json:"is_admin"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"detail": "请求参数错误: " + err.Error()})
		return
	}

	if h.authSvc.IsUsernameTaken(c.Request.Context(), req.Username, 0) {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"detail": "用户名已存在"})
		return
	}

	hashedPassword, err := h.authSvc.HashPassword(req.Password)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}

	email := ""
	if req.Email != nil {
		email = *req.Email
	}
	user, err := h.authSvc.CreateUser(c.Request.Context(), req.Username, email, hashedPassword, req.IsAdmin, true)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, toUserResponse(*user))
}

// UpdateUser 更新用户信息
func (h *UserHandler) UpdateUser(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"detail": "无效的用户ID"})
		return
	}

	currentUser, err := h.getCurrentUser(c)
	if err != nil || (!currentUser.IsAdmin && uint(userID) != currentUser.ID) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"detail": "无权限操作"})
		return
	}

	var req struct {
		Username *string `json:"username"`
		Email    *string `json:"email"`
		Password *string `json:"password"`
		IsAdmin  *bool   `json:"is_admin"`
		IsActive *bool   `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"detail": "请求参数错误: " + err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.Username != nil {
		if h.authSvc.IsUsernameTaken(c.Request.Context(), *req.Username, uint(userID)) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"detail": "用户名已存在"})
			return
		}
		updates["username"] = *req.Username
	}
	if req.Email != nil {
		updates["email"] = *req.Email
	}
	if req.Password != nil && *req.Password != "" {
		hashed, err := h.authSvc.HashPassword(*req.Password)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
			return
		}
		updates["password_hash"] = hashed
	}
	if req.IsAdmin != nil {
		updates["is_admin"] = *req.IsAdmin
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	if len(updates) > 0 {
		user, err := h.userSvc.Update(c.Request.Context(), uint(userID), updates)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
			return
		}
		c.JSON(http.StatusOK, toUserResponse(*user))
		return
	}

	user, err := h.userSvc.GetByID(c.Request.Context(), uint(userID))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, toUserResponse(*user))
}

// DeleteUser 删除用户（管理员权限）
func (h *UserHandler) DeleteUser(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"detail": "无效的用户ID"})
		return
	}

	currentUser, err := h.getCurrentUser(c)
	if err != nil || !currentUser.IsAdmin {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"detail": "无权限操作"})
		return
	}
	if uint(userID) == currentUser.ID {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"detail": "不能删除当前登录用户"})
		return
	}

	user, err := h.userSvc.GetByID(c.Request.Context(), uint(userID))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"detail": err.Error()})
		return
	}

	if err := h.userSvc.Delete(c.Request.Context(), uint(userID)); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, toUserResponse(*user))
}

// getCurrentUser from context
func (h *UserHandler) getCurrentUser(c *gin.Context) (*model.User, error) {
	username, exists := c.Get("username")
	if !exists {
		return nil, fmt.Errorf("未提供用户名")
	}
	usernameStr, ok := username.(string)
	if !ok {
		return nil, fmt.Errorf("无效的用户名")
	}
	return h.userSvc.GetByUsername(c.Request.Context(), usernameStr)
}
