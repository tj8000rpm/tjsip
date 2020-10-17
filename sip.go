package sip

import (
	"golang.org/x/net/http/httpguts"
	"io"
	"unicode/utf8"
)

var NoBody = noBody{}

type noBody struct{}

func (noBody) Read([]byte) (int, error)         { return 0, io.EOF }
func (noBody) Close() error                     { return nil }
func (noBody) WriteTo(io.Writer) (int64, error) { return 0, nil }

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
