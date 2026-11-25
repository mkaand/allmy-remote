package core

import "testing"

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name    string
		address string
		allow   string
		want    string
	}{
		{"default", "", "", "127.0.0.1:8080"},
		{"ipv6 loopback", "[::1]:9000", "", "[::1]:9000"},
		{"invalid port", "127.0.0.1:0", "", ""},
		{"missing port", "127.0.0.1", "", ""},
		{"wildcard denied", "0.0.0.0:8080", "", ""},
		{"hostname denied", "localhost:8080", "", ""},
		{"invalid flag", "127.0.0.1:8080", "yes", ""},
		{"container opt-in", "0.0.0.0:8080", "true", "0.0.0.0:8080"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := LoadConfig(func(key string) string {
				switch key {
				case "ALLMY_LISTEN":
					return tt.address
				case "ALLMY_ALLOW_NON_LOOPBACK":
					return tt.allow
				default:
					return ""
				}
			})
			if tt.want == "" {
				if err == nil {
					t.Fatal("expected invalid configuration")
				}
				return
			}
			if err != nil || cfg.Address != tt.want {
				t.Fatalf("got %v, %v; want %q", cfg, err, tt.want)
			}
		})
	}
}
