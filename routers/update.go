package routers

import (
	"sublink/api"

	"github.com/gin-gonic/gin"
)

func Update(r *gin.Engine, version string) {
	r.GET("/api/v1/update/check", api.CheckUpdate(version))
}
