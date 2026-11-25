package core

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"sync/atomic"
	"time"
)

//go:embed static/index.html static/style.css
var assets embed.FS

// Handler serves the static preview, status endpoints and optional authentication API.
func Handler(ready func() bool, auth *Auth) http.Handler {
	static, err := fs.Sub(assets, "static")
	if err != nil {
		panic(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, static, "index.html")
	})
	mux.HandleFunc("GET /style.css", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, static, "style.css")
	})
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		status(w, http.StatusOK, "ok")
	})
	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, _ *http.Request) {
		if !ready() {
			status(w, http.StatusServiceUnavailable, "not_ready")
			return
		}
		status(w, http.StatusOK, "ready")
	})
	if auth != nil {
		mux.HandleFunc("GET /api/v1/session", auth.session)
		mux.HandleFunc("POST /api/v1/session/login", auth.login)
		mux.HandleFunc("POST /api/v1/session/logout", auth.logout)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
		mux.ServeHTTP(w, r)
	})
}

func status(w http.ResponseWriter, code int, value string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": value})
}

// Run starts the development server and drains active requests on cancellation.
func Run(ctx context.Context, cfg Config) error {
	store, err := OpenStore(ctx, cfg.DataDir)
	if err != nil {
		return fmt.Errorf("initialize storage: %w", err)
	}
	defer store.Close()
	auth := NewAuth(store, cfg.Origin)
	if err := auth.close(ctx); err != nil {
		return fmt.Errorf("expire old sessions: %w", err)
	}
	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return fmt.Errorf("cannot open development listener: %w", err)
	}
	defer listener.Close()
	return serve(ctx, listener, auth)
}

func serve(ctx context.Context, listener net.Listener, auth *Auth) error {
	var ready atomic.Bool
	srv := &http.Server{
		Handler: Handler(func() bool {
			return ready.Load() && (auth == nil || auth.store.Ready())
		}, auth),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
	done := make(chan error, 1)
	ready.Store(true)
	go func() { done <- srv.Serve(listener) }()
	log.Print("AllMy Remote development preview started; not production-ready")

	select {
	case err := <-done:
		ready.Store(false)
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		ready.Store(false)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			_ = srv.Close()
			<-done
			return fmt.Errorf("graceful shutdown: %w", err)
		}
		if err := <-done; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}
