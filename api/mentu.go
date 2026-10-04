package api

import "github.com/gin-gonic/gin"

type Meta struct {
	Title           string   `json:"title"`
	Icon            string   `json:"icon"`
	Hidden          bool     `json:"hidden"`
	Roles           []string `json:"roles"`
	KeepAlive       bool     `json:"keepAlive,omitempty"`
	AlwaysShow      bool     `json:"alwaysShow,omitempty"`
	WorkspaceView   string   `json:"workspaceView,omitempty"`
	SettingsSection string   `json:"settingsSection,omitempty"`
}

type Child struct {
	Path      string  `json:"path"`
	Component string  `json:"component"`
	Name      string  `json:"name"`
	Meta      Meta    `json:"meta"`
	Redirect  string  `json:"redirect,omitempty"`
	Children  []Child `json:"children,omitempty"`
}

type Menu struct {
	Path      string  `json:"path"`
	Component string  `json:"component"`
	Redirect  string  `json:"redirect"`
	Name      string  `json:"name"`
	Meta      Meta    `json:"meta"`
	Children  []Child `json:"children"`
}

func workspaceMenus() []Menu {
	roles := []string{"ADMIN"}
	return []Menu{
		{
			Path: "/resources", Component: "Layout", Name: "Resources", Redirect: "/subcription/nodes",
			Meta: Meta{Title: "资源中心", Icon: "publish", Roles: roles, AlwaysShow: true},
			Children: []Child{
				{Path: "/subcription/nodes", Component: "subcription/nodes", Name: "Nodes", Meta: Meta{Title: "节点库", Icon: "link", Roles: roles, WorkspaceView: "nodes"}},
				{Path: "sources", Component: "RouteView", Name: "Sources", Redirect: "/resources/sources/api", Meta: Meta{Title: "来源接入", Icon: "client", Roles: roles, AlwaysShow: true}, Children: []Child{
					{Path: "api", Component: "subcription/nodes", Name: "APISources", Meta: Meta{Title: "API 面板", Icon: "client", Roles: roles, WorkspaceView: "api"}},
					{Path: "ssh", Component: "subcription/nodes", Name: "SSHSources", Meta: Meta{Title: "SSH / VPS", Icon: "system", Roles: roles, WorkspaceView: "ssh"}},
				}},
			},
		},
		{
			Path: "/subcription", Component: "Layout", Name: "Subscriptions", Redirect: "/subcription/subs",
			Meta: Meta{Title: "订阅中心", Icon: "link", Roles: roles, AlwaysShow: true},
			Children: []Child{
				{Path: "subs", Component: "subcription/subs", Name: "Subs", Meta: Meta{Title: "我的订阅", Icon: "link", Roles: roles, KeepAlive: true}},
				{Path: "template", Component: "subcription/template", Name: "Template", Meta: Meta{Title: "输出模板", Icon: "document", Roles: roles, KeepAlive: true}},
			},
		},
		{
			Path: "/system", Component: "Layout", Name: "System", Redirect: "/system/account",
			Meta: Meta{Title: "系统管理", Icon: "system", Roles: roles, AlwaysShow: true},
			Children: []Child{
				{Path: "account", Component: "system/user/set", Name: "Account", Meta: Meta{Title: "账号与安全", Icon: "role", Roles: roles, SettingsSection: "account"}},
				{Path: "update", Component: "system/user/set", Name: "SystemUpdate", Meta: Meta{Title: "版本更新", Icon: "document", Roles: roles, SettingsSection: "update"}},
				{Path: "user/set", Component: "system/user/set", Name: "LegacySettings", Redirect: "/system/account", Meta: Meta{Hidden: true, Roles: roles}},
			},
		},
	}
}

func GetMenus(c *gin.Context) {
	c.JSON(200, gin.H{"code": "00000", "data": workspaceMenus(), "msg": "获取成功"})
}
