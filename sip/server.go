package sip

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ServerContextKey = &contextKey{"sip-server"}
	RecieveBufSizeB  = 9000
)

type atomicBool int32

func (b *atomicBool) isSet() bool { return atomic.LoadInt32((*int32)(b)) != 0 }
func (b *atomicBool) setTrue()    { atomic.StoreInt32((*int32)(b), 1) }
func (b *atomicBool) setFalse()   { atomic.StoreInt32((*int32)(b), 0) }

// -----------------------------------------------------
// About Server
// -----------------------------------------------------
type Server struct {
	// Addr optionally specifies the TCP address for the server to listen on,
	// in the form "host:port". If empty, ":sip" (port 5060) is used.
	// The service names are defined in RFC 6335 and assigned by IANA.
	// See net.Dial for details of the address format.
	Addr string

	Handler Handler // handler to invoke, http.DefaultServeMux if nil

	// TODO: SIPS?
	// TLSConfig optionally provides a TLS configuration for use
	// by ServeTLS and ListenAndServeTLS. Note that this value is
	// cloned by ServeTLS and ListenAndServeTLS, so it's not
	// possible to modify the configuration with methods like
	// tls.Config.SetSessionTicketKeys. To use
	// SetSessionTicketKeys, use Server.Serve with a TLS Listener
	// instead.
	// TLSConfig *tls.Config

	// ReadTimeout is the maximum duration for reading the entire
	// request, including the body.
	//
	// Because ReadTimeout does not let Handlers make per-request
	// decisions on each request body's acceptable deadline or
	// upload rate, most users will prefer to use
	// ReadHeaderTimeout. It is valid to use them both.
	ReadTimeout time.Duration

	// ReadHeaderTimeout is the amount of time allowed to read
	// request headers. The connection's read deadline is reset
	// after reading the headers and the Handler can decide what
	// is considered too slow for the body. If ReadHeaderTimeout
	// is zero, the value of ReadTimeout is used. If both are
	// zero, there is no timeout.
	ReadHeaderTimeout time.Duration

	// WriteTimeout is the maximum duration before timing out
	// writes of the response. It is reset whenever a new
	// request's header is read. Like ReadTimeout, it does not
	// let Handlers make decisions on a per-request basis.
	WriteTimeout time.Duration

	// IdleTimeout is the maximum amount of time to wait for the
	// next request when keep-alives are enabled. If IdleTimeout
	// is zero, the value of ReadTimeout is used. If both are
	// zero, there is no timeout.
	IdleTimeout time.Duration

	// MaxHeaderBytes controls the maximum number of bytes the
	// server will read parsing the request header's keys and
	// values, including the request line. It does not limit the
	// size of the request body.
	// If zero, DefaultMaxHeaderBytes is used.
	MaxHeaderBytes int

	// TLSNextProto optionally specifies a function to take over
	// ownership of the provided TLS connection when an ALPN
	// protocol upgrade has occurred. The map key is the protocol
	// name negotiated. The Handler argument should be used to
	// handle HTTP requests and will initialize the Request's TLS
	// and RemoteAddr if not already set. The connection is
	// automatically closed when the function returns.
	// If TLSNextProto is not nil, HTTP/2 support is not enabled
	// automatically.
	// TLSNextProto map[string]func(*Server, *tls.Conn, Handler)

	// ErrorLog specifies an optional logger for errors accepting
	// connections, unexpected behavior from handlers, and
	// underlying FileSystem errors.
	// If nil, logging is done via the log package's standard logger.
	ErrorLog *log.Logger

	inShutdown atomicBool // true when when server is in shutdown

	mu sync.Mutex
	//listeners  map[*net.Listener]struct{}
	//activeConn map[*conn]struct{}
	doneChan   chan struct{}
	onShutdown []func()

	loglevel int
}

func (s *Server) logf(format string, args ...interface{}) {
	format = "sip: " + format
	if s.ErrorLog != nil {
		s.ErrorLog.Printf(format, args...)
	} else {
		log.Printf(format, args...)
	}
}

func (s *Server) log(args string) {
	if s.ErrorLog != nil {
		s.ErrorLog.Printf("sip: " + args)
	} else {
		log.Printf("sip: " + args)
	}
}

func (s *Server) criticalf(format string, args ...interface{}) {
	if LogLevel >= LogCritical {
		s.logf("[CRITICAL] "+format, args...)
	}
}

