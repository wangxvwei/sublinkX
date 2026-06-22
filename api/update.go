package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
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
	Message        string `json:"message"`
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
		info := updateInfo{
			CurrentVersion: currentVersion,
			DockerImage:    envOrDefault("DOCKER_IMAGE", "ghcr.io/wangxvwei/sublinkx"),
		}

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
		info.UpdateCommand = fmt.Sprintf("docker pull %s:latest && docker compose up -d", info.DockerImage)

		c.JSON(http.StatusOK, gin.H{
			"code": "00000",
			"data": info,
			"msg":  "update check",
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
