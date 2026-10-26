package main

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestVerifyChecksum(t *testing.T) {
	bin := []byte("binary")
	sum := sha256.Sum256(bin)
	sums := []byte(hex.EncodeToString(sum[:]) + "  pi-display-arm64\n" +
		"0000000000000000000000000000000000000000000000000000000000000000  pi-display-armhf\n")

	if err := verifyChecksum(sums, "pi-display-arm64", bin); err != nil {
		t.Errorf("matching checksum: %v", err)
	}
	if err := verifyChecksum(sums, "pi-display-armhf", bin); err == nil {
		t.Error("mismatched checksum: expected error")
	}
	if err := verifyChecksum(sums, "pi-display-riscv64", bin); err == nil {
		t.Error("missing entry: expected error")
	}
}
