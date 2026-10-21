package models

import (
	"fmt"
	"net/url"
	"path/filepath"
	"sublink/node"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestCDNNamingMigrationAndIsolation(t *testing.T) {
	previous := DB
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "naming.db")), &gorm.Config{})
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
	must(db.AutoMigrate(&Node{}, &GroupNode{}, &Subcription{}, &SubLogs{}, &CDNProfile{}))
	raw := "vless://u@example.com:443?security=tls&type=xhttp&path=%2Ftest&extra=%7B%22a%22%3A1%7D#original"
	template, err := node.ParseCDNTemplate(raw)
	must(err)
	link, _ := template.WithIP("104.16.0.1", "old long name")
	legacy := GroupNode{Name: "CDN 优选"}
	extra := GroupNode{Name: "收藏"}
	must(db.Create(&legacy).Error)
	must(db.Create(&extra).Error)
	original := Node{Name: "original", Link: raw}
	must(db.Create(&original).Error)
	profiles := []CDNProfile{}
	generated := []Node{}
	for _, name := range []string{"西大优选", "家里优选"} {
		p := CDNProfile{Name: name, TemplateLink: raw, CandidateMode: "recommend", SourceIP: "9.9.9.9", Count: 5, URL: "https://example.com/file", MaxDelay: 500, Duration: 5,
			GroupName: legacy.Name, Status: "completed", Results: []CDNResult{{IP: "104.16.0.1", Name: "old long name", Link: link, Recommendation: true}}}
		must(db.Create(&p).Error)
		n := Node{Name: name + " old", Link: link, LinkOverride: link, Source: fmt.Sprintf("cdn:%d", p.ID), SourceKey: "104.16.0.1", GroupNodes: []GroupNode{legacy, extra}}
		must(db.Create(&n).Error)
		profiles, generated = append(profiles, p), append(generated, n)
	}
	sub := Subcription{Name: "existing", Nodes: []Node{original, generated[1], generated[0]}}
	must(sub.Add())
	legacySub := Subcription{Name: "legacy order", Nodes: []Node{generated[1], original, generated[0]}}
	must(legacySub.Add())
	must(db.Model(&legacySub).Update("node_order_ids", "").Error)
	check := func(index int, name string) {
		t.Helper()
		var p CDNProfile
		must(db.First(&p, profiles[index].ID).Error)
		if p.GroupName != name || len(p.Results) != 1 || p.Results[0].Name != name+" · 01" || p.Status != "completed" {
			t.Fatal("profile results/name were lost", p.Name, p.GroupName, p.Status)
		}
		var n Node
		must(db.Preload("GroupNodes").First(&n, generated[index].ID).Error)
		if n.Name != p.Results[0].Name || n.Link != p.Results[0].Link {
			t.Fatal("preview, library and link names disagree", n.Name)
		}
		before, _ := url.Parse(link)
		for _, raw := range []string{n.Link, n.LinkOverride, p.Results[0].Link} {
			if raw == "" {
				continue
			}
			u, err := url.Parse(raw)
			must(err)
			if u.Fragment != n.Name || u.Host != before.Host || u.User.String() != before.User.String() || u.Query().Encode() != before.Query().Encode() {
				t.Fatal("rename changed connection parameters")
			}
		}
		groups := map[string]bool{}
		for _, g := range n.GroupNodes {
			groups[g.Name] = true
		}
		if len(groups) != 2 || !groups[name] || !groups["收藏"] {
			t.Fatal("group migration lost other membership or retained old group", groups)
		}
	}
	must(migrateCDNNaming())
	must(migrateCDNNaming()) // Startup migration is idempotent.
	check(0, "西大优选")
	check(1, "家里优选")
	var count int64
	must(db.Model(&GroupNode{}).Where("name = ?", "CDN 优选").Count(&count).Error)
	if count != 0 {
		t.Fatal("empty legacy default group survived")
	}
	var p CDNProfile
	must(db.First(&p, profiles[0].ID).Error)
	p.Name, p.GroupName = "学校优选", "ignored client group"
	if SaveCDNProfile(&p, false) == nil {
		t.Fatal("ordinary save silently renamed an existing profile")
	}
	check(0, "西大优选")
	must(SaveCDNProfile(&p, true))
	check(0, "学校优选")
	check(1, "家里优选")
	p.Name = "家里优选"
	if SaveCDNProfile(&p, true) == nil {
		t.Fatal("duplicate names would merge independent groups")
	}
	p.Name = " "
	if SaveCDNProfile(&p, false) == nil {
		t.Fatal("empty name received a default")
	}
	for i := 0; i < 2; i++ {
		_, err := ImportCDN(profiles[0].ID, []string{"104.16.0.1"}, sub.ID)
		must(err)
	}
	check(0, "学校优选")
	check(1, "家里优选")
	must(sub.Find())
	if len(sub.Nodes) != 3 || sub.NodeOrderIDs != fmt.Sprintf("%d,%d,%d", original.ID, generated[1].ID, generated[0].ID) {
		t.Fatal("rename/re-import changed subscription membership or order")
	}
	must(db.Model(&Node{}).Count(&count).Error)
	if count != 3 {
		t.Fatal("re-import created duplicates")
	}
	must(legacySub.Find())
	if legacySub.NodeOrderIDs != fmt.Sprintf("%d,%d,%d", generated[1].ID, original.ID, generated[0].ID) {
		t.Fatal("legacy subscription order was lost during rename")
	}
	var seed Node
	must(db.First(&seed, original.ID).Error)
	if seed.Name != original.Name || seed.Link != original.Link {
		t.Fatal("original node was changed")
	}
}

