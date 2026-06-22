package node

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
