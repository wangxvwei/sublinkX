package models

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestXUIPartialSyncPreservesFailedClients(t *testing.T) {
	for _, mode := range []string{"api", "local", "ssh"} {
		for _, failure := range []string{"404", "502", "invalid", "unmatched", "all"} {
			t.Run(mode+"/"+failure, func(t *testing.T) {
				dbPath := filepath.Join(t.TempDir(), "test.db")
				db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
				if err != nil {
					t.Fatal(err)
				}
				previous := DB
				DB = db
				t.Cleanup(func() { sql, _ := db.DB(); sql.Close(); DB = previous })
				must := func(err error) {
					t.Helper()
					if err != nil {
						t.Fatal(err)
					}
				}
				must(db.AutoMigrate(&Node{}, &GroupNode{}, &Subcription{}, &SubLogs{}))
				clients := `{"clients":[{"id":"good-client","email":"good","subId":"good"},{"id":"bad-client","email":"bad","subId":"bad"},{"id":"shared-client","email":"shared","subId":"bad"}]}`
				must(db.Exec("CREATE TABLE inbounds (id INTEGER, enable INTEGER, settings TEXT)").Error)
				must(db.Exec("INSERT INTO inbounds VALUES (1,1,?)", clients).Error)
				recovered := false
				requests := map[string]int{}
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests[r.URL.Path]++
					switch r.URL.Path {
					case "/panel/api/inbounds/list":
						fmt.Fprintf(w, `{"success":true,"obj":[{"id":1,"enable":true,"settings":%s}]}`, clients)
					case "/dingyue/good":
						if failure == "all" && !recovered {
							http.NotFound(w, r)
							return
						}
						fmt.Fprint(w, "vless://good-client@example.test:443#good")
					case "/dingyue/bad":
						if recovered {
							fmt.Fprint(w, "vless://bad-client@example.test:443#bad\nvless://shared-client@example.test:443#shared")
						} else {
							switch failure {
							case "502":
								http.Error(w, "unavailable", 502)
							case "invalid":
								fmt.Fprint(w, "invalid subscription")
							case "unmatched":
								fmt.Fprint(w, "vless://other@example.test:443#other\nvless://another@example.test:443#another")
							default:
								http.NotFound(w, r)
							}
						}
					default:
						http.NotFound(w, r)
					}
				}))
				defer server.Close()
				key := func(id, sub string) string {
					if mode == "ssh" {
						return fmt.Sprintf("inbound:1:client:%s:sub:%s", id, sub)
					}
					return fmt.Sprintf("inbound:1:client:%s:%s", id, sub)
				}
				old := Node{Name: "old bad", Link: "vless://bad-client@old.test:443#old", LinkOverride: "vless://bad-client@override.test:443#old", Source: "3x-ui-source:42", SourceKey: key("bad-client", "old-sub"), SubID: "old-sub", GroupNodes: []GroupNode{{Name: "favorites"}}}
				if mode == "local" {
					old.Source = xuiNodeSource
				}
				must(db.Create(&old).Error)
				sub := Subcription{Name: "keep", Token: "test", Nodes: []Node{old}}
				must(db.Create(&sub).Error)
				must(db.Preload("GroupNodes").First(&old, old.ID).Error)
				removed := Node{Name: "removed", Link: "vless://removed@old.test:443", Source: old.Source, SourceKey: key("removed", "removed"), SubID: "removed"}
				must(db.Create(&removed).Error)
				source := XUISource{ID: 42, AuthType: "apiToken", PanelBaseURL: server.URL, SubBaseURL: server.URL, SubPath: "dingyue", APIToken: "test", DeleteMissing: true, GroupName: "new"}
				sync := func() (XUISyncResult, error) {
					if mode == "local" {
						return SyncXUINodes(XUISyncOptions{XUIDBPath: dbPath, SubscriptionBaseURL: server.URL, SubscriptionPath: "dingyue", DeleteMissing: true, GroupName: "new"})
					}
					if mode == "api" {
						return source.Sync()
					}
					python, err := exec.LookPath("python")
					if err != nil {
						python, err = exec.LookPath("python3")
					}
					if err != nil {
						t.Skip("Python unavailable for SSH script check")
					}
					payload, _ := json.Marshal(map[string]string{"xui_db_path": dbPath, "sub_base_url": server.URL, "sub_path": "dingyue"})
					cmd := exec.Command(python, "-c", "import json\npayload=json.loads("+fmt.Sprintf("%q", string(payload))+")\n"+remoteXUISyncScript())
					out, err := cmd.CombinedOutput()
					if err != nil {
						t.Fatalf("SSH script: %v: %s", err, out)
					}
					var remote remoteXUIResponse
					must(json.Unmarshal(out, &remote))
					if remote.Error != "" {
						t.Fatal(remote.Error)
					}
					return UpsertXUINodeLinks(remote.Nodes, XUINodeLinkOptions{SourceName: old.Source, DeleteMissing: true, GroupName: "new"})
				}
				result, err := sync()
				must(err)
				wantSkipped, wantCreated := 2, 1
				if failure == "all" {
					wantSkipped, wantCreated = 3, 0
				}
				if result.Skipped != wantSkipped || result.Created != wantCreated || result.Deleted != 1 || len(result.SkippedOn) != wantSkipped {
					t.Fatalf("unexpected result: %+v", result)
				}
				if requests["/dingyue/bad"] != 1 {
					t.Fatal("shared failing subscription fetched repeatedly")
				}
				if !strings.Contains(result.SkippedOn[wantSkipped-1].Reason, map[string]string{"404": "404", "502": "502", "all": "404", "invalid": "subscription", "unmatched": "matching"}[failure]) {
					t.Fatal(result.SkippedOn)
				}
				var kept Node
				must(db.Preload("GroupNodes").First(&kept, old.ID).Error)
				if !reflect.DeepEqual(old, kept) {
					t.Fatalf("failed client's original node changed: %+v", kept)
				}
				must(db.Preload("Nodes").First(&sub, sub.ID).Error)
				if len(sub.Nodes) != 1 || sub.Nodes[0].ID != old.ID {
					t.Fatal("subscription association changed")
				}
				recovered = true
				result, err = sync()
				must(err)
				if result.Skipped != 0 || result.Deleted != 1 {
					t.Fatalf("recovery failed: %+v", result)
				}
				var count int64
				must(db.Model(&Node{}).Where("source = ?", old.Source).Count(&count).Error)
				if count != 3 {
					t.Fatalf("node count after recovery = %d", count)
				}
			})
		}
	}
}
