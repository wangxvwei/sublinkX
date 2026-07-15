package models

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

type XUISource struct {
	gorm.Model
	ID              int
	Name            string `gorm:"uniqueIndex"`
	Host            string
	SSHPort         int
	Username        string
	AuthType        string
	Password        string `json:"-"`
	PrivateKey      string `json:"-"`
	PanelBaseURL    string
	APIToken        string `json:"-"`
	XUIDBPath       string
	SubBaseURL      string
	SubPath         string
	GroupName       string
	NamePrefix      string
	RewriteRules    string `gorm:"type:text"`
	DeleteMissing   bool
	Enabled         bool
	LastSyncAt      *time.Time
	LastSyncStatus  string
	LastSyncMessage string `gorm:"type:text"`
}

type XUISourceInput struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Host          string `json:"host"`
	SSHPort       int    `json:"sshPort"`
	Username      string `json:"username"`
	AuthType      string `json:"authType"`
	Password      string `json:"password"`
	PrivateKey    string `json:"privateKey"`
	PanelBaseURL  string `json:"panelBaseUrl"`
	APIToken      string `json:"apiToken"`
	XUIDBPath     string `json:"xuiDbPath"`
	SubBaseURL    string `json:"subBaseUrl"`
	SubPath       string `json:"subPath"`
	GroupName     string `json:"groupName"`
	NamePrefix    string `json:"namePrefix"`
	RewriteRules  string `json:"rewriteRules"`
	DeleteMissing bool   `json:"deleteMissing"`
	Enabled       bool   `json:"enabled"`
}

type XUISourceView struct {
	ID              int        `json:"id"`
	Name            string     `json:"name"`
	Host            string     `json:"host"`
	SSHPort         int        `json:"sshPort"`
	Username        string     `json:"username"`
	AuthType        string     `json:"authType"`
	PanelBaseURL    string     `json:"panelBaseUrl"`
	XUIDBPath       string     `json:"xuiDbPath"`
	SubBaseURL      string     `json:"subBaseUrl"`
	SubPath         string     `json:"subPath"`
	GroupName       string     `json:"groupName"`
	NamePrefix      string     `json:"namePrefix"`
	RewriteRules    string     `json:"rewriteRules"`
	DeleteMissing   bool       `json:"deleteMissing"`
	Enabled         bool       `json:"enabled"`
	HasPassword     bool       `json:"hasPassword"`
	HasAPIToken     bool       `json:"hasApiToken"`
	LastSyncAt      *time.Time `json:"lastSyncAt"`
	LastSyncStatus  string     `json:"lastSyncStatus"`
	LastSyncMessage string     `json:"lastSyncMessage"`
}

func (s XUISource) View() XUISourceView {
	authType := s.normalizedAuthType()
	return XUISourceView{
		ID:              s.ID,
		Name:            s.Name,
		Host:            s.Host,
		SSHPort:         s.SSHPort,
		Username:        s.Username,
		AuthType:        authType,
		PanelBaseURL:    s.PanelBaseURL,
		XUIDBPath:       s.XUIDBPath,
		SubBaseURL:      s.SubBaseURL,
		SubPath:         s.SubPath,
		GroupName:       s.GroupName,
		NamePrefix:      s.NamePrefix,
		RewriteRules:    s.RewriteRules,
		DeleteMissing:   s.DeleteMissing,
		Enabled:         s.Enabled,
		HasPassword:     s.Password != "",
		HasAPIToken:     s.APIToken != "",
		LastSyncAt:      s.LastSyncAt,
		LastSyncStatus:  s.LastSyncStatus,
		LastSyncMessage: s.LastSyncMessage,
	}
}

func (s XUISource) normalizedAuthType() string {
	authType := strings.TrimSpace(s.AuthType)
	if authType == "" {
		if s.APIToken != "" {
			return "apiToken"
		}
		return "password"
	}
	if authType == "apiToken" {
		return "apiToken"
	}
	return "password"
}

func ListXUISources() ([]XUISourceView, error) {
	var sources []XUISource
	if err := DB.Order("id asc").Find(&sources).Error; err != nil {
		return nil, err
	}
	views := make([]XUISourceView, 0, len(sources))
	for _, source := range sources {
		views = append(views, source.View())
	}
	return views, nil
}

