package core

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRoutes(t *testing.T) {
	tests := []struct {
		method string
		path   string
		ready  bool
		code   int
		body   string
	}{
		{"GET", "/", true, 200, "AllMy Remote"},
		{"GET", "/style.css", true, 200, "color-scheme"},
		{"GET", "/app.js", true, 200, "AllMy Remote"},
		{"GET", "/allmy-theme-init.js", true, 200, "allmy-theme"},
		{"GET", "/allmy-theme-controls.css", true, 200, "allmy-theme-switch"},
		{"GET", "/health", false, 200, "\"ok\""},
		{"GET", "/ready", true, 200, "\"ready\""},
		{"GET", "/ready", false, 503, "\"not_ready\""},
		{"HEAD", "/health", true, 200, ""},
		{"GET", "/api/v1/devices", true, 404, ""},
		{"GET", "/missing", true, 404, ""},
		{"POST", "/health", true, 405, ""},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path+" "+tt.body, func(t *testing.T) {
			res := httptest.NewRecorder()
			Handler(func() bool { return tt.ready }, nil).ServeHTTP(res, httptest.NewRequest(tt.method, tt.path, nil))
			if res.Code != tt.code || !strings.Contains(res.Body.String(), tt.body) {
				t.Fatalf("unexpected response: %d %q", res.Code, res.Body.String())
			}
			if res.Header().Get("Cache-Control") != "no-store" ||
				res.Header().Get("X-Content-Type-Options") != "nosniff" ||
				res.Header().Get("Content-Security-Policy") == "" {
				t.Fatal("missing response security headers")
			}
		})
	}
}

func TestGracefulShutdown(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- serve(ctx, listener, nil) }()

	client := &http.Client{Timeout: 2 * time.Second}
	res, err := client.Get("http://" + listener.Addr().String() + "/ready")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	client.CloseIdleConnections()
	if res.StatusCode != 200 {
		t.Fatalf("expected ready, got %d", res.StatusCode)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("shutdown timed out")
	}
}
