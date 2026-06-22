package models

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

const xuiNodeSource = "3x-ui"

type XUISyncOptions struct {
	XUIDBPath           string
	SubscriptionBaseURL string
	SubscriptionPath    string
	GroupName           string
	NamePrefix          string
	DeleteMissing       bool
}

type XUISyncResult struct {
	Created   int              `json:"created"`
	Updated   int              `json:"updated"`
	Unchanged int              `json:"unchanged"`
	Skipped   int              `json:"skipped"`
	Deleted   int              `json:"deleted"`
	Nodes     []XUISyncedNode  `json:"nodes"`
	SkippedOn []XUISkippedNode `json:"skippedOn"`
}

type XUISyncedNode struct {
	Name      string `json:"name"`
	SubID     string `json:"subId"`
	SourceKey string `json:"sourceKey"`
	Action    string `json:"action"`
	Hash      string `json:"hash"`
}

type XUISkippedNode struct {
	Name   string `json:"name"`
	SubID  string `json:"subId"`
	Reason string `json:"reason"`
}

type XUINodeLink struct {
	Name         string `json:"name"`
	Link         string `json:"link"`
	LinkOverride string `json:"linkOverride"`
	SubID        string `json:"subId"`
	SourceKey    string `json:"sourceKey"`
}

type XUINodeLinkOptions struct {
	SourceName    string
	GroupName     string
	NamePrefix    string
	DeleteMissing bool
}

type xuiInbound struct {
	ID       int
	Enable   int
	Settings string
}

type xuiClient struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	SubID string `json:"subId"`
}

type xuiInboundSettings struct {
	Clients []xuiClient `json:"clients"`
}

type xuiSetting struct {
	Key   string `gorm:"column:key"`
	Value string `gorm:"column:value"`
}

func SyncXUINodes(opts XUISyncOptions) (XUISyncResult, error) {
	opts = normalizeXUISyncOptions(opts)
	var result XUISyncResult

	if _, err := os.Stat(opts.XUIDBPath); err != nil {
		return result, fmt.Errorf("x-ui db not found: %w", err)
	}

	xuiDB, err := gorm.Open(sqlite.Open(opts.XUIDBPath), &gorm.Config{})
	if err != nil {
		return result, err
	}

	if opts.SubscriptionPath == "" {
		opts.SubscriptionPath = readXUISetting(xuiDB, "subPath")
	}
	if opts.SubscriptionPath == "" {
		opts.SubscriptionPath = "dingyue"
	}

	clients, err := readXUIClients(xuiDB)
	if err != nil {
		return result, err
	}

	subCache := map[string][]string{}
	nodeLinks := []XUINodeLink{}

	for _, client := range clients {
		sourceKey := fmt.Sprintf("%s:%s", client.SourceKey(), client.SubID)

		if client.SubID == "" {
			result.Skipped++
			result.SkippedOn = append(result.SkippedOn, XUISkippedNode{
				Name:   client.DisplayName(),
				SubID:  client.SubID,
				Reason: "empty subId",
			})
			continue
		}

		links, ok := subCache[client.SubID]
		if !ok {
			links, err = fetchXUISubscription(opts.SubscriptionBaseURL, opts.SubscriptionPath, client.SubID)
			if err != nil {
				result.Skipped++
				result.SkippedOn = append(result.SkippedOn, XUISkippedNode{
					Name:   client.DisplayName(),
					SubID:  client.SubID,
					Reason: err.Error(),
				})
				continue
			}
			subCache[client.SubID] = links
		}

		link := findClientLink(links, client)
		if link == "" {
			result.Skipped++
			result.SkippedOn = append(result.SkippedOn, XUISkippedNode{
				Name:   client.DisplayName(),
				SubID:  client.SubID,
				Reason: "matching link not found in subscription",
			})
			continue
		}

		nodeLinks = append(nodeLinks, XUINodeLink{
			Name:      client.DisplayName(),
			Link:      link,
			SubID:     client.SubID,
			SourceKey: sourceKey,
		})
	}

	written, err := UpsertXUINodeLinks(nodeLinks, XUINodeLinkOptions{
		SourceName:    xuiNodeSource,
		GroupName:     opts.GroupName,
		NamePrefix:    opts.NamePrefix,
		DeleteMissing: opts.DeleteMissing,
	})
	if err != nil {
		return result, err
	}
	result.Created += written.Created
	result.Updated += written.Updated
	result.Unchanged += written.Unchanged
	result.Deleted += written.Deleted
	result.Nodes = append(result.Nodes, written.Nodes...)

	return result, nil
}

