package core

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestDeriveKeyRejectsUnsafeParams(t *testing.T) {
	base := KDFParams{
		Name:    "argon2id",
		Salt:    base64.StdEncoding.EncodeToString(make([]byte, 16)),
		Time:    3,
		Memory:  64 * 1024,
		Threads: 4,
		KeyLen:  32,
	}

	cases := []struct {
		name   string
		mutate func(*KDFParams)
	}{
		{"key length", func(k *KDFParams) { k.KeyLen = 16 }},
		{"time", func(k *KDFParams) { k.Time = 0 }},
		{"threads", func(k *KDFParams) { k.Threads = 0 }},
		{"memory", func(k *KDFParams) { k.Memory = 1 }},
		{"salt", func(k *KDFParams) { k.Salt = base64.StdEncoding.EncodeToString(make([]byte, 4)) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kdf := base
			tc.mutate(&kdf)
			if _, err := deriveKey("passphrase", &kdf); err == nil {
				t.Fatalf("deriveKey accepted unsafe KDF params: %+v", kdf)
			}
		})
	}
}

func TestReadLineKeepsRemainingInput(t *testing.T) {
	r := strings.NewReader("first\r\nsecond\nthird")

	for i, want := range []string{"first", "second", "third"} {
		got, err := readLine(r)
		if err != nil {
			t.Fatalf("readLine #%d: %v", i+1, err)
		}
		if got != want {
			t.Fatalf("readLine #%d = %q, want %q", i+1, got, want)
		}
	}

	got, err := readLine(r)
	if err != nil {
		t.Fatalf("readLine at EOF: %v", err)
	}
	if got != "" {
		t.Fatalf("readLine at EOF = %q, want empty string", got)
	}
}
