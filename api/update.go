package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type updateInfo struct {
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	HasUpdate      bool   `json:"hasUpdate"`
	ReleaseURL     string `json:"releaseUrl"`
	DockerImage    string `json:"dockerImage"`
	UpdateCommand  string `json:"updateCommand"`
	AutoUpdate     bool   `json:"autoUpdate"`
	AutoUpdateMsg  string `json:"autoUpdateMessage"`
	ContainerName  string `json:"containerName"`
	Message        string `json:"message"`
}

type updateApplyResult struct {
	ContainerName    string `json:"containerName"`
	DockerImage      string `json:"dockerImage"`
	UpdaterContainer string `json:"updaterContainer"`
	Message          string `json:"message"`
}

type updateStatus struct {
	Status        string `json:"status"`
	Message       string `json:"message"`
	Error         string `json:"error"`
	StartedAt     string `json:"startedAt"`
	FinishedAt    string `json:"finishedAt"`
	TargetImage   string `json:"targetImage"`
	PreviousImage string `json:"previousImage"`
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

type githubTag struct {
	Name string `json:"name"`
}

var updateHTTPClient = &http.Client{Timeout: 8 * time.Second}

func CheckUpdate(currentVersion string) gin.HandlerFunc {
	return func(c *gin.Context) {
		socketPath := dockerSocketPath()
		containerName := updateContainerName()
		info := updateInfo{
			CurrentVersion: currentVersion,
			DockerImage:    envOrDefault("DOCKER_IMAGE", "ghcr.io/wangxvwei/sublinkx"),
			ContainerName:  containerName,
		}
		info.AutoUpdate, info.AutoUpdateMsg = dockerSocketReady(socketPath)

		latest, releaseURL, err := fetchLatestVersion()
		if err != nil {
			info.Message = err.Error()
			c.JSON(http.StatusOK, gin.H{
				"code": "00000",
				"data": info,
				"msg":  "update check failed",
			})
			return
		}

		info.LatestVersion = latest
		info.ReleaseURL = releaseURL
		info.HasUpdate = compareVersion(latest, currentVersion) > 0
		if info.HasUpdate {
			info.Message = "发现新版本"
		} else {
			info.Message = "当前已是最新版本"
		}
		info.UpdateCommand = fmt.Sprintf("docker pull %s && docker compose up -d", dockerImageWithTag(info.DockerImage))

		c.JSON(http.StatusOK, gin.H{
			"code": "00000",
			"data": info,
			"msg":  "update check",
		})
	}
}

func UpdateStatus() gin.HandlerFunc {
	return func(c *gin.Context) {
		status, err := readUpdateStatus()
		if err != nil {
			status = updateStatus{
				Status:  "idle",
				Message: "暂无更新任务",
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"code": "00000",
			"data": status,
			"msg":  "update status",
		})
	}
}

func ApplyUpdate() gin.HandlerFunc {
	return func(c *gin.Context) {
		socketPath := dockerSocketPath()
		containerName := updateContainerName()
		image := dockerImageWithTag(envOrDefault("DOCKER_IMAGE", "ghcr.io/wangxvwei/sublinkx"))

		if ok, msg := dockerSocketReady(socketPath); !ok {
			c.JSON(http.StatusOK, gin.H{
				"code": "A0500",
				"msg":  msg,
			})
			return
		}

		docker, err := newDockerSocketClient(socketPath)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": "A0500", "msg": err.Error()})
			return
		}

		container, err := docker.inspectContainer(containerName)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code": "A0500",
				"msg":  fmt.Sprintf("读取容器 %s 配置失败：%s", containerName, err.Error()),
			})
			return
		}

		statusBind := updateStatusBind(container)
		if statusBind == "" {
			c.JSON(http.StatusOK, gin.H{
				"code": "A0500",
				"msg":  "未找到 /app/logs 挂载，无法记录更新状态和回滚结果，请先把 logs 目录挂载到容器。",
			})
			return
		}

		helperImage := envOrDefault("UPDATE_HELPER_IMAGE", "docker:27-cli")
		if err := docker.pullImage(helperImage); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code": "A0500",
				"msg":  fmt.Sprintf("准备更新助手镜像失败：%s", err.Error()),
			})
			return
		}

		previousImage := container.Image
		status := updateStatus{
			Status:        "running",
			Message:       "更新任务已创建，正在准备拉取新镜像。",
			StartedAt:     time.Now().UTC().Format(time.RFC3339),
			TargetImage:   image,
			PreviousImage: previousImage,
		}
		_ = writeUpdateStatus(status)

		script := buildUpdateScript(
			containerName,
			image,
			previousImage,
			buildReplacementRunCommand(container, containerName, image),
			buildReplacementRunCommand(container, containerName, previousImage),
			status.StartedAt,
		)

		updaterName := fmt.Sprintf("%s-updater-%d", containerName, time.Now().Unix())
		updaterID, err := docker.createUpdaterContainer(updaterName, helperImage, socketPath, statusBind, script)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code": "A0500",
				"msg":  fmt.Sprintf("创建更新助手容器失败：%s", err.Error()),
			})
			return
		}
		if err := docker.startContainer(updaterID); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code": "A0500",
				"msg":  fmt.Sprintf("启动更新助手容器失败：%s", err.Error()),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"code": "00000",
			"data": updateApplyResult{
				ContainerName:    containerName,
				DockerImage:      image,
				UpdaterContainer: updaterName,
				Message:          "更新已开始，容器会短暂重启，请稍后刷新页面。",
			},
			"msg": "update started",
		})
	}
}

