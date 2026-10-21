package api

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCDNPinnedIPRequiresExplicitKeep(t *testing.T) {
	r := gin.New()
	r.POST("/cdn/:id/pinned", CDNPinnedIP)
	for _, body := range []string{`{"ip":"104.16.0.1"}`, `{"ip":"104.16.0.1","keep":null}`, `{"ip":"104.16.0.1","keep":"false"}`, `{`} {
		response := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/cdn/1/pinned", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(response, req)
		if response.Code != 400 {
			t.Fatalf("malformed request could erase retention: %d", response.Code)
		}
	}
}