func TestCDNIndependentProfilesAndReplacingBatches(t *testing.T) {
	previous := DB
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "batches.db")), &gorm.Config{})
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
	must(db.AutoMigrate(&Node{}, &GroupNode{}, &Subcription{}, &SubLogs{}, &CDNProfile{}))
	p := CDNProfile{Name: "西大优选", TemplateLink: "vless://u@example.com:443?security=tls&type=ws", CandidateMode: "recommend", SourceIP: "9.9.9.9", Count: 10, URL: "https://example.com/file", MaxDelay: 500, Duration: 5}
	template, err := node.ParseCDNTemplate(p.TemplateLink)
	must(err)
	setResults := func(p *CDNProfile, offset int) []string {
		p.Results = nil
		ips := []string{}
		for i := 1; i <= 10; i++ {
			ip := fmt.Sprintf("104.16.0.%d", offset+i)
			link, name := template.WithIP(ip, cdnNodeName(p.Name, i-1))
			p.Results = append(p.Results, CDNResult{IP: ip, Link: link, Name: name, Recommendation: true})
			ips = append(ips, ip)
		}
		p.Status = "completed"
		must(db.Save(p).Error)
		return ips
	}
	profiles := []CDNProfile{}
	for _, name := range []string{"西大优选", "北林优选", "家里优选", "公司优选"} {
		p.ID, p.Name = 0, name // Copying input settings must create a fresh identity.
		must(SaveCDNProfile(&p, false))
		if p.Status != "idle" || len(p.Results) != 0 {
			t.Fatal("new profile inherited saved results")
		}
		ips := setResults(&p, 0)
		_, err := ImportCDN(p.ID, ips, 0)
		must(err)
		profiles = append(profiles, p)
	}
	p = profiles[0]
	source := fmt.Sprintf("cdn:%d", p.ID)
	var before []Node
	must(db.Where("source = ?", source).Order("id").Find(&before).Error)
	original := Node{Name: "original", Link: p.TemplateLink}
	must(db.Create(&original).Error)
	sub := Subcription{Name: "subscribed", Nodes: append([]Node{original}, before...)}
	must(sub.Add())
	legacy := Subcription{Name: "legacy", Nodes: []Node{before[9], original, before[0]}}
	must(legacy.Add())
	must(db.Model(&legacy).Update("node_order_ids", "").Error)
	beforeOrder := sub.NodeOrderIDs
	check := func(want int, ips []string) {
		t.Helper()
		var nodes []Node
		must(db.Preload("GroupNodes").Where("source = ?", source).Order("id").Find(&nodes).Error)
		if len(nodes) != want {
			t.Fatalf("batch accumulated: got %d want %d", len(nodes), want)
		}
		got := map[string]bool{}
		for _, n := range nodes {
			got[n.SourceKey] = true
			if len(n.GroupNodes) != 1 || n.GroupNodes[0].Name != p.Name {
				t.Fatal("incorrect group")
			}
			u, err := url.Parse(n.Link)
			must(err)
			if u.Hostname() != n.SourceKey || u.Fragment != n.Name || u.Query().Get("sni") != "example.com" {
				t.Fatal("node parameters corrupted")
			}
		}
		for _, ip := range ips {
			if !got[ip] {
				t.Fatal("new result missing", ip)
			}
		}
		for _, other := range profiles[1:] {
			var nodes []Node
			must(db.Where("source = ?", fmt.Sprintf("cdn:%d", other.ID)).Order("id").Find(&nodes).Error)
			if len(nodes) != 10 || nodes[0].Name != other.Name+" · 01" || nodes[0].Link != other.Results[0].Link {
				t.Fatal("another profile changed")
			}
		}
	}
	ips := setResults(&p, 20)
	_, err = ImportCDN(p.ID, ips, sub.ID)
	must(err)
	check(10, ips)
	must(sub.Find())
	if len(sub.Nodes) != 11 || sub.NodeOrderIDs != beforeOrder {
		t.Fatal("subscription lost slots or order on IP replacement")
	}
	must(legacy.Find())
	if len(legacy.Nodes) != 3 || legacy.Nodes[0].ID != before[9].ID || legacy.Nodes[1].ID != original.ID || legacy.Nodes[2].ID != before[0].ID {
		t.Fatal("untargeted legacy subscription changed order")
	}
	// Half-overlapping batch must reserve unchanged IPs before recycling slots.
	ips = setResults(&p, 25)
	_, err = ImportCDN(p.ID, ips, 0)
	must(err)
	check(10, ips)
	var retained Node
	must(db.First(&retained, before[5].ID).Error)
	if retained.SourceKey != "104.16.0.26" {
		t.Fatal("matching IP lost its ID")
	}
	// Simulate old versions' accumulated rows, then save fewer selections.
	stale := Node{Name: "stale", Source: source, SourceKey: "104.16.0.99", Link: p.Results[0].Link}
	must(db.Create(&stale).Error)
	_, err = ImportCDN(p.ID, ips[:5], sub.ID)
	must(err)
	check(5, ips[:5])
	must(sub.Find())
	if len(sub.Nodes) != 6 || sub.Nodes[0].ID != original.ID {
		t.Fatal("obsolete nodes survived or original node disappeared")
	}
	_, err = ImportCDN(p.ID, ips[:5], sub.ID)
	must(err)
	check(5, ips[:5])
}