func UpsertXUINodeLinks(nodeLinks []XUINodeLink, opts XUINodeLinkOptions) (XUISyncResult, error) {
	var result XUISyncResult
	if opts.SourceName == "" {
		opts.SourceName = xuiNodeSource
	}

	seen := map[string]bool{}
	usedNames := map[string]int{}
	for _, nodeLink := range nodeLinks {
		sourceKey := strings.TrimSpace(nodeLink.SourceKey)
		if sourceKey == "" {
			result.Skipped++
			result.SkippedOn = append(result.SkippedOn, XUISkippedNode{
				Name:   nodeLink.Name,
				SubID:  nodeLink.SubID,
				Reason: "empty source key",
			})
			continue
		}
		seen[sourceKey] = true
		if strings.TrimSpace(nodeLink.Link) == "" {
			result.Skipped++
			result.SkippedOn = append(result.SkippedOn, XUISkippedNode{
				Name:   nodeLink.Name,
				SubID:  nodeLink.SubID,
				Reason: "empty node link",
			})
			continue
		}

		name := uniqueXUIName(opts.NamePrefix+nodeLink.Name, usedNames)
		action, hash, err := upsertXUINode(name, nodeLink.Link, nodeLink.LinkOverride, opts.SourceName, sourceKey, nodeLink.SubID, opts.GroupName)
		if err != nil {
			result.Skipped++
			result.SkippedOn = append(result.SkippedOn, XUISkippedNode{
				Name:   name,
				SubID:  nodeLink.SubID,
				Reason: err.Error(),
			})
			continue
		}

		switch action {
		case "created":
			result.Created++
		case "updated":
			result.Updated++
		default:
			result.Unchanged++
		}
		result.Nodes = append(result.Nodes, XUISyncedNode{
			Name:      name,
			SubID:     nodeLink.SubID,
			SourceKey: sourceKey,
			Action:    action,
			Hash:      hash,
		})
	}

	if opts.DeleteMissing && len(seen) > 0 {
		deleted, err := deleteMissingXUINodes(opts.SourceName, seen)
		if err != nil {
			return result, err
		}
		result.Deleted = deleted
	}

	return result, nil
}

type xuiClientEntry struct {
	InboundID int
	ID        string
	Email     string
	SubID     string
}

func (c xuiClientEntry) SourceKey() string {
	return fmt.Sprintf("inbound:%d:client:%s", c.InboundID, c.ID)
}

func (c xuiClientEntry) DisplayName() string {
	if strings.TrimSpace(c.Email) != "" {
		return strings.TrimSpace(c.Email)
	}
	return c.ID
}

func normalizeXUISyncOptions(opts XUISyncOptions) XUISyncOptions {
	if opts.XUIDBPath == "" {
		opts.XUIDBPath = "/etc/x-ui/x-ui.db"
	}
	if opts.SubscriptionBaseURL == "" {
		opts.SubscriptionBaseURL = "https://127.0.0.1:2096"
	}
	if opts.GroupName == "" {
		opts.GroupName = "3x-ui"
	}
	return opts
}

func readXUIClients(xuiDB *gorm.DB) ([]xuiClientEntry, error) {
	var inbounds []xuiInbound
	if err := xuiDB.Raw("select id, enable, settings from inbounds where enable = 1").Scan(&inbounds).Error; err != nil {
		return nil, err
	}

	var clients []xuiClientEntry
	for _, inbound := range inbounds {
		var settings xuiInboundSettings
		if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
			continue
		}
		for _, client := range settings.Clients {
			if strings.TrimSpace(client.ID) == "" {
				continue
			}
			clients = append(clients, xuiClientEntry{
				InboundID: inbound.ID,
				ID:        strings.TrimSpace(client.ID),
				Email:     strings.TrimSpace(client.Email),
				SubID:     strings.TrimSpace(client.SubID),
			})
		}
	}
	return clients, nil
}

func readXUISetting(xuiDB *gorm.DB, key string) string {
	var setting xuiSetting
	if err := xuiDB.Raw("select key, value from settings where key = ? limit 1", key).Scan(&setting).Error; err != nil {
		return ""
	}
	return strings.Trim(setting.Value, "/ ")
}

func fetchXUISubscription(baseURL, subPath, subID string) ([]string, error) {
	u := strings.TrimRight(baseURL, "/") + "/" + strings.Trim(subPath, "/") + "/" + url.PathEscape(subID)
	client := http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: insecureTLSConfig(),
		},
	}

	resp, err := client.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("subscription returned %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return decodeSubscription(string(body))
}

