package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/coder/websocket"
)

const (
	grantTTL = 5 * time.Minute
	heartbeatInterval = 15 * time.Second
	offlineAfter = 45 * time.Second
	maxDevices = 100
	maxGrants = 20
)

type DeviceRegistry struct {
	store *Store
	mu sync.Mutex
	connections sync.WaitGroup
	active map[string]*deviceConnection
	ctx context.Context
	cancel context.CancelFunc
	now func() time.Time
	attemptStart time.Time
	attempts int
}

type deviceConnection struct {
	cancel context.CancelFunc
	lastSeen time.Time
}

func NewDeviceRegistry(store *Store) *DeviceRegistry {
	ctx, cancel := context.WithCancel(context.Background())
	return &DeviceRegistry{
		store: store,
		active: make(map[string]*deviceConnection),
		ctx: ctx,
		cancel: cancel,
		now: time.Now,
	}
}

func (d *DeviceRegistry) Close() {
	d.mu.Lock()
	d.cancel()
	d.mu.Unlock()
	d.connections.Wait()
}

func (d *DeviceRegistry) register(mux *http.ServeMux, auth *Auth) {
	operator := func(mutate bool, h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if mutate && !auth.sameOrigin(r) {
				status(w, 403, "forbidden")
				return
			}
			ok, err := auth.valid(r)
			if err != nil {
				status(w, 503, "unavailable")
				return
			}
			if !ok {
				status(w, 401, "unauthorized")
				return
			}
			h(w, r)
		}
	}
	mux.HandleFunc("GET /api/v1/devices", operator(false, d.list))
	mux.HandleFunc("POST /api/v1/enrollment/grants", operator(true, d.grant))
	mux.HandleFunc("POST /api/v1/enrollment/exchange", d.exchange)
	mux.HandleFunc("POST /api/v1/devices/{id}/revoke", operator(true, d.revoke))
	mux.HandleFunc("GET /api/v1/device/channel", d.channel)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func decodeRequest(w http.ResponseWriter, r *http.Request, v any) bool {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		status(w, 415, "unsupported_media_type")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if decoder.Decode(v) != nil || decoder.Decode(new(any)) != io.EOF {
		status(w, 400, "invalid_request")
		return false
	}
	return true
}

func newToken() (string, [32]byte, error) {
	raw, err := randomBytes(32)
	if err != nil {
		return "", [32]byte{}, err
	}
	return base64.RawURLEncoding.EncodeToString(raw), sha256.Sum256(raw), nil
}

func tokenHash(token string) ([32]byte, bool) {
	if len(token) != 43 {
		return [32]byte{}, false
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(token)
	if err != nil || len(raw) != 32 {
		return [32]byte{}, false
	}
	return sha256.Sum256(raw), true
}

func validName(name string) bool {
	if name != strings.TrimSpace(name) || name == "" || len(name) > 96 || !utf8.ValidString(name) {
		return false
	}
	for _, r := range name {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return false
		}
	}
	return true
}

func (d *DeviceRegistry) grant(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name string `json:"name"`
	}
	if !decodeRequest(w, r, &input) {
		return
	}
	if !validName(input.Name) {
		status(w, 400, "invalid_name")
		return
	}
	token, hash, err := newToken()
	if err != nil {
		status(w, 503, "unavailable")
		return
	}
	now := d.now()
	tx, err := d.store.db.BeginTx(r.Context(), nil)
	if err != nil {
		status(w, 503, "unavailable")
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), "DELETE FROM enrollment_grants WHERE expires_at<=?", now.Unix()); err != nil {
		status(w, 503, "unavailable")
		return
	}
	var count int
	if err = tx.QueryRowContext(r.Context(), "SELECT count(*) FROM enrollment_grants").Scan(&count); err != nil {
		status(w, 503, "unavailable")
		return
	}
	if count >= maxGrants {
		status(w, 409, "grant_limit")
		return
	}
	expiry := now.Add(grantTTL).Unix()
	if _, err = tx.ExecContext(r.Context(), "INSERT INTO enrollment_grants(token_hash,name,expires_at) VALUES(?,?,?)", hash[:], input.Name, expiry); err != nil {
		status(w, 503, "unavailable")
		return
	}
	if err = tx.Commit(); err != nil {
		status(w, 503, "unavailable")
		return
	}
	writeJSON(w, 201, map[string]any{"grant": token, "expires_at": expiry})
}

