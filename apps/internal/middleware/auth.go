package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/vort-ads/vort-ads-template/apps/internal/platform/apperrors"
	"github.com/vort-ads/vort-ads-template/apps/internal/platform/response"
	"github.com/vort-ads/vort-ads-template/apps/internal/platform/security"
)

const PrincipalKey = "principal"

func Auth(manager security.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		scheme, token, ok := strings.Cut(c.GetHeader("Authorization"), " ")
		if !ok || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
			abortUnauthorized(c)
			return
		}

		claims, err := manager.VerifyAccessToken(strings.TrimSpace(token))
		if err != nil {
			abortUnauthorized(c)
			return
		}

		c.Set(PrincipalKey, security.Principal{
			UserID:      claims.UserID,
			Email:       claims.Email,
			Roles:       claims.Roles,
			Permissions: claims.Permissions,
		})
		c.Next()
	}
}

func GetPrincipal(c *gin.Context) (security.Principal, bool) {
	value, exists := c.Get(PrincipalKey)
	if !exists {
		return security.Principal{}, false
	}
	principal, ok := value.(security.Principal)
	return principal, ok
}

func abortUnauthorized(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, response.Error(
		GetRequestID(c),
		apperrors.CodeUnauthorized,
		"login required",
	))
}
