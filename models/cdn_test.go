package models

import (
	"fmt"
	"net/netip"
	"path/filepath"
	"strings"
	"sublink/node"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestCDNRemoteRecommendations(t *testing.T) {
	p := CDNProfile{Name: "remote", CandidateMode: "recommend", SourceIP: "9.9.9.9", TemplateLink: "vless://u@example.com:443?security=tls&type=ws", IPs: "9.9.9.9", Count: 5, URL: "https://example.com/file", MaxDelay: 500, MaxLoss: 0, Duration: 5}
	if err := normalizeCDN(&p); err != nil || p.IPs != "" || p.SourceIP != "9.9.9.9" {
		t.Fatal("home IP was treated as a CDN destination", err)
	}
	bad := p
	bad.SourceIP = "127.0.0.1"
	if normalizeCDN(&bad) == nil {
		t.Fatal("accepted a private source IP")
	}
	bad.SourceIP = ""
	if normalizeCDN(&bad) == nil {
		t.Fatal("accepted missing target IP")
	}
	for input, want := range map[string]string{"广西南宁市 某大学": "", "AS4837 CHINA UNICOM China169 Backbone": "CU", "江苏 电信": "CT", "China Mobile": "CM"} {
		if got := cdnCarrier(input); got != want {
			t.Fatal("wrong carrier", input, got)
		}
	}
	for raw, want := range map[string]string{
		`{"ip":"9.9.9.9","asn":{"org":"CHINA UNICOM China169 Backbone"}}`: "CU",
		`{"ip":"9.9.9.9","asn":{"org":"China Telecom"}}`:                  "CT",
		`{"ip":"1.1.1.1","asn":{"org":"China Mobile"}}`:                   "",
		`{"ip":"9.9.9.9","asn":null}`:                                     "",
	} {
		if got := cdnASNCarrier([]byte(raw), "9.9.9.9"); got != want {
			t.Fatal("ASN carrier lookup failed", got, want)
		}
	}
	template, err := node.ParseCDNTemplate(p.TemplateLink)
	if err != nil {
		t.Fatal(err)
	}
	p.Network.Carrier = "CU"
	ranges := []netip.Prefix{netip.MustParsePrefix("104.16.0.0/13")}
	now := time.Date(2026, 10, 21, 19, 0, 0, 0, time.FixedZone("UTC+8", 8*3600))
	data := []byte(fmt.Sprintf(`162.159.198.1:443#麒麟优选 | %s | BestCF.pages.dev
104.16.0.1:443#麒麟优选 | 联通 | 104.16.0.1 | 30ms | 10mb/s
104.16.0.1:443#麒麟优选 | 联通 | 104.16.0.1 | 30ms | 10mb/s
104.16.0.2:443#麒麟优选 | 电信 | 104.16.0.2 | 30ms | 10mb/s
104.16.0.3:8443#麒麟优选 | 联通 | 104.16.0.3 | 30ms | 10mb/s
127.0.0.1:443#麒麟优选 | 联通 | 127.0.0.1 | 30ms | 10mb/s
9.9.9.9:443#麒麟优选 | 联通 | 9.9.9.9 | 30ms | 10mb/s
8.8.8.8:443#麒麟优选 | 联通 | 8.8.8.8 | 30ms | 10mb/s
104.16.0.4:443#麒麟优选 | 联通 | 104.16.0.4 | 30ms | 10mb/s
162.159.197.1:443#麒麟优选 | 更多内容 | BestCF.pages.dev`, now.Format("01-02 15:04")))
	candidates, _, err := readCDNFeed(data, "uouin", ranges, now)
	if err != nil {
		t.Fatal(err)
	}
	results, err := selectCDNRecommendations(&p, template, ranges, candidates, nil, now)
	if err != nil || len(results) != 2 || results[0].IP != "104.16.0.1" || results[1].IP != "104.16.0.4" {
		t.Fatal("wrong filtering/ranking", results, err)
	}
	for _, r := range results {
		if !r.Recommendation || r.TLSOK || r.Speed != 0 || !strings.Contains(r.Link, "sni=example.com") {
			t.Fatal("recommendation claimed a measurement or lost SNI", r)
		}
	}
	if _, _, err := readCDNFeed(data, "uouin", ranges, now.Add(49*time.Hour)); err == nil {
		t.Fatal("accepted stale recommendations")
	}
	yearEnd := []byte(strings.Replace(string(data), "10-21 19:00", "12-31 23:00", 1))
	if _, _, err := readCDNFeed(yearEnd, "uouin", ranges, time.Date(2027, 1, 1, 0, 0, 0, 0, now.Location())); err != nil {
		t.Fatal("year boundary rejected fresh feed", err)
	}
}

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
	if err := SaveCDNProfile(&p, false); err != nil {
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
		if count, err := ImportCDN(p.ID, []string{results[0].IP, results[1].IP}, sub.ID); err != nil || count != 2 {
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
	if _, err := ImportCDN(p.ID, []string{"104.16.0.99"}, sub.ID); err == nil {
		t.Fatal("accepted forged selection")
	}
	if _, err := ImportCDN(p.ID, []string{results[0].IP}, 999); err == nil {
		t.Fatal("accepted missing subscription")
	}
	DB.Model(&GroupNode{}).Where("name = ?", "should-not-exist").Count(&count)
	if count != 0 {
		t.Fatal("failed import was not atomic")
	}
	// Explicit recommendations may be imported without pretending TLS was tested.
	p.Results[0].TLSOK = false
	p.Results[0].Recommendation = true
	if err := DB.Save(&p).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := ImportCDN(p.ID, []string{results[0].IP}, sub.ID); err == nil {
		t.Fatal("manual result bypassed TLS requirement")
	}
	p.CandidateMode = "recommend"
	if err := DB.Save(&p).Error; err != nil {
		t.Fatal(err)
	}
	if n, err := ImportCDN(p.ID, []string{results[0].IP}, sub.ID); err != nil || n != 1 {
		t.Fatal("recommendation import failed", n, err)
	}
}
