package sip

import (
	"crypto/rand"
	"fmt"
	"strings"
)

const (
	MagicViaBranch = "z9hG4bK"
)

var (
	TagLenghtWithoutMagicCookie = 20
	TagLength                   = 20
)

func GenerateBranchParam() string {
	ret, err := GenerateRandomString(TagLenghtWithoutMagicCookie)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%v%v", MagicViaBranch, ret)
}

func GenerateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func GenerateRandomString(n int) (string, error) {
	const letters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-"
	bytes, err := GenerateRandomBytes(n)
	if err != nil {
		return "", err
	}
	for i, b := range bytes {
		bytes[i] = letters[b%byte(len(letters))]
	}
	return string(bytes), nil
}

func IsValidBarnchParam(test string) bool {
	return strings.HasPrefix(test, "z9hG4bK")
}

func GenerateTag() string {
	ret, err := GenerateRandomString(TagLength)
	if err != nil {
		return ""
	}
	return ret
}