func decodeSubscription(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("empty subscription")
	}
	if strings.Contains(raw, "://") {
		return splitSubscriptionLinks(raw), nil
	}

	candidates := []string{raw, padBase64(raw)}
	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	for _, candidate := range candidates {
		for _, encoding := range encodings {
			decoded, err := encoding.DecodeString(candidate)
			if err == nil && strings.Contains(string(decoded), "://") {
				return splitSubscriptionLinks(string(decoded)), nil
			}
		}
	}
	return nil, errors.New("subscription is not valid base64 node content")
}

func splitSubscriptionLinks(raw string) []string {
	parts := strings.Fields(strings.ReplaceAll(raw, "\r\n", "\n"))
	var links []string
	for _, part := range parts {
		if strings.Contains(part, "://") {
			links = append(links, strings.TrimSpace(part))
		}
	}
	return links
}

func padBase64(s string) string {
	switch len(s) % 4 {
	case 2:
		return s + "=="
	case 3:
		return s + "="
	default:
		return s
	}
}

func findClientLink(links []string, client xuiClientEntry) string {
	for _, link := range links {
		u, err := url.Parse(link)
		if err == nil && u.User != nil && u.User.Username() == client.ID {
			return link
		}
	}
	for _, link := range links {
		if strings.Contains(link, client.ID) {
			return link
		}
	}
	for _, link := range links {
		u, err := url.Parse(link)
		if err != nil {
			continue
		}
		name, _ := url.QueryUnescape(u.Fragment)
		if name == client.Email {
			return link
		}
	}
	if len(links) == 1 {
		return links[0]
	}
	return ""
}

func uniqueXUIName(base string, used map[string]int) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "3x-ui node"
	}
	used[base]++
	if used[base] == 1 {
		return base
	}
	return fmt.Sprintf("%s (%d)", base, used[base])
}

func upsertXUINode(name, link, linkOverride, source, sourceKey, subID, groupName string) (string, string, error) {
	if linkOverride == link {
		linkOverride = ""
	}
	hash := hashLink(link + "\n" + linkOverride)
	var existing Node
	err := DB.Unscoped().Where("source = ? and source_key = ?", source, sourceKey).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = DB.Where("link = ?", link).First(&existing).Error
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = DB.Unscoped().
			Where("source_key = ? and (source like ? or source like ?)", sourceKey, "3x-ui:%", "3x-ui-source:%").
			First(&existing).Error
	}

	action := "unchanged"
	if errors.Is(err, gorm.ErrRecordNotFound) {
		existing = Node{
			Name:         name,
			Link:         link,
			LinkOverride: linkOverride,
			Source:       source,
			SourceKey:    sourceKey,
			SubID:        subID,
		}
		if err := DB.Create(&existing).Error; err != nil {
			return "", "", err
		}
		action = "created"
	} else if err != nil {
		return "", "", err
	} else {
		changed := existing.Name != name || existing.Link != link || existing.LinkOverride != linkOverride || existing.Source != source || existing.SourceKey != sourceKey || existing.SubID != subID || existing.DeletedAt.Valid
		if changed {
			existing.Name = name
			existing.Link = link
			existing.LinkOverride = linkOverride
			existing.Source = source
			existing.SourceKey = sourceKey
			existing.SubID = subID
			existing.DeletedAt = gorm.DeletedAt{}
			if err := DB.Unscoped().Save(&existing).Error; err != nil {
				return "", "", err
			}
			action = "updated"
		}
	}

	if groupName != "" {
		if err := replaceNodeGroup(&existing, groupName); err != nil {
			return "", "", err
		}
	}

	return action, hash, nil
}

func replaceNodeGroup(node *Node, groupName string) error {
	groupName = strings.TrimSpace(groupName)
	if groupName == "" {
		return nil
	}

	var current Node
	if err := DB.Preload("GroupNodes").First(&current, node.ID).Error; err != nil {
		return err
	}
	oldGroups := current.GroupNodes

	group := GroupNode{Name: groupName}
	if err := DB.FirstOrCreate(&group, GroupNode{Name: groupName}).Error; err != nil {
		return err
	}
	if err := DB.Model(&current).Association("GroupNodes").Replace(&group); err != nil {
		return err
	}
	return IsGroupNotDel(oldGroups)
}

func deleteMissingXUINodes(source string, seen map[string]bool) (int, error) {
	var nodes []Node
	if err := DB.Where("source = ?", source).Find(&nodes).Error; err != nil {
		return 0, err
	}
	deleted := 0
	for _, node := range nodes {
		if seen[node.SourceKey] {
			continue
		}
		if err := node.Del(); err != nil {
			return deleted, err
		}
		deleted++
	}
	return deleted, nil
}

func hashLink(link string) string {
	sum := sha256.Sum256([]byte(link))
	return hex.EncodeToString(sum[:])[:16]
}

func insecureTLSConfig() *tls.Config {
	return &tls.Config{InsecureSkipVerify: true}
}
