// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// HTTP Request reading and parsing.

package sip

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/net/idna"
)

const (
	defaultMaxMemory = 32 << 20 // 32 MB
)

var (
	CallIdContextKey = &contextKey{"call-id"}
)

func badStringError(what, val string) error { return fmt.Errorf("%s %q", what, val) }

var mandatoryHeaders = []string{
	"From",
	"To",
	"Call-ID",
	"CSeq",
	"Via",
	"Max-Forwards",
}

// A Request represents an SIP request received by a server
// or to be sent by a client.
//
// The field semantics differ slightly between client and server
// usage. In addition to the notes on the fields below, see the
// documentation for Request.Write and RoundTripper.
type Message struct {
	RemoteAddr string

	// Sepcified for SIP Request
	Request    bool
	Method     string
	RequestURI *URI

	// Sepcified for SIP Response
	Response     bool
	StatusCode   int // e.g. 200
	ReasonPhrase string

	Proto      string // "SIP/2.0"
	ProtoMajor int    // 2
	ProtoMinor int    // 0

	To          *To
	From        *From
	Via         *ViaHeaders
	MaxForwards *MaxForwards
	CSeq        *CSeq
	CallID      *CallID
	Contact     *ContactHeaders
	Header      http.Header

	Body          []byte
	ContentLength int64
	Close         bool
	Cancel        <-chan struct{}

	ctx context.Context
}