func fetchLatestVersion() (string, string, error) {
	repo := envOrDefault("UPDATE_REPO", "wangxvwei/sublinkX")
	releaseURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	req, err := http.NewRequest(http.MethodGet, releaseURL, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "sublinkX-update-checker")

	resp, err := updateHTTPClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var release githubRelease
		if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
			return "", "", err
		}
		if release.TagName != "" {
			return release.TagName, release.HTMLURL, nil
		}
	}

	tagURL := fmt.Sprintf("https://api.github.com/repos/%s/tags?per_page=20", repo)
	req, err = http.NewRequest(http.MethodGet, tagURL, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "sublinkX-update-checker")

	resp, err = updateHTTPClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("GitHub 返回状态码 %d", resp.StatusCode)
	}

	var tags []githubTag
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return "", "", err
	}
	if len(tags) == 0 {
		return "", "", fmt.Errorf("未找到可用版本标签")
	}

	return tags[0].Name, fmt.Sprintf("https://github.com/%s/releases", repo), nil
}

func compareVersion(a, b string) int {
	left := versionNumbers(a)
	right := versionNumbers(b)
	max := len(left)
	if len(right) > max {
		max = len(right)
	}
	for i := 0; i < max; i++ {
		var l, r int
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			r = right[i]
		}
		if l > r {
			return 1
		}
		if l < r {
			return -1
		}
	}
	return 0
}

