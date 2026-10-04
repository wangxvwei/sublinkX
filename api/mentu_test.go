package api

import (
	"path"
	"testing"
)

func TestWorkspaceMenuHierarchy(t *testing.T) {
	menus := workspaceMenus()
	if len(menus) != 3 || menus[0].Meta.Title != "资源中心" || menus[1].Meta.Title != "订阅中心" || menus[2].Meta.Title != "系统管理" {
		t.Fatal("workspace sections are not ordered by workflow")
	}
	paths := map[string]Child{}
	var walk func(string, []Child)
	walk = func(parent string, children []Child) {
		for _, child := range children {
			full := child.Path
			if !path.IsAbs(full) {
				full = path.Join(parent, full)
			}
			if _, duplicate := paths[full]; duplicate {
				t.Fatalf("duplicate menu path: %s", full)
			}
			if len(child.Meta.Roles) != 1 || child.Meta.Roles[0] != "ADMIN" {
				t.Fatalf("missing role guard: %s", full)
			}
			paths[full] = child
			walk(full, child.Children)
		}
	}
	for _, menu := range menus {
		walk(menu.Path, menu.Children)
	}
	if paths["/resources/sources"].Component != "RouteView" || len(paths["/resources/sources"].Children) != 2 {
		t.Fatal("source modes need a real nested route group")
	}
	if paths["/resources/sources/api"].Meta.WorkspaceView != "api" || paths["/resources/sources/ssh"].Meta.WorkspaceView != "ssh" {
		t.Fatal("API and SSH source pages are not separated")
	}
	if paths["/subcription/nodes"].Meta.WorkspaceView != "nodes" {
		t.Fatal("legacy node URL must still open the node library")
	}
	if paths["/system/account"].Meta.SettingsSection != "account" || paths["/system/update"].Meta.SettingsSection != "update" {
		t.Fatal("account and updater must have independent pages")
	}
	if paths["/system/user/set"].Redirect != "/system/account" || !paths["/system/user/set"].Meta.Hidden {
		t.Fatal("legacy settings URL must redirect without another visible menu")
	}
}
