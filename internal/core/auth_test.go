package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAuthLifecycle(t *testing.T) {
	s, _ := testStore(t)
	if err := s.InitAdmin(context.Background(), "test-password-long"); err != nil {
		t.Fatal(err)
	}
	a := NewAuth(s, "http://localhost:8080")
	now := time.Now()
	a.now = func() time.Time { return now }
	h := Handler(func() bool { return true }, a)
	request := func(method, path, origin, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Origin", origin)
		r.Header.Set("Content-Type", "application/json")
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	check := func(w *httptest.ResponseRecorder, code int) {
		t.Helper()
		if w.Code != code {
			t.Fatalf("got %d %s, want %d", w.Code, w.Body.String(), code)
		}
	}
	login := "/api/v1/session/login"
	session := "/api/v1/session"
	logout := "/api/v1/session/logout"
	origin := a.origin
	good := `{"username":"admin","password":"test-password-long"}`
	check(request("GET", session, "", "", nil), 401)
	check(request("POST", login, "", good, nil), 403)
	check(request("POST", login, "http://other.example.com", good, nil), 403)
	check(request("POST", login, origin, good+"{}", nil), 400)
	check(request("POST", login, origin, `{"username":"admin","password":"wrong"}`, nil), 401)
	w := request("POST", login, origin, good, nil)
	check(w, 200)
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal("expected session cookie")
	}
	first := cookies[0]
	if !first.HttpOnly || !first.Secure || first.SameSite != http.SameSiteStrictMode || first.MaxAge <= 0 {
		t.Fatal("insecure session cookie")
	}
	var stored []byte
	if err := s.db.QueryRow("SELECT token_hash FROM operator_sessions").Scan(&stored); err != nil || len(stored) != 32 || string(stored) == first.Value {
		t.Fatal("invalid token storage")
	}
	check(request("GET", session, "", "", first), 200)
	w = request("POST", login, origin, good, first)
	check(w, 200)
	second := w.Result().Cookies()[0]
	if first.Value == second.Value {
		t.Fatal("session not rotated")
	}
	check(request("GET", session, "", "", first), 401)
	check(request("POST", logout, "http://other.example.com", "", second), 403)
	check(request("GET", session, "", "", second), 200)
	check(request("POST", logout, origin, "", second), 200)
	check(request("GET", session, "", "", second), 401)
	w = request("POST", login, origin, good, nil)
	check(w, 200)
	third := w.Result().Cookies()[0]
	now = now.Add(sessionTTL + time.Second)
	check(request("GET", session, "", "", third), 401)
}

func TestLoginRateLimit(t *testing.T) {
	s, _ := testStore(t)
	a := NewAuth(s, "http://localhost:8080")
	for i := 0; i < 5; i++ {
		if !a.allowLogin() {
			t.Fatal("premature rate limit")
		}
	}
	if a.allowLogin() {
		t.Fatal("rate limit missing")
	}
}
