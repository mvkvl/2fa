package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"strings"
	"time"
)

func normalizeSecret(secret string) string {
	secret = strings.Join(strings.Fields(secret), "")
	secret = strings.ToUpper(secret)
	secret = strings.TrimRight(secret, "=")
	return secret
}

func decodeSecret(secret string) ([]byte, error) {
	normalized := normalizeSecret(secret)
	if normalized == "" {
		return nil, fmt.Errorf("секретный ключ не может быть пустым")
	}
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(normalized)
	if err != nil {
		return nil, fmt.Errorf("ошибка декодирования секретного ключа: %w", err)
	}
	return key, nil
}

func normalizeAndValidateSecret(secret string) (string, error) {
	if _, err := decodeSecret(secret); err != nil {
		return "", err
	}
	return normalizeSecret(secret), nil
}

func generateTOTPAt(secret string, t time.Time) (string, error) {
	key, err := decodeSecret(secret)
	if err != nil {
		return "", err
	}

	timeStep := t.Unix() / 30
	timeBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(timeBytes, uint64(timeStep))

	mac := hmac.New(sha1.New, key)
	if _, err := mac.Write(timeBytes); err != nil {
		return "", err
	}
	hash := mac.Sum(nil)
	offset := hash[len(hash)-1] & 0x0f
	binaryCode := (int32(hash[offset])&0x7f)<<24 |
		(int32(hash[offset+1])&0xff)<<16 |
		(int32(hash[offset+2])&0xff)<<8 |
		(int32(hash[offset+3]) & 0xff)

	otp := binaryCode % 1_000_000
	return fmt.Sprintf("%06d", otp), nil
}

func generateTOTP(secret string) (string, error) {
	return generateTOTPAt(secret, time.Now())
}