func versionNumbers(version string) []int {
	re := regexp.MustCompile(`\d+`)
	matches := re.FindAllString(strings.TrimPrefix(version, "v"), -1)
	if len(matches) > 3 {
		matches = matches[:3]
	}
	nums := make([]int, 0, len(matches))
	for _, item := range matches {
		n, _ := strconv.Atoi(item)
		nums = append(nums, n)
	}
	return nums
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func updateStatusPath() string {
	return filepath.Clean(envOrDefault("UPDATE_STATUS_FILE", "logs/update-status.json"))
}

func readUpdateStatus() (updateStatus, error) {
	var status updateStatus
	data, err := os.ReadFile(updateStatusPath())
	if err != nil {
		return status, err
	}
	err = json.Unmarshal(data, &status)
	return status, err
}

func writeUpdateStatus(status updateStatus) error {
	path := updateStatusPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(status)
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

type dockerSocketClient struct {
	client *http.Client
}

type dockerContainerInspect struct {
	Name   string `json:"Name"`
	Image  string `json:"Image"`
	Config struct {
		Env  []string `json:"Env"`
		User string   `json:"User"`
	} `json:"Config"`
	HostConfig struct {
		NetworkMode   string `json:"NetworkMode"`
		RestartPolicy struct {
			Name              string `json:"Name"`
			MaximumRetryCount int    `json:"MaximumRetryCount"`
		} `json:"RestartPolicy"`
	} `json:"HostConfig"`
	Mounts []struct {
		Type        string `json:"Type"`
		Name        string `json:"Name"`
		Source      string `json:"Source"`
		Destination string `json:"Destination"`
		Mode        string `json:"Mode"`
		RW          bool   `json:"RW"`
	} `json:"Mounts"`
	NetworkSettings struct {
		Ports map[string][]struct {
			HostIP   string `json:"HostIp"`
			HostPort string `json:"HostPort"`
		} `json:"Ports"`
	} `json:"NetworkSettings"`
}

func dockerSocketPath() string {
	return envOrDefault("DOCKER_SOCKET", "/var/run/docker.sock")
}

func updateContainerName() string {
	return strings.TrimPrefix(envOrDefault("UPDATE_CONTAINER_NAME", "sublinkx"), "/")
}

func dockerSocketReady(path string) (bool, string) {
	info, err := os.Stat(path)
	if err != nil {
		return false, "未检测到 Docker socket，请先把 /var/run/docker.sock 挂载到容器。"
	}
	if info.IsDir() {
		return false, "Docker socket 路径不是文件，请检查挂载配置。"
	}
	conn, err := net.DialTimeout("unix", path, 2*time.Second)
	if err != nil {
		return false, "无法访问 Docker socket，请确认容器有权限访问 /var/run/docker.sock。"
	}
	conn.Close()
	return true, "已启用网页一键更新"
}

func newDockerSocketClient(socketPath string) (*dockerSocketClient, error) {
	if ok, msg := dockerSocketReady(socketPath); !ok {
		return nil, fmt.Errorf("%s", msg)
	}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
		},
	}
	return &dockerSocketClient{client: &http.Client{Transport: transport, Timeout: 120 * time.Second}}, nil
}

func (d *dockerSocketClient) do(method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, "http://docker"+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("Docker API %s %s returned %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}

func (d *dockerSocketClient) inspectContainer(name string) (dockerContainerInspect, error) {
	var result dockerContainerInspect
	err := d.do(http.MethodGet, "/containers/"+url.PathEscape(strings.TrimPrefix(name, "/"))+"/json", nil, &result)
	return result, err
}

func (d *dockerSocketClient) pullImage(image string) error {
	name, tag := splitImageTag(image)
	query := url.Values{}
	query.Set("fromImage", name)
	if tag != "" {
		query.Set("tag", tag)
	}
	return d.do(http.MethodPost, "/images/create?"+query.Encode(), nil, nil)
}

func (d *dockerSocketClient) createUpdaterContainer(name, image, socketPath, statusBind, script string) (string, error) {
	var response struct {
		ID string `json:"Id"`
	}
	body := map[string]any{
		"Image": image,
		"Cmd":   []string{"sh", "-c", script},
		"HostConfig": map[string]any{
			"AutoRemove": true,
			"Binds": []string{
				socketPath + ":/var/run/docker.sock",
				statusBind + ":/update-status",
			},
		},
	}
	err := d.do(http.MethodPost, "/containers/create?name="+url.QueryEscape(name), body, &response)
	return response.ID, err
}

func (d *dockerSocketClient) startContainer(id string) error {
	return d.do(http.MethodPost, "/containers/"+url.PathEscape(id)+"/start", nil, nil)
}

func buildReplacementRunCommand(container dockerContainerInspect, containerName, image string) string {
	args := []string{"docker", "run", "-d", "--name", containerName}

	restart := container.HostConfig.RestartPolicy
	if restart.Name != "" && restart.Name != "no" {
		args = append(args, "--restart")
		if restart.Name == "on-failure" && restart.MaximumRetryCount > 0 {
			args = append(args, fmt.Sprintf("on-failure:%d", restart.MaximumRetryCount))
		} else {
			args = append(args, restart.Name)
		}
	}

	if container.HostConfig.NetworkMode != "" && container.HostConfig.NetworkMode != "default" && container.HostConfig.NetworkMode != "bridge" {
		args = append(args, "--network", container.HostConfig.NetworkMode)
	}

	if container.Config.User != "" {
		args = append(args, "--user", container.Config.User)
	}

	for _, portArg := range containerPortArgs(container) {
		args = append(args, "-p", portArg)
	}
	for _, envArg := range containerEnvArgs(container, containerName, image) {
		args = append(args, "-e", envArg)
	}
	for _, mountArg := range containerMountArgs(container) {
		args = append(args, "-v", mountArg)
	}

	args = append(args, image)
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, shellQuote(arg))
	}
	return strings.Join(quoted, " ")
}

