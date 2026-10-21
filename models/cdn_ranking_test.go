package models

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"path/filepath"
	"sublink/node"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestCDNMultiSourceHistoryAndRetention(t *testing.T) {
	previous := DB
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "recommend.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	DB = db
	t.Cleanup(func() { sql, _ := db.DB(); sql.Close(); DB = previous })
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(db.AutoMigrate(&CDNObservation{}, &CDNProfile{}, &Node{}, &GroupNode{}, &Subcription{}))
	now := time.Date(2026, 10, 21, 20, 5, 0, 0, time.FixedZone("UTC+8", 8*3600))
	ranges := []netip.Prefix{netip.MustParsePrefix("104.16.0.0/13")}
	p := CDNProfile{Name: "Campus", TemplateLink: "vless://u@example.com:443?security=tls&type=xhttp&path=%2Fhello", CandidateMode: "recommend", SourceIP: "9.9.9.9", Count: 5, URL: "https://example.com/file", MaxDelay: 500, Duration: 5, Network: CDNNetwork{Carrier: "CU"}}
	template, err := node.ParseCDNTemplate(p.TemplateLink)
	must(err)
	row := func(ip, source string, quality float64) cdnCandidate {
		return cdnCandidate{IP: ip, Source: source, Carrier: "CU", Port: 443, Quality: quality, UpdatedAt: now}
	}
	stable, fresh := row("104.16.0.1", "uouin", 0.8), row("104.17.0.1", "uouin", 1)
	must(db.Create(&CDNObservation{Source: "uouin", Carrier: "CU", Port: 443, IP: "104.16.0.99", Window: now.UTC().Add(-8 * 24 * time.Hour)}).Error)
	for i := 2; i >= 0; i-- {
		c := stable
		c.UpdatedAt = now.Add(-time.Duration(i) * 30 * time.Minute)
		must(saveCDNObservations([]cdnCandidate{c}, now))
	}
	must(saveCDNObservations([]cdnCandidate{stable, fresh}, now))
	must(saveCDNObservations([]cdnCandidate{stable, fresh}, now))
	// A refresh within the same provider time window is still one observation.
	again := stable
	again.UpdatedAt = now.Add(time.Minute)
	must(saveCDNObservations([]cdnCandidate{again}, now))
	var count int64
	must(db.Model(&CDNObservation{}).Count(&count).Error)
	if count != 4 {
		t.Fatal("repeated snapshots inflated history", count)
	}
	var history []CDNObservation
	must(db.Find(&history).Error)
	results, err := selectCDNRecommendations(&p, template, ranges, []cdnCandidate{stable, fresh}, history, now)
	must(err)
	if results[0].IP != stable.IP || results[0].HistorySamples != 3 || p.Network.HistoryWindows != 3 {
		t.Fatal("history stability was ignored")
	}
	old := row("104.18.0.1", "uouin", 1)
	old.UpdatedAt = now.Add(-40 * time.Hour)
	results, err = selectCDNRecommendations(&p, template, ranges, []cdnCandidate{fresh, old}, nil, now)
	must(err)
	if results[0].IP != fresh.IP {
		t.Fatal("old data outranked fresh data")
	}
	corroborated := row(fresh.IP, "wetest", 1)
	results, err = selectCDNRecommendations(&p, template, ranges, []cdnCandidate{fresh, corroborated, corroborated, stable}, nil, now)
	must(err)
	if results[0].IP != fresh.IP || len(results[0].Sources) != 2 {
		t.Fatal("same provider or IP was counted twice")
	}
	diverse := []cdnCandidate{}
	for i := 1; i <= 5; i++ {
		diverse = append(diverse, row(fmt.Sprintf("104.16.0.%d", i), "uouin", 1))
	}
	diverse = append(diverse, row("104.17.0.20", "wetest", 0.5), row("104.18.0.20", "cfyes", 0.5), row("104.19.0.20", "cfyes", 0.5))
	results, err = selectCDNRecommendations(&p, template, ranges, diverse, nil, now)
	must(err)
	near := 0
	for _, r := range results {
		if netip.MustParsePrefix("104.16.0.0/24").Contains(netip.MustParseAddr(r.IP)) {
			near++
		}
	}
	if len(results) != 5 || near > 2 {
		t.Fatal("recommendations clustered in one /24")
	}
	p.PinnedIPs = []string{"104.16.0.99"}
	results, err = selectCDNRecommendations(&p, template, ranges, diverse, nil, now)
	must(err)
	if results[0].IP != p.PinnedIPs[0] || !results[0].Pinned || results[0].TLSOK || results[0].Speed != 0 || results[0].SourceUpdatedAt != nil {
		t.Fatal("retention claimed a fresh measurement or was dropped")
	}
	results, err = selectCDNRecommendations(&p, template, ranges, nil, nil, now)
	must(err)
	if len(results) != 1 || results[0].IP != p.PinnedIPs[0] {
		t.Fatal("source outage discarded retained IP")
	}
	p.PinnedIPs = nil
	if _, err = selectCDNRecommendations(&p, template, ranges, nil, nil, now); err == nil {
		t.Fatal("source outage produced invented results")
	}
	old.UpdatedAt = now.Add(-49 * time.Hour)
	if _, err = selectCDNRecommendations(&p, template, ranges, []cdnCandidate{old}, nil, now); err == nil {
		t.Fatal("stale source accepted")
	}
	// Three provider formats, one failed provider, and no actual network requests.
	stamp := now.Format("01-02 15:04")
	uouin := []byte(fmt.Sprintf("header | %s | BestCF.pages.dev\n104.16.1.1:443#test | 联通 | 104.16.1.1 | 30ms | 10mb/s\n104.16.1.2:443#test | 联通 | 104.16.1.2 | NaNms | 10mb/s", stamp))
	wetest := []byte(fmt.Sprintf("header | %s | BestCF.pages.dev\n104.17.1.1:443#test | 联通 | HKG | 104.17.1.1", stamp))
	cfyes := []byte(fmt.Sprintf("header | %s | BestCF.pages.dev\n104.18.1.1:443#test | 联通 | 104.18.1.1", stamp))
	for id, data := range map[string][]byte{"uouin": uouin, "wetest": wetest, "cfyes": cfyes} {
		parsed, _, err := readCDNFeed(data, id, ranges, now)
		must(err)
		if len(parsed) != 1 {
			t.Fatal("invalid feed schema or metrics accepted", id, len(parsed))
		}
	}
	cdnFeedCache.Lock()
	previousCache := cdnFeedCache.Entries
	cdnFeedCache.Entries = make(map[string]cdnCachedFeed)
	for _, f := range cdnFeeds {
		data := uouin
		if f.ID == "wetest" {
			data = wetest
		}
		cdnFeedCache.Entries[f.URL] = cdnCachedFeed{Data: data, At: time.Now()}
	}
	cdnFeedCache.Entries[cdnFeeds[2].URL] = cdnCachedFeed{Err: errors.New("source unavailable"), At: time.Now()}
	cdnFeedCache.Unlock()
	t.Cleanup(func() { cdnFeedCache.Lock(); cdnFeedCache.Entries = previousCache; cdnFeedCache.Unlock() })
	results, err = rankCDNRecommendations(context.Background(), &p, template, ranges, now)
	must(err)
	if len(results) != 2 || len(p.Network.Sources) != 3 || p.Network.Sources[2].Error == "" {
		t.Fatal("partial outage aborted or was hidden")
	}
	// Retention is server-owned, profile-scoped, and cleared by network/template changes.
	p.Results, p.Status, p.PinnedIPs = results, "completed", nil
	must(db.Create(&p).Error)
	p, err = SetCDNPinnedIP(p.ID, results[0].IP, true)
	must(err)
	p, err = SetCDNPinnedIP(p.ID, results[0].IP, true)
	must(err)
	if len(p.PinnedIPs) != 1 {
		t.Fatal("duplicate retention")
	}
	if _, err = SetCDNPinnedIP(p.ID, "104.16.9.9", true); err == nil {
		t.Fatal("untrusted IP retained")
	}
	copy := p
	copy.ID, copy.Name = 0, "Home"
	must(SaveCDNProfile(&copy, false))
	if len(copy.PinnedIPs) != 0 || len(copy.Results) != 0 {
		t.Fatal("new profile inherited retained IPs/results")
	}
	must(db.First(&p, p.ID).Error)
	p.PinnedIPs = nil // A stale ordinary-save payload cannot erase retention.
	must(SaveCDNProfile(&p, false))
	if len(p.PinnedIPs) != 1 {
		t.Fatal("ordinary save erased retention")
	}
	p.Name = "Renamed"
	must(SaveCDNProfile(&p, true))
	if len(p.PinnedIPs) != 1 {
		t.Fatal("rename erased retention")
	}
	p.Count = 10
	for i := 1; i <= 6; i++ {
		ip := fmt.Sprintf("104.19.1.%d", i)
		link, name := template.WithIP(ip, cdnNodeName(p.Name, len(p.Results)))
		p.Results = append(p.Results, CDNResult{IP: ip, Link: link, Name: name, Recommendation: true})
	}
	must(db.Save(&p).Error)
	for _, r := range p.Results {
		p, err = SetCDNPinnedIP(p.ID, r.IP, true)
		must(err)
	}
	p.Count = 5
	if SaveCDNProfile(&p, false) == nil {
		t.Fatal("count reduction silently dropped retained IPs")
	}
	p.SourceIP = "1.1.1.1"
	must(SaveCDNProfile(&p, false))
	if len(p.PinnedIPs) != 0 {
		t.Fatal("retention leaked to a different target network")
	}
}
