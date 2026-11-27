// Package agent implements the explicitly enrolled, control-only native client.
package agent

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	randv2 "math/rand/v2"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coder/websocket"
)

type State struct {
	Origin     string `json:"origin"`
	ID         string `json:"id"`
	Credential string `json:"credential"`
}

var ErrDenied = errors.New("access denied; check enrollment or access policy")

func Origin(value string) (string, error) {
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || (u.Path != "" && u.Path != "/") || u.RawPath != "" {
		return "", errors.New("server must be an HTTPS origin without a path or credentials")
	}
	return strings.TrimSuffix(u.String(), "/"), nil
}

func token(value string, size int) bool {
	b, err := base64.RawURLEncoding.Strict().DecodeString(value)
	return err == nil && len(b) == size && base64.RawURLEncoding.EncodeToString(b) == value
}

func (s State) Valid() bool {
	o, err := Origin(s.Origin)
	return err == nil && o == s.Origin && token(s.ID, 16) && token(s.Credential, 32)
}

// Disable redirects, including cross-origin redirects, for every secret-bearing request.
func client(base *http.Client) *http.Client {
	c := *base
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &c
}

func Enroll(ctx context.Context, origin, grant string, base *http.Client) (State, error) {
	origin, err := Origin(origin)
	if err != nil || !token(grant, 32) {
		return State{}, errors.New("invalid server or enrollment grant")
	}
	body, _ := json.Marshal(map[string]string{"grant": grant, "platform": "windows"})
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", origin+"/api/v1/enrollment/exchange", bytes.NewReader(body))
	if err != nil {
		return State{}, errors.New("invalid enrollment request")
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := client(base).Do(req)
	if err != nil {
		return State{}, errors.New("enrollment connection failed")
	}
	defer res.Body.Close()
	if res.StatusCode != 201 {
		return State{}, fmt.Errorf("enrollment rejected (HTTP %d)", res.StatusCode)
	}
	var result struct {
		ID         string `json:"id"`
		Credential string `json:"credential"`
		Platform   string `json:"platform"`
	}
	dec := json.NewDecoder(io.LimitReader(res.Body, 4097))
	dec.DisallowUnknownFields()
	if dec.Decode(&result) != nil || dec.Decode(new(any)) != io.EOF {
		return State{}, errors.New("invalid enrollment response")
	}
	s := State{Origin: origin, ID: result.ID, Credential: result.Credential}
	if result.Platform != "windows" || !s.Valid() {
		return State{}, errors.New("invalid enrollment identity")
	}
	return s, nil
}

type message struct {
	Version       int    `json:"version"`
	Type          string `json:"type"`
	ID            string `json:"id"`
	CorrelationID string `json:"correlation_id,omitempty"`
}

func exchange(ctx context.Context, conn *websocket.Conn, kind string) error {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return errors.New("message ID unavailable")
	}
	id := base64.RawURLEncoding.EncodeToString(raw[:])
	body, _ := json.Marshal(message{Version: 1, Type: kind, ID: id})
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageText, body); err != nil {
		return errors.New("channel write failed")
	}
	typ, data, err := conn.Read(ctx)
	if err != nil || typ != websocket.MessageText {
		return errors.New("channel acknowledgement missing")
	}
	var ack message
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if dec.Decode(&ack) != nil || dec.Decode(new(any)) != io.EOF || ack.Version != 1 || ack.Type != kind+"_ack" || ack.ID != id || ack.CorrelationID != id {
		return errors.New("invalid channel acknowledgement")
	}
	return nil
}

func session(ctx context.Context, s State, base *http.Client, online func()) error {
	headers := http.Header{"Authorization": {"Bearer " + s.Credential}}
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	conn, response, err := websocket.Dial(dialCtx, "wss"+strings.TrimPrefix(s.Origin, "https")+"/api/v1/device/channel", &websocket.DialOptions{
		HTTPClient: client(base), HTTPHeader: headers,
	})
	cancel()
	if err != nil {
		if response != nil && (response.StatusCode == 401 || response.StatusCode == 403 || response.StatusCode/100 == 3) {
			return ErrDenied
		}
		return errors.New("channel connection failed")
	}
	defer conn.CloseNow()
	conn.SetReadLimit(64 << 10)
	if err := exchange(ctx, conn, "hello"); err != nil {
		return err
	}
	online()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := exchange(ctx, conn, "heartbeat"); err != nil {
				return err
			}
		}
	}
}

// Run never executes inbound commands. It accepts only acknowledgements to its own messages.
func Run(ctx context.Context, s State, base *http.Client, report func(string)) error {
	if !s.Valid() {
		return errors.New("invalid saved enrollment")
	}
	delay := time.Second
	for ctx.Err() == nil {
		err := session(ctx, s, base, func() {
			delay = time.Second
			report("Connected; heartbeat only.")
		})
		if ctx.Err() != nil {
			break
		}
		if errors.Is(err, ErrDenied) {
			return err
		}
		report("Disconnected; retrying.")
		wait := delay/2 + time.Duration(randv2Int(int64(delay/2)+1))
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
		delay = min(60*time.Second, 2*delay)
	}
	return nil
}

func randv2Int(n int64) int64 { return randv2.Int64N(n) }
