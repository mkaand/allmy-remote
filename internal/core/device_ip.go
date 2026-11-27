package core

import "net/http"

// Only a live authenticated device sample may be revealed to the operator.
// External addresses remain unavailable until the deployment's proxy chain
// can prove the client IP. Never trust arbitrary forwarding headers.
func (d *DeviceRegistry) revealIP(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Kind string `json:"kind"`
	}
	if !decodeRequest(w, r, &input) {
		return
	}
	if input.Kind != "internal" && input.Kind != "external" {
		status(w, 400, "invalid_ip_kind")
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	c := d.active[r.PathValue("id")]
	if c == nil || c.lastSeen.IsZero() || d.now().Sub(c.lastSeen) >= offlineAfter || c.sample == nil || input.Kind != "internal" || c.sample.InternalIP == "" {
		status(w, 404, "ip_unavailable")
		return
	}
	writeJSON(w, 200, map[string]string{"ip": c.sample.InternalIP})
}
