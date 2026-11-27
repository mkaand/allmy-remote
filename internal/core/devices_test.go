package core

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/mkaand/allmy-remote/internal/agent"
)

func TestDeviceRegistryLifecycle(t *testing.T) {
	s, _ := testStore(t)
	if err := s.InitAdmin(context.Background(), "test-password-long"); err != nil {
		t.Fatal(err)
	}
	a := NewAuth(s, "http://localhost:8080")
	d := a.devices
	var clock atomic.Int64
	clock.Store(time.Now().Unix())
	d.now = func() time.Time { return time.Unix(clock.Load(), 0) }
	server := httptest.NewServer(Handler(func() bool { return true }, a))
	defer server.Close()
	defer d.Close()
	client := server.Client()

	request := func(method, path, origin, body string, cookie *http.Cookie, want int) []byte {
		t.Helper()
		r, err := http.NewRequest(method, server.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		r.Header.Set("Content-Type", "application/json")
		if cookie != nil {
			r.AddCookie(cookie)
		}
		res, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != want {
			t.Fatalf("%s %s: got %d %s, want %d", method, path, res.StatusCode, b, want)
		}
		return b
	}
	// Seed a session using the same hashed-token representation as the auth API.
	// The separate authentication lifecycle test exercises password and cookie login.
	raw, hash, err := newToken()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec("INSERT INTO operator_sessions(token_hash,operator_id,expires_at) VALUES(?,1,?)", hash[:], time.Now().Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	cookie := sessionCookie(raw, 3600)
	grants := "/api/v1/enrollment/grants"
	exchange := "/api/v1/enrollment/exchange"
	request("GET", "/api/v1/devices", "", "", nil, 401)
	request("POST", grants, a.origin, `{"name":"Mock device"}`, nil, 401)
	request("POST", grants, "http://other.example.com", `{"name":"Mock device"}`, cookie, 403)
	request("POST", grants, a.origin, `{"name":"  "}`, cookie, 400)

	createGrant := func() string {
		var response struct {
			Grant string `json:"grant"`
		}
		b := request("POST", grants, a.origin, `{"name":"Mock device"}`, cookie, 201)
		if err := json.Unmarshal(b, &response); err != nil || response.Grant == "" {
			t.Fatal("missing grant")
		}
		return response.Grant
	}
	exchangeBody := func(grant string) string {
		b, err := json.Marshal(map[string]string{"grant": grant})
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	expired := createGrant()
	clock.Add(int64(grantTTL.Seconds()) + 1)
	request("POST", exchange, "", exchangeBody(expired), nil, 401)
	grant := createGrant()
	request("POST", exchange, a.origin, exchangeBody(grant), nil, 403)
	var issued struct {
		ID         string `json:"id"`
		Credential string `json:"credential"`
		Platform   string `json:"platform"`
	}
	b := request("POST", exchange, "", exchangeBody(grant), nil, 201)
	if err := json.Unmarshal(b, &issued); err != nil || issued.ID == "" || issued.Credential == "" || issued.Platform != "mock" {
		t.Fatal("invalid enrollment")
	}
	request("POST", exchange, "", exchangeBody(grant), nil, 401)
	var stored []byte
	wantHash, _ := tokenHash(issued.Credential)
	if err := s.db.QueryRow("SELECT credential_hash FROM devices WHERE id=?", issued.ID).Scan(&stored); err != nil || !bytes.Equal(stored, wantHash[:]) {
		t.Fatal("credential hash not stored")
	}
	checkState := func(want string) {
		t.Helper()
		var views []deviceView
		b := request("GET", "/api/v1/devices", "", "", cookie, 200)
		if err := json.Unmarshal(b, &views); err != nil || len(views) != 1 || views[0].Status != want || views[0].Platform != "mock" {
			t.Fatalf("unexpected device status; wanted %s", want)
		}
		if bytes.Contains(b, []byte(issued.Credential)) {
			t.Fatal("credential leaked in listing")
		}
	}
	checkState("offline")

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/device/channel"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	dial := func(token, origin string) (*websocket.Conn, *http.Response, error) {
		h := http.Header{}
		if token != "" {
			h.Set("Authorization", "Bearer "+token)
		}
		if origin != "" {
			h.Set("Origin", origin)
		}
		return websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: h})
	}
	denied := func(token, origin string, code int) {
		t.Helper()
		conn, response, err := dial(token, origin)
		if conn != nil {
			conn.CloseNow()
		}
		if err == nil || response == nil || response.StatusCode != code {
			t.Fatalf("expected websocket rejection %d", code)
		}
	}
	denied("", "", 401)
	denied(issued.Credential, a.origin, 403)
	conn, _, err := dial(issued.Credential, "")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	denied(issued.Credential, "", 409)
	send := func(kind string) {
		t.Helper()
		msg := controlMessage{Version: 1, Type: kind, ID: kind + "-1"}
		raw, err := json.Marshal(msg)
		if err != nil {
			t.Fatal(err)
		}
		if err := conn.Write(ctx, websocket.MessageText, raw); err != nil {
			t.Fatal(err)
		}
		_, raw, err = conn.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var ack controlMessage
		if err := json.Unmarshal(raw, &ack); err != nil || ack.Type != kind+"_ack" || ack.CorrelationID != msg.ID {
			t.Fatal("invalid acknowledgement")
		}
	}
	send("hello")
	checkState("online")
	clock.Add(int64(heartbeatInterval.Seconds()))
	send("heartbeat")
	checkState("online")
	clock.Add(int64(offlineAfter.Seconds()) + 1)
	checkState("offline")
	send("heartbeat")
	checkState("online")

	revoke := "/api/v1/devices/" + issued.ID + "/revoke"
	request("POST", revoke, "http://other.example.com", "", cookie, 403)
	request("POST", revoke, a.origin, "", cookie, 200)
	checkState("revoked")
	closeCtx, stop := context.WithTimeout(context.Background(), 2*time.Second)
	defer stop()
	if _, _, err := conn.Read(closeCtx); err == nil || closeCtx.Err() != nil {
		t.Fatal("revocation did not close connection promptly")
	}
	denied(issued.Credential, "", 401)
}

