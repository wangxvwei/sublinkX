package models

import (
	"net/url"
	"testing"
)

func TestRewriteNodeLinksByRulesAddsXHTTPTLSFields(t *testing.T) {
	originalLink := "vless://uuid@162.159.137.225:443?type=xhttp&path=%2Fapi%2Fv1%2Fsync#cdn-xhttp"
	nodes := []XUINodeLink{{
		Name: "cdn-xhttp",
		Link: originalLink,
	}}
	rules := `[
		{
			"transport": "xhttp",
			"security": "tls",
			"sni": "example.com",
			"host": "example.com",
			"fingerprint": "chrome",
			"alpn": "h2,http/1.1"
		}
	]`

	rewritten, err := applyNodeLinkOverrides(nodes, "", rules)
	if err != nil {
		t.Fatalf("applyNodeLinkOverrides returned error: %v", err)
	}
	if rewritten[0].Link != originalLink {
		t.Fatalf("original link changed: got %q, want %q", rewritten[0].Link, originalLink)
	}
	if rewritten[0].LinkOverride == "" {
		t.Fatal("expected link override to be set")
	}
	parsed, err := url.Parse(rewritten[0].LinkOverride)
	if err != nil {
		t.Fatalf("parse rewritten link: %v", err)
	}
	query := parsed.Query()
	assertQueryValue(t, query, "security", "tls")
	assertQueryValue(t, query, "sni", "example.com")
	assertQueryValue(t, query, "host", "example.com")
	assertQueryValue(t, query, "fp", "chrome")
	assertQueryValue(t, query, "alpn", "h2,http/1.1")
	assertQueryValue(t, query, "type", "xhttp")
	assertQueryValue(t, query, "path", "/api/v1/sync")
}

func TestRewriteNodeLinksByRulesSkipsOtherTransports(t *testing.T) {
	nodes := []XUINodeLink{{
		Name: "reality",
		Link: "vless://uuid@203.0.113.10:8443?type=tcp&security=reality#reality",
	}}
	rules := `[{"transport":"xhttp","security":"tls","sni":"example.com"}]`

	rewritten, err := applyNodeLinkOverrides(nodes, "", rules)
	if err != nil {
		t.Fatalf("applyNodeLinkOverrides returned error: %v", err)
	}
	if rewritten[0].Link != nodes[0].Link {
		t.Fatalf("expected tcp/reality link to remain unchanged, got %q", rewritten[0].Link)
	}
	if rewritten[0].LinkOverride != "" {
		t.Fatalf("expected tcp/reality override to remain empty, got %q", rewritten[0].LinkOverride)
	}
}

func assertQueryValue(t *testing.T, query url.Values, key, want string) {
	t.Helper()
	if got := query.Get(key); got != want {
		t.Fatalf("query %s = %q, want %q", key, got, want)
	}
}
