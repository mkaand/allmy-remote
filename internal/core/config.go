package core

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// Config holds the local-only development listener settings.
type Config struct {
	Address string
}

// LoadConfig allows only loopback listeners unless explicitly overridden for
// isolated containers. This does not make the preview suitable for public use.
func LoadConfig(getenv func(string) string) (Config, error) {
	address := strings.TrimSpace(getenv("ALLMY_LISTEN"))
	if address == "" {
		address = "127.0.0.1:8080"
	}

	allow := strings.TrimSpace(getenv("ALLMY_ALLOW_NON_LOOPBACK"))
	if allow != "" && allow != "true" && allow != "false" {
		return Config{}, fmt.Errorf("invalid ALLMY_ALLOW_NON_LOOPBACK value")
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return Config{}, fmt.Errorf("ALLMY_LISTEN must contain an IP address and port")
	}
	ip := net.ParseIP(host)
	n, err := strconv.Atoi(port)
	if ip == nil || err != nil || n < 1 || n > 65535 {
		return Config{}, fmt.Errorf("ALLMY_LISTEN must contain an IP address and valid port")
	}
	if !ip.IsLoopback() && allow != "true" {
		return Config{}, fmt.Errorf("non-loopback listener requires explicit isolated-container opt-in")
	}
	return Config{Address: address}, nil
}
