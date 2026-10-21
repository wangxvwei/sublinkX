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
	must(SaveCDNProfile(&p))
	check(0, "学校优选")
	check(1, "家里优选")
	p.Name = "家里优选"
	if SaveCDNProfile(&p) == nil {
		t.Fatal("duplicate names would merge independent groups")
	}
	p.Name = " "
	if SaveCDNProfile(&p) == nil {
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
