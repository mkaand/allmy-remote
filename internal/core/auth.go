package core

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
)

const (
	cookieName = "allmy_session"
	sessionTTL = 12 * time.Hour
)

func randomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	return b, err
}

func passwordHash(password string, salt []byte) []byte {
	return argon2.IDKey([]byte(password), salt, 3, 32*1024, 1, 32)
}

type Auth struct {
	store  *Store
	origin string
	mu     sync.Mutex
	start  time.Time
	count  int
	hash   chan struct{}
	now    func() time.Time
}

func NewAuth(store *Store, origin string) *Auth {
	return &Auth{store: store, origin: origin, hash: make(chan struct{}, 1), now: time.Now}
}

// A bounded global limiter is deliberate for the single-operator preview.
// Production requires an abuse-resistant policy behind the trusted proxy.
func (a *Auth) allowLogin() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := a.now()
	if a.start.IsZero() || now.Sub(a.start) >= time.Minute {
		a.start, a.count = now, 0
	}
	if a.count >= 5 {
		return false
	}
	a.count++
	return true
}

func (a *Auth) sameOrigin(r *http.Request) bool {
	return r.Header.Get("Origin") == a.origin
}

func (a *Auth) session(w http.ResponseWriter, r *http.Request) {
	ok, err := a.valid(r)
	if err != nil {
		status(w, 503, "unavailable")
		return
	}
	if !ok {
		status(w, 401, "unauthorized")
		return
	}
	status(w, 200, "authenticated")
}

func (a *Auth) login(w http.ResponseWriter, r *http.Request) {
	if !a.sameOrigin(r) {
		status(w, 403, "forbidden")
		return
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		status(w, 415, "unsupported_media_type")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || decoder.Decode(new(any)) != io.EOF || len(input.Password) > 256 {
		status(w, 400, "invalid_request")
		return
	}
	if !a.allowLogin() {
		w.Header().Set("Retry-After", "60")
		status(w, 429, "rate_limited")
		return
	}
	select {
	case a.hash <- struct{}{}:
		defer func() { <-a.hash }()
	default:
		w.Header().Set("Retry-After", "1")
		status(w, 429, "rate_limited")
		return
	}
	var salt, expected []byte
	err = a.store.db.QueryRowContext(r.Context(), "SELECT salt, password_hash FROM operators WHERE id=1").Scan(&salt, &expected)
	exists := err == nil
	if errors.Is(err, sql.ErrNoRows) {
		salt, expected = make([]byte, 16), make([]byte, 32)
	} else if err != nil {
		status(w, 503, "unavailable")
		return
	}
	if len(salt) != 16 || len(expected) != 32 {
		status(w, 503, "unavailable")
		return
	}
	match := subtle.ConstantTimeCompare(passwordHash(input.Password, salt), expected) == 1
	if !exists || !match || input.Username != "admin" {
		status(w, 401, "unauthorized")
		return
	}
	raw, err := randomBytes(32)
	if err != nil {
		status(w, 503, "unavailable")
		return
	}
	hash := sha256.Sum256(raw)
	now := a.now()
	tx, err := a.store.db.BeginTx(r.Context(), nil)
	if err != nil {
		status(w, 503, "unavailable")
		return
	}
	defer tx.Rollback()
	// One active operator session initially; a new login revokes previous sessions.
	if _, err = tx.ExecContext(r.Context(), "DELETE FROM operator_sessions WHERE operator_id=1"); err != nil {
		status(w, 503, "unavailable")
		return
	}
	if _, err = tx.ExecContext(r.Context(), "INSERT INTO operator_sessions(token_hash,operator_id,expires_at) VALUES(?,1,?)", hash[:], now.Add(sessionTTL).Unix()); err != nil {
		status(w, 503, "unavailable")
		return
	}
	if err = tx.Commit(); err != nil {
		status(w, 503, "unavailable")
		return
	}
	http.SetCookie(w, sessionCookie(base64.RawURLEncoding.EncodeToString(raw), int(sessionTTL.Seconds())))
	status(w, 200, "authenticated")
}

func sessionCookie(value string, age int) *http.Cookie {
	return &http.Cookie{Name: cookieName, Value: value, Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode, MaxAge: age}
}

func cookieHash(r *http.Request) ([32]byte, bool) {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return [32]byte{}, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil || len(raw) != 32 {
		return [32]byte{}, false
	}
	return sha256.Sum256(raw), true
}

func (a *Auth) valid(r *http.Request) (bool, error) {
	hash, ok := cookieHash(r)
	if !ok {
		return false, nil
	}
	var id int
	err := a.store.db.QueryRowContext(r.Context(), "SELECT operator_id FROM operator_sessions WHERE token_hash=? AND expires_at>?", hash[:], a.now().Unix()).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil && id == 1, err
}

func (a *Auth) logout(w http.ResponseWriter, r *http.Request) {
	if !a.sameOrigin(r) {
		status(w, 403, "forbidden")
		return
	}
	hash, ok := cookieHash(r)
	if !ok {
		status(w, 401, "unauthorized")
		return
	}
	result, err := a.store.db.ExecContext(r.Context(), "DELETE FROM operator_sessions WHERE token_hash=? AND expires_at>?", hash[:], a.now().Unix())
	if err != nil {
		status(w, 503, "unavailable")
		return
	}
	n, err := result.RowsAffected()
	if err != nil {
		status(w, 503, "unavailable")
		return
	}
	http.SetCookie(w, sessionCookie("", -1))
	if n != 1 {
		status(w, 401, "unauthorized")
		return
	}
	status(w, 200, "logged_out")
}

func (a *Auth) close(ctx context.Context) error {
	_, err := a.store.db.ExecContext(ctx, "DELETE FROM operator_sessions WHERE expires_at<=?", a.now().Unix())
	return err
}
