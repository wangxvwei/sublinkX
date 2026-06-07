package api

import (
	"strconv"
	"sublink/models"

	"github.com/gin-gonic/gin"
)

func XUISourceList(c *gin.Context) {
	sources, err := models.ListXUISources()
	if err != nil {
		c.JSON(400, gin.H{"msg": err.Error()})
		return
	}
	c.JSON(200, gin.H{
		"code": "00000",
		"data": sources,
		"msg":  "x-ui source list",
	})
}

func XUISourceSave(c *gin.Context) {
	var input models.XUISourceInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"msg": err.Error()})
		return
	}
	source, err := models.SaveXUISource(input)
	if err != nil {
		c.JSON(400, gin.H{"msg": err.Error()})
		return
	}
	c.JSON(200, gin.H{
		"code": "00000",
		"data": source,
		"msg":  "x-ui source saved",
	})
}

func XUISourceDelete(c *gin.Context) {
	id, err := strconv.Atoi(c.Query("id"))
	if err != nil || id == 0 {
		c.JSON(400, gin.H{"msg": "invalid source id"})
		return
	}
	if err := models.DeleteXUISource(id); err != nil {
		c.JSON(400, gin.H{"msg": err.Error()})
		return
	}
	c.JSON(200, gin.H{
		"code": "00000",
		"msg":  "x-ui source deleted",
	})
}

func XUISourceSync(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id == 0 {
		c.JSON(400, gin.H{"msg": "invalid source id"})
		return
	}
	result, err := models.SyncXUISourceByID(id)
	if err != nil {
		c.JSON(400, gin.H{"msg": err.Error(), "data": result})
		return
	}
	c.JSON(200, gin.H{
		"code": "00000",
		"data": result,
		"msg":  "x-ui source synced",
	})
}

func XUISourceSyncAll(c *gin.Context) {
	result, err := models.SyncAllEnabledXUISources()
	if err != nil {
		c.JSON(400, gin.H{"msg": err.Error()})
		return
	}
	c.JSON(200, gin.H{
		"code": "00000",
		"data": result,
		"msg":  "x-ui sources synced",
	})
}
