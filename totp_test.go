package main

import (
	"testing"
	"time"
)

func TestGenerateTOTPAtRFC6238(t *testing.T) {
	const secret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	cases := []struct {
		unix int64
		want string
	}{
		{59, "287082"},
		{1111111109, "081804"},
		{1111111111, "050471"},
		{1234567890, "005924"},
		{2000000000, "279037"},
		{20000000000, "353130"},
	}
	for _, tc := range cases {
		got, err := generateTOTPAt(secret, time.Unix(tc.unix, 0))
		if err != nil {
			t.Fatalf("generateTOTPAt(%d): %v", tc.unix, err)
		}
		if got != tc.want {
			t.Errorf("generateTOTPAt(%d) = %q, want %q", tc.unix, got, tc.want)
		}
	}
}

func TestNormalizeAndValidateSecret(t *testing.T) {
	got, err := normalizeAndValidateSecret("  gezd gnbv gy3t qojq gezd gnbv gy3t qojq== ")
	if err != nil {
		t.Fatalf("normalizeAndValidateSecret: %v", err)
	}
	const want = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	if got != want {
		t.Errorf("normalized secret = %q, want %q", got, want)
	}
}

func TestDecodeSecretInvalid(t *testing.T) {
	if _, err := decodeSecret("not-base32!"); err == nil {
		t.Fatal("expected error for invalid base32")
	}
	if _, err := decodeSecret("   "); err == nil {
		t.Fatal("expected error for empty secret")
	}
}
