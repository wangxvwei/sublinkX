package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"runtime"
)

// Read the version from the exact published image, never from an unrelated Git
// tag or a moving branch. Digest comparison still decides container updates.
func fetchRemoteImageVersion(image, digest string) (string, error) {
	validDigest := regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
	if !validDigest.MatchString(digest) {
		return "", fmt.Errorf("镜像摘要无效")
	}
	name, _ := splitImageTag(image)
	registry, repository := splitRegistryRepository(name)
	token := ""
	get := func(kind, ref string, target interface{}) error {
		if !validDigest.MatchString(ref) {
			return fmt.Errorf("镜像元数据摘要无效")
		}
		endpoint := fmt.Sprintf("https://%s/v2/%s/%s/%s", registry, registryPath(repository), kind, url.PathEscape(ref))
		for attempt := 0; attempt < 2; attempt++ {
			req, err := http.NewRequest(http.MethodGet, endpoint, nil)
			if err != nil {
				return err
			}
			req.Header.Set("Accept", "application/vnd.oci.image.index.v1+json, application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.docker.distribution.manifest.v2+json")
			req.Header.Set("User-Agent", "sublinkX-update-checker")
			if token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}
			resp, err := updateHTTPClient.Do(req)
			if err != nil {
				return err
			}
			if resp.StatusCode == http.StatusUnauthorized && attempt == 0 {
				auth := resp.Header.Get("Www-Authenticate")
				resp.Body.Close()
				token, err = fetchRegistryToken(auth)
				if err != nil {
					return err
				}
				continue
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("镜像元数据返回状态码 %d", resp.StatusCode)
			}
			return json.NewDecoder(io.LimitReader(resp.Body, 2*1024*1024)).Decode(target)
		}
		return fmt.Errorf("无法读取镜像元数据")
	}
	for depth := 0; depth < 3; depth++ {
		var manifest struct {
			Config    struct{ Digest string }
			Manifests []struct {
				Digest   string
				Platform struct{ OS, Architecture string }
			}
		}
		if err := get("manifests", digest, &manifest); err != nil {
			return "", err
		}
		if manifest.Config.Digest != "" {
			var config struct {
				Config struct{ Labels map[string]string }
			}
			if err := get("blobs", manifest.Config.Digest, &config); err != nil {
				return "", err
			}
			version := config.Config.Labels["org.opencontainers.image.version"]
			if !regexp.MustCompile(`^v?\d+\.\d+\.\d+([-+][0-9A-Za-z.-]+)?$`).MatchString(version) {
				return "", fmt.Errorf("镜像未标注有效的应用版本号")
			}
			return version, nil
		}
		next := ""
		for _, m := range manifest.Manifests {
			if m.Platform.OS == "linux" && m.Platform.Architecture == runtime.GOARCH {
				next = m.Digest
				break
			}
		}
		if next == "" {
			return "", fmt.Errorf("镜像缺少当前平台的版本信息")
		}
		digest = next
	}
	return "", fmt.Errorf("镜像索引层级过多")
}
