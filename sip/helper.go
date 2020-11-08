package sip

import (
	"crypto/rand"
	"fmt"
	"golang.org/x/net/http/httpguts"
	"math/big"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	MagicViaBranch = "z9hG4bK"
)

var (
	CallIdRandomLength          = 20
	TagLenghtWithoutMagicCookie = 20
	TagLength                   = 20
)

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

func isNotToken(r rune) bool {
	return !httpguts.IsTokenRune(r)
}

// ParseSIPVersion parses an SIP version string.
// "SIP/2.0" returns (2, 0, true).
func ParseSIPVersion(vers string) (major, minor int, ok bool) {
	const Big = 1000000 // arbitrary upper bound
	switch vers {
	case "SIP/2.0":
		return 2, 0, true
	}
	if !strings.HasPrefix(vers, "SIP/") {
		return 0, 0, false
	}
	dot := strings.Index(vers, ".")
	if dot < 0 {
		return 0, 0, false
	}
	major, err := strconv.Atoi(vers[5:dot])
	if err != nil || major < 0 || major > Big {
		return 0, 0, false
	}
	minor, err = strconv.Atoi(vers[dot+1:])
	if err != nil || minor < 0 || minor > Big {
		return 0, 0, false
	}
	return major, minor, true
}

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

func GenerateCallID() (string, error) {
	randStr, err := GenerateRandomString(CallIdRandomLength)
	if err != nil {
		return "", err
	}
	return randStr, nil
}

func GenerateInitCSeq() (int64, error) {
	ret, err := rand.Int(rand.Reader, big.NewInt(2<<31))
	if err != nil {
		return 0, err
	}
	return ret.Int64(), nil
}