func SaveXUISource(input XUISourceInput) (XUISourceView, error) {
	source := XUISource{}
	if input.ID > 0 {
		if err := DB.First(&source, input.ID).Error; err != nil {
			return XUISourceView{}, err
		}
	} else {
		name := strings.TrimSpace(input.Name)
		if name != "" {
			var existing XUISource
			if err := DB.Unscoped().Where("name = ?", name).First(&existing).Error; err == nil {
				source = existing
				source.DeletedAt = gorm.DeletedAt{}
			}
		}
	}

	source.Name = strings.TrimSpace(input.Name)
	source.Host = strings.TrimSpace(input.Host)
	source.SSHPort = input.SSHPort
	source.Username = strings.TrimSpace(input.Username)
	source.PanelBaseURL = strings.TrimRight(strings.TrimSpace(input.PanelBaseURL), "/")
	source.XUIDBPath = strings.TrimSpace(input.XUIDBPath)
	source.SubBaseURL = strings.TrimSpace(input.SubBaseURL)
	source.SubPath = strings.Trim(input.SubPath, "/ ")
	source.GroupName = strings.TrimSpace(input.GroupName)
	source.NamePrefix = strings.TrimSpace(input.NamePrefix)
	source.RewriteRules = strings.TrimSpace(input.RewriteRules)
	source.DeleteMissing = input.DeleteMissing
	source.Enabled = input.Enabled
	if _, err := parseXUINodeRewriteRules(source.RewriteRules); err != nil {
		return XUISourceView{}, err
	}
	if source.SSHPort == 0 {
		source.SSHPort = 22
	}
	if source.XUIDBPath == "" {
		source.XUIDBPath = "/etc/x-ui/x-ui.db"
	}
	if source.SubBaseURL == "" {
		if strings.TrimSpace(input.AuthType) == "apiToken" && source.PanelBaseURL != "" {
			source.SubBaseURL = source.PanelBaseURL
		} else {
			source.SubBaseURL = "https://127.0.0.1:2096"
		}
	}
	if source.SubPath == "" {
		source.SubPath = "dingyue"
	}
	if source.GroupName == "" {
		source.GroupName = source.Name
	}
	if source.NamePrefix == "" && source.Name != "" {
		source.NamePrefix = "[" + source.Name + "] "
	}
	authType := strings.TrimSpace(input.AuthType)
	if authType == "" {
		authType = source.normalizedAuthType()
	}
	switch authType {
	case "password":
		if strings.TrimSpace(input.Password) != "" {
			source.Password = input.Password
		}
		source.PrivateKey = ""
		source.APIToken = ""
	case "apiToken":
		if strings.TrimSpace(input.APIToken) != "" {
			source.APIToken = strings.TrimSpace(input.APIToken)
		}
		source.Password = ""
		source.PrivateKey = ""
	default:
		return XUISourceView{}, errors.New("authType must be password or apiToken")
	}
	source.AuthType = authType
	if source.Name == "" {
		return XUISourceView{}, errors.New("name is required")
	}
	if authType == "password" {
		if source.Host == "" || source.Username == "" {
			return XUISourceView{}, errors.New("host and username are required")
		}
		if source.Password == "" {
			return XUISourceView{}, errors.New("password is required")
		}
	}
	if authType == "apiToken" {
		if source.PanelBaseURL == "" {
			return XUISourceView{}, errors.New("panel base url is required")
		}
		if source.APIToken == "" {
			return XUISourceView{}, errors.New("api token is required")
		}
	}

	if source.ID == 0 {
		if err := DB.Create(&source).Error; err != nil {
			return XUISourceView{}, err
		}
	} else {
		if err := DB.Save(&source).Error; err != nil {
			return XUISourceView{}, err
		}
	}
	return source.View(), nil
}

func DeleteXUISource(id int) error {
	return DB.Unscoped().Delete(&XUISource{}, id).Error
}

func SyncXUISourceByID(id int) (XUISyncResult, error) {
	var source XUISource
	if err := DB.First(&source, id).Error; err != nil {
		return XUISyncResult{}, err
	}
	result, err := source.Sync()
	source.markSync(err)
	return result, err
}

