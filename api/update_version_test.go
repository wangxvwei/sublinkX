package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
)

func TestPublishedImageVersion(t *testing.T) {
	index := "sha256:" + strings.Repeat("a", 64)
	manifest := "sha256:" + strings.Repeat("b", 64)
	config := "sha256:" + strings.Repeat("c", 64)
	version := "2.3.7"
	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			fmt.Fprint(w, `{"token":"test-token"}`)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.Header().Set("Www-Authenticate", `Bearer realm="`+server.URL+`/token",service="test",scope="repository:test/app:pull"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/v2/test/app/manifests/" + index:
			fmt.Fprintf(w, `{"manifests":[{"digest":%q,"platform":{"os":"unknown","architecture":"unknown"}},{"digest":%q,"platform":{"os":"linux","architecture":%q}}]}`, config, manifest, runtime.GOARCH)
		case "/v2/test/app/manifests/" + manifest:
			fmt.Fprintf(w, `{"config":{"digest":%q}}`, config)
		case "/v2/test/app/blobs/" + config:
			fmt.Fprintf(w, `{"config":{"Labels":{"org.opencontainers.image.version":%q}}}`, version)
		default:
			t.Errorf("read unexpected image reference: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	previous := updateHTTPClient
	updateHTTPClient = server.Client()
	t.Cleanup(func() { updateHTTPClient = previous })
	image := strings.TrimPrefix(server.URL, "https://") + "/test/app:latest"
	for _, digest := range []string{index, manifest} {
		if got, err := fetchRemoteImageVersion(image, digest); err != nil || got != "2.3.7" {
			t.Fatal("failed to read exact image version", got, err)
		}
	}
	version = "main"
	if _, err := fetchRemoteImageVersion(image, index); err == nil {
		t.Fatal("branch label was accepted as an app version")
	}
	if _, err := fetchRemoteImageVersion(image, "latest"); err == nil {
		t.Fatal("accepted a moving tag instead of the checked digest")
	}
}
