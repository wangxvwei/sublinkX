package models

import (
	"context"
	"crypto/tls"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/netip"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sublink/node"
	"sync"
	"time"

	"gorm.io/gorm"
)

type CDNResult struct {
	IP              string     `json:"ip"`
	Delay           float64    `json:"delay"`
	Loss            float64    `json:"loss"`
	Speed           float64    `json:"speed"`
	Colo            string     `json:"colo"`
	TLSOK           bool       `json:"tlsOK"`
	Link            string     `json:"link"`
	Name            string     `json:"name"`
	Recommendation  bool       `json:"recommendation,omitempty"`
	Pinned          bool       `json:"pinned,omitempty"`
	Sources         []string   `json:"sources,omitempty"`
	SourceUpdatedAt *time.Time `json:"sourceUpdatedAt,omitempty"`
	HistorySamples  int        `json:"historySamples,omitempty"`
	Basis           string     `json:"basis,omitempty"`
	Cached          bool       `json:"cached,omitempty"`
}

type CDNProfile struct {
	ID            uint        `json:"id" gorm:"primaryKey"`
	Name          string      `json:"name"`
	TemplateLink  string      `json:"templateLink" gorm:"type:text"`
	CandidateMode string      `json:"candidateMode"`
	SourceIP      string      `json:"sourceIP"`
	Carrier       string      `json:"carrier"`
	Network       CDNNetwork  `json:"network" gorm:"serializer:json;type:text"`
	IPs           string      `json:"ips" gorm:"type:text"`
	Count         int         `json:"count"`
	URL           string      `json:"url"`
	MaxDelay      float64     `json:"maxDelay"`
	MaxLoss       float64     `json:"maxLoss"`
	MinSpeed      float64     `json:"minSpeed"`
	Duration      int         `json:"duration"`
	GroupName     string      `json:"groupName"`
	NamingVersion int         `json:"-"`
	Status        string      `json:"status"`
	Message       string      `json:"message"`
	Results       []CDNResult `json:"results" gorm:"serializer:json;type:text"`
	PinnedIPs     []string    `json:"pinnedIPs" gorm:"serializer:json;type:text"`
	StartedAt     *time.Time  `json:"startedAt"`
	FinishedAt    *time.Time  `json:"finishedAt"`
	UpdatedAt     time.Time   `json:"updatedAt"`
}

// ponytail: one process per installation avoids competing bandwidth tests;
// introduce workers only if independent network interfaces need parallel runs.
var cdnTask struct {
	sync.Mutex
	id     uint
	cancel context.CancelFunc
}

func CDNExecutable() (string, error) {
	path := os.Getenv("CFST_PATH")
	if path == "" {
		path = "cfst"
	}
	return exec.LookPath(path)
}

func PublicCDNIP(raw string) (string, error) {
	a, err := netip.ParseAddr(strings.TrimSpace(raw))
	if err != nil || a.Zone() != "" {
		return "", fmt.Errorf("不是有效 IP：%s", raw)
	}
	a = a.Unmap()
	if !a.IsGlobalUnicast() || a.IsPrivate() || a.IsLoopback() || a.IsLinkLocalUnicast() ||
		netip.MustParsePrefix("100.64.0.0/10").Contains(a) || netip.MustParsePrefix("198.18.0.0/15").Contains(a) ||
		netip.MustParsePrefix("192.0.2.0/24").Contains(a) || netip.MustParsePrefix("198.51.100.0/24").Contains(a) ||
		netip.MustParsePrefix("203.0.113.0/24").Contains(a) || netip.MustParsePrefix("240.0.0.0/4").Contains(a) ||
		netip.MustParsePrefix("2001:db8::/32").Contains(a) || netip.MustParsePrefix("0.0.0.0/8").Contains(a) {
		return "", fmt.Errorf("请提供有效的公网 IP：%s", raw)
	}
	return a.String(), nil
}