func SyncAllEnabledXUISources() ([]XUISourceSyncResult, error) {
	var sources []XUISource
	if err := DB.Where("enabled = ?", true).Order("id asc").Find(&sources).Error; err != nil {
		return nil, err
	}
	results := make([]XUISourceSyncResult, 0, len(sources))
	for _, source := range sources {
		result, err := source.Sync()
		source.markSync(err)
		item := XUISourceSyncResult{
			Source: source.View(),
			Result: result,
		}
		if err != nil {
			item.Error = err.Error()
		}
		results = append(results, item)
	}
	return results, nil
}

type XUISourceSyncResult struct {
	Source XUISourceView `json:"source"`
	Result XUISyncResult `json:"result"`
	Error  string        `json:"error,omitempty"`
}

func (s *XUISource) markSync(syncErr error) {
	now := time.Now()
	s.LastSyncAt = &now
	if syncErr != nil {
		s.LastSyncStatus = "failed"
		s.LastSyncMessage = syncErr.Error()
	} else {
		s.LastSyncStatus = "success"
		s.LastSyncMessage = ""
	}
	DB.Model(s).Select("LastSyncAt", "LastSyncStatus", "LastSyncMessage").Updates(s)
}

func (s XUISource) Sync() (XUISyncResult, error) {
	nodes, err := s.fetchRemoteNodes()
	if err != nil {
		return XUISyncResult{}, err
	}
	nodes, err = applyNodeLinkOverrides(nodes, s.publicHost(), s.RewriteRules)
	if err != nil {
		return XUISyncResult{}, err
	}
	return UpsertXUINodeLinks(nodes, XUINodeLinkOptions{
		SourceName:    fmt.Sprintf("3x-ui-source:%d", s.ID),
		GroupName:     s.GroupName,
		NamePrefix:    s.NamePrefix,
		DeleteMissing: s.DeleteMissing,
	})
}

func (s XUISource) publicHost() string {
	if strings.TrimSpace(s.Host) != "" {
		return strings.TrimSpace(s.Host)
	}
	if strings.TrimSpace(s.PanelBaseURL) == "" {
		return ""
	}
	parsed, err := url.Parse(s.PanelBaseURL)
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}

func applyNodeLinkOverrides(nodes []XUINodeLink, publicHost, rawRules string) ([]XUINodeLink, error) {
	rules, err := parseXUINodeRewriteRules(rawRules)
	if err != nil {
		return nodes, err
	}
	for i := range nodes {
		outputLink := nodes[i].Link
		if link, ok := rewriteLocalhostNodeLink(outputLink, publicHost); ok {
			outputLink = link
		}
		if len(rules) > 0 {
			link, changed, err := rewriteNodeLinkByRules(XUINodeLink{
				Name:      nodes[i].Name,
				Link:      outputLink,
				SubID:     nodes[i].SubID,
				SourceKey: nodes[i].SourceKey,
			}, rules)
			if err != nil {
				return nodes, err
			}
			if changed {
				outputLink = link
			}
		}
		if outputLink != nodes[i].Link {
			nodes[i].LinkOverride = outputLink
		} else {
			nodes[i].LinkOverride = ""
		}
	}
	return nodes, nil
}

func rewriteLocalhostNodeLinks(nodes []XUINodeLink, publicHost string) []XUINodeLink {
	publicHost = strings.TrimSpace(publicHost)
	if publicHost == "" {
		return nodes
	}
	for i := range nodes {
		link, ok := rewriteLocalhostNodeLink(nodes[i].Link, publicHost)
		if ok {
			nodes[i].Link = link
		}
	}
	return nodes
}

func rewriteLocalhostNodeLink(link, publicHost string) (string, bool) {
	parsed, err := url.Parse(link)
	if err != nil || parsed.Host == "" {
		return link, false
	}
	host := strings.Trim(strings.ToLower(parsed.Hostname()), "[]")
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return link, false
	}
	if port := parsed.Port(); port != "" {
		parsed.Host = net.JoinHostPort(publicHost, port)
	} else {
		parsed.Host = publicHost
	}
	return parsed.String(), true
}

