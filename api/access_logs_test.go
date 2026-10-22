package api

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"sublink/middlewares"
	"sublink/models"
	"sublink/node"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestDownloadPassesSubscriptionToAccessLogger(t *testing.T) {
	previous := models.DB
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "access.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	models.DB = db
	t.Cleanup(func() { sql, _ := db.DB(); sql.Close(); models.DB = previous })
	if err := db.AutoMigrate(&models.Node{}, &models.Subcription{}, &models.SubLogs{}); err != nil {
		t.Fatal(err)
	}
	config, _ := json.Marshal(node.SqlConfig{Clash: filepath.Join("..", "template", "clash.yaml"), Surge: filepath.Join("..", "template", "surge.conf")})
	sub := models.Subcription{Name: "北京", Token: "beijing", Config: string(config)}
	if err := db.Create(&sub).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	cached := models.SubLogs{IP: "127.0.0.1", SubcriptionID: sub.ID, IPInfoAt: &now, IPInfoStatus: "local"}
	if err := db.Create(&cached).Error; err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.SetTrustedProxies(nil)
	r.GET("/c/", middlewares.GetIp, GetClient)
	for _, client := range []string{"v2ray", "clash", "surge"} {
		req := httptest.NewRequest("GET", "/c/?token=beijing&client="+client, nil)
		req.RemoteAddr = "127.0.0.1:1234"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("%s download status %d: %s", client, w.Code, w.Body.String())
		}
	}
	var entry models.SubLogs
	db.First(&entry, cached.ID)
	if entry.Count != 3 || entry.Client != "surge" {
		t.Fatalf("download was not logged: %+v", entry)
	}
	bad := models.Node{Name: "invalid", Link: "unsupported://invalid"}
	db.Create(&bad)
	db.Model(&sub).Association("Nodes").Append(&bad)
	req := httptest.NewRequest("GET", "/c/?token=beijing&client=clash", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("expected invalid node response, got %d", w.Code)
	}
	db.First(&entry, cached.ID)
	if entry.Count != 3 {
		t.Fatal("failed download incremented access count")
	}
}
