package api

import (
	"net/url"
	"testing"

	"sublink/models"
	"sublink/node"
)

func TestCollectNodeLinksRewritesURLFragment(t *testing.T) {
	links, err := collectNodeLinks([]models.Node{{
		Name: "[107] vps",
		Link: "vless://00000000-0000-0000-0000-000000000000@example.com:443?type=xhttp&security=tls#old-name",
	}})
	if err != nil {
		t.Fatalf("collectNodeLinks returned error: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}

	parsed, err := url.Parse(links[0])
	if err != nil {
		t.Fatalf("parse rewritten link: %v", err)
	}
	if parsed.Fragment != "[107] vps" {
		t.Fatalf("expected fragment to use node name, got %q in %q", parsed.Fragment, links[0])
	}
}

func TestCollectNodeLinksRewritesVMessPS(t *testing.T) {
	raw := node.EncodeVmessURL(node.Vmess{
		Add:  "example.com",
		Port: "443",
		Aid:  "0",
		Id:   "00000000-0000-0000-0000-000000000000",
		Net:  "ws",
		Ps:   "old-name",
	})

	links, err := collectNodeLinks([]models.Node{{
		Name: "[144] vmess",
		Link: raw,
	}})
	if err != nil {
		t.Fatalf("collectNodeLinks returned error: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}

	vmess, err := node.DecodeVMESSURL(links[0])
	if err != nil {
		t.Fatalf("decode rewritten vmess link: %v", err)
	}
	if vmess.Ps != "[144] vmess" {
		t.Fatalf("expected vmess ps to use node name, got %q", vmess.Ps)
	}
}
