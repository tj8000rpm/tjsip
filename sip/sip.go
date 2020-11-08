package sip

import (
	"io"
	"time"
)

// ProtocolError represents an SIP protocol error.
//
// Deprecated: Not all errors in the http package related to protocol errors
// are of type ProtocolError.
type ProtocolError struct {
	ErrorString string
}

func (pe *ProtocolError) Error() string { return pe.ErrorString }

var (
	// ErrNotSupported is returned by the Push method of Pusher
	// implementations to indicate that HTTP/2 Push support is not
	// available.
	ErrNotSupported = &ProtocolError{"feature not supported"}

	// Deprecated: ErrUnexpectedTrailer is no longer returned by
	// anything in the net/http package. Callers should not
	// compare errors against this variable.
	ErrUnexpectedTrailer = &ProtocolError{"trailer header without chunked transfer encoding"}

	// ErrMissingBoundary is returned by Request.MultipartReader when the
	// request's Content-Type does not include a "boundary" parameter.
	ErrMissingBoundary = &ProtocolError{"no multipart boundary param in Content-Type"}

	// Deprecated: ErrHeaderTooLong is no longer returned by
	// anything in the net/http package. Callers should not
	// compare errors against this variable.
	ErrHeaderTooLong = &ProtocolError{"header too long"}

	// Deprecated: ErrShortBody is no longer returned by
	// anything in the net/http package. Callers should not
	// compare errors against this variable.
	ErrShortBody = &ProtocolError{"entity body too short"}

	// Deprecated: ErrMissingContentLength is no longer returned by
	// anything in the net/http package. Callers should not
	// compare errors against this variable.
	ErrMissingContentLength = &ProtocolError{"missing ContentLength in HEAD response"}

	ErrMissingMandatoryHeader = &ProtocolError{"missing mandatory header in message"}
	ErrHeaderParseError       = &ProtocolError{"malformed headder"}
	ErrMalformedMessage       = &ProtocolError{"malformed message"}
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
