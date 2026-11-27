package main

import (
	"bytes"
	"testing"
)

func TestDPAPIRoundTrip(t *testing.T) {
	plain := []byte("test-only credential")
	protected, err := crypt(plain, true)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(protected, plain) {
		t.Fatal("plaintext persisted")
	}
	out, err := crypt(protected, false)
	if err != nil || !bytes.Equal(out, plain) {
		t.Fatal("DPAPI round trip failed")
	}
	protected[len(protected)/2] ^= 1
	if _, err := crypt(protected, false); err == nil {
		t.Fatal("tampered DPAPI data accepted")
	}
}
