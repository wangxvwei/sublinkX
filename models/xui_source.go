package models

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

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
	Password        string `json:"-"`
	PrivateKey      string `json:"-"`
	XUIDBPath       string
	SubBaseURL      string
	SubPath         string
	GroupName       string
	NamePrefix      string
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
	XUIDBPath     string `json:"xuiDbPath"`
	SubBaseURL    string `json:"subBaseUrl"`
	SubPath       string `json:"subPath"`
	GroupName     string `json:"groupName"`
	NamePrefix    string `json:"namePrefix"`
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
	XUIDBPath       string     `json:"xuiDbPath"`
	SubBaseURL      string     `json:"subBaseUrl"`
	SubPath         string     `json:"subPath"`
	GroupName       string     `json:"groupName"`
	NamePrefix      string     `json:"namePrefix"`
	DeleteMissing   bool       `json:"deleteMissing"`
	Enabled         bool       `json:"enabled"`
	HasPassword     bool       `json:"hasPassword"`
	HasPrivateKey   bool       `json:"hasPrivateKey"`
	LastSyncAt      *time.Time `json:"lastSyncAt"`
	LastSyncStatus  string     `json:"lastSyncStatus"`
	LastSyncMessage string     `json:"lastSyncMessage"`
}

func (s XUISource) View() XUISourceView {
	authType := "password"
	if s.PrivateKey != "" && s.Password == "" {
		authType = "privateKey"
	}
	return XUISourceView{
		ID:              s.ID,
		Name:            s.Name,
		Host:            s.Host,
		SSHPort:         s.SSHPort,
		Username:        s.Username,
		AuthType:        authType,
		XUIDBPath:       s.XUIDBPath,
		SubBaseURL:      s.SubBaseURL,
		SubPath:         s.SubPath,
		GroupName:       s.GroupName,
		NamePrefix:      s.NamePrefix,
		DeleteMissing:   s.DeleteMissing,
		Enabled:         s.Enabled,
		HasPassword:     s.Password != "",
		HasPrivateKey:   s.PrivateKey != "",
		LastSyncAt:      s.LastSyncAt,
		LastSyncStatus:  s.LastSyncStatus,
		LastSyncMessage: s.LastSyncMessage,
	}
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
	}

	source.Name = strings.TrimSpace(input.Name)
	source.Host = strings.TrimSpace(input.Host)
	source.SSHPort = input.SSHPort
	source.Username = strings.TrimSpace(input.Username)
	source.XUIDBPath = strings.TrimSpace(input.XUIDBPath)
	source.SubBaseURL = strings.TrimSpace(input.SubBaseURL)
	source.SubPath = strings.Trim(input.SubPath, "/ ")
	source.GroupName = strings.TrimSpace(input.GroupName)
	source.NamePrefix = strings.TrimSpace(input.NamePrefix)
	source.DeleteMissing = input.DeleteMissing
	source.Enabled = input.Enabled
	if source.SSHPort == 0 {
		source.SSHPort = 22
	}
	if source.XUIDBPath == "" {
		source.XUIDBPath = "/etc/x-ui/x-ui.db"
	}
	if source.SubBaseURL == "" {
		source.SubBaseURL = "https://127.0.0.1:2096"
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
		authType = "password"
		if strings.TrimSpace(input.PrivateKey) != "" {
			authType = "privateKey"
		}
	}
	switch authType {
	case "password":
		if strings.TrimSpace(input.Password) != "" {
			source.Password = input.Password
		}
		source.PrivateKey = ""
	case "privateKey":
		if strings.TrimSpace(input.PrivateKey) != "" {
			source.PrivateKey = input.PrivateKey
		}
		source.Password = ""
	default:
		return XUISourceView{}, errors.New("authType must be password or privateKey")
	}
	if source.Name == "" || source.Host == "" || source.Username == "" {
		return XUISourceView{}, errors.New("name, host and username are required")
	}
	if source.Password == "" && source.PrivateKey == "" {
		return XUISourceView{}, errors.New("password or private key is required")
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
	return DB.Delete(&XUISource{}, id).Error
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
	return UpsertXUINodeLinks(nodes, XUINodeLinkOptions{
		SourceName:    "3x-ui:" + s.Name,
		GroupName:     s.GroupName,
		NamePrefix:    s.NamePrefix,
		DeleteMissing: s.DeleteMissing,
	})
}

func (s XUISource) fetchRemoteNodes() ([]XUINodeLink, error) {
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