func (s *Server) errorf(format string, args ...interface{}) {
	if LogLevel >= LogError {
		s.logf("[ERROR] "+format, args...)
	}
}

func (s *Server) warnf(format string, args ...interface{}) {
	if LogLevel >= LogWarn {
		s.logf("[WARNING] "+format, args...)
	}
}

func (s *Server) infof(format string, args ...interface{}) {
	if LogLevel >= LogInfo {
		s.logf("[INFO] "+format, args...)
	}
}

func (s *Server) debugf(format string, args ...interface{}) {
	if LogLevel >= LogDebug {
		s.logf("[DEBUG] "+format, args...)
	}
}

func (srv *Server) handle(stage int, msg *Message) error {
	handler := srv.Handler
	if handler == nil {
		handler = DefaultServeMux
	}
	err := handler.ServeSIP(stage, msg)
	return err
}

func (srv *Server) transport(msg *Message) error {
	return srv.handle(StageTransport, msg)
}

func (srv *Server) ingress(msg *Message) error {
	return srv.handle(StageIngress, msg)
}

func (*Server) newBufioReader(r io.Reader) *bufio.Reader {
	return bufio.NewReader(r)
}

func (srv *Server) packetProcessing(ctx context.Context, buf []byte, size int, addr *net.UDPAddr) {

	bufr := srv.newBufioReader(bytes.NewReader(buf[:size]))
	msg := CreateMessage(fmt.Sprintf("%v", addr))
	msg.ctx = ctx

	if err := srv.transport(msg); err != nil {
		// packet ignored
		return
	}

	if err := ReadMessage(msg, bufr); err != nil {
		srv.debugf("sip: Malformed packet - %v", err)
		// packet ignored
		return
	}

	if err := srv.ingress(msg); err != nil {
		srv.warnf("Packet was dropped: %v", err)
		return
	}
}

func (s *Server) getDoneChan() <-chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getDoneChanLocked()
}

func (s *Server) getDoneChanLocked() chan struct{} {
	if s.doneChan == nil {
		s.doneChan = make(chan struct{})
	}
	return s.doneChan
}

func (s *Server) closeDoneChanLocked() {
	ch := s.getDoneChanLocked()
	select {
	case <-ch:
		// Already closed. Don't close again.
	default:
		// Safe to close here. We're the only closer, guarded
		// by s.mu.
		close(ch)
	}
}

func (srv *Server) Serve(udpLn *net.UDPConn) error {
	srv.doneChan = make(chan struct{})
	defer udpLn.Close()

	baseCtx := context.Background()
	ctx := context.WithValue(baseCtx, ServerContextKey, srv)

	var tempDelay time.Duration // how long to sleep on accept failure

	buf := make([]byte, RecieveBufSizeB)

	for {
		n, addr, err := udpLn.ReadFromUDP(buf)
		if err != nil {
			select {
			case <-srv.getDoneChan():
				return ErrServerClosed
			default:
			}
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				srv.logf("sip: Accept error: %v; retrying in %v", err, tempDelay)
				time.Sleep(tempDelay)
				continue
			}
			return err
		}
		connCtx := ctx
		tempDelay = 0
		go srv.packetProcessing(connCtx, buf, n, addr)
	}

	return nil
}

func (srv *Server) ListenAndServe() error {
	if srv.shuttingDown() {
		return ErrServerClosed
	}

	addr := srv.Addr
	if addr == "" {
		addr = ":sip"
	}

	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		log.Fatalln(err)
		return ErrServerClosed
	}

	ln, err := net.ListenUDP("udp", udpAddr)
	defer srv.infof("UDP Server has been stoped")
	if err != nil {
		return err
	}
	srv.infof("Starting UDP Server...")
	return srv.Serve(ln)
}

func (s *Server) shuttingDown() bool {
	return s.inShutdown.isSet()
}

// -----------------------------------------------------

// Copy from net/http original
// ErrServerClosed is returned by the Server's Serve, ServeTLS, ListenAndServe,
// and ListenAndServeTLS methods after a call to Shutdown or Close.
var ErrServerClosed = errors.New("sip: Server closed")

func ListenAndServe(addr string, handler Handler) error {
	server := &Server{Addr: addr, Handler: handler}
	return server.ListenAndServe()
}

