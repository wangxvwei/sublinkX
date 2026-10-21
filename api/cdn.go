package api

import (
	"net/http"
	"strconv"
	"sublink/models"

	"github.com/gin-gonic/gin"
)

func CDNList(c *gin.Context) {
	var profiles []models.CDNProfile
	if err := models.DB.Order("updated_at desc").Find(&profiles).Error; err != nil {
		c.JSON(500, gin.H{"msg": "读取优选配置失败"})
		return
	}
	_, err := models.CDNExecutable()
	c.JSON(200, gin.H{"code": "00000", "data": gin.H{"profiles": profiles, "engineReady": err == nil}})
}

func CDNSave(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 512*1024)
	var p models.CDNProfile
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(400, gin.H{"msg": "配置格式不正确或内容过长"})
		return
	}
	if err := models.SaveCDNProfile(&p); err != nil {
		c.JSON(400, gin.H{"msg": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": "00000", "data": p})
}

func cdnID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		c.JSON(400, gin.H{"msg": "配置 ID 不正确"})
		return 0, false
	}
	return uint(id), true
}

func CDNRun(c *gin.Context) {
	id, ok := cdnID(c)
	if !ok {
		return
	}
	if err := models.StartCDN(id); err != nil {
		c.JSON(400, gin.H{"msg": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": "00000", "msg": "已开始优选"})
}

func CDNCancel(c *gin.Context) {
	id, ok := cdnID(c)
	if !ok {
		return
	}
	if err := models.CancelCDN(id); err != nil {
		c.JSON(400, gin.H{"msg": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": "00000", "msg": "正在停止任务"})
}

func CDNImport(c *gin.Context) {
	id, ok := cdnID(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8192)
	var input struct {
		IPs            []string `json:"ips"`
		GroupName      string   `json:"groupName"`
		SubscriptionID int      `json:"subscriptionID"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"msg": "保存请求格式不正确"})
		return
	}
	n, err := models.ImportCDN(id, input.IPs, input.GroupName, input.SubscriptionID)
	if err != nil {
		c.JSON(400, gin.H{"msg": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": "00000", "data": n, "msg": "已保存到节点库"})
}
