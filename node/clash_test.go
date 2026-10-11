package node

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestEncodeClashVLESSXHTTPUsesXHTTPOpts(t *testing.T) {
	templatePath := filepath.Join(t.TempDir(), "clash.yaml")
	template := []byte("proxies: []\nproxy-groups:\n  - name: PROXY\n    type: select\n    proxies: []\n")
	if err := os.WriteFile(templatePath, template, 0600); err != nil {
		t.Fatalf("write template: %v", err)
	}

	payload := "11111111-1111-1111-1111-111111111111@example.com:443?encryption=none&security=tls&type=xhttp&path=%2Fxhttp&host=cdn.example.com&mode=auto&sni=example.com#xhttp-node"
	link := "vless://" + Base64Encode(payload)

	got, err := EncodeClash([]string{link}, SqlConfig{Clash: templatePath})
	if err != nil {
		t.Fatalf("EncodeClash() error = %v", err)
	}

	output := string(got)
	for _, want := range []string{
		"network: xhttp",
		"xhttp-opts:",
		"path: /xhttp",
		"host: cdn.example.com",
		"mode: auto",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("EncodeClash() output missing %q:\n%s", want, output)
		}
	}

	for _, unwanted := range []string{"ws-opts:", "grpc-opts:"} {
		if strings.Contains(output, unwanted) {
			t.Fatalf("EncodeClash() output contains %q:\n%s", unwanted, output)
		}
	}
}

func TestEncodeClashHY2CertificatePin(t *testing.T) {
	templatePath := filepath.Join(t.TempDir(), "clash.yaml")
	if err := os.WriteFile(templatePath, []byte("proxies: []\n"), 0600); err != nil {
		t.Fatal(err)
	}
	pin := strings.Repeat("ab", 32)
	for _, input := range []string{"", pin, strings.TrimSuffix(strings.Repeat("AB:", 32), ":")} {
		link := "hysteria2://test-password@example.com:443?sni=example.com&alpn=h3&obfs=salamander&obfs-password=test-obfs"
		if input != "" {
			link += "&pinSHA256=" + url.QueryEscape(input)
		}
		hy2, err := DecodeHY2URL(link)
		if err != nil {
			t.Fatal(err)
		}
		want := ""
		if input != "" {
			want = pin
		}
		roundtrip, err := DecodeHY2URL(EncodeHY2URL(hy2))
		if err != nil || roundtrip.PinSHA256 != want {
			t.Fatalf("certificate pin lost on URI roundtrip: %q, %v", roundtrip.PinSHA256, err)
		}
		got, err := EncodeClash([]string{link}, SqlConfig{Clash: templatePath})
		if err != nil {
			t.Fatal(err)
		}
		var config Config
		if err := yaml.Unmarshal(got, &config); err != nil {
			t.Fatal(err)
		}
		if len(config.Proxies) != 1 {
			t.Fatalf("expected one HY2 proxy, got %d", len(config.Proxies))
		}
		p := config.Proxies[0]
		if p.Fingerprint != want || p.Skip_cert_verify || p.Sni != "example.com" || p.Obfs_password != "test-obfs" || len(p.Alpn) != 1 || p.Alpn[0] != "h3" {
			t.Fatalf("incorrect HY2 TLS or transport settings: %#v", p)
		}
		if input == "" && strings.Contains(string(got), "fingerprint:") {
			t.Fatal("emitted a certificate fingerprint for an unpinned node")
		}
	}
	for _, invalid := range []string{"bad", strings.Repeat("a", 63), strings.Repeat("g", 64), " ", pin + "," + pin} {
		if _, err := DecodeHY2URL("hy2://test@example.com:443?pinSHA256=" + url.QueryEscape(invalid)); err == nil {
			t.Fatalf("accepted invalid certificate fingerprint %q", invalid)
		}
	}
}

func TestDefaultClashTemplateEnablesDNS(t *testing.T) {
	got, err := DecodeClash(nil, "../template/clash.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := yaml.Unmarshal(got, &config); err != nil {
		t.Fatal(err)
	}
	dns, ok := config["dns"].(map[string]any)
	if !ok || dns["enable"] != true || dns["enabled"] != nil {
		t.Fatalf("incorrect DNS enable field: %#v", dns)
	}
}
