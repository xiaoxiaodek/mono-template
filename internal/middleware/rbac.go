package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/vort-ads/vort-ads-template/internal/platform/apperrors"
	"github.com/vort-ads/vort-ads-template/internal/platform/response"
)

func RequirePermissions(required ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, ok := GetPrincipal(c)
		if !ok || !hasEveryPermission(principal.Permissions, required) {
			c.AbortWithStatusJSON(http.StatusForbidden, response.Error(
				GetRequestID(c),
				apperrors.CodeForbidden,
				"permission denied",
			))
			return
		}
		c.Next()
	}
}

func hasEveryPermission(granted []string, required []string) bool {
	permissions := make(map[string]struct{}, len(granted))
	for _, permission := range granted {
		permissions[permission] = struct{}{}
	}
	for _, permission := range required {
		if _, ok := permissions[permission]; !ok {
			return false
		}
	}
	return true
}
