package api

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sublink/models"
	"sublink/node"
	"time"

	"github.com/gin-gonic/gin"
)

var subscriptionHTTPClient = &http.Client{Timeout: 15 * time.Second}

func Md5(src string) string {
	m := md5.New()
	m.Write([]byte(src))
	return hex.EncodeToString(m.Sum(nil))
}

func GetClient(c *gin.Context) {
	token := strings.ToLower(strings.TrimSpace(c.Query("token")))
	if token == "" {
		c.String(http.StatusBadRequest, "token is required")
		return
	}

	sub, ok := findSubscriptionByToken(token)
	if !ok {
		c.String(http.StatusNotFound, "subscription not found")
		return
	}

	switch normalizeClient(c.Query("client"), c.GetHeader("User-Agent")) {
	case "clash":
		GetClash(c, sub.Name)
	case "surge":
		GetSurge(c, sub.Name)
	default:
		GetV2ray(c, sub.Name)
	}
}

func findSubscriptionByToken(token string) (models.Subcription, bool) {
	var subModel models.Subcription
	list, err := subModel.List()
	if err != nil {
		log.Println("list subscriptions:", err)
		return models.Subcription{}, false
	}
	for _, sub := range list {
		if subscriptionToken(sub) == token {
			return sub, true
		}
	}
	return models.Subcription{}, false
}

func subscriptionToken(sub models.Subcription) string {
	token := strings.ToLower(strings.TrimSpace(sub.Token))
	if token != "" {
		return token
	}
	return strings.ToLower(Md5(sub.Name))
}

func normalizeClient(queryClient, userAgent string) string {
	client := strings.ToLower(strings.TrimSpace(queryClient))
	switch client {
	case "clash", "mihomo", "clash.meta", "clash-verge", "clash-verge-rev", "verge":
		return "clash"
	case "surge":
		return "surge"
	case "v2ray":
		return "v2ray"
	}

	ua := strings.ToLower(userAgent)
	switch {
	case strings.Contains(ua, "surge"):
		return "surge"
	case strings.Contains(ua, "clash"), strings.Contains(ua, "mihomo"), strings.Contains(ua, "verge"):
		return "clash"
	default:
		return "v2ray"
	}
}

func GetV2ray(c *gin.Context, subName string) {
	sub, err := loadSubscription(subName)
	if err != nil {
		c.String(http.StatusNotFound, "subscription not found: %s", subName)
		return
	}

	links, err := collectNodeLinks(sub.Nodes)
	if err != nil {
		c.String(http.StatusBadGateway, err.Error())
		return
	}

	filename := fmt.Sprintf("%s.txt", sub.Name)
	setInlineFilename(c, filename)
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, node.Base64Encode(strings.Join(links, "\n")+"\n"))
}

func GetClash(c *gin.Context, subName string) {
	sub, err := loadSubscription(subName)
	if err != nil {
		c.String(http.StatusNotFound, "subscription not found: %s", subName)
		return
	}

	links, err := collectNodeLinks(sub.Nodes)
	if err != nil {
		c.String(http.StatusBadGateway, err.Error())
		return
	}

	configs, err := parseSubConfig(sub.Config)
	if err != nil {
		c.String(http.StatusBadRequest, "invalid subscription config: %s", err.Error())
		return
	}

	clashConfig, err := node.EncodeClash(links, configs)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	filename := fmt.Sprintf("%s.yaml", sub.Name)
	setInlineFilename(c, filename)
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, string(clashConfig))
}

func GetSurge(c *gin.Context, subName string) {
	sub, err := loadSubscription(subName)
	if err != nil {
		c.String(http.StatusNotFound, "subscription not found: %s", subName)
		return
	}

	links, err := collectNodeLinks(sub.Nodes)
	if err != nil {
		c.String(http.StatusBadGateway, err.Error())
		return
	}

	configs, err := parseSubConfig(sub.Config)
	if err != nil {
		c.String(http.StatusBadRequest, "invalid subscription config: %s", err.Error())
		return
	}

	surgeConfig, err := node.EncodeSurge(links, configs)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	filename := fmt.Sprintf("%s.conf", sub.Name)
	setInlineFilename(c, filename)
	c.Header("Content-Type", "text/plain; charset=utf-8")

	if strings.Contains(surgeConfig, "#!MANAGED-CONFIG") {
		c.String(http.StatusOK, surgeConfig)
		return
	}
	managedHeader := fmt.Sprintf("#!MANAGED-CONFIG %s%s interval=86400 strict=false", c.Request.Host, c.Request.URL.String())
	c.String(http.StatusOK, managedHeader+"\n"+surgeConfig)
}

func loadSubscription(name string) (models.Subcription, error) {
	sub := models.Subcription{Name: name}
	err := sub.Find()
	return sub, err
}

func parseSubConfig(raw string) (node.SqlConfig, error) {
	config := node.SqlConfig{
		Clash: "./template/clash.yaml",
		Surge: "./template/surge.conf",
	}
	if strings.TrimSpace(raw) == "" {
		return config, nil
	}
	err := json.Unmarshal([]byte(raw), &config)
	return config, err
}

func collectNodeLinks(nodes []models.Node) ([]string, error) {
	links := make([]string, 0, len(nodes))
	for _, item := range nodes {
		nodeLinks, err := expandNodeLink(item.SubscriptionLink())
		if err != nil {
			return nil, err
		}
		links = append(links, nodeLinks...)
	}
	return links, nil
}

func expandNodeLink(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		body, err := fetchRemoteSubscription(raw)
		if err != nil {
			return nil, err
		}
		return splitLinks(node.Base64Decode(body)), nil
	}
	return splitLinks(raw), nil
}

func fetchRemoteSubscription(link string) (string, error) {
	resp, err := subscriptionHTTPClient.Get(link)
	if err != nil {
		return "", fmt.Errorf("fetch remote subscription failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("fetch remote subscription failed: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read remote subscription failed: %w", err)
	}
	return string(body), nil
}

func splitLinks(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '\n' || r == '\r' || r == ','
	})
	links := make([]string, 0, len(fields))
	for _, field := range fields {
		if link := strings.TrimSpace(field); link != "" {
			links = append(links, link)
		}
	}
	return links
}

func setInlineFilename(c *gin.Context, filename string) {
	encodedFilename := url.QueryEscape(filename)
	c.Header("Content-Disposition", "inline; filename*=utf-8''"+encodedFilename)
}
