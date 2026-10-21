package routers

import (
	"sublink/api"

	"github.com/gin-gonic/gin"
)

func Nodes(r *gin.Engine) {
	NodesGroup := r.Group("/api/v1/nodes")
	{
		NodesGroup.POST("/add", api.NodeAdd)
		NodesGroup.DELETE("/delete", api.NodeDel)
		NodesGroup.GET("/get", api.NodeGet)
		NodesGroup.POST("/update", api.NodeUpdadte)
		NodesGroup.POST("/sync/xui", api.NodeSyncXUI)
		NodesGroup.GET("/cdn", api.CDNList)
		NodesGroup.POST("/cdn/save", api.CDNSave)
		NodesGroup.POST("/cdn/:id/run", api.CDNRun)
		NodesGroup.POST("/cdn/:id/cancel", api.CDNCancel)
		NodesGroup.POST("/cdn/:id/import", api.CDNImport)

	}
	// 分组
	Group := NodesGroup.Group("/group")
	{
		Group.GET("/get", api.GroupNodeGet)  // 添加分组
		Group.POST("/set", api.GroupNodeSet) // 绑定创建分组
		// Group.DELETE("/delete", api.GroupNodeDel) // 删除分组
		// Group.POST("/update", api.GroupNodeUpdate) // 更新分组
	}
}
