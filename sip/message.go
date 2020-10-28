// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// HTTP Request reading and parsing.

package sip

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	urlpkg "net/url"
	"sort"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/net/idna"
)

const (
	defaultMaxMemory = 32 << 20 // 32 MB
)

// ProtocolError represents an HTTP protocol error.
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

var (
	CallIdContextKey = &contextKey{"call-id"}
)

func badStringError(what, val string) error { return fmt.Errorf("%s %q", what, val) }

var responseMandatoryHeaders = []string{
	"From",
	"Call-ID",
	"CSeq",
	"Via",
	"To",
}

// A Request represents an SIP request received by a server
// or to be sent by a client.
//
// The field semantics differ slightly between client and server
// usage. In addition to the notes on the fields below, see the
// documentation for Request.Write and RoundTripper.
type Message struct {
	// Sepcified for SIP Request
	Request bool
	Method  string
	URL     *url.URL

	// Sepcified for SIP Response
	Response bool
	// Status       string // e.g. "200 OK"
	StatusCode   int // e.g. 200
	ReasonPhrase string

	Proto      string // "SIP/2.0"
	ProtoMajor int    // 2
	ProtoMinor int    // 0

	Header http.Header

	Body          io.ReadCloser
	GetBody       func() (io.ReadCloser, error)
	ContentLength int64
	Close         bool
	Trailer       http.Header
	RemoteAddr    string
	RequestURI    string
	Cancel        <-chan struct{}

	ForRequest *Message

	ctx context.Context
}

// Context returns the request's context. To change the context, use
// WithContext.
//
// The returned context is always non-nil; it defaults to the
// background context.
//
// For outgoing client requests, the context controls cancellation.
//
// For incoming server requests, the context is canceled when the
// client's connection closes, the request is canceled (with HTTP/2),
// or when the ServeHTTP method returns.
func (r *Message) Context() context.Context {
	if r.ctx != nil {
		return r.ctx
	}
	return context.Background()
}

// WithContext returns a shallow copy of r with its context changed
// to ctx. The provided ctx must be non-nil.
//
// For outgoing client request, the context controls the entire
// lifetime of a request and its response: obtaining a connection,
// sending the request, and reading the response headers and body.
//
// To create a new request with a context, use NewRequestWithContext.
// To change the context of a request, such as an incoming request you
// want to modify before sending back out, use Request.Clone. Between
// those two uses, it's rare to need WithContext.
func (r *Message) WithContext(ctx context.Context) *Message {
	if ctx == nil {
		panic("nil context")
	}
	r2 := new(Message)
	*r2 = *r
	r2.ctx = ctx

	r2.URL = cloneURL(r.URL) // legacy behavior; TODO: try to remove. Issue 23544
	return r2
}

// Clone returns a deep copy of r with its context changed to ctx.
// The provided ctx must be non-nil.
//
// For an outgoing client request, the context controls the entire
// lifetime of a request and its response: obtaining a connection,
// sending the request, and reading the response headers and body.
func (r *Message) Clone(ctx context.Context) *Message {
	if ctx == nil {
		panic("nil context")
	}
	r2 := new(Message)
	*r2 = *r
	r2.ctx = ctx
	r2.URL = cloneURL(r.URL)
	if r.Header != nil {
		r2.Header = r.Header.Clone()
	}
	if r.Trailer != nil {
		r2.Trailer = r.Trailer.Clone()
	}
	return r2
}

// ProtoAtLeast reports whether the HTTP protocol used
// in the request is at least major.minor.
func (r *Message) ProtoAtLeast(major, minor int) bool {
	return r.ProtoMajor > major ||
		r.ProtoMajor == major && r.ProtoMinor >= minor
}

// UserAgent returns the client's User-Agent, if sent in the request.
func (r *Message) UserAgent() string {
	return r.Header.Get("User-Agent")
}

// Return value if nonempty, def otherwise.
func valueOrDefault(value, def string) string {
	if value != "" {
		return value
	}
	return def
}

// NOTE: This is not intended to reflect the actual Go version being used.
// It was changed at the time of Go 1.1 release because the former User-Agent
// had ended up blocked by some intrusion detection systems.
// See https://codereview.appspot.com/7532043.
const defaultUserAgent = "Go-tj-sip-client/1.0"

