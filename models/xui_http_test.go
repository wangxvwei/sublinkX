package models

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type xuiTestTransport func(*http.Request) (*http.Response, error)

func (f xuiTestTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

type xuiTestTimeout struct{}

func (xuiTestTimeout) Error() string   { return "TLS handshake timeout" }
func (xuiTestTimeout) Timeout() bool   { return true }
func (xuiTestTimeout) Temporary() bool { return true }

func TestXUIRequestRetriesTransientTLSFailureAndKeepsBodyReadable(t *testing.T) {
	attempts := 0
	var responseContext context.Context
	client := &http.Client{Transport: xuiTestTransport(func(req *http.Request) (*http.Response, error) {
		attempts++
		if req.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatal("retry lost authorization")
		}
		if attempts == 1 {
			return nil, xuiTestTimeout{}
		}
		responseContext = req.Context()
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("nodes")), Header: make(http.Header)}, nil
	})}
	req, _ := http.NewRequest(http.MethodGet, "https://example.test/panel/api/inbounds/list", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := doXUIRequestWithClient(client, req)
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
	if err := responseContext.Err(); err != nil {
		t.Fatalf("body context already canceled: %v", err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil || string(body) != "nodes" {
		t.Fatalf("body = %q, error = %v", body, err)
	}
	resp.Body.Close()
	if !errors.Is(responseContext.Err(), context.Canceled) {
		t.Fatal("closing body did not release context")
	}
}

func TestXUIRequestDoesNotRetryAuthenticationResponse(t *testing.T) {
	attempts := 0
	client := &http.Client{Transport: xuiTestTransport(func(req *http.Request) (*http.Response, error) {
		attempts++
		return &http.Response{StatusCode: 401, Body: io.NopCloser(strings.NewReader("unauthorized")), Header: make(http.Header)}, nil
	})}
	req, _ := http.NewRequest(http.MethodGet, "https://example.test", nil)
	resp, err := doXUIRequestWithClient(client, req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if attempts != 1 || resp.StatusCode != 401 {
		t.Fatalf("attempts = %d, status = %d", attempts, resp.StatusCode)
	}
}

func TestXUIRequestStopsAfterThreeFailures(t *testing.T) {
	attempts := 0
	client := &http.Client{Transport: xuiTestTransport(func(*http.Request) (*http.Response, error) {
		attempts++
		return nil, xuiTestTimeout{}
	})}
	req, _ := http.NewRequest(http.MethodGet, "https://example.test", nil)
	_, err := doXUIRequestWithClient(client, req)
	if err == nil || attempts != 3 {
		t.Fatalf("attempts = %d, error = %v", attempts, err)
	}
}

func TestXUIRequestDoesNotRetryCanceledRequest(t *testing.T) {
	attempts := 0
	client := &http.Client{Transport: xuiTestTransport(func(*http.Request) (*http.Response, error) {
		attempts++
		return nil, context.Canceled
	})}
	req, _ := http.NewRequest(http.MethodGet, "https://example.test", nil)
	_, err := doXUIRequestWithClient(client, req)
	if !errors.Is(err, context.Canceled) || attempts != 1 {
		t.Fatalf("attempts = %d, error = %v", attempts, err)
	}
}

func TestXUIPartialSubscriptionFailureDoesNotReturnIncompleteNodeSet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/panel/api/inbounds/list":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"success":true,"obj":[{"id":1,"enable":true,"settings":{"clients":[{"id":"good-client","email":"good","subId":"good"},{"id":"bad-client","email":"bad","subId":"bad"}]}}]}`)
		case "/dingyue/good":
			io.WriteString(w, "vless://good-client@example.test:443#good")
		case "/dingyue/bad":
			http.Error(w, "upstream unavailable", http.StatusBadGateway)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	source := XUISource{PanelBaseURL: server.URL, SubBaseURL: server.URL, SubPath: "dingyue", APIToken: "test"}
	nodes, err := source.fetchAPINodes()
	if err == nil || len(nodes) != 0 {
		t.Fatalf("partial fetch returned nodes=%d error=%v", len(nodes), err)
	}
	if !strings.Contains(err.Error(), "existing nodes were kept") {
		t.Fatalf("unexpected error: %v", err)
	}
}
