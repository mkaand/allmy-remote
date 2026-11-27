package core

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mkaand/allmy-remote/internal/telemetry"
)

func TestLiveIPDisclosure(t *testing.T) {
	s, _ := testStore(t)
	if err := s.InitAdmin(context.Background(), "test-password-long"); err != nil {
		t.Fatal(err)
	}
	a := NewAuth(s, "https://example.com")
	d := a.devices
	defer d.Close()
	raw, hash, err := newToken()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec("INSERT INTO operator_sessions(token_hash,operator_id,expires_at) VALUES(?,1,?)", hash[:], time.Now().Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	const id = "test-device"
	if _, err := s.db.Exec("INSERT INTO devices(id,name,platform,credential_hash,created_at) VALUES(?,?,?,zeroblob(32),1)", id, "Test device", "windows"); err != nil {
		t.Fatal(err)
	}
	entry := &deviceConnection{lastSeen: d.now(), sample: &telemetry.Sample{InternalIP: "192.0.2.5"}}
	d.active[id] = entry
	h := Handler(func() bool { return true }, a)
	req := func(method, path, origin, body string, authorized bool, want int) string {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if authorized {
			r.AddCookie(sessionCookie(raw, 3600))
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("status %d, want %d", w.Code, want)
		}
		return w.Body.String()
	}
	path := "/api/v1/devices/" + id + "/ip"
	body := `{"kind":"internal"}`
	list := req("GET", "/api/v1/devices", "", "", true, 200)
	if strings.Contains(list, "192.0.2.5") || !strings.Contains(list, `"internal_ip_available":true`) {
		t.Fatal("IP leaked in listing or availability missing")
	}
	req("POST", path, a.origin, body, false, 401)
	req("POST", path, "https://other.example.com", body, true, 403)
	if !strings.Contains(req("POST", path, a.origin, body, true, 200), "192.0.2.5") {
		t.Fatal("authorized reveal missing")
	}
	req("POST", path, a.origin, `{"kind":"external"}`, true, 404)
	entry.lastSeen = d.now().Add(-offlineAfter)
	req("POST", path, a.origin, body, true, 404)
	list = req("GET", "/api/v1/devices", "", "", true, 200)
	if strings.Contains(list, "192.0.2.5") || strings.Contains(list, `"internal_ip_available":true`) {
		t.Fatal("expired IP exposed")
	}
	delete(d.active, id)
}