func (d *DeviceRegistry) allowExchange() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	now := d.now()
	if d.attemptStart.IsZero() || now.Sub(d.attemptStart) >= time.Minute {
		d.attemptStart, d.attempts = now, 0
	}
	if d.attempts >= 30 {
		return false
	}
	d.attempts++
	return true
}

func (d *DeviceRegistry) exchange(w http.ResponseWriter, r *http.Request) {
	// Enrollment is a native client operation, not a browser-cookie endpoint.
	if len(r.Header.Values("Origin")) != 0 {
		status(w, 403, "forbidden")
		return
	}
	if !d.allowExchange() {
		w.Header().Set("Retry-After", "60")
		status(w, 429, "rate_limited")
		return
	}
	var input struct {
		Grant string `json:"grant"`
	}
	if !decodeRequest(w, r, &input) {
		return
	}
	hash, ok := tokenHash(input.Grant)
	if !ok {
		status(w, 401, "invalid_grant")
		return
	}
	credential, credentialHash, err := newToken()
	if err != nil {
		status(w, 503, "unavailable")
		return
	}
	idRaw, err := randomBytes(16)
	if err != nil {
		status(w, 503, "unavailable")
		return
	}
	id := base64.RawURLEncoding.EncodeToString(idRaw)
	now := d.now().Unix()
	tx, err := d.store.db.BeginTx(r.Context(), nil)
	if err != nil {
		status(w, 503, "unavailable")
		return
	}
	defer tx.Rollback()
	var name string
	err = tx.QueryRowContext(r.Context(), "SELECT name FROM enrollment_grants WHERE token_hash=? AND expires_at>?", hash[:], now).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		status(w, 401, "invalid_grant")
		return
	}
	if err != nil {
		status(w, 503, "unavailable")
		return
	}
	var count int
	if err = tx.QueryRowContext(r.Context(), "SELECT count(*) FROM devices").Scan(&count); err != nil {
		status(w, 503, "unavailable")
		return
	}
	if count >= maxDevices {
		status(w, 409, "device_limit")
		return
	}
	if _, err = tx.ExecContext(r.Context(), "DELETE FROM enrollment_grants WHERE token_hash=?", hash[:]); err != nil {
		status(w, 503, "unavailable")
		return
	}
	if _, err = tx.ExecContext(r.Context(), "INSERT INTO devices(id,name,credential_hash,created_at) VALUES(?,?,?,?)", id, name, credentialHash[:], now); err != nil {
		status(w, 503, "unavailable")
		return
	}
	if err = tx.Commit(); err != nil {
		status(w, 503, "unavailable")
		return
	}
	writeJSON(w, 201, map[string]string{"id": id, "credential": credential, "platform": "mock"})
}

type deviceView struct {
	ID string `json:"id"`
	Name string `json:"name"`
	Platform string `json:"platform"`
	Status string `json:"status"`
	LastSeen *int64 `json:"last_seen"`
}

func (d *DeviceRegistry) list(w http.ResponseWriter, r *http.Request) {
	devices, err := d.snapshot(r.Context())
	if err != nil {
		status(w, 503, "unavailable")
		return
	}
	writeJSON(w, 200, devices)
}

func (d *DeviceRegistry) snapshot(ctx context.Context) ([]deviceView, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	rows, err := d.store.db.QueryContext(ctx, "SELECT id,name,last_seen,revoked_at FROM devices ORDER BY created_at,id LIMIT 100")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	devices := make([]deviceView, 0)
	for rows.Next() {
		var v deviceView
		var seen, revoked sql.NullInt64
		if err = rows.Scan(&v.ID, &v.Name, &seen, &revoked); err != nil {
			return nil, err
		}
		v.Platform, v.Status = "mock", "offline"
		if seen.Valid {
			t := seen.Int64
			v.LastSeen = &t
		}
		if revoked.Valid {
			v.Status = "revoked"
		} else if c := d.active[v.ID]; c != nil && !c.lastSeen.IsZero() && d.now().Sub(c.lastSeen) < offlineAfter {
			v.Status = "online"
		}
		devices = append(devices, v)
	}
	return devices, rows.Err()
}

