package node

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSubscriptionConversionRegression(t *testing.T) {
	templatePath := filepath.Join(t.TempDir(), "clash.yaml")
	if err := os.WriteFile(templatePath, []byte("proxies: []\n"), 0600); err != nil {
		t.Fatal(err)
	}
	vless := "vless://11111111-1111-1111-1111-111111111111@example.com:443?security=reality&type=tcp&pbk=public-key&sid=abcd"
	for _, parameter := range []string{"", "true", "false", "1", "0"} {
		for _, encoded := range []bool{false, true} {
			t.Run(fmt.Sprintf("mlkem/%s/base64=%t", parameter, encoded), func(t *testing.T) {
				link := vless
				if parameter != "" {
					link += "&support-x25519mlkem768=" + parameter
				}
				if encoded {
					link = "vless://" + Base64Encode(link[len("vless://"):])
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
					t.Fatalf("expected one proxy: %s", got)
				}
				opts := config.Proxies[0].Reality_opts
				value, present := opts["support-x25519mlkem768"]
				if present != (parameter != "") || (present && value != (parameter == "true" || parameter == "1")) {
					t.Fatalf("ML-KEM value/type not preserved: %#v", opts)
				}
			})
		}
	}

	t.Run("vless-roundtrip", func(t *testing.T) {
		link := "vless://id@[2001:db8::1]:443?security=reality&type=grpc&serviceName=service&mode=multi&alpn=h2,http/1.1&pbk=key&support-x25519mlkem768=false#custom-name"
		original, err := DecodeVLESSURL(link)
		if err != nil {
			t.Fatal(err)
		}
		got, err := DecodeVLESSURL(EncodeVLESSURL(original))
		if err != nil || !reflect.DeepEqual(got, original) {
			t.Fatalf("VLESS roundtrip lost fields: %#v, %v", got, err)
		}
	})

	t.Run("vmess-numeric-port-and-tls", func(t *testing.T) {
		link := "vmess://" + Base64Encode(`{"add":"example.com","port":443,"id":"uuid","net":"tcp","tls":"tls","sni":"tls.example.com","fp":"chrome","alpn":"h2,http/1.1"}`)
		vmess, err := DecodeVMESSURL(link)
		if err != nil || vmess.Ps != "example.com:443" {
			t.Fatalf("numeric VMess port failed: %#v, %v", vmess, err)
		}
		vmess.Ps = ""
		got, err := EncodeClash([]string{EncodeVmessURL(vmess)}, SqlConfig{Clash: templatePath})
		if err != nil {
			t.Fatal(err)
		}
		var config Config
		if err := yaml.Unmarshal(got, &config); err != nil {
			t.Fatal(err)
		}
		p := config.Proxies[0]
		if p.Servername != "tls.example.com" || p.Client_fingerprint != "chrome" || len(p.Alpn) != 2 || p.Ws_opts != nil || p.AlterId != "0" {
			t.Fatalf("VMess TLS fields missing or wrong transport options: %#v", p)
		}
	})

	t.Run("ss-and-ssr-ipv6", func(t *testing.T) {
		ss, err := DecodeSSURL("ss://" + Base64Encode("aes-256-gcm:pass:word") + "@[2001:db8:443::1]:443#ss")
		if err != nil || ss.Server != "2001:db8:443::1" || ss.Param.Password != "pass:word" {
			t.Fatalf("SS IPv6 or password corrupted: %#v, %v", ss, err)
		}
		ssr, err := DecodeSSRURL("ssr://" + Base64Encode("[2001:db8::1]:443:origin:aes-256-cfb:plain:"+Base64Encode("secret")+"/?remarks="+Base64Encode("name")))
		if err != nil || ssr.Server != "2001:db8::1" || ssr.Password != "secret" || ssr.Qurey.Remarks != "name" {
			t.Fatalf("SSR fields corrupted: %#v, %v", ssr, err)
		}
	})

	t.Run("relay-does-not-stop-later-groups", func(t *testing.T) {
		template := "proxy-groups:\n  - name: chain\n    type: relay\n    proxies: [DIRECT]\n  - name: select\n    type: select\n    proxies: []\n"
		if err := os.WriteFile(templatePath, []byte(template), 0600); err != nil {
			t.Fatal(err)
		}
		got, err := DecodeClash([]Proxy{{Name: "node"}}, templatePath)
		if err != nil {
			t.Fatal(err)
		}
		var config Config
		if err := yaml.Unmarshal(got, &config); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(config.Proxy_groups[0].Proxies, []string{"DIRECT"}) || !reflect.DeepEqual(config.Proxy_groups[1].Proxies, []string{"node"}) {
			t.Fatalf("incorrect group insertion: %s", got)
		}
	})

	t.Run("invalid-inputs-return-errors", func(t *testing.T) {
		for _, link := range []string{"vless://id@example.com:0", "vless://example.com:443", vless + "&support-x25519mlkem768=invalid"} {
			if _, err := DecodeVLESSURL(link); err == nil {
				t.Fatalf("accepted invalid VLESS link: %s", link)
			}
		}
		if _, err := DecodeVMESSURL("vmess://" + Base64Encode(`{"add":"example.com","port":443.5,"id":"uuid"}`)); err == nil {
			t.Fatal("accepted fractional VMess port")
		}
		if _, err := DecodeSSRURL("ssr://" + Base64Encode("example.com:443:origin:aes-256-cfb:plain:cGFzcw==/?broken")); err == nil {
			t.Fatal("accepted malformed SSR query")
		}
		for _, template := range []string{"proxy-groups: invalid\n", "proxy-groups:\n  - name: bad\n    proxies: invalid\n"} {
			if err := os.WriteFile(templatePath, []byte(template), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := DecodeClash(nil, templatePath); err == nil {
				t.Fatal("accepted malformed Clash proxy groups")
			}
		}
	})
}
