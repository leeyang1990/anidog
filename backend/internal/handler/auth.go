package handler

import (
	"errors"
	"net/http"

	authsvc "github.com/anidog/anidog-go/internal/service/auth"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authSvc *authsvc.Service
}

func NewAuthHandler(authSvc *authsvc.Service) *AuthHandler {
	return &AuthHandler{authSvc: authSvc}
}

func (h *AuthHandler) RegisterRoutes(rg *gin.RouterGroup) {
	auth := rg.Group("/auth")
	auth.POST("/login", h.Login)
	auth.POST("/refresh", h.Refresh)
	auth.POST("/register", h.Register)
}

func (h *AuthHandler) Login(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")
	if username == "" || password == "" {
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{"detail": "用户名和密码不能为空"})
		return
	}

	user, err := h.authSvc.Authenticate(c.Request.Context(), username, password)
	if err != nil {
		status := http.StatusUnauthorized
		if errors.Is(err, authsvc.ErrUserDisabled) {
			status = http.StatusBadRequest
		}
		c.AbortWithStatusJSON(status, gin.H{"detail": err.Error()})
		return
	}

	accessToken, refreshToken, _ := h.authSvc.CreateTokenPair(user.Username)
	c.JSON(http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"token_type":    "bearer",
	})
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	username, _ := c.Get("username")
	usernameStr, ok := username.(string)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"detail": "无效的认证凭据"})
		return
	}

	user, err := h.authSvc.ValidateUserActive(c.Request.Context(), usernameStr)
	if err != nil {
		status := http.StatusUnauthorized
		if errors.Is(err, authsvc.ErrUserDisabled) {
			status = http.StatusBadRequest
		}
		c.AbortWithStatusJSON(status, gin.H{"detail": err.Error()})
		return
	}

	accessToken, refreshToken, _ := h.authSvc.CreateTokenPair(user.Username)
	c.JSON(http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"token_type":    "bearer",
	})
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req struct {
		Username string  `json:"username" binding:"required"`
		Email    *string `json:"email"`
		Password string  `json:"password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"detail": "请求参数错误: " + err.Error()})
		return
	}

	hasUsers := h.authSvc.HasAnyUsers(c.Request.Context())
	if hasUsers && h.authSvc.HasAdmin(c.Request.Context()) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"detail": "管理员账户已存在，仅允许管理员创建新用户"})
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

	isAdmin := !hasUsers
	email := ""
	if req.Email != nil {
		email = *req.Email
	}
	user, err := h.authSvc.CreateUser(c.Request.Context(), req.Username, email, hashedPassword, isAdmin, true)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, toUserResponse(*user))
}