func (d *DeviceRegistry) revoke(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if len(id) != 22 {
		status(w, 404, "not_found")
		return
	}
	ok, err := d.revokeDevice(r.Context(), id)
	if err != nil {
		status(w, 503, "unavailable")
		return
	}
	if !ok {
		status(w, 404, "not_found")
		return
	}
	status(w, 200, "revoked")
}

func (d *DeviceRegistry) revokeDevice(ctx context.Context, id string) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	result, err := d.store.db.ExecContext(ctx, "UPDATE devices SET revoked_at=? WHERE id=? AND revoked_at IS NULL", d.now().Unix(), id)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err != nil || n != 1 {
		return false, err
	}
	if c := d.active[id]; c != nil {
		c.cancel()
		delete(d.active, id)
	}
	return true, nil
}

type controlMessage struct {
	Version int `json:"version"`
	Type string `json:"type"`
	ID string `json:"id"`
	CorrelationID string `json:"correlation_id,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

func parseControl(data []byte, expected string) (controlMessage, bool) {
	var m controlMessage
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&m) != nil || decoder.Decode(new(any)) != io.EOF {
		return m, false
	}
	p := bytes.TrimSpace(m.Payload)
	return m, m.Version == 1 && m.Type == expected && validName(m.ID) && m.CorrelationID == "" &&
		(len(p) == 0 || bytes.Equal(p, []byte("null")) || bytes.Equal(p, []byte("{}")))
}

func (d *DeviceRegistry) channel(w http.ResponseWriter, r *http.Request) {
	if len(r.Header.Values("Origin")) != 0 {
		status(w, 403, "forbidden")
		return
	}
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	hash, valid := tokenHash(token)
	if !ok || !valid || len(r.Header.Values("Authorization")) != 1 {
		status(w, 401, "unauthorized")
		return
	}
	d.mu.Lock()
	var id string
	err := d.store.db.QueryRowContext(r.Context(), "SELECT id FROM devices WHERE credential_hash=? AND revoked_at IS NULL", hash[:]).Scan(&id)
	if err != nil {
		d.mu.Unlock()
		if errors.Is(err, sql.ErrNoRows) {
			status(w, 401, "unauthorized")
		} else {
			status(w, 503, "unavailable")
		}
		return
	}
	if d.ctx.Err() != nil || d.active[id] != nil || len(d.active) >= maxDevices {
		d.mu.Unlock()
		status(w, 409, "connection_unavailable")
		return
	}
	ctx, cancel := context.WithCancel(d.ctx)
	entry := &deviceConnection{cancel: cancel}
	d.active[id] = entry
	d.connections.Add(1)
	d.mu.Unlock()
	defer d.connections.Done()
	defer func() {
		cancel()
		d.mu.Lock()
		if d.active[id] == entry {
			delete(d.active, id)
		}
		d.mu.Unlock()
	}()
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.CloseNow()
	conn.SetReadLimit(64 << 10)

	expected := "hello"
	var previous time.Time
	for {
		deadline := offlineAfter
		if expected == "hello" {
			deadline = 5 * time.Second
		}
		readCtx, stop := context.WithTimeout(ctx, deadline)
		kind, data, err := conn.Read(readCtx)
		stop()
		if err != nil || kind != websocket.MessageText {
			return
		}
		msg, valid := parseControl(data, expected)
		now := d.now()
		if !valid || (!previous.IsZero() && now.Sub(previous) < time.Second) {
			return
		}
		d.mu.Lock()
		if d.active[id] != entry || ctx.Err() != nil {
			d.mu.Unlock()
			return
		}
		result, err := d.store.db.ExecContext(ctx, "UPDATE devices SET last_seen=? WHERE id=? AND revoked_at IS NULL", now.Unix(), id)
		if err != nil {
			d.mu.Unlock()
			return
		}
		n, err := result.RowsAffected()
		if err != nil || n != 1 {
			d.mu.Unlock()
			return
		}
		entry.lastSeen = now
		d.mu.Unlock()
		previous = now
		ack := controlMessage{Version: 1, Type: expected + "_ack", ID: msg.ID, CorrelationID: msg.ID}
		data, err = json.Marshal(ack)
		if err != nil {
			return
		}
		writeCtx, stopWrite := context.WithTimeout(ctx, 5*time.Second)
		err = conn.Write(writeCtx, websocket.MessageText, data)
		stopWrite()
		if err != nil {
			return
		}
		expected = "heartbeat"
	}
}