func buildUpdateScript(containerName, targetImage, previousImage, runNewCommand, runRollbackCommand, startedAt string) string {
	lines := []string{
		"set +e",
		"STATUS_FILE=/update-status/update-status.json",
		"CONTAINER_NAME=" + shellQuote(containerName),
		"TARGET_IMAGE=" + shellQuote(targetImage),
		"PREVIOUS_IMAGE=" + shellQuote(previousImage),
		"STARTED_AT=" + shellQuote(startedAt),
		"mkdir -p /update-status",
		"now_utc() { date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date; }",
		"sanitize() { printf '%s' \"$1\" | tr '\"' \"'\" | tr '\\n' ' '; }",
		"write_status() {",
		"  status=\"$1\"",
		"  message=$(sanitize \"$2\")",
		"  finished=\"$3\"",
		"  error=$(sanitize \"${4:-}\")",
		"  printf '{\"status\":\"%s\",\"message\":\"%s\",\"error\":\"%s\",\"startedAt\":\"%s\",\"finishedAt\":\"%s\",\"targetImage\":\"%s\",\"previousImage\":\"%s\"}\\n' \"$status\" \"$message\" \"$error\" \"$STARTED_AT\" \"$finished\" \"$TARGET_IMAGE\" \"$PREVIOUS_IMAGE\" > \"$STATUS_FILE\"",
		"}",
		"wait_container() {",
		"  name=\"$1\"",
		"  for i in $(seq 1 40); do",
		"    state=$(docker inspect -f '{{.State.Status}} {{if .State.Health}}{{.State.Health.Status}}{{end}}' \"$name\" 2>/dev/null || true)",
		"    case \"$state\" in",
		"      'running healthy'|'running ') return 0 ;;",
		"      *unhealthy*|*exited*|*dead*) return 1 ;;",
		"    esac",
		"    sleep 3",
		"  done",
		"  return 1",
		"}",
		"write_status running '正在拉取新镜像' ''",
		"if ! docker pull \"$TARGET_IMAGE\" >/tmp/update-pull.log 2>&1; then",
		"  err=$(tail -n 20 /tmp/update-pull.log 2>/dev/null)",
		"  write_status failed '更新失败，旧版本仍在运行' \"$(now_utc)\" \"$err\"",
		"  exit 1",
		"fi",
		"write_status running '正在重建容器' ''",
		"docker stop \"$CONTAINER_NAME\" >/tmp/update-stop.log 2>&1 || true",
		"docker rm \"$CONTAINER_NAME\" >/tmp/update-rm.log 2>&1 || true",
		"if ! new_id=$(" + runNewCommand + " 2>/tmp/update-run.log); then",
		"  err=$(tail -n 30 /tmp/update-run.log 2>/dev/null)",
		"  write_status running '新版本创建失败，正在回滚' '' \"$err\"",
		"  docker rm -f \"$CONTAINER_NAME\" >/dev/null 2>&1 || true",
		"  if rollback_id=$(" + runRollbackCommand + " 2>>/tmp/update-run.log); then",
		"    if wait_container \"$CONTAINER_NAME\"; then",
		"      write_status rolled_back '更新失败，已回滚到上一版本' \"$(now_utc)\" \"$err\"",
		"      exit 1",
		"    fi",
		"  fi",
		"  err=$(tail -n 60 /tmp/update-run.log 2>/dev/null)",
		"  write_status failed '更新失败，回滚也失败，请手动处理' \"$(now_utc)\" \"$err\"",
		"  exit 1",
		"fi",
		"if wait_container \"$CONTAINER_NAME\"; then",
		"  write_status success '更新成功，请刷新页面' \"$(now_utc)\" ''",
		"  exit 0",
		"fi",
		"docker logs --tail 80 \"$CONTAINER_NAME\" >/tmp/update-run.log 2>&1 || true",
		"err=$(cat /tmp/update-run.log 2>/dev/null)",
		"write_status running '新版本启动失败，正在回滚' '' \"$err\"",
		"docker rm -f \"$CONTAINER_NAME\" >/dev/null 2>&1 || true",
		"if rollback_id=$(" + runRollbackCommand + " 2>>/tmp/update-run.log); then",
		"  if wait_container \"$CONTAINER_NAME\"; then",
		"    write_status rolled_back '更新失败，已回滚到上一版本' \"$(now_utc)\" \"$err\"",
		"    exit 1",
		"  fi",
		"fi",
		"err=$(tail -n 80 /tmp/update-run.log 2>/dev/null)",
		"write_status failed '更新失败，回滚也失败，请手动处理' \"$(now_utc)\" \"$err\"",
		"exit 1",
	}
	return strings.Join(lines, "\n")
}

