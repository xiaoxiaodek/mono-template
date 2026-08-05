package observability

import (
	httppprof "net/http/pprof"
	runtimepprof "runtime/pprof"

	"github.com/gin-gonic/gin"
)

// RegisterPprofRoutes registers the standard net/http/pprof endpoints below
// the supplied engine or router group. Disabled diagnostics add no routes.
func RegisterPprofRoutes(router gin.IRouter, enabled bool) {
	if !enabled {
		return
	}

	group := router.Group("/debug/pprof")
	group.GET("/", gin.WrapF(httppprof.Index))
	group.GET("/cmdline", gin.WrapF(httppprof.Cmdline))
	group.GET("/profile", gin.WrapF(httppprof.Profile))
	group.GET("/symbol", gin.WrapF(httppprof.Symbol))
	group.POST("/symbol", gin.WrapF(httppprof.Symbol))
	group.GET("/trace", gin.WrapF(httppprof.Trace))

	for _, profile := range runtimepprof.Profiles() {
		name := profile.Name()
		group.GET("/"+name, gin.WrapH(httppprof.Handler(name)))
	}
}
