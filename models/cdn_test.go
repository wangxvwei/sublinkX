package models

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestCDNSelectionAndAtomicImport(t *testing.T) {
	p := CDNProfile{Name: "test", TemplateLink: "vless://u@example.com:443?security=tls&type=ws", IPs: "104.16.0.1,104.16.0.2\n104.16.0.1\n104.16.0.3\n104.16.0.4", Count: 5, URL: "https://example.com/file", MaxDelay: 500, MaxLoss: 0, MinSpeed: 1, Duration: 5}
	if err := normalizeCDN(&p); err != nil {
		t.Fatal(err)
	}
	if len(strings.Split(p.IPs, "\n")) != 4 {
		t.Fatal("IPs were not deduplicated")
	}
	for _, ip := range []string{"127.0.0.1", "192.168.1.1", "169.254.169.254", "::ffff:127.0.0.1", "fe80::1%eth0", "100.64.0.1", "104.16.0.0/24"} {
		if _, err := PublicCDNIP(ip); err == nil {
			t.Fatalf("accepted non-public candidate %s", ip)
		}
	}
	csv := "IP,sent,received,loss,delay,speed,colo\n104.16.0.1,4,4,0,20,2,HKG\n104.16.0.2,4,4,0,60,3,NRT\n104.16.0.3,4,4,0,40,0,SIN\n104.16.0.4,4,4,0,40,NaN,SIN\n104.16.0.9,4,4,0,40,99,SIN\n"
	results, err := readCDNCSV(strings.NewReader(csv), p)
	if err != nil || len(results) != 2 || results[0].IP != "104.16.0.2" {
		t.Fatal("incorrect ranking/filtering", err, results)
	}
	previous := DB
	DB, err = gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sql, _ := DB.DB(); sql.Close(); DB = previous })
	if err := DB.AutoMigrate(&Node{}, &GroupNode{}, &Subcription{}, &SubLogs{}, &CDNProfile{}); err != nil {
		t.Fatal(err)
	}
	if err := SaveCDNProfile(&p); err != nil {
		t.Fatal(err)
	}
	for i := range results {
		results[i].TLSOK = true
		results[i].Link = "vless://u@" + results[i].IP + ":443?security=tls&type=ws&sni=example.com&host=example.com"
		results[i].Name = results[i].IP
	}
	p.Results, p.Status = results, "completed"
	if err := DB.Save(&p).Error; err != nil {
		t.Fatal(err)
	}
	original := Node{Name: "original", Link: p.TemplateLink}
	if err := DB.Create(&original).Error; err != nil {
		t.Fatal(err)
	}
	sub := Subcription{Name: "existing", Nodes: []Node{original}}
	if err := sub.Add(); err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 2; n++ {
		if count, err := ImportCDN(p.ID, []string{results[0].IP, results[1].IP}, "CDN", sub.ID); err != nil || count != 2 {
			t.Fatal(count, err)
		}
	}
	var count int64
	DB.Model(&Node{}).Count(&count)
	if count != 3 {
		t.Fatalf("re-import created duplicates: %d", count)
	}
	if err := sub.Find(); err != nil || len(sub.Nodes) != 3 || sub.Nodes[0].ID != original.ID {
		t.Fatal("subscription append lost existing nodes", err)
	}
	if _, err := ImportCDN(p.ID, []string{"104.16.0.99"}, "bad", sub.ID); err == nil {
		t.Fatal("accepted forged selection")
	}
	if _, err := ImportCDN(p.ID, []string{results[0].IP}, "should-not-exist", 999); err == nil {
		t.Fatal("accepted missing subscription")
	}
	DB.Model(&GroupNode{}).Where("name = ?", "should-not-exist").Count(&count)
	if count != 0 {
		t.Fatal("failed import was not atomic")
	}
}