func normalizeCDN(p *CDNProfile) error {
	p.Name = strings.TrimSpace(p.Name)
	p.TemplateLink = strings.TrimSpace(p.TemplateLink)
	p.GroupName = strings.TrimSpace(p.GroupName)
	p.URL = strings.TrimSpace(p.URL)
	p.SourceIP = strings.TrimSpace(p.SourceIP)
	if p.Name == "" || len(p.Name) > 120 || len(p.GroupName) > 120 || len(p.IPs) > 256*1024 {
		return errors.New("配置名称必填，且输入内容不能过长")
	}
	if _, err := node.ParseCDNTemplate(p.TemplateLink); err != nil {
		return err
	}
	// Existing saved profiles keep their explicit candidate lists.
	if p.CandidateMode == "" {
		p.CandidateMode = "manual"
	}
	if p.CandidateMode != "recommend" && p.CandidateMode != "manual" {
		return errors.New("请选择异地线路推荐或本机测速")
	}
	if p.CandidateMode == "recommend" && p.SourceIP == "" {
		return errors.New("请填写目标地区的公网 IPv4")
	}
	if p.Carrier != "" && p.Carrier != "CT" && p.Carrier != "CU" && p.Carrier != "CM" {
		return errors.New("运营商须为自动识别、电信、联通或移动")
	}
	if p.SourceIP != "" {
		ip, err := PublicCDNIP(p.SourceIP)
		if err != nil || !netip.MustParseAddr(ip).Is4() {
			return errors.New("请输入目标地区有效的公网 IPv4")
		}
		p.SourceIP = ip
	}
	if p.Count < 5 || p.Count > 10 || p.Duration < 3 || p.Duration > 20 ||
		math.IsNaN(p.MaxDelay) || math.IsInf(p.MaxDelay, 0) || p.MaxDelay < 1 || p.MaxDelay > 9999 ||
		math.IsNaN(p.MaxLoss) || math.IsInf(p.MaxLoss, 0) || p.MaxLoss < 0 || p.MaxLoss > 1 ||
		math.IsNaN(p.MinSpeed) || math.IsInf(p.MinSpeed, 0) || p.MinSpeed < 0 || p.MinSpeed > 10000 {
		return errors.New("数量须为 5–10，测速时长 3–20 秒，延迟/失败率/速度阈值须在有效范围内")
	}
	u, err := url.Parse(p.URL)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || len(p.URL) > 2048 || strings.ContainsAny(p.URL, "\r\n") {
		return errors.New("测速地址必须是有效的 HTTPS 下载地址")
	}
	if p.CandidateMode == "recommend" {
		p.IPs = "" // A home public IP must never become a CDN destination.
		return nil
	}
	parts := strings.FieldsFunc(p.IPs, func(r rune) bool { return r == ',' || r == '，' || r == '\n' || r == '\r' || r == ' ' || r == '\t' })
	if len(parts) == 0 || len(parts) > 4096 {
		return errors.New("请输入 1–4096 个 IP 地址，每行一个；当前不展开 CIDR 网段")
	}
	ips := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, s := range parts {
		ip, err := PublicCDNIP(s)
		if err != nil {
			return err
		}
		if !seen[ip] {
			ips = append(ips, ip)
			seen[ip] = true
		}
	}
	p.IPs = strings.Join(ips, "\n")
	return nil
}

func SaveCDNProfile(p *CDNProfile, rename bool) error {
	if err := normalizeCDN(p); err != nil {
		return err
	}
	cdnTask.Lock()
	defer cdnTask.Unlock()
	if cdnTask.id != 0 && cdnTask.id == p.ID {
		return errors.New("任务运行中，请先停止再修改配置")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var duplicates int64
		if err := tx.Model(&CDNProfile{}).Where("name = ? AND id <> ?", p.Name, p.ID).Count(&duplicates).Error; err != nil {
			return err
		}
		if duplicates > 0 {
			return errors.New("已有同名优选配置，请填写不同名称以区分各地线路")
		}
		old := CDNProfile{}
		if p.ID != 0 {
			if err := tx.First(&old, p.ID).Error; err != nil {
				return err
			}
			if p.Name != old.Name && !rename {
				return errors.New("保存配置不能直接改名；新增地点请新建配置，修改名称请使用重命名")
			}
		}
		// Keep saved results when only the user-defined name changes.
		settingsUnchanged := old.ID != 0 && old.TemplateLink == p.TemplateLink && old.CandidateMode == p.CandidateMode &&
			old.SourceIP == p.SourceIP && old.Carrier == p.Carrier && old.IPs == p.IPs && old.Count == p.Count &&
			old.URL == p.URL && old.MaxDelay == p.MaxDelay && old.MaxLoss == p.MaxLoss && old.MinSpeed == p.MinSpeed && old.Duration == p.Duration
		p.PinnedIPs = old.PinnedIPs // Retention changes use the dedicated, validated action.
		if old.TemplateLink != p.TemplateLink || old.SourceIP != p.SourceIP || old.Carrier != p.Carrier || old.CandidateMode != p.CandidateMode {
			p.PinnedIPs = nil
		}
		if len(p.PinnedIPs) > p.Count {
			return fmt.Errorf("已保留 %d 个 IP，请先取消部分保留，再减少推荐数量", len(p.PinnedIPs))
		}
		p.GroupName, p.NamingVersion, p.Results = old.GroupName, old.NamingVersion, old.Results
		p.Status, p.Message, p.Network, p.StartedAt, p.FinishedAt = old.Status, old.Message, old.Network, old.StartedAt, old.FinishedAt
		if p.ID != 0 {
			if err := syncCDNNaming(tx, p); err != nil {
				return err
			}
		}
		if !settingsUnchanged {
			p.Status, p.Message, p.Results, p.StartedAt, p.FinishedAt = "idle", "配置已保存，点击开始优选", nil, nil, nil
			p.Network = CDNNetwork{}
		}
		p.GroupName, p.NamingVersion = p.Name, 1
		return tx.Save(p).Error
	})
}