/*
// Write writes an HTTP/1.1 request, which is the header and body, in wire format.
// This method consults the following fields of the request:
//	Host
//	URL
//	Method (defaults to "GET")
//	Header
//	ContentLength
//	TransferEncoding
//	Body
//
// If Body is present, Content-Length is <= 0 and TransferEncoding
// hasn't been set to "identity", Write adds "Transfer-Encoding:
// chunked" to the header. Body is closed after it is sent.
*/
func (r *Message) Write(w io.Writer) error {
	return r.write(w)
}

func (r *Message) writeResponse(w io.Writer) (err error) {
	text := r.ReasonPhrase
	if text == "" {
		var ok bool
		text, ok = statusText[r.StatusCode]
		if !ok {
			text = "Unknown"
		}
	}
	if _, err := fmt.Fprintf(w, "SIP/%d.%d %03d %s\r\n",
		r.ProtoMajor, r.ProtoMinor, r.StatusCode, text); err != nil {
		return err
	}

	keys := make([]string, len(r.Header))
	idx := 0
	for key, _ := range r.Header {
		keys[idx] = key
		idx++
	}
	sort.Strings(keys)

	//for key, header := range r.Header {
	for _, key := range keys {
		header := r.Header[key]
		for _, value := range header {
			switch key {
			case "Cseq":
				key = "CSeq"
				break
			case "Call-Id":
				key = "Call-ID"
				break
			}
			fmt.Fprintf(w, "%v: %v\r\n", key, value)
		}

	}
	fmt.Fprintf(w, "\r\n")
	// TODO: Write Body
	return nil
}

func (r *Message) writeRequest(w io.Writer) (err error) {
	if _, err := fmt.Fprintf(w, "%s %s SIP/%d.%d\r\n",
		r.Method, r.RequestURI, r.ProtoMajor, r.ProtoMinor); err != nil {
		return err
	}

	keys := make([]string, len(r.Header))
	idx := 0
	for key, _ := range r.Header {
		keys[idx] = key
		idx++
	}
	sort.Strings(keys)

	//for key, header := range r.Header {
	for _, key := range keys {
		header := r.Header[key]
		for _, value := range header {
			switch key {
			case "Cseq":
				key = "CSeq"
				break
			case "Call-Id":
				key = "Call-ID"
				break
			}
			fmt.Fprintf(w, "%v: %v\r\n", key, value)
		}

	}
	fmt.Fprintf(w, "\r\n")
	// TODO: Write Body
	return nil
}

func (r *Message) write(w io.Writer) (err error) {
	var bw *bufio.Writer
	if _, ok := w.(io.ByteWriter); !ok {
		bw = bufio.NewWriter(w)
		w = bw
	}
	if r.Request {
		return r.writeRequest(w)
	} else if r.Response {
		return r.writeResponse(w)
	}
	return nil
}

// requestBodyReadError wraps an error from (*Message).write to indicate
// that the error came from a Read call on the Request.Body.
// This error type should not escape the net/http package to users.
type requestBodyReadError struct{ error }

func idnaASCII(v string) (string, error) {
	// TODO: Consider removing this check after verifying performance is okay.
	// Right now punycode verification, length checks, context checks, and the
	// permissible character tests are all omitted. It also prevents the ToASCII
	// call from salvaging an invalid IDN, when possible. As a result it may be
	// possible to have two IDNs that appear identical to the user where the
	// ASCII-only version causes an error downstream whereas the non-ASCII
	// version does not.
	// Note that for correct ASCII IDNs ToASCII will only do considerably more
	// work, but it will not cause an allocation.
	if isASCII(v) {
		return v, nil
	}
	return idna.Lookup.ToASCII(v)
}

