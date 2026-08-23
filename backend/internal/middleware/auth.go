package middleware

import (
	"net/http"
	"strings"

	"github.com/anidog/anidog-go/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// AuthMiddleware JWT 认证中间件
func AuthMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// 跳过不需要认证的路径
		if isPublicPath(path) {
			c.Next()
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"detail": "未提供认证凭据"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"detail": "无效的认证格式"})
			return
		}

		tokenStr := parts[1]
		token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(cfg.SecretKey), nil
		}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithExpirationRequired())

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"detail": "无效的认证凭据"})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"detail": "无效的认证凭据"})
			return
		}
		tokenType, _ := claims["type"].(string)
		// 兼容升级前没有 type 字段的 access token；refresh token 绝不能
		// 被当作 7 天有效的普通访问令牌使用。
		if tokenType == "refresh" || (tokenType != "" && tokenType != "access") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"detail": "无效的认证凭据"})
			return
		}

		username, _ := claims["sub"].(string)
		if username == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"detail": "无效的认证凭据"})
			return
		}
		c.Set("username", username)
		c.Set("token", tokenStr)
		c.Next()
	}
}

func isPublicPath(path string) bool {
	publicPrefixes := []string{
		"/api/v1/auth/login",
		"/api/v1/auth/refresh",
		"/api/v1/auth/register",
		"/api/v1/default-rules",
		"/api/v1/calendar",
		"/ws/",
		"/healthcheck",
	}
	for _, prefix := range publicPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}