func (msg *Message) Clone() (cpMsg *Message) {
	cpMsg = CreateMessage("")
	if cpMsg == nil {
		return nil
	}
	cpMsg.Response = msg.Response
	cpMsg.StatusCode = msg.StatusCode
	cpMsg.ReasonPhrase = msg.ReasonPhrase

	cpMsg.Request = msg.Request
	cpMsg.Method = msg.Method
	cpMsg.RequestURI = msg.RequestURI.Clone()

	cpMsg.StatusCode = msg.StatusCode

	cpMsg.Proto = msg.Proto
	cpMsg.ProtoMajor = msg.ProtoMajor
	cpMsg.ProtoMinor = msg.ProtoMinor

	if msg.To != nil {
		cpMsg.To = msg.To.Clone()
	}
	if msg.From != nil {
		cpMsg.From = msg.From.Clone()
	}
	if msg.Via != nil {
		cpMsg.Via = msg.Via.Clone()
	}
	if msg.CallID != nil {
		cpMsg.CallID = msg.CallID.Clone()
	}
	if msg.CSeq != nil {
		cpMsg.CSeq = msg.CSeq.Clone()
	}
	if msg.MaxForwards != nil {
		cpMsg.MaxForwards = msg.MaxForwards.Clone()
	}
	if msg.Contact != nil {
		cpMsg.Contact = msg.Contact.Clone()
	}

	if msg.Header != nil {
		cpMsg.Header = msg.Header.Clone()
	}

	cpMsg.Body = make([]byte, len(msg.Body))
	copy(cpMsg.Body, msg.Body)

	cpMsg.ContentLength = msg.ContentLength
	cpMsg.Close = msg.Close

	cpMsg.Cancel = make(<-chan struct{})
	cpMsg.ctx = msg.ctx

	return cpMsg
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

	// r2.URL = cloneURL(r.URL) // legacy behavior; TODO: try to remove. Issue 23544
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

func writeHeader(w io.Writer, r *Message) {
	ignores := make(map[string]bool)
	if r.To != nil {
		ignores["to"] = true
		fmt.Fprintf(w, "To: %s\r\n", r.To)
	}
	if r.From != nil {
		ignores["from"] = true
		fmt.Fprintf(w, "From: %s\r\n", r.From)
	}
	if r.Via != nil {
		ignores["via"] = true
		// Write order controlled by ViaHeaders Structure
		// Because it is stored as reverse order by ViaHeaders
		fmt.Fprintf(w, "%s", r.Via.WriteHeader())
	}
	if r.MaxForwards != nil {
		ignores["max-forwards"] = true
		fmt.Fprintf(w, "Max-Forwards: %s\r\n", r.MaxForwards)
	}
	if r.CallID != nil {
		ignores["call-id"] = true
		fmt.Fprintf(w, "Call-ID: %s\r\n", r.CallID)
	}
	if r.CSeq != nil {
		ignores["cseq"] = true
		fmt.Fprintf(w, "CSeq: %s\r\n", r.CSeq)
	}
	if r.Contact != nil {
		ignores["contact"] = true
		fmt.Fprintf(w, "%s", r.Contact.WriteHeader())
	}

	keys := make([]string, len(r.Header))
	orig := make(map[string]string)
	idx := 0
	for key, _ := range r.Header {
		keys[idx] = strings.ToLower(key)
		orig[strings.ToLower(key)] = key
		idx++
	}

	sort.Strings(keys)
	for _, key := range keys {
		if _, ok := ignores[key]; ok {
			continue
		}
		header := r.Header.Values(key)
		key = orig[key]
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

	writeHeader(w, r)

	fmt.Fprintf(w, "\r\n")

	// TODO: Write Body
	return nil
}

func (r *Message) writeRequest(w io.Writer) (err error) {
	if _, err := fmt.Fprintf(w, "%s %s SIP/%d.%d\r\n",
		r.Method, r.RequestURI.String(), r.ProtoMajor, r.ProtoMinor); err != nil {
		return err
	}

	writeHeader(w, r)

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
func NewRequest(method, url string, body []byte) (*Message, error) {
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
func NewRequestWithContext(ctx context.Context, method, url string, body []byte) (*Message, error) {
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
	u, err := Parse(url)
	if err != nil {
		return nil, err
	}
	// The host's colon:port should be normalized. See Issue 14836.
	msg := &Message{
		ctx:        ctx,
		Method:     method,
		RequestURI: u,
		Proto:      "SIP/2.0",
		ProtoMajor: 2,
		ProtoMinor: 0,
		Header:     make(http.Header),
		Body:       body,
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
		uri, err := Parse(firstLine2)
		if err != nil {
			return ErrMalformedMessage
		}
		msg.RequestURI = uri //firstLine2
		msg.Proto = firstLine3
		if !validMethod(msg.Method) {
			return badStringError("invalid method", msg.Method)
		}
		// rawurl := msg.RequestURI
		// if msg.URL, err = url.ParseRequestURI(rawurl); err != nil {
		// 	return err
		// }
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

	msg.parseHeader()

	len, err := b.Read(msg.Body)
	if err != nil {
		return err
	}
	msg.ContentLength = int64(len)

	return nil
}

func (msg *Message) parseHeader() {
	// To Header
	if to := msg.Header.Get("to"); to != "" {
		msg.To = ParseTo(to)
	}
	// From Header
	if from := msg.Header.Get("from"); from != "" {
		msg.From = ParseFrom(from)
	}
	// CallID Header
	if callid := msg.Header.Get("call-id"); callid != "" {
		msg.CallID = ParseCallID(callid)
	}
	// CSeq Header
	if cseq := msg.Header.Get("cseq"); cseq != "" {
		msg.CSeq = ParseCSeq(cseq)
	}
	// MaxForwards Header
	if maxforwards := msg.Header.Get("max-forwards"); maxforwards != "" {
		msg.MaxForwards = ParseMaxForwards(maxforwards)
	}
	// Via Header: It will have multiple contents
	if vias := msg.Header.Values("via"); len(vias) > 0 {
		msg.Via = NewViaHeaders()
		for _, v := range vias {
			err := ParseVias(v, msg.Via)
			if err != nil {
				// TODO: Should I have any action?
				continue
			}
		}
	}
	// Contact Header: It will have multiple contents
	if contacts := msg.Header.Values("Contact"); len(contacts) > 0 {
		msg.Contact = NewContactHeaders()
		for _, c := range contacts {
			err := ParseContacts(c, msg.Contact)
			if err != nil {
				// TODO: Should I have any action?
				continue
			}
		}
	}
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

	resp.To = msg.To
	resp.From = msg.From
	resp.Via = msg.Via
	resp.CallID = msg.CallID
	resp.CSeq = msg.CSeq
	resp.MaxForwards = msg.MaxForwards
	for _, key := range mandatoryHeaders {
		//for key, headers := range msg.Header {
		for _, header := range msg.Header.Values(key) {
			resp.Header.Add(key, header)
		}
		//}
	}

	resp.ctx = context.WithValue(msg.ctx, CallIdContextKey, resp.Header.Get("Call-ID"))

	return resp
}

func (msg *Message) AddFromTag() (err error) {
	if msg.From == nil {
		err = ErrMissingMandatoryHeader
	}
	if tag := msg.From.Parameter().Get("tag"); len(tag) != 0 {
		return nil
	}
	newParam := "tag=" + GenerateTag() + ";" + msg.From.RawParameter
	msg.From.RawParameter = newParam
	return nil
}

func (msg *Message) AddToTag() (err error) {
	if msg.To == nil {
		err = ErrMissingMandatoryHeader
	}
	if tag := msg.To.Parameter().Get("tag"); len(tag) != 0 {
		return nil
	}
	newParam := "tag=" + GenerateTag() + ";" + msg.To.RawParameter
	msg.To.RawParameter = newParam
	return nil
}

func (msg *Message) GetTopMostVia() (proto, sentBy string, params map[string][]string, err error) {
	if msg.Via == nil {
		return "", "", nil, ErrMissingMandatoryHeader
	}
	v := msg.Via.TopMost()
	proto = v.SentProtocol
	sentBy = v.SentBy
	params = v.Parameter()
	return
}

func (msg *Message) GetCSeq() (cseqMethod string, cseqNum int64, err error) {
	if msg.CSeq == nil {
		return "", 0, ErrMissingMandatoryHeader
	}
	return msg.CSeq.Method, msg.CSeq.Sequence, nil
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

// outgoingLength reports the Content-Length of this outgoing (Client) request.
// It maps 0 into -1 (unknown) when the Body is non-nil.
func (r *Message) outgoingLength() int64 {
	if r.Body == nil {
		return 0
	}
	if r.ContentLength != 0 {
		return r.ContentLength
	}
	return -1
}

func GenerateAckFromRequestAndResponse(req *Message, res *Message) (ack *Message, err error) {
	ack = CreateACK(req.RemoteAddr)
	if res == nil || req == nil || ack == nil {
		return nil, ErrMalformedMessage
	}
	if !req.Request || !res.Response {
		return nil, ErrMalformedMessage
	}

	if res.StatusCode >= 200 && res.StatusCode < 300 {
		ack.RequestURI = req.RequestURI
		ack.Via = NewViaHeaders()
		var via *Via
		for idx, v := range req.Via.Header {
			if idx == 0 {
				proto := v.SentProtocol
				setnBy := v.SentBy
				params := v.Parameter()
				if len(params["branch"]) == 0 {
					return nil, ErrMalformedMessage
				}

				params["branch"][0] = GenerateBranchParam()

				rawParam := ""
				for key, values := range params {
					for _, value := range values {
						rawParam += key
						rawParam += "="
						rawParam += value
					}
					rawParam += ";"
				}
				via = NewViaHeader(proto, setnBy, rawParam)
			} else {
				via = v
			}
			ack.Via.Append(via)
		}
		copyFromRequestHeadersIfPresent := []string{
			"Route",
		}

		ack.From = req.From
		ack.CallID = req.CallID
		ack.MaxForwards = req.MaxForwards
		ack.To = res.To

		for _, key := range copyFromRequestHeadersIfPresent {
			value := req.Header.Get(key)
			if value != "" {
				ack.Header.Add(key, value)
			}
		}
	} else {
		// TODO: No RFC comply implemantation
		// uri, err := Parse(res.Header.Get("Contact"))
		// uri := res.Contact.Header[0].Addr.Uri
		ack.RequestURI = req.RequestURI
		// ack.RequestURI = uri
		v := req.Via.TopMost()
		if v == nil {
			return nil, ErrHeaderParseError
		}
		ack.Via = NewViaHeaders()
		ack.Via.Insert(v)

		copyFromRequestHeadersIfPresent := []string{
			"Route",
		}

		ack.From = req.From
		ack.CallID = req.CallID
		ack.MaxForwards = req.MaxForwards
		ack.To = res.To
		for _, key := range copyFromRequestHeadersIfPresent {
			value := req.Header.Get(key)
			if value != "" {
				ack.Header.Add(key, value)
			}
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

func CreateMessage(addr string) (msg *Message) {
	msg = new(Message)
	if msg == nil {
		return nil
	}
	msg.RemoteAddr = addr
	msg.Proto = "SIP/2.0"
	msg.ProtoMajor = 2
	msg.ProtoMinor = 0

	msg.Header = make(http.Header)
	return msg
}

func CreateRequest(addr string) (msg *Message) {
	msg = CreateMessage(addr)
	msg.Request = true
	msg.MaxForwards = NewMaxForwardsHeader()
	msg.CallID = NewCallIDHeader()
	return msg
}

func CreateINVITE(addr, ruri string) (msg *Message) {
	msg = CreateRequest(addr)
	msg.Method = "INVITE"
	uri, err := Parse(ruri)
	if err != nil {
		return nil
	}
	msg.RequestURI = uri
	return msg
}

func CreateACK(addr string) (msg *Message) {
	msg = CreateRequest(addr)
	msg.Method = "ACK"
	return msg
}

func CreateBYE(addr, ruri string) (msg *Message) {
	msg = CreateRequest(addr)
	msg.Method = "BYE"
	uri, err := Parse(ruri)
	if err != nil {
		return nil
	}
	msg.RequestURI = uri
	return msg
}
