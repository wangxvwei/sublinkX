package routers

import (
	"sublink/api"

	"github.com/gin-gonic/gin"
)

func XUISources(r *gin.Engine) {
	group := r.Group("/api/v1/xui-sources")
	{
		group.GET("/get", api.XUISourceList)
		group.POST("/save", api.XUISourceSave)
		group.DELETE("/delete", api.XUISourceDelete)
		group.POST("/:id/sync", api.XUISourceSync)
		group.POST("/sync-all", api.XUISourceSyncAll)
	}
}