func updateStatusBind(container dockerContainerInspect) string {
	for _, mount := range container.Mounts {
		if path.Clean(mount.Destination) != "/app/logs" {
			continue
		}
		if mount.Type == "volume" && mount.Name != "" {
			return mount.Name
		}
		return mount.Source
	}
	return ""
}

func containerPortArgs(container dockerContainerInspect) []string {
	keys := make([]string, 0, len(container.NetworkSettings.Ports))
	for key := range container.NetworkSettings.Ports {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	seen := map[string]bool{}
	var args []string
	for _, containerPort := range keys {
		for _, binding := range container.NetworkSettings.Ports[containerPort] {
			if binding.HostPort == "" {
				continue
			}
			dedupeKey := binding.HostPort + ":" + containerPort
			if seen[dedupeKey] {
				continue
			}
			seen[dedupeKey] = true
			if binding.HostIP != "" && binding.HostIP != "0.0.0.0" && binding.HostIP != "::" {
				args = append(args, binding.HostIP+":"+binding.HostPort+":"+containerPort)
			} else {
				args = append(args, binding.HostPort+":"+containerPort)
			}
		}
	}
	return args
}

func containerEnvArgs(container dockerContainerInspect, containerName, image string) []string {
	envMap := map[string]string{}
	order := []string{}
	for _, item := range container.Config.Env {
		key, value, ok := strings.Cut(item, "=")
		if !ok || key == "" || key == "HOSTNAME" || key == "PATH" {
			continue
		}
		if _, exists := envMap[key]; !exists {
			order = append(order, key)
		}
		envMap[key] = value
	}
	for _, item := range []struct{ key, value string }{
		{"DOCKER_IMAGE", strings.TrimSuffix(image, ":latest")},
		{"UPDATE_CONTAINER_NAME", containerName},
	} {
		if _, exists := envMap[item.key]; !exists {
			order = append(order, item.key)
		}
		envMap[item.key] = item.value
	}

	var args []string
	for _, key := range order {
		args = append(args, key+"="+envMap[key])
	}
	return args
}

func containerMountArgs(container dockerContainerInspect) []string {
	var args []string
	for _, mount := range container.Mounts {
		source := mount.Source
		if mount.Type == "volume" && mount.Name != "" {
			source = mount.Name
		}
		if source == "" || mount.Destination == "" {
			continue
		}
		mode := mount.Mode
		if mode == "" && !mount.RW {
			mode = "ro"
		}
		arg := source + ":" + mount.Destination
		if mode != "" {
			arg += ":" + mode
		}
		args = append(args, arg)
	}
	return args
}

func splitImageTag(image string) (string, string) {
	lastSlash := strings.LastIndex(image, "/")
	lastColon := strings.LastIndex(image, ":")
	if lastColon > lastSlash {
		return image[:lastColon], image[lastColon+1:]
	}
	return image, "latest"
}

func dockerImageWithTag(image string) string {
	name, tag := splitImageTag(image)
	if tag == "" {
		tag = "latest"
	}
	return name + ":" + tag
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
