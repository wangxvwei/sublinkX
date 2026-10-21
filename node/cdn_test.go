package node

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

func TestCDNLinkPreservesTemplate(t *testing.T) {
	raw := "vless://test-uuid@origin.example.com:8443?security=tls&type=xhttp&sni=tls.example.com&host=http.example.com&path=%2Fx%3Fed%3D2048&mode=auto&alpn=h2,http%2F1.1&extra=%7B%22a%22%3A1%7D#original"
	template, err := ParseCDNTemplate(raw)
	if err != nil {
		t.Fatal(err)
	}
	if encoded, err := ParseCDNTemplate("vless://" + Base64Encode(strings.TrimPrefix(raw, "vless://"))); err != nil || encoded.SNI != template.SNI {
		t.Fatal("base64 VLESS template not supported", err)
	}
	link, name := template.WithIP("2606:4700::1111")
	u, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	original, _ := url.Parse(raw)
	if u.Host != "[2606:4700::1111]:8443" || u.Fragment != name || u.User.String() != original.User.String() || u.Query().Encode() != original.Query().Encode() {
		t.Fatal("address substitution changed authentication or transport parameters")
	}
	fallback, err := ParseCDNTemplate("trojan://password@origin.example.com:443?type=ws&path=%2Fws")
	if err != nil || fallback.SNI != "origin.example.com" || fallback.Host != "origin.example.com" {
		t.Fatal("domain defaults were lost", err)
	}
	for _, bad := range []string{
		"vless://u@example.com:443?security=reality&type=tcp",
		"vless://u@example.com:443?security=tls&type=ws&flow=xtls-rprx-vision",
		"vless://u@104.16.0.1:443?security=tls&type=ws",
		"vless://example.com:443?security=tls&type=ws",
		"hysteria2://p@example.com:443",
	} {
		if _, err := ParseCDNTemplate(bad); err == nil {
			t.Fatalf("accepted unsupported template %q", bad)
		}
	}
	vmess := "vmess://" + Base64Encode(`{"add":"origin.example.com","port":443,"id":"test","net":"ws","tls":"tls","ps":"test","custom":{"value":9007199254740993}}`)
	v, err := ParseCDNTemplate(vmess)
	if err != nil {
		t.Fatal(err)
	}
	generated, _ := v.WithIP("104.16.0.1")
	generated, err = RenameVMess(generated, "new name")
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]json.RawMessage
	if err := json.Unmarshal([]byte(Base64Decode(strings.TrimPrefix(generated, "vmess://"))), &data); err != nil {
		t.Fatal(err)
	}
	if string(data["custom"]) != `{"value":9007199254740993}` || string(data["sni"]) != `"origin.example.com"` || string(data["add"]) != `"104.16.0.1"` {
		t.Fatal("VMess extensions or TLS hostname lost")
	}
}
