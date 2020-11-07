package sip

import (
	"golang.org/x/net/http/httpguts"
	"io"
	"time"
	"unicode/utf8"
)

const (
	CallInit = iota
	CallSetup
	CallEstablished
	CallReleasing
	CallReleased
)

const (
	LogCritical = iota
	LogError
	LogWarn
	LogInfo
	LogDebug
)

var (
	T1     = 500 * time.Millisecond
	T2     = 4 * time.Second
	T4     = 5 * time.Second
	TimerC = 3 * time.Minute
)

var (
	InitMaxForward = 70
)

var LogLevel = LogWarn

func IsCallEstablished(callStatus int) bool {
	return callStatus >= CallEstablished
}

// contextKey is a value for use with context.WithValue. It's used as
// a pointer so it fits in an interface{} without allocation.
type contextKey struct {
	name string
}

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