type xuiNodeRewriteRule struct {
	NameContains string `json:"nameContains"`
	Protocol     string `json:"protocol"`
	Transport    string `json:"transport"`
	Security     string `json:"security"`
	SNI          string `json:"sni"`
	Host         string `json:"host"`
	Fingerprint  string `json:"fp"`
	Fingerprint2 string `json:"fingerprint"`
	ALPN         string `json:"alpn"`
	Path         string `json:"path"`
	Flow         string `json:"flow"`
	Address      string `json:"address"`
	Port         string `json:"port"`
}

func parseXUINodeRewriteRules(raw string) ([]xuiNodeRewriteRule, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var rules []xuiNodeRewriteRule
	if err := json.Unmarshal([]byte(raw), &rules); err == nil {
		return rules, nil
	}
	var rule xuiNodeRewriteRule
	if err := json.Unmarshal([]byte(raw), &rule); err != nil {
		return nil, fmt.Errorf("rewriteRules must be valid JSON: %w", err)
	}
	return []xuiNodeRewriteRule{rule}, nil
}

func rewriteNodeLinksByRules(nodes []XUINodeLink, rawRules string) ([]XUINodeLink, error) {
	rules, err := parseXUINodeRewriteRules(rawRules)
	if err != nil {
		return nodes, err
	}
	if len(rules) == 0 {
		return nodes, nil
	}
	for i := range nodes {
		link, changed, err := rewriteNodeLinkByRules(nodes[i], rules)
		if err != nil {
			return nodes, err
		}
		if changed {
			nodes[i].Link = link
		}
	}
	return nodes, nil
}

func rewriteNodeLinkByRules(node XUINodeLink, rules []xuiNodeRewriteRule) (string, bool, error) {
	parsed, err := url.Parse(node.Link)
	if err != nil || parsed.Host == "" {
		return node.Link, false, nil
	}
	changed := false
	query := parsed.Query()
	for _, rule := range rules {
		if !xuiRewriteRuleMatches(rule, node, parsed, query) {
			continue
		}
		if applyXUIRewriteRule(rule, parsed, query) {
			changed = true
		}
	}
	if !changed {
		return node.Link, false, nil
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), true, nil
}

func xuiRewriteRuleMatches(rule xuiNodeRewriteRule, node XUINodeLink, parsed *url.URL, query url.Values) bool {
	if rule.NameContains != "" && !strings.Contains(strings.ToLower(node.Name), strings.ToLower(rule.NameContains)) {
		return false
	}
	if rule.Protocol != "" && !strings.EqualFold(parsed.Scheme, rule.Protocol) {
		return false
	}
	if rule.Transport != "" {
		transport := firstNonEmpty(query.Get("type"), query.Get("network"))
		if !strings.EqualFold(transport, rule.Transport) {
			return false
		}
	}
	return true
}

