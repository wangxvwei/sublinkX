package middlewares

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sublink/models"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"golang.org/x/text/encoding/simplifiedchinese"
	"gorm.io/gorm"
)

type lookupTransport func(*http.Request) (*http.Response, error)

func (fn lookupTransport) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }

func TestSubscriptionAccessRecording(t *testing.T) {
	previousDB, previousClient := models.DB, ipInfoClient
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "access.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	models.DB = db
	t.Cleanup(func() { sqlDB.Close(); models.DB, ipInfoClient = previousDB, previousClient })
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	// Upgrade a real legacy table, retaining old counts and location text.
	must(db.Exec("CREATE TABLE sub_logs (id integer primary key, ip text, date text, addr text, count integer, subcription_id integer)").Error)
	must(db.Exec("INSERT INTO sub_logs VALUES (1, '8.8.8.8', '2026-01-01 00:00:00', '旧归属地', 8, 1)").Error)
	must(db.AutoMigrate(&models.Node{}, &models.Subcription{}, &models.SubLogs{}))
	first := models.Subcription{Name: "北京", Token: "beijing"}
	second := models.Subcription{Name: "其他地区", Token: "other"}
	must(db.Create(&first).Error)
	must(db.Create(&second).Error)
	ipInfoClient = &http.Client{Transport: lookupTransport(func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("Authorization") != "" || req.Header.Get("User-Agent") != "" || req.URL.Query().Get("token") != "" {
			t.Error("subscription credentials/client metadata leaked to IP provider")
		}
		ip := req.URL.Query().Get("q")
		body, status := "", 200
		if ip == "8.8.8.8" {
			body = `{"ip":"8.8.8.8","country":"中国","region":"北京","city":"北京","company":"测试网络","asn":"AS123 测试网络"}`
		} else if ip == "2606:4700::1111" {
			body = `{"ip":"2606:4700::1111","location":{"country":"China","state":"Beijing","city":"Beijing"},"company":{"name":"IPv6 网络"},"asn":{"asn":456,"org":"IPv6 网络"}}`
		} else if req.URL.Host == "whois.pconline.com.cn" && req.URL.Query().Get("ip") == "1.0.0.1" {
			raw := `{"ip":"1.0.0.1","pro":"北京市","city":"北京市","region":"海淀区","addr":"北京市 联通"}`
			data, _ := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(raw))
			body = string(data)
		} else {
			status = 503
			body = "unavailable"
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	must(r.SetTrustedProxies(nil))
	r.GET("/c/", GetIp, func(c *gin.Context) {
		var sub models.Subcription
		if db.Where("token = ?", c.Query("token")).First(&sub).Error != nil {
			c.Status(404)
			return
		}
		c.Set("subscriptionID", sub.ID)
		c.Set("subscriptionClient", "v2ray")
		c.String(200, "subscription")
	})
	hit := func(token, ip string) {
		req := httptest.NewRequest("GET", "/c/?token="+token+"&client=v2ray", nil)
		req.RemoteAddr = "[" + ip + "]:1234"
		req.Header.Set("User-Agent", "v2rayN/test")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Errorf("download failed: %d", w.Code)
		}
	}
	waitInfo := func(ip, status string) models.SubLogs {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			var entry models.SubLogs
			db.Where("ip = ?", ip).First(&entry)
			_, pending := ipInfoPending.Load(ip)
			if entry.IPInfoStatus == status && !pending {
				return entry
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("lookup did not finish for %s", ip)
		return models.SubLogs{}
	}
	hit(first.Token, "8.8.8.8")
	entry := waitInfo("8.8.8.8", "resolved")
	if entry.Count != 9 || entry.Province != "北京" || entry.ASN != "AS123 测试网络" || entry.Client != "v2ray" || entry.UserAgent != "v2rayN/test" {
		t.Fatalf("unexpected record: %+v", entry)
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); hit(first.Token, "8.8.8.8") }()
	}
	wg.Wait()
	hit(second.Token, "8.8.8.8")
	var rows []models.SubLogs
	must(db.Where("ip = ?", "8.8.8.8").Order("subcription_id").Find(&rows).Error)
	if len(rows) != 2 || rows[0].Count != 29 || rows[1].Count != 1 {
		t.Fatalf("counts mixed or lost: %+v", rows)
	}
	hit(first.Token, "1.1.1.1")
	if waitInfo("1.1.1.1", "failed").Count != 1 {
		t.Fatal("failed lookup lost access")
	}
	hit(first.Token, "1.0.0.1")
	if waitInfo("1.0.0.1", "resolved").City != "北京市 海淀区" {
		t.Fatal("GBK fallback location lost")
	}
	hit(first.Token, "2606:4700::1111")
	if waitInfo("2606:4700::1111", "resolved").ASN != "AS456 IPv6 网络" {
		t.Fatal("IPv6/full response not handled")
	}
	hit(first.Token, "127.0.0.1")
	waitInfo("127.0.0.1", "local")
	var before int64
	db.Model(&models.SubLogs{}).Count(&before)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/c/?token=invalid", nil))
	if w.Code != 404 {
		t.Fatal(w.Code)
	}
	var after int64
	db.Model(&models.SubLogs{}).Count(&after)
	if before != after {
		t.Fatal("invalid token was recorded")
	}
	// Failed detail refresh preserves old attribution and never overwrites counts.
	must(db.Model(&models.SubLogs{}).Where("id = ?", entry.ID).Updates(ipInfoFields(models.SubLogs{IPInfoStatus: "failed"})).Error)
	must(db.First(&entry, entry.ID).Error)
	if entry.Count != 29 || entry.Province != "北京" {
		t.Fatal("refresh erased existing record")
	}
	// An IP mismatch from a provider is rejected rather than attributed to the caller.
	mismatched := lookupAccessIP(context.Background(), "9.9.9.9")
	if mismatched.IPInfoStatus != "failed" {
		t.Fatal(fmt.Sprintf("unexpected lookup: %+v", mismatched))
	}
}
