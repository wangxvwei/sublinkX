package models

import (
	"fmt"
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

func TestRewriteHY2ProtocolAliases(t *testing.T) {
	for _, scheme := range []string{"hy2", "hysteria2"} {
		for _, protocol := range []string{"hy2", "Hysteria2"} {
			t.Run(scheme+"/"+protocol, func(t *testing.T) {
				link := scheme + "://test-auth@panel.example.com:43605?sni=panel.example.com&alpn=h3&obfs=salamander&obfs-password=test-obfs#hy2-vps"
				other := "vless://uuid@panel.example.com:443?security=reality#hy2-other"
				nodes := []XUINodeLink{{Name: "hy2-vps", Link: link}, {Name: "hy2-other", Link: other}}
				rules := fmt.Sprintf(`[{"protocol":%q,"nameContains":"hy2","address":"203.0.113.11","alpn":"h3"}]`, protocol)
				rewritten, err := applyNodeLinkOverrides(nodes, "", rules)
				if err != nil {
					t.Fatal(err)
				}
				parsed, err := url.Parse(rewritten[0].LinkOverride)
				if err != nil {
					t.Fatal(err)
				}
				if parsed.Host != "203.0.113.11:43605" || parsed.Scheme != scheme || parsed.User.String() != "test-auth" {
					t.Fatalf("unexpected rewritten endpoint: %s", parsed)
				}
				assertQueryValue(t, parsed.Query(), "sni", "panel.example.com")
				assertQueryValue(t, parsed.Query(), "alpn", "h3")
				assertQueryValue(t, parsed.Query(), "obfs-password", "test-obfs")
				if rewritten[0].Link != link || rewritten[1].Link != other || rewritten[1].LinkOverride != "" {
					t.Fatal("original links or a different protocol were modified")
				}
				unchanged, err := applyNodeLinkOverrides(nodes, "", `[{"protocol":"hy2","transport":"xhttp","address":"203.0.113.11"}]`)
				if err != nil || unchanged[0].LinkOverride != "" {
					t.Fatal("XHTTP rule should not match HY2")
				}
			})
		}
	}
}

func assertQueryValue(t *testing.T, query url.Values, key, want string) {
	t.Helper()
	if got := query.Get(key); got != want {
		t.Fatalf("query %s = %q, want %q", key, got, want)
	}
}