func StartCDN(id uint) error {
	cdnTask.Lock()
	defer cdnTask.Unlock()
	if cdnTask.id != 0 {
		return errors.New("已有测速任务运行中，请等待完成或停止它")
	}
	var p CDNProfile
	if err := DB.First(&p, id).Error; err != nil {
		return err
	}
	if err := normalizeCDN(&p); err != nil {
		return err
	}
	var bin string
	if p.CandidateMode != "recommend" {
		var err error
		bin, err = CDNExecutable()
		if err != nil {
			return errors.New("未找到 CloudflareSpeedTest，请配置 CFST_PATH")
		}
	}
	now := time.Now()
	p.Status, p.Message, p.Results, p.StartedAt, p.FinishedAt = "running", "正在测试 IP 延迟和下载速度", nil, &now, nil
	p.Network = CDNNetwork{}
	if p.CandidateMode == "recommend" {
		p.Message = "正在识别目标地区和运营商，获取公开线路推荐"
	}
	if err := DB.Save(&p).Error; err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	cdnTask.id, cdnTask.cancel = id, cancel
	go runCDN(ctx, cancel, p, bin)
	return nil
}

func CancelCDN(id uint) error {
	cdnTask.Lock()
	defer cdnTask.Unlock()
	if cdnTask.id != id || cdnTask.cancel == nil {
		return errors.New("该任务未运行")
	}
	cdnTask.cancel()
	return nil
}

func runCDN(ctx context.Context, cancel context.CancelFunc, p CDNProfile, bin string) {
	defer func() {
		cancel()
		cdnTask.Lock()
		cdnTask.id, cdnTask.cancel = 0, nil
		cdnTask.Unlock()
	}()
	var results []CDNResult
	var err error
	if p.CandidateMode == "recommend" {
		results, err = recommendCDN(ctx, &p)
	} else {
		results, err = measureCDN(ctx, p, bin)
	}
	status, message := "completed", fmt.Sprintf("优选完成：目标 %d 个，实际 %d 个；已校验原节点 SNI 的 TLS 连接", p.Count, len(results))
	if p.CandidateMode == "recommend" {
		message = fmt.Sprintf("已生成 %d 条线路推荐（目标 %d 条）；综合公开来源、新鲜度与历史表现，非目标省份实测", len(results), p.Count)
	}
	if err != nil {
		status, message = "failed", err.Error()
	}
	if ctx.Err() != nil {
		status, message, results = "cancelled", "任务已停止，原有节点保留", nil
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			status, message = "failed", "任务超过 15 分钟，已停止"
		}
	}
	if status == "completed" && len(results) == 0 {
		message = "没有找到达标 IP。请检查测速地址、网络出口及阈值；未改动已有节点。"
	}
	now := time.Now()
	p.Status, p.Message, p.Results, p.FinishedAt = status, message, results, &now
	// Only this worker writes a running profile; HTTP edits are blocked above.
	if err := DB.Save(&p).Error; err != nil {
		fmt.Fprintln(os.Stderr, "保存 CDN 测速结果失败:", err)
	}
}