func applyXUIRewriteRule(rule xuiNodeRewriteRule, parsed *url.URL, query url.Values) bool {
	changed := false
	setQuery := func(key, value string) {
		value = strings.TrimSpace(value)
		if value == "" || query.Get(key) == value {
			return
		}
		query.Set(key, value)
		changed = true
	}
	setQuery("security", rule.Security)
	setQuery("sni", rule.SNI)
	setQuery("host", rule.Host)
	setQuery("fp", firstNonEmpty(rule.Fingerprint, rule.Fingerprint2))
	setQuery("alpn", rule.ALPN)
	setQuery("path", rule.Path)
	setQuery("flow", rule.Flow)

	address := strings.TrimSpace(rule.Address)
	port := strings.TrimSpace(rule.Port)
	if address != "" || port != "" {
		if address == "" {
			address = parsed.Hostname()
		}
		if port == "" {
			port = parsed.Port()
		}
		nextHost := address
		if port != "" {
			nextHost = net.JoinHostPort(address, port)
		}
		if parsed.Host != nextHost {
			parsed.Host = nextHost
			changed = true
		}
	}
	return changed
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (s XUISource) fetchRemoteNodes() ([]XUINodeLink, error) {
	if s.normalizedAuthType() == "apiToken" {
		return s.fetchAPINodes()
	}
	client, err := s.sshClient()
	if err != nil {
		return nil, err
	}
	defer client.Close()

	script := remoteXUISyncScript()
	payload := map[string]string{
		"xui_db_path":  s.XUIDBPath,
		"sub_base_url": s.SubBaseURL,
		"sub_path":     s.SubPath,
	}
	payloadJSON, _ := json.Marshal(payload)
	cmd := fmt.Sprintf("python3 - <<'PY'\nimport base64, json\npayload=json.loads(base64.b64decode(%q).decode())\n%s\nPY", base64.StdEncoding.EncodeToString(payloadJSON), script)

	var stdout, stderr bytes.Buffer
	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()
	session.Stdout = &stdout
	session.Stderr = &stderr
	if err := session.Run(cmd); err != nil {
		return nil, fmt.Errorf("remote sync failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	var remote remoteXUIResponse
	if err := json.Unmarshal(stdout.Bytes(), &remote); err != nil {
		return nil, fmt.Errorf("parse remote sync output failed: %w: %s", err, stdout.String())
	}
	if remote.Error != "" {
		return nil, errors.New(remote.Error)
	}
	return remote.Nodes, nil
}

type xuiAPIResponse struct {
	Success bool            `json:"success"`
	Msg     string          `json:"msg"`
	Obj     json.RawMessage `json:"obj"`
}

type xuiAPIInbound struct {
	ID       int             `json:"id"`
	Enable   any             `json:"enable"`
	Settings json.RawMessage `json:"settings"`
}

func (s XUISource) fetchAPINodes() ([]XUINodeLink, error) {
	panelBaseURL := strings.TrimRight(s.PanelBaseURL, "/")
	if panelBaseURL == "" {
		return nil, errors.New("missing panel base url")
	}
	inbounds, err := s.fetchAPIInbounds(panelBaseURL)
	if err != nil {
		return nil, err
	}

	subBaseURL := strings.TrimRight(s.SubBaseURL, "/")
	subPath := strings.Trim(s.SubPath, "/ ")
	if subBaseURL == "" || subBaseURL == panelBaseURL || subPath == "" {
		detectedBaseURL, detectedPath, err := s.detectAPISubscriptionSettings(panelBaseURL)
		if err == nil {
			if subBaseURL == "" || subBaseURL == panelBaseURL {
				subBaseURL = detectedBaseURL
			}
			if subPath == "" || subPath == "dingyue" {
				subPath = detectedPath
			}
		}
	}
	if subBaseURL == "" {
		subBaseURL = panelOriginURL(panelBaseURL)
	}
	if subPath == "" {
		subPath = "dingyue"
	}

	subCache := map[string][]string{}
	nodes := []XUINodeLink{}
	clientsSeen := 0
	var lastSubscriptionErr error
	for _, inbound := range inbounds {
		if !isXUIInboundEnabled(inbound.Enable) {
			continue
		}
		settings, err := parseXUIInboundSettings(inbound.Settings)
		if err != nil {
			continue
		}
		for _, client := range settings.Clients {
			client.ID = strings.TrimSpace(client.ID)
			client.Email = strings.TrimSpace(client.Email)
			client.SubID = strings.TrimSpace(client.SubID)
			if client.ID == "" || client.SubID == "" {
				continue
			}
			clientsSeen++
			links, ok := subCache[client.SubID]
			if !ok {
				links, err = fetchXUISubscription(subBaseURL, subPath, client.SubID)
				if err != nil {
					lastSubscriptionErr = err
					subCache[client.SubID] = []string{}
					continue
				}
				subCache[client.SubID] = links
			}
			entry := xuiClientEntry{
				InboundID: inbound.ID,
				ID:        client.ID,
				Email:     client.Email,
				SubID:     client.SubID,
			}
			link := findClientLink(links, entry)
			if link == "" {
				continue
			}
			nodes = append(nodes, XUINodeLink{
				Name:      entry.DisplayName(),
				Link:      link,
				SubID:     client.SubID,
				SourceKey: fmt.Sprintf("%s:%s", entry.SourceKey(), client.SubID),
			})
		}
	}
	if len(nodes) == 0 && clientsSeen > 0 {
		if lastSubscriptionErr != nil {
			return nil, fmt.Errorf("found %d x-ui clients but no subscription links were fetched from %s/%s: %w", clientsSeen, subBaseURL, subPath, lastSubscriptionErr)
		}
		return nil, fmt.Errorf("found %d x-ui clients but no matching subscription links were found from %s/%s", clientsSeen, subBaseURL, subPath)
	}
	return nodes, nil
}

func isXUIInboundEnabled(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case float64:
		return v != 0
	case string:
		v = strings.TrimSpace(strings.ToLower(v))
		return v == "true" || v == "1"
	case nil:
		return true
	default:
		return true
	}
}

func (s XUISource) fetchAPIInbounds(panelBaseURL string) ([]xuiAPIInbound, error) {
	endpoint, err := joinPanelAPIURL(panelBaseURL, "/panel/api/inbounds/list")
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.APIToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	client := http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: insecureTLSConfig(),
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("3x-ui api returned %s", resp.Status)
	}
	var apiResp xuiAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("parse 3x-ui api response failed: %w", err)
	}
	if !apiResp.Success {
		if strings.TrimSpace(apiResp.Msg) != "" {
			return nil, errors.New(apiResp.Msg)
		}
		return nil, errors.New("3x-ui api returned success=false")
	}
	var inbounds []xuiAPIInbound
	if err := json.Unmarshal(apiResp.Obj, &inbounds); err != nil {
		return nil, fmt.Errorf("parse 3x-ui inbounds failed: %w", err)
	}
	return inbounds, nil
}