// removeZone removes IPv6 zone identifier from host.
// E.g., "[fe80::1%en0]:8080" to "[fe80::1]:8080"
func removeZone(host string) string {
	if !strings.HasPrefix(host, "[") {
		return host
	}
	i := strings.LastIndex(host, "]")
	if i < 0 {
		return host
	}
	j := strings.LastIndex(host[:i], "%")
	if j < 0 {
		return host
	}
	return host[:j] + host[i:]
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

func validMethod(method string) bool {
	/* TODO:Update from RFC exclude 3261
	         Method            =  INVITEm / ACKm / OPTIONSm / BYEm
	                              / CANCELm / REGISTERm
	                              / extension-method
		   extension-method = token
		     token          = 1*<any CHAR except CTLs or separators>
	*/
	return len(method) > 0 && strings.IndexFunc(method, isNotToken) == -1
}

// NewRequest wraps NewRequestWithContext using the background context.
func NewRequest(method, url string, body io.Reader) (*Message, error) {
	return NewRequestWithContext(context.Background(), method, url, body)
}

// NewRequestWithContext returns a new Request given a method, URL, and
// optional body.
//
// If the provided body is also an io.Closer, the returned
// Request.Body is set to body and will be closed by the Client
// methods Do, Post, and PostForm, and Transport.RoundTrip.
//
// NewRequestWithContext returns a Request suitable for use with
// Client.Do or Transport.RoundTrip. To create a request for use with
// testing a Server Handler, either use the NewRequest function in the
// net/http/httptest package, use ReadRequest, or manually update the
// Request fields. For an outgoing client request, the context
// controls the entire lifetime of a request and its response:
// obtaining a connection, sending the request, and reading the
// response headers and body. See the Request type's documentation for
// the difference between inbound and outbound request fields.
//
// If body is of type *bytes.Buffer, *bytes.Reader, or
// *strings.Reader, the returned request's ContentLength is set to its
// exact value (instead of -1), GetBody is populated (so 307 and 308
// redirects can replay the body), and Body is set to NoBody if the
// ContentLength is 0.
func NewRequestWithContext(ctx context.Context, method, url string, body io.Reader) (*Message, error) {
	if method == "" {
		// We document that "" means "INVITE" for Request.Method, and people have
		// relied on that from NewRequest, so keep that working.
		// We still enforce validMethod for non-empty methods.
		method = "INVITE"
	}
	if !validMethod(method) {
		return nil, fmt.Errorf("tjsip: invalid method %q", method)
	}
	if ctx == nil {
		return nil, errors.New("jtsip: nil Context")
	}
	u, err := urlpkg.Parse(url)
	if err != nil {
		return nil, err
	}
	rc, ok := body.(io.ReadCloser)
	if !ok && body != nil {
		rc = ioutil.NopCloser(body)
	}
	// The host's colon:port should be normalized. See Issue 14836.
	msg := &Message{
		ctx:        ctx,
		Method:     method,
		URL:        u,
		Proto:      "SIP/2.0",
		ProtoMajor: 2,
		ProtoMinor: 0,
		Header:     make(http.Header),
		Body:       rc,
	}
	if body != nil {
		switch v := body.(type) {
		case *bytes.Buffer:
			msg.ContentLength = int64(v.Len())
			buf := v.Bytes()
			msg.GetBody = func() (io.ReadCloser, error) {
				r := bytes.NewReader(buf)
				return ioutil.NopCloser(r), nil
			}
		case *bytes.Reader:
			msg.ContentLength = int64(v.Len())
			snapshot := *v
			msg.GetBody = func() (io.ReadCloser, error) {
				r := snapshot
				return ioutil.NopCloser(&r), nil
			}
		case *strings.Reader:
			msg.ContentLength = int64(v.Len())
			snapshot := *v
			msg.GetBody = func() (io.ReadCloser, error) {
				r := snapshot
				return ioutil.NopCloser(&r), nil
			}
		default:
			// This is where we'd set it to -1 (at least
			// if body != NoBody) to mean unknown, but
			// that broke people during the Go 1.8 testing
			// period. People depend on it being 0 I
			// guess. Maybe retry later. See Issue 18117.
		}
		// For client requests, Request.ContentLength of 0
		// means either actually 0, or unknown. The only way
		// to explicitly say that the ContentLength is zero is
		// to set the Body to nil. But turns out too much code
		// depends on NewRequest returning a non-nil Body,
		// so we use a well-known ReadCloser variable instead
		// and have the http package also treat that sentinel
		// variable to mean explicitly zero.
		if msg.GetBody != nil && msg.ContentLength == 0 {
			msg.Body = NoBody
			msg.GetBody = func() (io.ReadCloser, error) { return NoBody, nil }
		}
	}

	return msg, nil
}

// BasicAuth returns the username and password provided in the request's
// Authorization header, if the request uses HTTP Basic Authentication.
// See RFC 2617, Section 2.
func (r *Message) BasicAuth() (username, password string, ok bool) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return
	}
	return parseBasicAuth(auth)
}

