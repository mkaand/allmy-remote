package agent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func testToken(n int) string { return base64.RawURLEncoding.EncodeToString(make([]byte, n)) }

func TestOrigin(t *testing.T) {
	for _, invalid := range []string{"http://example.com", "https://user:pass@example.com", "https://example.com/path", "https://example.com?x=y", "https://example.com/#x"} {
		if _, err := Origin(invalid); err == nil {
			t.Fatal("unsafe origin accepted")
		}
	}
	if origin, err := Origin("https://example.com/"); err != nil || origin != "https://example.com" {
		t.Fatal("valid HTTPS origin rejected")
	}
}

func TestEnrollmentAndChannel(t *testing.T) {
	grant, credential, id := testToken(32), testToken(32), testToken(16)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/enrollment/exchange", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		if json.NewDecoder(r.Body).Decode(&body) != nil || body["platform"] != "windows" || body["grant"] != grant {
			w.WriteHeader(400)
			return
		}
		w.WriteHeader(201)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": id, "credential": credential, "platform": "windows"})
	})
	mux.HandleFunc("GET /api/v1/device/channel", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+credential {
			w.WriteHeader(401)
			return
		}
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		_, body, err := conn.Read(ctx)
		if err != nil {
			return
		}
		var m message
		if json.Unmarshal(body, &m) != nil || m.Type != "hello" || m.Version != 1 {
			return
		}
		m.Type, m.CorrelationID = "hello_ack", m.ID
		body, _ = json.Marshal(m)
		_ = conn.Write(ctx, websocket.MessageText, body)
		<-ctx.Done()
	})
	server := httptest.NewTLSServer(mux)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	s, err := Enroll(ctx, server.URL, grant, server.Client())
	if err != nil || !s.Valid() {
		t.Fatal("enrollment failed")
	}
	online := false
	if err := Run(ctx, s, server.Client(), func(status string) {
		if status == "Connected; heartbeat only." {
			online = true
			cancel()
		}
	}); err != nil || !online {
		t.Fatal("agent failed to connect")
	}
	bad := s
	bad.Credential = base64.RawURLEncoding.EncodeToString([]byte("12345678901234567890123456789012"))
	if err := Run(context.Background(), bad, server.Client(), func(string) {}); !errors.Is(err, ErrDenied) {
		t.Fatal("unauthorized agent did not stop")
	}
}

func TestEnrollmentDoesNotFollowRedirect(t *testing.T) {
	called := false
	destination := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(500)
	}))
	defer destination.Close()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destination.URL, 307)
	}))
	defer server.Close()
	if _, err := Enroll(context.Background(), server.URL, testToken(32), server.Client()); err == nil || called {
		t.Fatal("redirect was not rejected")
	}
}
