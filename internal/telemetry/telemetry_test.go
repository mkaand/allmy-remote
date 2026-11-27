package telemetry

import "testing"

func TestSampleValidation(t *testing.T) {
	for _, raw := range []string{
		`{"cpu_percent":101}`,
		`{"ram_percent":-1}`,
		`{"disk_percent":0.5}`,
		`{"internal_ip":"not an ip"}`,
		`{"internal_ip":"127.0.0.1"}`,
		`{"external_ip":"192.0.2.1"}`,
		`{"command":"run"}`,
		`{}{}`,
	} {
		if _, err := Parse([]byte(raw)); err == nil {
			t.Fatal("invalid sample accepted")
		}
	}
	s, err := Parse([]byte(`{"cpu_percent":0,"ram_percent":100,"internal_ip":"192.0.2.5"}`))
	if err != nil || s.CPU == nil || *s.CPU != 0 || s.RAM == nil || *s.RAM != 100 || s.Disk != nil || s.InternalIP != "192.0.2.5" {
		t.Fatal("valid sample rejected")
	}
}