func (s XUISource) detectAPISubscriptionSettings(panelBaseURL string) (string, string, error) {
	endpoint, err := joinPanelAPIURL(panelBaseURL, "/panel/api/server/getDb")
	if err != nil {
		return "", "", err
	}
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+s.APIToken)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	client := http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: insecureTLSConfig(),
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("3x-ui getDb returned %s", resp.Status)
	}
	if !bytes.HasPrefix(body, []byte("SQLite format 3")) {
		return "", "", errors.New("3x-ui getDb did not return a sqlite database")
	}

	tmp, err := os.CreateTemp("", "sublink-xui-*.db")
	if err != nil {
		return "", "", err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		return "", "", err
	}
	if err := tmp.Close(); err != nil {
		return "", "", err
	}

	xuiDB, err := gorm.Open(sqlite.Open(tmpPath), &gorm.Config{})
	if err != nil {
		return "", "", err
	}
	subEnable := strings.TrimSpace(strings.ToLower(readXUISetting(xuiDB, "subEnable")))
	if subEnable == "false" || subEnable == "0" {
		return "", "", errors.New("3x-ui subscription is disabled")
	}
	subPath := readXUISetting(xuiDB, "subPath")
	if subPath == "" {
		subPath = "dingyue"
	}
	subPort := readXUISetting(xuiDB, "subPort")
	subBaseURL := panelOriginURL(panelBaseURL)
	if subPort != "" {
		subBaseURL = panelOriginURLWithPort(panelBaseURL, subPort)
	}
	return subBaseURL, subPath, nil
}

func parseXUIInboundSettings(raw json.RawMessage) (xuiInboundSettings, error) {
	var settings xuiInboundSettings
	if len(raw) == 0 || string(raw) == "null" {
		return settings, nil
	}
	if raw[0] == '"' {
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return settings, err
		}
		err := json.Unmarshal([]byte(text), &settings)
		return settings, err
	}
	err := json.Unmarshal(raw, &settings)
	return settings, err
}

func joinPanelAPIURL(baseURL, apiPath string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("panel base url must include scheme and host")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + apiPath
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func panelOriginURL(baseURL string) string {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return strings.TrimRight(baseURL, "/")
	}
	parsed.Path = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/")
}

func panelOriginURLWithPort(baseURL, port string) string {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return strings.TrimRight(baseURL, "/")
	}
	host := parsed.Hostname()
	if host == "" {
		return panelOriginURL(baseURL)
	}
	parsed.Host = net.JoinHostPort(host, strings.TrimSpace(port))
	parsed.Path = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/")
}

