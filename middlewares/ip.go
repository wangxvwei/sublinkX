package middlewares

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"sublink/models"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/text/encoding/simplifiedchinese"
)

var ipInfoClient = &http.Client{Timeout: 3 * time.Second}
var ipInfoSlots = make(chan struct{}, 4)
var ipInfoPending sync.Map
var ipAPILimitMu sync.Mutex
var ipAPIBlockedUntil time.Time

func GetIp(c *gin.Context) {
	c.Next()
	id, ok := c.Get("subscriptionID")
	if !ok || c.Writer.Status() < 200 || c.Writer.Status() >= 300 {
		return
	}
	ip, err := netip.ParseAddr(c.ClientIP())
	if err != nil {
		return
	}
	ua := []rune(c.GetHeader("User-Agent"))
	if len(ua) > 512 {
		ua = ua[:512]
	}
	entry, err := models.RecordSubscriptionAccess(id.(int), ip.Unmap().String(), c.GetString("subscriptionClient"), string(ua))
	if err != nil {
		log.Println("保存订阅访问记录失败:", err)
		return
	}
	enrichAccessIP(entry)
}

// Persist the access first. Bounded background lookups never delay subscription downloads.
func enrichAccessIP(entry models.SubLogs) {
	var cached models.SubLogs
	models.DB.Where("ip = ? AND ip_info_at IS NOT NULL", entry.IP).Order("ip_info_at DESC").First(&cached)
	if cached.IPInfoAt != nil && time.Since(*cached.IPInfoAt) < time.Hour {
		if entry.ID != cached.ID {
			models.DB.Model(&models.SubLogs{}).Where("id = ?", entry.ID).Updates(ipInfoFields(cached))
		}
		return
	}
	if _, busy := ipInfoPending.LoadOrStore(entry.IP, true); busy {
		return
	}
	select {
	case ipInfoSlots <- struct{}{}:
	default:
		ipInfoPending.Delete(entry.IP)
		return
	}
	go func() {
		defer func() { <-ipInfoSlots; ipInfoPending.Delete(entry.IP) }()
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer cancel()
		info := lookupAccessIP(ctx, entry.IP)
		now := time.Now()
		info.IPInfoAt = &now
		if err := models.DB.Model(&models.SubLogs{}).Where("ip = ?", entry.IP).Updates(ipInfoFields(info)).Error; err != nil {
			log.Println("更新订阅 IP 信息失败:", err)
		}
	}()
}

func ipInfoFields(info models.SubLogs) map[string]interface{} {
	fields := map[string]interface{}{"ip_info_at": info.IPInfoAt, "ip_info_status": info.IPInfoStatus}
	// A failed refresh must not erase previously resolved location details.
	if info.IPInfoStatus == "resolved" || info.IPInfoStatus == "local" {
		fields["addr"], fields["country"], fields["province"], fields["city"] = info.Addr, info.Country, info.Province, info.City
		fields["isp"], fields["asn"], fields["ip_info_source"] = info.ISP, info.ASN, info.IPInfoSource
	}
	return fields
}

func ipInfoJSON(ctx context.Context, address string, gbk bool, target interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return err
	}
	if req.URL.Host == "api.ipapi.is" {
		ipAPILimitMu.Lock()
		blocked := time.Now().Before(ipAPIBlockedUntil)
		ipAPILimitMu.Unlock()
		if blocked {
			return fmt.Errorf("IP lookup rate limited")
		}
	}
	resp, err := ipInfoClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if req.URL.Host == "api.ipapi.is" && (resp.StatusCode == 429 || resp.StatusCode == 403) {
		ipAPILimitMu.Lock()
		ipAPIBlockedUntil = time.Now().Add(24 * time.Hour)
		ipAPILimitMu.Unlock()
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("IP lookup HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 65537))
	if err != nil || len(data) > 65536 {
		return fmt.Errorf("invalid IP lookup response")
	}
	if gbk {
		data, err = simplifiedchinese.GBK.NewDecoder().Bytes(data)
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func lookupAccessIP(ctx context.Context, ip string) models.SubLogs {
	info := models.SubLogs{IPInfoStatus: "failed"}
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return info
	}
	if !addr.IsGlobalUnicast() || addr.IsPrivate() {
		info.Addr, info.IPInfoStatus = "内网 / 本机地址", "local"
		return info
	}
	var primary struct {
		IP       string          `json:"ip"`
		Country  string          `json:"country"`
		Region   string          `json:"region"`
		City     string          `json:"city"`
		Company  json.RawMessage `json:"company"`
		ASN      json.RawMessage `json:"asn"`
		Location struct {
			Country string `json:"country"`
			State   string `json:"state"`
			City    string `json:"city"`
		} `json:"location"`
	}
	if ipInfoJSON(ctx, "https://api.ipapi.is/?q="+url.QueryEscape(ip), false, &primary) == nil && sameIP(primary.IP, ip) {
		info.Country, info.Province, info.City = primary.Country, primary.Region, primary.City
		if info.Country == "" {
			info.Country, info.Province, info.City = primary.Location.Country, primary.Location.State, primary.Location.City
		}
		if json.Unmarshal(primary.Company, &info.ISP) != nil {
			var company struct {
				Name string `json:"name"`
			}
			json.Unmarshal(primary.Company, &company)
			info.ISP = company.Name
		}
		if json.Unmarshal(primary.ASN, &info.ASN) != nil {
			var asn struct {
				Number int    `json:"asn"`
				Org    string `json:"org"`
			}
			json.Unmarshal(primary.ASN, &asn)
			if asn.Number > 0 {
				info.ASN = fmt.Sprintf("AS%d %s", asn.Number, asn.Org)
			}
		}
		info.Addr = strings.TrimSpace(strings.Join([]string{info.Country, info.Province, info.City}, " "))
		if info.Addr != "" || info.ISP != "" || info.ASN != "" {
			info.IPInfoSource, info.IPInfoStatus = "ipapi.is", "resolved"
			return info
		}
	}
	var fallback struct {
		IP       string `json:"ip"`
		Addr     string `json:"addr"`
		Province string `json:"pro"`
		City     string `json:"city"`
		Region   string `json:"region"`
	}
	if ipInfoJSON(ctx, "https://whois.pconline.com.cn/ipJson.jsp?json=true&ip="+url.QueryEscape(ip), true, &fallback) == nil && sameIP(fallback.IP, ip) && fallback.Addr != "" {
		info = models.SubLogs{Addr: fallback.Addr, Province: fallback.Province, City: strings.TrimSpace(fallback.City + " " + fallback.Region), ISP: fallback.Addr, IPInfoSource: "太平洋 IP 查询", IPInfoStatus: "resolved"}
	}
	return info
}

func sameIP(a, b string) bool {
	x, err := netip.ParseAddr(a)
	y, err2 := netip.ParseAddr(b)
	return err == nil && err2 == nil && x.Unmap() == y.Unmap()
}