func TestControlValidation(t *testing.T) {
	for _, input := range []string{
		`{"version":2,"type":"hello","id":"1"}`,
		`{"version":1,"type":"command","id":"1"}`,
		`{"version":1,"type":"hello","id":"1","device_id":"other"}`,
		`{"version":1,"type":"hello","id":"1","payload":{"command":"x"}}`,
		`{"version":1,"type":"hello","id":"1"}{}`,
		`{"version":1,"type":"hello","id":""}`,
	} {
		if _, ok := parseControl([]byte(input), "hello"); ok {
			t.Fatal("invalid control message accepted")
		}
	}
	if _, ok := parseControl([]byte(`{"version":1,"type":"hello","id":"1"}`), "hello"); !ok {
		t.Fatal("valid hello rejected")
	}
}

func TestDeviceMigrationFromV1(t *testing.T) {
	s, dir := testStore(t)
	if err := s.InitAdmin(context.Background(), "test-password-long"); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{"DROP TABLE devices", "DROP TABLE enrollment_grants", "PRAGMA user_version=1"} {
		if _, err := s.db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenStore(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	var count int
	if err := reopened.db.QueryRow("SELECT count(*) FROM operators").Scan(&count); err != nil || count != 1 {
		t.Fatal("operator lost during migration")
	}
	if err := reopened.db.QueryRow("SELECT count(*) FROM devices").Scan(&count); err != nil || count != 0 {
		t.Fatal("device schema missing")
	}
}

func TestWindowsEnrollmentAndV2Migration(t *testing.T) {
	s, dir := testStore(t)
	if err := s.InitAdmin(context.Background(), "test-password-long"); err != nil {
		t.Fatal(err)
	}
	// Recreate the exact v2 device table with an existing legacy row.
	for _, query := range []string{
		"ALTER TABLE devices DROP COLUMN platform",
		"INSERT INTO devices(id,name,credential_hash,created_at) VALUES('legacy','Legacy mock',zeroblob(32),1)",
		"PRAGMA user_version=2",
	} {
		if _, err := s.db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := OpenStore(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	d := NewDeviceRegistry(s)
	defer d.Close()
	views, err := d.snapshot(context.Background())
	if err != nil || len(views) != 1 || views[0].Platform != "mock" {
		t.Fatal("legacy device not preserved")
	}
	grant, hash, err := newToken()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec("INSERT INTO enrollment_grants(token_hash,name,expires_at) VALUES(?,?,?)", hash[:], "Windows test", time.Now().Add(time.Minute).Unix()); err != nil {
		t.Fatal(err)
	}
	for _, platform := range []string{"unknown", "windows"} {
		body, err := json.Marshal(map[string]string{"grant": grant, "platform": platform})
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest("POST", "/api/v1/enrollment/exchange", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		d.exchange(w, r)
		if platform == "unknown" {
			if w.Code != 400 {
				t.Fatal("unknown platform accepted")
			}
		} else if w.Code != 201 || !strings.Contains(w.Body.String(), `"platform":"windows"`) {
			t.Fatal("Windows enrollment failed")
		}
	}
	views, err = d.snapshot(context.Background())
	if err != nil || len(views) != 2 || views[1].Platform != "windows" {
		t.Fatal("Windows platform not persisted")
	}
}

func TestNativeAgentAgainstCore(t *testing.T) {
	s, _ := testStore(t)
	a := NewAuth(s, "https://example.com")
	d := a.devices
	server := httptest.NewTLSServer(Handler(func() bool { return true }, a))
	defer server.Close()
	defer d.Close()
	grant, hash, err := newToken()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec("INSERT INTO enrollment_grants(token_hash,name,expires_at) VALUES(?,?,?)", hash[:], "Native test", time.Now().Add(time.Minute).Unix()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	state, err := agent.Enroll(ctx, server.URL, grant, server.Client())
	if err != nil {
		t.Fatal("native enrollment failed")
	}
	connected := make(chan struct{}, 1)
	done := make(chan error, 1)
	go func() {
		done <- agent.Run(ctx, state, server.Client(), func(status string) {
			if status == "Connected; heartbeat only." {
				select {
				case connected <- struct{}{}:
				default:
				}
			}
		})
	}()
	select {
	case <-connected:
	case <-ctx.Done():
		t.Fatal("native hello failed")
	}
	views, err := d.snapshot(ctx)
	if err != nil || len(views) != 1 || views[0].Platform != "windows" || views[0].Status != "online" || views[0].LastSeen == nil {
		t.Fatal("native device not online")
	}
	firstSeen := *views[0].LastSeen
	for {
		views, err = d.snapshot(ctx)
		if err != nil {
			t.Fatal("snapshot failed")
		}
		if views[0].LastSeen != nil && *views[0].LastSeen > firstSeen {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("native heartbeat not received")
		case <-time.After(100 * time.Millisecond):
		}
	}
	ok, err := d.revokeDevice(ctx, state.ID)
	if err != nil || !ok {
		t.Fatal("revoke failed")
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal("agent did not stop")
	}
	deniedCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	if err := agent.Run(deniedCtx, state, server.Client(), func(string) {}); err != agent.ErrDenied {
		t.Fatal("revoked agent reconnected")
	}
}