// parseBasicAuth parses an HTTP Basic Authentication string.
// "Basic QWxhZGRpbjpvcGVuIHNlc2FtZQ==" returns ("Aladdin", "open sesame", true).
func parseBasicAuth(auth string) (username, password string, ok bool) {
	const prefix = "Basic "
	// Case insensitive prefix match. See Issue 22736.
	if len(auth) < len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) {
		return
	}
	c, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
	if err != nil {
		return
	}
	cs := string(c)
	s := strings.IndexByte(cs, ':')
	if s < 0 {
		return
	}
	return cs[:s], cs[s+1:], true
}

// SetBasicAuth sets the request's Authorization header to use HTTP
// Basic Authentication with the provided username and password.
//
// With HTTP Basic Authentication the provided username and password
// are not encrypted.
//
// Some protocols may impose additional requirements on pre-escaping the
// username and password. For instance, when used with OAuth2, both arguments
// must be URL encoded first with url.QueryEscape.
func (r *Message) SetBasicAuth(username, password string) {
	r.Header.Set("Authorization", "Basic "+basicAuth(username, password))
}

// parseRequestLine parses "INVITE /foo SIp/2.0" into its three parts.
func parseRequestLine(line string) (method, requestURI, proto string, ok bool) {
	s1 := strings.Index(line, " ")
	s2 := strings.Index(line[s1+1:], " ")
	if s1 < 0 || s2 < 0 {
		return
	}
	s2 += s1 + 1
	return line[:s1], line[s1+1 : s2], line[s2+1:], true
}

var textprotoReaderPool sync.Pool

func newTextprotoReader(br *bufio.Reader) *Reader {
	if v := textprotoReaderPool.Get(); v != nil {
		//tr := v.(*textproto.Reader)
		tr := v.(*Reader)
		tr.R = br
		return tr
	}
	//return textproto.NewReader(br)
	return NewReader(br)
}

//func putTextprotoReader(r *textproto.Reader) {
func putTextprotoReader(r *Reader) {
	r.R = nil
	textprotoReaderPool.Put(r)
}

// ReadRequest reads and parses an incoming request from b.
//
// ReadRequest is a low-level function and should only be used for
// specialized applications; most code should use the Server to read
// requests and handle them via the Handler interface. ReadRequest
// only supports HTTP/1.x requests. For HTTP/2, use golang.org/x/net/http2.
func ReadMessage(req *Message, b *bufio.Reader) error {
	return readMessage(req, b)
}

func CreateMessage(addr string) (msg *Message) {
	msg = new(Message)
	if msg != nil {
		msg.RemoteAddr = addr
	}
	return msg
}