func measureCDN(ctx context.Context, p CDNProfile, bin string) ([]CDNResult, error) {
	template, err := node.ParseCDNTemplate(p.TemplateLink)
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "sublink-cfst-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	input, output := filepath.Join(dir, "ips.txt"), filepath.Join(dir, "result.csv")
	if err := os.WriteFile(input, []byte(p.IPs), 0600); err != nil {
		return nil, err
	}
	args := []string{"-f", input, "-o", output, "-n", "32", "-t", "4", "-dn", strconv.Itoa(p.Count * 3), "-dt", strconv.Itoa(p.Duration),
		"-tl", strconv.Itoa(int(math.Ceil(p.MaxDelay))), "-tlr", strconv.FormatFloat(p.MaxLoss, 'f', -1, 64),
		"-tp", strconv.Itoa(template.Port), "-url", p.URL, "-p", "0", "-debug"}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	var outputLog cdnOutput
	cmd.Stdout, cmd.Stderr = &outputLog, &outputLog
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("测速程序未正常完成，请检查可执行程序与网络连接：%w", err)
	}
	file, err := os.Open(output)
	if errors.Is(err, os.ErrNotExist) {
		return nil, errors.New("没有 IP 通过延迟和探测失败率筛选，请检查出口网络或调整阈值")
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	results, err := readCDNCSV(file, p)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		if code := regexp.MustCompile(`HTTP 状态码: (\d{3})`).FindSubmatch(outputLog.data); len(code) > 1 {
			return nil, fmt.Errorf("下载测速地址返回 HTTP %s，请在高级设置中更换测速地址", code[1])
		}
		return nil, errors.New("没有达到下载速度要求的 IP；可更换测速地址、延长测速时长或调整阈值")
	}
	DB.Model(&CDNProfile{}).Where("id = ?", p.ID).Update("message", "测速结束，正在校验原节点域名的 TLS 连接")
	selected := make([]CDNResult, 0, p.Count)
	for _, result := range results {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		checkCtx, stop := context.WithTimeout(ctx, 4*time.Second)
		dialer := tls.Dialer{NetDialer: &net.Dialer{Timeout: 4 * time.Second}, Config: &tls.Config{ServerName: template.SNI, MinVersion: tls.VersionTLS12}}
		conn, err := dialer.DialContext(checkCtx, "tcp", net.JoinHostPort(result.IP, strconv.Itoa(template.Port)))
		if conn != nil {
			conn.Close()
		}
		stop()
		if err != nil {
			continue
		}
		result.TLSOK = true
		result.Link, result.Name = template.WithIP(result.IP, cdnNodeName(p.Name, len(selected)))
		selected = append(selected, result)
		if len(selected) == p.Count {
			break
		}
	}
	if len(selected) == 0 {
		return nil, errors.New("CDN 测速有结果，但原节点 SNI 的 TLS 检查均未通过，请检查模板域名和 CDN 配置")
	}
	return selected, nil
}

// Keep diagnostics bounded and return only status codes, never credentialed URLs.
type cdnOutput struct{ data []byte }

func (b *cdnOutput) Write(p []byte) (int, error) {
	n := len(p)
	if len(p) > 16384 {
		p = p[len(p)-16384:]
	}
	b.data = append(b.data, p...)
	if len(b.data) > 16384 {
		b.data = b.data[len(b.data)-16384:]
	}
	return n, nil
}

func readCDNCSV(reader io.Reader, p CDNProfile) ([]CDNResult, error) {
	r := csv.NewReader(io.LimitReader(reader, 4*1024*1024))
	r.FieldsPerRecord = -1
	if _, err := r.Read(); err != nil {
		return nil, errors.New("测速结果文件无效")
	}
	allowed, seen := map[string]bool{}, map[string]bool{}
	for _, ip := range strings.Split(p.IPs, "\n") {
		allowed[ip] = true
	}
	results := []CDNResult{}
	for {
		row, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil || len(row) < 6 {
			return nil, errors.New("测速 CSV 格式不正确")
		}
		ip, err := PublicCDNIP(row[0])
		if err != nil || !allowed[ip] || seen[ip] {
			continue
		}
		values := make([]float64, 3)
		valid := true
		for i, col := range []int{3, 4, 5} {
			values[i], err = strconv.ParseFloat(row[col], 64)
			if err != nil || math.IsNaN(values[i]) || math.IsInf(values[i], 0) || values[i] < 0 {
				valid = false
			}
		}
		if !valid || values[0] > p.MaxLoss || values[1] > p.MaxDelay || values[2] <= 0 || values[2] < p.MinSpeed {
			continue
		}
		item := CDNResult{IP: ip, Loss: values[0], Delay: values[1], Speed: values[2]}
		if len(row) > 6 {
			item.Colo = row[6]
		}
		seen[ip] = true
		results = append(results, item)
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Speed != results[j].Speed {
			return results[i].Speed > results[j].Speed
		}
		return results[i].Delay < results[j].Delay
	})
	return results, nil
}

