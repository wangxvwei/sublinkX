package node

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// CDNTemplate keeps the original URL/JSON so replacing an address cannot discard
// transport extensions that the subscription converters do not yet understand.
type CDNTemplate struct {
	Name, Server, SNI, Host, Transport string
	Port                               int
	u                                  *url.URL
	v                                  map[string]json.RawMessage
}

func ParseCDNTemplate(raw string) (CDNTemplate, error) {
	t := CDNTemplate{}
	raw = strings.TrimSpace(raw)
	if len(raw) > 65536 || strings.ContainsAny(raw, "\r\n") {
		return t, fmt.Errorf("请输入一条完整节点链接")
	}
	if strings.HasPrefix(raw, "vmess://") {
		if err := json.Unmarshal([]byte(Base64Decode(strings.TrimPrefix(raw, "vmess://"))), &t.v); err != nil || t.v == nil {
			return t, fmt.Errorf("VMess 链接格式不正确")
		}
		get := func(k string) string { var s string; _ = json.Unmarshal(t.v[k], &s); return s }
		t.Server, t.SNI, t.Host, t.Name, t.Transport = get("add"), get("sni"), get("host"), get("ps"), get("net")
		var port interface{}
		_ = json.Unmarshal(t.v["port"], &port)
		t.Port, _ = convertToInt(port)
		if get("tls") != "tls" || t.Transport != "ws" || get("id") == "" {
			return t, fmt.Errorf("VMess 优选支持 WS + TLS，且必须有 UUID")
		}
	} else {
		if strings.HasPrefix(raw, "vless://") && !strings.Contains(strings.TrimPrefix(raw, "vless://"), "@") {
			raw = "vless://" + Base64Decode(strings.TrimPrefix(raw, "vless://"))
		}
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "vless" && u.Scheme != "trojan") {
			return t, fmt.Errorf("请选择 VLESS、VMess 或 Trojan 的 CDN 节点")
		}
		if u.User == nil || u.User.Username() == "" {
			return t, fmt.Errorf("节点缺少 UUID 或密码")
		}
		t.u = u
		q := u.Query()
		t.Server, t.SNI, t.Host, t.Name, t.Transport = u.Hostname(), q.Get("sni"), q.Get("host"), u.Fragment, q.Get("type")
		t.Port, _ = strconv.Atoi(u.Port())
		security := q.Get("security")
		if u.Scheme == "trojan" && security == "" {
			security = "tls"
		}
		if t.SNI == "" && u.Scheme == "trojan" {
			t.SNI = q.Get("peer")
		}
		if security != "tls" || q.Get("flow") != "" || (t.Transport != "ws" && t.Transport != "xhttp" && t.Transport != "grpc") || (u.Scheme == "trojan" && t.Transport != "ws") {
			return t, fmt.Errorf("需要 TLS CDN 节点：VLESS WS/XHTTP/gRPC 或 Trojan WS；不支持 Reality/直连 TCP")
		}
	}
	if t.Server == "" || t.Port < 1 || t.Port > 65535 {
		return t, fmt.Errorf("节点地址或端口不正确")
	}
	if t.Host == "" {
		if net.ParseIP(t.Server) == nil {
			t.Host = t.Server
		} else if t.SNI != "" {
			t.Host = t.SNI
		}
	}
	if t.SNI == "" {
		if net.ParseIP(t.Server) == nil {
			t.SNI = t.Server
		} else {
			t.SNI = t.Host
		}
	}
	if t.SNI == "" || net.ParseIP(t.SNI) != nil || strings.ContainsAny(t.SNI, ":/ \t\r\n") {
		return t, fmt.Errorf("请在原节点中填写有效的 SNI 域名，换 IP 时需要保留它")
	}
	if t.Host == "" || strings.ContainsAny(t.Host, "\r\n") {
		return t, fmt.Errorf("原节点缺少有效的 Host 域名")
	}
	if t.Name == "" {
		t.Name = t.Server
	}
	return t, nil
}

func (t CDNTemplate) WithIP(ip string) (string, string) {
	name := t.Name + " · CDN · " + ip
	if t.v != nil {
		v := make(map[string]json.RawMessage, len(t.v))
		for k, value := range t.v {
			v[k] = value
		}
		for k, value := range map[string]string{"add": ip, "ps": name, "sni": t.SNI, "host": t.Host} {
			v[k], _ = json.Marshal(value)
		}
		data, _ := json.Marshal(v)
		return "vmess://" + Base64Encode(string(data)), name
	}
	u := *t.u
	u.Host = net.JoinHostPort(ip, strconv.Itoa(t.Port))
	u.Fragment, u.RawFragment = name, ""
	q := u.Query()
	q.Set("sni", t.SNI)
	q.Set("host", t.Host)
	u.RawQuery = q.Encode()
	return u.String(), name
}

// RenameVMess preserves extensions during the existing subscription export path.
func RenameVMess(raw, name string) (string, error) {
	var data map[string]json.RawMessage
	if err := json.Unmarshal([]byte(Base64Decode(strings.TrimPrefix(raw, "vmess://"))), &data); err != nil || data == nil {
		return "", fmt.Errorf("invalid VMess JSON")
	}
	data["ps"], _ = json.Marshal(name)
	b, err := json.Marshal(data)
	return "vmess://" + Base64Encode(string(b)), err
}
