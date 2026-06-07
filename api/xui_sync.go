package api

import (
	"strings"
	"sublink/models"

	"github.com/gin-gonic/gin"
)

func NodeSyncXUI(c *gin.Context) {
	result, err := models.SyncXUINodes(models.XUISyncOptions{
		XUIDBPath:           c.PostForm("xui_db_path"),
		SubscriptionBaseURL: c.PostForm("subscription_base_url"),
		SubscriptionPath:    c.PostForm("subscription_path"),
		GroupName:           c.PostForm("group"),
		NamePrefix:          c.PostForm("name_prefix"),
		DeleteMissing:       strings.EqualFold(c.PostForm("delete_missing"), "true"),
	})
	if err != nil {
		c.JSON(400, gin.H{
			"msg": err.Error(),
		})
		return
	}
	c.JSON(200, gin.H{
		"code": "00000",
		"data": result,
		"msg":  "x-ui sync complete",
	})
}
