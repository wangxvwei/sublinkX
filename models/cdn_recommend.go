package models

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sublink/node"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"
)

const cdnRecommendationSource = "https://bestcf.pages.dev/uouin/all.txt"

type CDNNetwork struct {
	Location        string     `json:"location"`
	Province        string     `json:"province"`
	Carrier         string     `json:"carrier"`
	Source          string     `json:"source"`
	SourceUpdatedAt *time.Time `json:"sourceUpdatedAt"`
	Warning         string     `json:"warning"`
}

func cdnHTTP(ctx context.Context, address string) ([]byte, error) {
	dialer := net.Dialer{Timeout: 8 * time.Second}
	transport := &http.Transport{DialContext: dialer.DialContext, TLSHandshakeTimeout: 8 * time.Second}
	defer transport.CloseIdleConnections()
	client := http.Client{Transport: transport, Timeout: 12 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "sublinkX-CDN/2.3.6")
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.New("网络请求未完成")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("服务返回 HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 262145))
	if len(data) > 262144 {
		return nil, errors.New("服务返回内容过长")
	}
	return data, err
}

func cdnCarrier(text string) string {
	text = strings.ToLower(text)
	// Do not guess a university/company's carrier from its province.
	if strings.Contains(text, "联通") || strings.Contains(text, "unicom") || strings.Contains(text, "网通") {
		return "CU"
	}
	if strings.Contains(text, "电信") || strings.Contains(text, "telecom") {
		return "CT"
	}
	if strings.Contains(text, "移动") || strings.Contains(text, "mobile") || strings.Contains(text, "铁通") {
		return "CM"
	}
	return ""
}

func lookupCDNNetwork(ctx context.Context, p *CDNProfile) error {
	// Same attribution service as subscription access logs; only the target IP
	// is sent to the provider, never the template's credentials.
	data, err := cdnHTTP(ctx, "https://whois.pconline.com.cn/ipJson.jsp?json=true&ip="+url.QueryEscape(p.SourceIP))
	if err != nil {
		return fmt.Errorf("目标 IP 归属地查询失败：%w", err)
	}
	data, err = simplifiedchinese.GBK.NewDecoder().Bytes(data)
	if err != nil {
		return errors.New("IP 归属地响应编码无效")
	}
	var info struct {
		IP       string `json:"ip"`
		Addr     string `json:"addr"`
		Province string `json:"pro"`
	}
	if err := json.Unmarshal(data, &info); err != nil || info.IP != p.SourceIP || len(info.Addr) > 512 || info.Addr == "" || len(info.Province) > 128 {
		return errors.New("IP 归属地服务返回无效结果")
	}
	p.Network = CDNNetwork{Location: strings.TrimSpace(info.Addr), Province: info.Province, Carrier: p.Carrier}
	if p.Network.Carrier == "" {
		p.Network.Carrier = cdnCarrier(info.Addr)
	}
	if p.Network.Carrier == "" {
		// Ownership names may describe a university instead of its upstream ISP.
		if data, err := cdnHTTP(ctx, "https://api.ipapi.is/?q="+url.QueryEscape(p.SourceIP)); err == nil {
			var asn struct {
				IP  string `json:"ip"`
				ASN string `json:"asn"`
			}
			if json.Unmarshal(data, &asn) == nil && asn.IP == p.SourceIP {
				p.Network.Carrier = cdnCarrier(asn.ASN)
			}
		}
	}
	if p.Network.Carrier == "" {
		return errors.New("已识别地区，但无法确认电信/联通/移动；请在运营商选项中手动指定实际线路")
	}
	return nil
}

func cloudflareRanges(ctx context.Context) ([]netip.Prefix, error) {
	data, err := cdnHTTP(ctx, "https://api.cloudflare.com/client/v4/ips")
	if err != nil {
		return nil, fmt.Errorf("Cloudflare 官方地址段查询失败：%w", err)
	}
	var reply struct {
		Success bool `json:"success"`
		Result  struct {
			IPv4 []string `json:"ipv4_cidrs"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &reply); err != nil || !reply.Success || len(reply.Result.IPv4) == 0 || len(reply.Result.IPv4) > 64 {
		return nil, errors.New("Cloudflare 官方地址段列表无效")
	}
	ranges := []netip.Prefix{}
	for _, text := range reply.Result.IPv4 {
		prefix, err := netip.ParsePrefix(text)
		if err != nil || !prefix.Addr().Is4() || prefix.Bits() < 8 || prefix.Bits() > 32 {
			return nil, errors.New("Cloudflare 官方地址段格式无效")
		}
		ranges = append(ranges, prefix.Masked())
	}
	return ranges, nil
}

func inCDNRanges(ip netip.Addr, ranges []netip.Prefix) bool {
	for _, prefix := range ranges {
		if prefix.Contains(ip.Unmap()) {
			return true
		}
	}
	return false
}

func recommendCDN(ctx context.Context, p *CDNProfile) ([]CDNResult, error) {
	if err := lookupCDNNetwork(ctx, p); err != nil {
		return nil, err
	}
	p.Network.Warning = "数据源未提供该省独立测速记录，以下为同运营商通用推荐；非目标省份实测，延迟、下载速度与代理可用性均未验证。"
	p.Network.Source = cdnRecommendationSource
	ranges, err := cloudflareRanges(ctx)
	if err != nil {
		return nil, err
	}
	template, err := node.ParseCDNTemplate(p.TemplateLink)
	if err != nil {
		return nil, err
	}
	dnsCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	addresses, err := net.DefaultResolver.LookupNetIP(dnsCtx, "ip4", template.SNI)
	if err != nil {
		return nil, errors.New("原节点 SNI 域名解析失败，请检查 DNS")
	}
	isCloudflare := false
	for _, ip := range addresses {
		if inCDNRanges(ip, ranges) {
			isCloudflare = true
			break
		}
	}
	if !isCloudflare {
		return nil, errors.New("原节点域名未解析到 Cloudflare 官方地址；线路推荐目前仅支持 Cloudflare")
	}
	data, err := cdnHTTP(ctx, cdnRecommendationSource)
	if err != nil {
		return nil, fmt.Errorf("公开推荐数据暂时不可用：%w，请稍后重试", err)
	}
	return readCDNRecommendations(data, p, template, ranges, time.Now())
}

func readCDNRecommendations(data []byte, p *CDNProfile, template node.CDNTemplate, ranges []netip.Prefix, now time.Time) ([]CDNResult, error) {
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	header := strings.Split(lines[0], "|")
	if len(header) != 3 || len(lines) > 1000 {
		return nil, errors.New("公开推荐数据格式无效")
	}
	// The public feed timestamps its snapshots in China time without a year.
	zone := time.FixedZone("UTC+8", 8*3600)
	localNow := now.In(zone)
	updated, err := time.ParseInLocation("2006 01-02 15:04", fmt.Sprintf("%d %s", localNow.Year(), strings.TrimSpace(header[1])), zone)
	if err == nil && localNow.Month() == time.January && updated.Month() == time.December {
		updated = updated.AddDate(-1, 0, 0)
	}
	if err != nil || now.Sub(updated) > 48*time.Hour || updated.After(now.Add(5*time.Minute)) {
		return nil, errors.New("公开推荐数据已过期或更新时间无效，请稍后重试")
	}
	p.Network.SourceUpdatedAt = &updated
	results := []CDNResult{}
	seen := map[string]bool{}
	carriers := map[string]string{"电信": "CT", "联通": "CU", "移动": "CM"}
	for _, line := range lines[1:] {
		address, description, ok := strings.Cut(line, "#")
		fields := strings.Split(description, "|")
		if !ok || len(fields) != 5 || carriers[strings.TrimSpace(fields[1])] != p.Network.Carrier {
			continue
		}
		host, portText, err := net.SplitHostPort(strings.TrimSpace(address))
		if err != nil {
			continue
		}
		port, err := strconv.Atoi(portText)
		if err != nil || port != template.Port {
			continue
		}
		ip, err := PublicCDNIP(host)
		if err != nil || seen[ip] || ip == p.SourceIP || strings.TrimSpace(fields[2]) != ip || !inCDNRanges(netip.MustParseAddr(ip), ranges) {
			continue
		}
		seen[ip] = true
		link, name := template.WithIP(ip, cdnNodeName(p.Name, len(results)))
		results = append(results, CDNResult{IP: ip, Name: name, Link: link, Recommendation: true})
		if len(results) == p.Count {
			break
		}
	}
	if len(results) == 0 {
		return nil, errors.New("该运营商和端口暂时没有有效的 Cloudflare 推荐地址")
	}
	return results, nil
}