func readMessage(msg *Message, b *bufio.Reader) (err error) {
	tp := newTextprotoReader(b)

	// First line: INVITE sip:alice@atlanta.example.com SIP/2.0
	var line string
	if line, err = tp.ReadLine(); err != nil {
		return err
	}
	defer func() {
		putTextprotoReader(tp)
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
	}()

	var ok bool
	firstLine1, firstLine2, firstLine3, ok := parseRequestLine(line)
	// msg.Method, msg.RequestURI, msg.Proto, ok = parseRequestLine(line)
	msg.Request = false
	msg.Response = false

	if !ok {
		return badStringError("malformed SIP message", line)
	}

	if msg.ProtoMajor, msg.ProtoMinor, ok = ParseSIPVersion(firstLine3); ok {
		// this message will be Request Message
		msg.Method = firstLine1
		msg.RequestURI = firstLine2
		msg.Proto = firstLine3
		if !validMethod(msg.Method) {
			return badStringError("invalid method", msg.Method)
		}
		rawurl := msg.RequestURI
		if msg.URL, err = url.ParseRequestURI(rawurl); err != nil {
			return err
		}
		msg.Request = true
	} else {
		// this message will be Response Message
		msg.Proto = firstLine1
		if msg.ProtoMajor, msg.ProtoMinor, ok = ParseSIPVersion(msg.Proto); !ok {
			return badStringError("malformed SIP version", msg.Proto)
		}
		// status := strings.TrimLeft(firstLine2+" "+firstLine3, " ")
		msg.ReasonPhrase = firstLine3
		statusCode := firstLine2
		if len(statusCode) != 3 {
			return badStringError("malformed SIP status code", statusCode)
		}
		msg.StatusCode, err = strconv.Atoi(statusCode)
		if err != nil || msg.StatusCode < 0 {
			return badStringError("malformed SIP status code", statusCode)
		}
		msg.Response = true
	}

	// Subsequent lines: Key: value.
	mimeHeader, err := tp.ReadMIMEHeader()
	if err != nil {
		return err
	}
	msg.Header = http.Header(mimeHeader)

	return nil
}

func (msg *Message) GenerateResponseFromRequest() (resp *Message) {
	resp = CreateMessage(msg.RemoteAddr)
	if resp == nil {
		return nil
	}
	resp.Response = true
	resp.Request = false

	resp.StatusCode = StatusTrying

	resp.Proto = "SIP/2.0"
	resp.ProtoMajor = 2
	resp.ProtoMinor = 0

	resp.Header = make(http.Header)

	for _, key := range responseMandatoryHeaders {
		//for key, headers := range msg.Header {
		for _, header := range msg.Header.Values(key) {
			resp.Header.Add(key, header)
		}
		//}
	}

	resp.ctx = context.WithValue(msg.ctx, CallIdContextKey, resp.Header.Get("Call-ID"))

	return resp
}

func (msg *Message) addTag(headerkey string) error {
	to := msg.Header.Get(headerkey)
	if to == "" {
		return ErrMissingMandatoryHeader
	}
	for _, content := range strings.Split(to, ";") {
		if strings.HasPrefix(strings.Trim(content, " "), "tag=") {
			return nil
		}
	}
	generatedTag, err := GenerateRandomString(15)
	if err != nil {
		return err
	}
	msg.Header.Set(headerkey, to+";tag="+generatedTag)

	return nil
}

func (msg *Message) AddFromTag() (err error) {
	if err = msg.addTag("From"); err != nil {
		err = msg.addTag("f")
	}
	return err
}

func (msg *Message) AddToTag() (err error) {
	if err = msg.addTag("To"); err != nil {
		err = msg.addTag("t")
	}
	return err
}

func ViaParser(s string) (proto, sentBy string, via_params map[string]string, err error) {

	temp_string_slice := strings.SplitN(strings.Trim(s, " "), " ", 2)
	if len(temp_string_slice) < 2 {
		return "", "", nil, ErrHeaderParseError
	}

	proto = temp_string_slice[0]

	temp_string_slice = strings.Split(strings.Trim(temp_string_slice[1], " "), ";")

	sentBy = temp_string_slice[0]
	for _, param := range temp_string_slice[1:] {
		if via_params == nil {
			via_params = make(map[string]string)
		}
		key_val_slic := strings.SplitN(param, "=", 2)
		key := key_val_slic[0]
		val := key_val_slic[1]
		via_params[key] = val
	}

	return proto, sentBy, via_params, nil
}

func (msg *Message) GetTopMostVia() (proto, sentBy string, params map[string]string, err error) {
	topmostvia := msg.Header.Get("Via")
	if topmostvia == "" {
		return "", "", nil, ErrMissingMandatoryHeader
	}
	topmostvia = strings.Split(topmostvia, ",")[0]

	return ViaParser(topmostvia)
}

func (msg *Message) GetCSeq() (cseqMethod string, cseqNum int, err error) {
	cseq := msg.Header.Get("CSeq")
	if cseq == "" {
		return "", 0, ErrMissingMandatoryHeader
	}
	cseqItem := strings.SplitN(cseq, " ", 2)
	cseqNum, err = strconv.Atoi(cseqItem[0])
	if err != nil {
		return "", 0, ErrHeaderParseError
	}
	cseqMethod = cseqItem[1]
	return cseqMethod, cseqNum, nil
}

