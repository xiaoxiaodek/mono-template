package middleware

import (
	"net/http"
	"time"

	gintimeout "github.com/gin-contrib/timeout"
	"github.com/gin-gonic/gin"

	"github.com/vort-ads/vort-ads-template/apps/internal/platform/apperrors"
	"github.com/vort-ads/vort-ads-template/apps/internal/platform/response"
)

func Timeout(duration time.Duration) gin.HandlerFunc {
	return gintimeout.New(
		gintimeout.WithTimeout(duration),
		gintimeout.WithResponse(func(c *gin.Context) {
			c.AbortWithStatusJSON(http.StatusGatewayTimeout, response.Error(
				GetRequestID(c),
				apperrors.CodeDependencyError,
				"request timed out",
			))
		}),
	)
}
