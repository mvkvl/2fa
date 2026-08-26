package main

import (
	"bufio"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
	"time"
)

func generateTOTP(secret string) (string, error) {
	secret = strings.TrimSpace(secret)
	secret = strings.ReplaceAll(secret, " ", "")
	secret = strings.ToUpper(secret)

	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		return "", fmt.Errorf("ошибка декодирования секретного ключа: %v", err)
	}

	epoch := time.Now().Unix()
	timeStep := epoch / 30

	timeBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(timeBytes, uint64(timeStep))

	hmacSha1 := hmac.New(sha1.New, key)
	hmacSha1.Write(timeBytes)
	hash := hmacSha1.Sum(nil)

	offset := hash[len(hash)-1] & 0xf
	binaryCode := (int32(hash[offset])&0x7f)<<24 |
		(int32(hash[offset+1])&0xff)<<16 |
		(int32(hash[offset+2])&0xff)<<8 |
		(int32(hash[offset+3])&0xff)

	otp := binaryCode % 1_000_000
	return fmt.Sprintf("%06d", otp), nil
}

func main() {
	var secret string

	if len(os.Args) > 1 {
		secret = strings.Join(os.Args[1:], " ")
	} else {
		fmt.Print("Введите секретный ключ (Base32): ")
		reader := bufio.NewReader(os.Stdin)
		secret, _ = reader.ReadString('\n')
		secret = strings.TrimSpace(secret)
	}

	if secret == "" {
		fmt.Fprintln(os.Stderr, "Ошибка: секретный ключ не может быть пустым")
		os.Exit(1)
	}

	code, err := generateTOTP(secret)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		os.Exit(1)
	}

	fmt.Println(code)
}
