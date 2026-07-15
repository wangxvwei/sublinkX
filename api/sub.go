// api/subcription.go

package api

import (
	// 导入 json 包，用于解析 config 字符串

	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"sublink/models" // 导入 models 包

	"github.com/gin-gonic/gin"
)

func SubTotal(c *gin.Context) {
	var Sub models.Subcription
	subs, err := Sub.List()
	count := len(subs)
	if err != nil {
		c.JSON(500, gin.H{
			"msg": "取得订阅总数失败",
		})
		return
	}
	c.JSON(200, gin.H{
		"code": "00000",
		"data": count,
		"msg":  "取得订阅总数",
	})
}

// 获取订阅列表
func SubGet(c *gin.Context) {
	var Sub models.Subcription
	Subs, err := Sub.List()
	if err != nil {
		c.JSON(500, gin.H{
			"msg": "node list error",
		})
		return
	}
	c.JSON(200, gin.H{
		"code": "00000",
		"data": Subs,
		"msg":  "node get",
	})
}

// 添加订阅
func SubAdd(c *gin.Context) {
	name := c.PostForm("name")
	token := c.PostForm("token")
	configs := c.PostForm("config") // 这里的 configString 是前端传来的 JSON 字符串
	nodes := c.PostForm("nodes")
	nodeIDs := c.PostForm("nodeIds")

	if name == "" || (nodes == "" && nodeIDs == "") {
		c.JSON(400, gin.H{
			"msg": "订阅名称或节点不能为空",
		})
		return
	}
	normalizedToken, err := normalizeSubscriptionToken(name, token)
	if err != nil {
		c.JSON(400, gin.H{"msg": err.Error()})
		return
	}
	if err := ensureSubscriptionTokenAvailable(normalizedToken, 0); err != nil {
		c.JSON(400, gin.H{"msg": err.Error()})
		return
	}

	NodesData, normalizedNodeIDs, err := loadSubscriptionNodes(nodeIDs, nodes)
	if err != nil {
		log.Println(err)
		c.JSON(400, gin.H{
			"msg": err.Error(),
		})
		return
	}
	sub := models.Subcription{
		Name:         name,
		Token:        normalizedToken,
		Config:       configs,
		NodeOrder:    nodes,
		NodeOrderIDs: normalizedNodeIDs,
		Nodes:        NodesData,
	}
	err = sub.Add()
	if err != nil {
		c.JSON(400, gin.H{
			"msg": "添加订阅失败: " + err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"code": "00000",
		"msg":  "添加订阅成功",
	})
}

// 更新订阅
func SubUpdate(c *gin.Context) {
	NewName := c.PostForm("name")
	OldName := c.PostForm("oldname")
	token := c.PostForm("token")
	configs := c.PostForm("config") // 这里的 configString 是前端传来的 JSON 字符串
	nodes := c.PostForm("nodes")
	nodeIDs := c.PostForm("nodeIds")

	if NewName == "" || (nodes == "" && nodeIDs == "") {
		c.JSON(400, gin.H{
			"msg": "订阅名称或节点不能为空",
		})
		return
	}
	OldSub := models.Subcription{
		Name: OldName,
	}
	if err := OldSub.Find(); err != nil {
		c.JSON(400, gin.H{
			"msg": "查找订阅失败: " + err.Error(),
		})
		return
	}
	if strings.TrimSpace(token) == "" {
		token = OldSub.Token
		if strings.TrimSpace(token) == "" {
			token = Md5(OldSub.Name)
		}
	}
	normalizedToken, err := normalizeSubscriptionToken(NewName, token)
	if err != nil {
		c.JSON(400, gin.H{"msg": err.Error()})
		return
	}
	if err := ensureSubscriptionTokenAvailable(normalizedToken, OldSub.ID); err != nil {
		c.JSON(400, gin.H{"msg": err.Error()})
		return
	}

	NodesData, normalizedNodeIDs, err := loadSubscriptionNodes(nodeIDs, nodes)
	if err != nil {
		log.Println(err)
		c.JSON(400, gin.H{
			"msg": err.Error(),
		})
		return
	}
	NewSub := models.Subcription{
		Name:         NewName,
		Token:        normalizedToken,
		Config:       configs,
		NodeOrder:    nodes,
		NodeOrderIDs: normalizedNodeIDs,
		Nodes:        NodesData,
	}

	err = OldSub.Update(&NewSub)
	if err != nil {
		c.JSON(400, gin.H{
			"msg": "更新订阅失败: " + err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"code": "00000",
		"msg":  "更新订阅成功",
	})
}

func loadSubscriptionNodes(nodeIDs, nodeNames string) ([]models.Node, string, error) {
	var result []models.Node
	var normalizedIDs []string
	seen := map[int]bool{}

	for _, rawID := range strings.Split(nodeIDs, ",") {
		rawID = strings.TrimSpace(rawID)
		if rawID == "" {
			continue
		}
		id, err := strconv.Atoi(rawID)
		if err != nil || id <= 0 {
			return nil, "", fmt.Errorf("无效的节点 ID: %s", rawID)
		}
		if seen[id] {
			continue
		}
		var item models.Node
		if err := models.DB.First(&item, id).Error; err != nil {
			return nil, "", fmt.Errorf("查找节点 %d 失败: %w", id, err)
		}
		result = append(result, item)
		normalizedIDs = append(normalizedIDs, strconv.Itoa(item.ID))
		seen[id] = true
	}

	if len(result) > 0 {
		return result, strings.Join(normalizedIDs, ","), nil
	}

	for _, nodeName := range strings.Split(nodeNames, ",") {
		nodeName = strings.TrimSpace(nodeName)
		if nodeName == "" {
			continue
		}
		var item models.Node
		if err := models.DB.Where("name = ?", nodeName).First(&item).Error; err != nil {
			return nil, "", fmt.Errorf("查找节点 %s 失败: %w", nodeName, err)
		}
		if seen[item.ID] {
			continue
		}
		result = append(result, item)
		normalizedIDs = append(normalizedIDs, strconv.Itoa(item.ID))
		seen[item.ID] = true
	}

	if len(result) == 0 {
		return nil, "", fmt.Errorf("订阅至少需要一个有效节点")
	}
	return result, strings.Join(normalizedIDs, ","), nil
}

func normalizeSubscriptionToken(name, token string) (string, error) {
	token = strings.ToLower(strings.TrimSpace(token))
	if token == "" {
		token = randomSubscriptionToken()
	}
	if token == "" {
		token = Md5(name)
	}
	if len(token) < 6 || len(token) > 64 {
		return "", fmt.Errorf("订阅链接标识长度需要在 6 到 64 位之间")
	}
	if !regexp.MustCompile(`^[a-z0-9_-]+$`).MatchString(token) {
		return "", fmt.Errorf("订阅链接标识只能包含小写字母、数字、下划线和短横线")
	}
	return token, nil
}

func randomSubscriptionToken() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	return hex.EncodeToString(buf)
}

func ensureSubscriptionTokenAvailable(token string, currentID int) error {
	var subModel models.Subcription
	list, err := subModel.List()
	if err != nil {
		return err
	}
	for _, sub := range list {
		if sub.ID == currentID {
			continue
		}
		existingToken := subscriptionToken(sub)
		if strings.EqualFold(existingToken, token) {
			return fmt.Errorf("订阅链接标识已被「%s」使用", sub.Name)
		}
	}
	return nil
}

// 删除订阅 (无需修改)
func SubDel(c *gin.Context) {
	var sub models.Subcription
	id := c.Query("id")
	if id == "" {
		c.JSON(400, gin.H{
			"msg": "id 不能为空",
		})
		return
	}
	x, err := strconv.Atoi(id) // 增加错误检查
	if err != nil {
		c.JSON(400, gin.H{
			"msg": "无效的 ID: " + err.Error(),
		})
		return
	}
	sub.ID = x
	err = sub.Find()
	if err != nil {
		c.JSON(400, gin.H{
			"msg": "查找订阅失败: " + err.Error(),
		})
		return
	}
	err = sub.Del()
	if err != nil {
		c.JSON(400, gin.H{
			"msg": "删除订阅失败: " + err.Error(),
		})
		return
	}
	c.JSON(200, gin.H{
		"code": "00000",
		"msg":  "删除订阅成功",
	})
}
