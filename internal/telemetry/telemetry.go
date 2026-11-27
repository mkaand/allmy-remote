// Package telemetry defines bounded, self-reported live device measurements.
package telemetry

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/netip"
)

const Protocol = "allmy.telemetry.v1"

type Metrics struct {
	CPU  *uint8 `json:"cpu_percent,omitempty"`
	RAM  *uint8 `json:"ram_percent,omitempty"`
	Disk *uint8 `json:"disk_percent,omitempty"`
}

type Sample struct {
	Metrics
	InternalIP string `json:"internal_ip,omitempty"`
}

func Address(value string) string {
	addr, err := netip.ParseAddr(value)
	if err != nil || addr.Zone() != "" {
		return ""
	}
	addr = addr.Unmap()
	if !addr.IsGlobalUnicast() && !addr.IsLinkLocalUnicast() {
		return ""
	}
	return addr.String()
}

func Parse(data []byte) (Sample, error) {
	var s Sample
	if len(data) > 512 {
		return s, errors.New("telemetry too large")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if dec.Decode(&s) != nil || dec.Decode(new(any)) != io.EOF {
		return Sample{}, errors.New("invalid telemetry")
	}
	for _, v := range []*uint8{s.CPU, s.RAM, s.Disk} {
		if v != nil && *v > 100 {
			return Sample{}, errors.New("invalid percentage")
		}
	}
	if s.InternalIP != "" && Address(s.InternalIP) != s.InternalIP {
		return Sample{}, errors.New("invalid address")
	}
	return s, nil
}