// -----------------------------------------------------
// About Handler
// -----------------------------------------------------
type Handler interface {
	ServeSIP(int, *Message) error
}

// -----------------------------------------------------
// About ServerMux
// -----------------------------------------------------
// ServeMux is an HTTP request multiplexer.
// It matches the URL of each incoming request against a list of registered
// patterns and calls the handler for the pattern that
// most closely matches the URL.
//
// Patterns name fixed, rooted paths, like "/favicon.ico",
// or rooted subtrees, like "/images/" (note the trailing slash).
// Longer patterns take precedence over shorter ones, so that
// if there are handlers registered for both "/images/"
// and "/images/thumbnails/", the latter handler will be
// called for paths beginning "/images/thumbnails/" and the
// former will receive requests for any other paths in the
// "/images/" subtree.
//
// Note that since a pattern ending in a slash names a rooted subtree,
// the pattern "/" matches all paths not matched by other registered
// patterns, not just the URL with Path == "/".
//
// If a subtree has been registered and a request is received naming the
// subtree root without its trailing slash, ServeMux redirects that
// request to the subtree root (adding the trailing slash). This behavior can
// be overridden with a separate registration for the path without
// the trailing slash. For example, registering "/images/" causes ServeMux
// to redirect a request for "/images" to "/images/", unless "/images" has
// been registered separately.
//
// Patterns may optionally begin with a host name, restricting matches to
// URLs on that host only. Host-specific patterns take precedence over
// general patterns, so that a handler might register for the two patterns
// "/codesearch" and "codesearch.google.com/" without also taking over
// requests for "http://www.google.com/".
//
// ServeMux also takes care of sanitizing the URL request path and the Host
// header, stripping the port number and redirecting any request containing . or
// .. elements or repeated slashes to an equivalent, cleaner URL.
type ServeMux struct {
	mu sync.RWMutex
	m  map[int]map[string]muxEntry
	es []muxEntry // slice of entries sorted from longest to shortest.
}

type muxEntry struct {
	h     Handler
	stage int // TODO: change int to custom type
	id    string
}

// NewServeMux allocates and returns a new ServeMux.
func NewServeMux() *ServeMux { return new(ServeMux) }

var DefaultServeMux = &defaultServeMux

var defaultServeMux ServeMux

func (mux *ServeMux) ServeSIP(stage int, msg *Message) error {
	handlers := mux.m[stage]
	if handlers == nil {
		return nil
	}
	for _, h := range handlers {
		err := h.h.ServeSIP(stage, msg)
		if err != nil {
			return err
		}
	}
	return nil
}

func (mux *ServeMux) HandleFunc(stage int, id string, handler func(int, *Message) error) {
	if id == "" {
		panic("sip: invalid id")
	}
	if handler == nil {
		panic("sip: nil handler")
	}
	mux.Handle(stage, id, HandlerFunc(handler))
}

// Handle registers the handler for the given pattern.
// If a handler already exists for pattern, Handle panics.
func (mux *ServeMux) Handle(stage int, id string, handler Handler) {
	mux.mu.Lock()
	defer mux.mu.Unlock()

	/* TODO : compare iota
	if stage == "" {
		panic("sip: invalid stage")
	}
	*/
	if id == "" {
		panic("sip: invalid id")
	}
	if handler == nil {
		panic("sip: nil handler")
	}
	if stg, exist_stg := mux.m[stage]; exist_stg {
		if _, exist_id := stg[id]; exist_id {
			panic(fmt.Sprintf("sip: multiple registrations for %v/%v", stage, id))
		}
	}

	if mux.m == nil {
		mux.m = make(map[int]map[string]muxEntry)
	}
	if mux.m[stage] == nil {
		mux.m[stage] = make(map[string]muxEntry)
	}
	e := muxEntry{h: handler, stage: stage, id: id}
	mux.m[stage][id] = e
}

// Handle registers the handler for the given pattern
// in the DefaultServeMux.
// The documentation for ServeMux explains how patterns are matched.
func Handle(stage int, id string, handler Handler) { DefaultServeMux.Handle(stage, id, handler) }

// HandleFunc registers the handler function for the given condition function
// in the DefaultServeMux.
func HandleFunc(stage int, id string, handler func(int, *Message) error) {
	DefaultServeMux.HandleFunc(stage, id, handler)
}