func ImportCDN(id uint, ips []string, subscriptionID int) (int, error) {
	cdnTask.Lock()
	defer cdnTask.Unlock()
	if cdnTask.id == id {
		return 0, errors.New("请等待测速完成再保存")
	}
	if len(ips) == 0 || len(ips) > 10 || subscriptionID < 0 {
		return 0, errors.New("请选择 1–10 个测速结果和有效分组")
	}
	var p CDNProfile
	if err := DB.First(&p, id).Error; err != nil {
		return 0, err
	}
	if p.Status != "completed" {
		return 0, errors.New("本次任务尚未完成")
	}
	if err := nameCDNResults(&p); err != nil {
		return 0, err
	}
	selected := map[string]bool{}
	for _, ip := range ips {
		selected[ip] = true
	}
	results := []CDNResult{}
	for _, result := range p.Results {
		if selected[result.IP] && (result.TLSOK || (p.CandidateMode == "recommend" && result.Recommendation)) && result.Link != "" {
			results = append(results, result)
			delete(selected, result.IP)
		}
	}
	if len(selected) != 0 {
		return 0, errors.New("选中 IP 不属于本次有效结果")
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		// Freeze legacy subscription order before replacing names/IPs.
		if err := syncCDNNaming(tx, &p); err != nil {
			return err
		}
		var sub Subcription
		if subscriptionID != 0 {
			if err := tx.Preload("Nodes").First(&sub, subscriptionID).Error; err != nil {
				return errors.New("目标订阅不存在")
			}
		}
		group := GroupNode{Name: p.Name}
		if err := tx.Where("name = ?", p.Name).FirstOrCreate(&group).Error; err != nil {
			return err
		}
		source := fmt.Sprintf("cdn:%d", id)
		var existing []Node
		if err := tx.Where("source = ?", source).Order("id").Find(&existing).Error; err != nil {
			return err
		}
		wanted := map[string]bool{}
		for _, result := range results {
			wanted[result.IP] = true
		}
		byIP := map[string]Node{}
		spare := []Node{}
		for _, n := range existing {
			if wanted[n.SourceKey] && byIP[n.SourceKey].ID == 0 {
				byIP[n.SourceKey] = n
			} else {
				spare = append(spare, n)
			}
		}
		nodes := []Node{}
		for _, result := range results {
			n := byIP[result.IP]
			// Reuse outgoing slots so existing subscriptions keep their node IDs.
			if n.ID == 0 && len(spare) > 0 {
				n, spare = spare[0], spare[1:]
			}
			n.Name, n.Link, n.Source, n.SourceKey = result.Name, result.Link, source, result.IP
			n.LinkOverride = ""
			if err := tx.Save(&n).Error; err != nil {
				return err
			}
			if err := tx.Model(&n).Association("GroupNodes").Append(&group); err != nil {
				return err
			}
			nodes = append(nodes, n)
		}
		if len(spare) > 0 {
			ids := make([]int, 0, len(spare))
			for _, n := range spare {
				ids = append(ids, n.ID)
			}
			if err := tx.Where("id IN ?", ids).Delete(&Node{}).Error; err != nil {
				return err
			}
			for _, table := range []string{"group_node_nodes", "subcription_nodes"} {
				if err := tx.Table(table).Where("node_id IN ?", ids).Delete(nil).Error; err != nil {
					return err
				}
			}
		}
		if subscriptionID != 0 {
			// Reload after obsolete slots were removed.
			if err := tx.Preload("Nodes").First(&sub, subscriptionID).Error; err != nil {
				return err
			}
			ordered := orderSubscriptionNodes(sub.Nodes, sub.NodeOrderIDs, sub.NodeOrder)
			seen := map[int]bool{}
			ids := []string{}
			for _, n := range append(ordered, nodes...) {
				if !seen[n.ID] {
					ids = append(ids, strconv.Itoa(n.ID))
					seen[n.ID] = true
				}
			}
			if err := tx.Model(&sub).Association("Nodes").Append(&nodes); err != nil {
				return err
			}
			if err := tx.Model(&sub).Update("node_order_ids", strings.Join(ids, ",")).Error; err != nil {
				return err
			}
		}
		return tx.Save(&p).Error
	})
	return len(results), err
}