func copyValues(dst, src url.Values) {
	for k, vs := range src {
		dst[k] = append(dst[k], vs...)
	}
}

// TODO Modify to adopt for 100 rel
func (r *Message) expectsContinue() bool {
	return hasToken(r.Header.Get("Expect"), "100-continue")
}

func (r *Message) closeBody() {
	if r.Body != nil {
		r.Body.Close()
	}
}

// outgoingLength reports the Content-Length of this outgoing (Client) request.
// It maps 0 into -1 (unknown) when the Body is non-nil.
func (r *Message) outgoingLength() int64 {
	if r.Body == nil || r.Body == NoBody {
		return 0
	}
	if r.ContentLength != 0 {
		return r.ContentLength
	}
	return -1
}

func GenerateAckFromRequestAndResponse(req *Message, res *Message) (ack *Message, err error) {
	ack = CreateMessage(req.RemoteAddr)
	if res == nil || req == nil || ack == nil {
		return nil, ErrMalformedMessage
	}
	if !req.Request || !res.Response {
		return nil, ErrMalformedMessage
	}

	ack.Response = false
	ack.Request = true
	ack.Method = "ACK"
	ack.Proto = "SIP/2.0"
	ack.ProtoMajor = 2
	ack.ProtoMinor = 0
	ack.Header = make(http.Header)

	reqVia := req.Header.Values("Via")
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		ack.RequestURI = req.RequestURI
		for idx, viaHeader := range reqVia {
			if idx == 0 {
				proto, sentBy, params, err := ViaParser(viaHeader)
				if err != nil {
					return nil, err
				}
				params["branch"] = GenerateBranchParam()
				newVia := fmt.Sprintf("%s %s", proto, sentBy)
				for key, val := range params {
					newVia += fmt.Sprintf(";%s=%s", key, val)
				}
				ack.Header.Add("Via", newVia)
			}
			ack.Header.Add("Via", viaHeader)

		}
		copyFromRequestHeaders := []string{
			"Call-ID",
			"From",
			"Max-Forward",
		}
		copyFromRequestHeadersIfPresent := []string{
			"Route",
		}
		copyFromResponseHeaders := []string{
			"To",
		}
		for _, key := range copyFromRequestHeaders {
			ack.Header.Add(key, req.Header.Get(key))
		}
		for _, key := range copyFromRequestHeadersIfPresent {
			value := req.Header.Get(key)
			if value != "" {
				ack.Header.Add(key, value)
			}
		}
		for _, key := range copyFromResponseHeaders {
			ack.Header.Add(key, res.Header.Get(key))
		}
	} else {
		// TODO: No RFC comply implemantation
		ack.RequestURI = res.Header.Get("Contact")
		topmostvia := req.Header.Get("Via")
		if topmostvia == "" {
			return nil, ErrHeaderParseError
		}
		topmostvia = strings.Split(topmostvia, ",")[0]
		ack.Header.Add("Via", topmostvia)

		copyFromRequestHeaders := []string{
			"Call-ID",
			"From",
			"Max-Forward",
		}
		copyFromRequestHeadersIfPresent := []string{
			"Route",
		}
		copyFromResponseHeaders := []string{
			"To",
		}

		for _, key := range copyFromRequestHeaders {
			ack.Header.Add(key, req.Header.Get(key))
		}
		for _, key := range copyFromRequestHeadersIfPresent {
			value := req.Header.Get(key)
			if value != "" {
				ack.Header.Add(key, value)
			}
		}
		for _, key := range copyFromResponseHeaders {
			ack.Header.Add(key, res.Header.Get(key))
		}
	}
	_, cseqNum, err := req.GetCSeq()
	if err != nil {
		return nil, err
	}
	ack.Header.Add("CSeq", fmt.Sprintf("%d ACK", cseqNum))

	ack.ctx = req.ctx //context.WithValue(req.ctx, CallIdContextKey, req.Header.Get("Call-ID"))

	return ack, nil
}