// -----------------------------------------------------
// About HandlerFunc
// -----------------------------------------------------
// The HandlerFunc type is an adapter to allow the use of
// ordinary functions as HTTP handlers. If f is a function
// with the appropriate signature, HandlerFunc(f) is a
// Handler that calls f.
type HandlerFunc func(int, *Message) error

// ServeSIP calls f(stage, m).
func (f HandlerFunc) ServeSIP(stage int, m *Message) error {
	return f(stage, m)
}

// -----------------------------------------------------
// About response and ResponseWriter
// -----------------------------------------------------
/*
type response struct {
	req         *Message // request for this response
	reqBody     io.ReadCloser
	cancelCtx   context.CancelFunc // when ServeHTTP exits
	wroteHeader bool               // reply header has been (logically) written

	w *bufio.Writer // buffers output in chunks to chunkWriter

	// handlerHeader is the Header that Handlers get access to,
	// which may be retained and mutated even after WriteHeader.
	// handlerHeader is copied into cw.header at WriteHeader
	// time, and privately mutated thereafter.
	handlerHeader http.Header
	calledHeader  bool // handler accessed handlerHeader via Header

	written       int64 // number of bytes written in body
	contentLength int64 // explicitly-declared Content-Length; or -1
	status        int   // status code passed to WriteHeader

	// requestBodyLimitHit is set by requestTooLarge when
	// maxBytesReader hits its max size. It is checked in
	// WriteHeader, to make sure we don't consume the
	// remaining request body to try to advance to the next HTTP
	// request. Instead, when this is set, we stop reading
	// subsequent requests on this connection and stop reading
	// input from it.
	requestBodyLimitHit bool

	// trailers are the headers to be sent after the handler
	// finishes writing the body. This field is initialized from
	// the Trailer response header when the response header is
	// written.
	trailers []string

	handlerDone atomicBool // set true when the handler exits

	// Buffers for Date, Content-Length, and status code
	dateBuf   [len(TimeFormat)]byte
	clenBuf   [10]byte
	statusBuf [3]byte

	// closeNotifyCh is the channel returned by CloseNotify.
	// TODO(bradfitz): this is currently (for Go 1.8) always
	// non-nil. Make this lazily-created again as it used to be?
	closeNotifyCh  chan bool
	didCloseNotify int32 // atomic (only 0->1 winner should send)
}

func (w *response) WriteHeader(code int) {
	if w.conn.hijacked() {
		caller := relevantCaller()
		w.conn.server.logf("http: response.WriteHeader on hijacked connection from %s (%s:%d)", caller.Function, path.Base(caller.File), caller.Line)
		return
	}
	if w.wroteHeader {
		caller := relevantCaller()
		w.conn.server.logf("http: superfluous response.WriteHeader call from %s (%s:%d)", caller.Function, path.Base(caller.File), caller.Line)
		return
	}
	checkWriteHeaderCode(code)
	w.wroteHeader = true
	w.status = code

	if w.calledHeader && w.cw.header == nil {
		w.cw.header = w.handlerHeader.Clone()
	}

	if cl := w.handlerHeader.get("Content-Length"); cl != "" {
		v, err := strconv.ParseInt(cl, 10, 64)
		if err == nil && v >= 0 {
			w.contentLength = v
		} else {
			w.conn.server.logf("http: invalid Content-Length of %q", cl)
			w.handlerHeader.Del("Content-Length")
		}
	}
}

func (w *response) Write(data []byte) (n int, err error) {
	return w.write(len(data), data, "")
}

func (w *response) WriteString(data string) (n int, err error) {
	return w.write(len(data), nil, data)
}

// either dataB or dataS is non-zero.
func (w *response) write(lenData int, dataB []byte, dataS string) (n int, err error) {

	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if lenData == 0 {
		return 0, nil
	}
	if !w.bodyAllowed() {
		return 0, http.ErrBodyNotAllowed
	}

	w.written += int64(lenData) // ignoring errors, for errorKludge
	if w.contentLength != -1 && w.written > w.contentLength {
		return 0, http.ErrContentLength
	}
	if dataB != nil {
		return w.w.Write(dataB)
	} else {
		return w.w.WriteString(dataS)
	}
}
*/

type ResponseWriter interface {
	Header() http.Header
	Write([]byte) (int, error)
	WriteHeader(statusCode int)
}