func (s XUISource) sshClient() (*ssh.Client, error) {
	auth := []ssh.AuthMethod{}
	if s.PrivateKey != "" {
		signer, err := ssh.ParsePrivateKey([]byte(s.PrivateKey))
		if err != nil {
			return nil, err
		}
		auth = append(auth, ssh.PublicKeys(signer))
	}
	if s.Password != "" {
		auth = append(auth, ssh.Password(s.Password))
	}
	if len(auth) == 0 {
		return nil, errors.New("missing ssh auth method")
	}
	config := &ssh.ClientConfig{
		User:            s.Username,
		Auth:            auth,
		Timeout:         15 * time.Second,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	return ssh.Dial("tcp", net.JoinHostPort(s.Host, strconv.Itoa(s.SSHPort)), config)
}

type remoteXUIResponse struct {
	Nodes []XUINodeLink `json:"nodes"`
	Error string        `json:"error"`
}

func remoteXUISyncScript() string {
	return `
import base64
import json
import sqlite3
import ssl
import sys
import urllib.parse
import urllib.request

def emit(obj):
    print(json.dumps(obj, ensure_ascii=False))

def read_setting(cur, key, default=""):
    try:
        row = cur.execute("select value from settings where key=? limit 1", (key,)).fetchone()
        if row and row[0]:
            return str(row[0]).strip("/ ")
    except Exception:
        pass
    return default

def split_links(raw):
    raw = raw.strip()
    if "://" in raw:
        return [x for x in raw.replace("\r\n", "\n").split() if "://" in x]
    for candidate in (raw, raw + "=" * ((4 - len(raw) % 4) % 4)):
        for decoder in (base64.b64decode, base64.urlsafe_b64decode):
            try:
                decoded = decoder(candidate).decode()
                if "://" in decoded:
                    return [x for x in decoded.replace("\r\n", "\n").split() if "://" in x]
            except Exception:
                pass
    return []

def find_link(links, client_id, email):
    for link in links:
        try:
            u = urllib.parse.urlparse(link)
            if u.username == client_id:
                return link
        except Exception:
            pass
    for link in links:
        if client_id and client_id in link:
            return link
    for link in links:
        try:
            if urllib.parse.unquote(urllib.parse.urlparse(link).fragment) == email:
                return link
        except Exception:
            pass
    if len(links) == 1:
        return links[0]
    return ""

try:
    db_path = payload.get("xui_db_path") or "/etc/x-ui/x-ui.db"
    sub_base_url = (payload.get("sub_base_url") or "https://127.0.0.1:2096").rstrip("/")
    sub_path = (payload.get("sub_path") or "dingyue").strip("/")
    con = sqlite3.connect(db_path)
    cur = con.cursor()
    if not payload.get("sub_path"):
        sub_path = read_setting(cur, "subPath", sub_path)
    rows = cur.execute("select id, settings from inbounds where enable=1").fetchall()
    sub_cache = {}
    nodes = []
    ctx = ssl._create_unverified_context()
    for inbound_id, settings_raw in rows:
        try:
            settings = json.loads(settings_raw or "{}")
        except Exception:
            continue
        for client in settings.get("clients", []):
            client_id = str(client.get("id") or "").strip()
            email = str(client.get("email") or client_id).strip()
            sub_id = str(client.get("subId") or "").strip()
            if not client_id or not sub_id:
                continue
            if sub_id not in sub_cache:
                url = sub_base_url + "/" + sub_path + "/" + urllib.parse.quote(sub_id)
                try:
                    with urllib.request.urlopen(url, context=ctx, timeout=15) as resp:
                        body = resp.read().decode()
                    sub_cache[sub_id] = split_links(body)
                except Exception:
                    sub_cache[sub_id] = []
            link = find_link(sub_cache[sub_id], client_id, email)
            if not link:
                continue
            nodes.append({
                "name": email,
                "link": link,
                "subId": sub_id,
                "sourceKey": "inbound:%s:client:%s:sub:%s" % (inbound_id, client_id, sub_id),
            })
    emit({"nodes": nodes})
except Exception as exc:
    emit({"error": str(exc)})
`
}

func LoadXUISourcesFromEnv() error {
	raw := strings.TrimSpace(os.Getenv("SUBLINK_XUI_SOURCES_JSON"))
	if raw == "" {
		return nil
	}
	var inputs []XUISourceInput
	if err := json.Unmarshal([]byte(raw), &inputs); err != nil {
		return err
	}
	for _, input := range inputs {
		var existing XUISource
		if err := DB.Where("name = ?", input.Name).First(&existing).Error; err == nil {
			input.ID = existing.ID
		}
		if _, err := SaveXUISource(input); err != nil {
			return err
		}
	}
	return nil
}
