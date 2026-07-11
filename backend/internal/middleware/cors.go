package middleware

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// CORS automatically allows requests from the same public host. The frontend
// reverse proxy passes X-Forwarded-Host, so this works with IP addresses,
// custom ports, domains, and HTTPS without a deployment-specific allowlist.
func CORS() gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOriginWithContextFunc: func(c *gin.Context, origin string) bool {
			parsed, err := url.Parse(origin)
			if err != nil || parsed.Host == "" {
				return false
			}

			requestHost := c.GetHeader("X-Forwarded-Host")
			if requestHost == "" {
				requestHost = c.Request.Host
			}
			return strings.EqualFold(parsed.Host, requestHost)
		},
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch},
		AllowHeaders:     []string{"*"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	})
}
