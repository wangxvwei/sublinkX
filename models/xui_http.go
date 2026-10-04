package models

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"syscall"
	"time"
)

// Share connections across API and subscription requests instead of doing a
// fresh TLS handshake for every client in a source.
var xuiHTTPClient = &http.Client{
	Timeout: 15 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig:       insecureTLSConfig(),
		ForceAttemptHTTP2:     true,
		DialContext:           (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		MaxIdleConns:          32,
		MaxIdleConnsPerHost:   8,
		IdleConnTimeout:       90 * time.Second,
	},
}

func doXUIRequest(req *http.Request) (*http.Response, error) {
	return doXUIRequestWithClient(xuiHTTPClient, req)
}

func doXUIRequestWithClient(client *http.Client, req *http.Request) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(req.Context(), 20*time.Second)
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		resp, err := client.Do(req.Clone(ctx))
		if err == nil {
			// Keep the request context alive until the caller finishes reading.
			resp.Body = &xuiResponseBody{ReadCloser: resp.Body, cancel: cancel}
			return resp, nil
		}
		lastErr = err
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		if req.Method != http.MethodGet || !isTransientXUIError(err) || ctx.Err() != nil || attempt == 2 {
			cancel()
			return nil, fmt.Errorf("x-ui request failed after %d attempt(s): %w", attempt+1, err)
		}
		timer := time.NewTimer(time.Duration(attempt+1) * 250 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			cancel()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	cancel()
	return nil, lastErr
}

func isTransientXUIError(err error) bool {
	var networkErr net.Error
	return (errors.As(err, &networkErr) && networkErr.Timeout()) ||
		errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, syscall.ECONNRESET)
}

type xuiResponseBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *xuiResponseBody) Close() error {
	defer b.cancel()
	return b.ReadCloser.Close()
}
