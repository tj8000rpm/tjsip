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
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ServerContextKey = &contextKey{"sip-server"}
	RecieveBufSizeB  = 9000
)

const (
	LayerSocket = iota
	LayerParserIngress
	LayerParserEgress
	LayerTransport
	LayerTransaction
	LayerCore
)

type atomicBool int32

func (b *atomicBool) isSet() bool { return atomic.LoadInt32((*int32)(b)) != 0 }
func (b *atomicBool) setTrue()    { atomic.StoreInt32((*int32)(b), 1) }
func (b *atomicBool) setFalse()   { atomic.StoreInt32((*int32)(b), 0) }

// -----------------------------------------------------
// About Server
// -----------------------------------------------------
type Server struct {
	Conn *net.UDPConn

	Host string
	AoR  string

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

	//transactionController TransactionController

	serverTransactions ServerTransactions
	clientTransactions ClientTransactions

	sentQueue chan *Message
}

type ServerTransactions struct {
	Mu           sync.Mutex
	Transactions map[ServerTransactionKey]*ServerTransaction
}

type ClientTransactions struct {
	Mu           sync.Mutex
	Transactions map[ClientTransactionKey]*ClientTransaction
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

func (s *Server) Criticalf(format string, args ...interface{}) {
	if LogLevel >= LogCritical {
		s.logf("[CRITICAL] "+format, args...)
	}
}

func (s *Server) Errorf(format string, args ...interface{}) {
	if LogLevel >= LogError {
		s.logf("[ERROR] "+format, args...)
	}
}

func (s *Server) Warnf(format string, args ...interface{}) {
	if LogLevel >= LogWarn {
		s.logf("[WARNING] "+format, args...)
	}
}

func (s *Server) Infof(format string, args ...interface{}) {
	if LogLevel >= LogInfo {
		s.logf("[INFO] "+format, args...)
	}
}

func (s *Server) Debugf(format string, args ...interface{}) {
	if LogLevel >= LogDebug {
		s.logf("[DEBUG] "+format, args...)
	}
}

func (srv *Server) handle(layer int, msg *Message) error {
	handler := srv.Handler
	if handler == nil {
		handler = DefaultServeMux
	}
	return handler.ServeSIP(layer, srv, msg)
}

func (srv *Server) socketHandler(msg *Message) error {
	return srv.handle(LayerSocket, msg)
}

func (srv *Server) ingress(msg *Message) error {
	return srv.handle(LayerParserIngress, msg)
}

func (srv *Server) egress(msg *Message) error {
	return srv.handle(LayerParserEgress, msg)
}

func (srv *Server) handleToCore(msg *Message) error {
	return srv.handle(LayerCore, msg)
}

func (srv *Server) HandleInTransaction(msg *Message) error {
	return srv.handleInTransaction(msg)
}

func (srv *Server) handleInTransaction(msg *Message) error {
	return srv.handle(LayerTransaction, msg)
}

func (*Server) newBufioReader(r io.Reader) *bufio.Reader {
	return bufio.NewReader(r)
}

func (srv *Server) Address() string {
	localAddr := srv.Conn.LocalAddr().String()
	if srv.Host != "" {
		localAddr = srv.Host
	}
	return localAddr
}

func (srv *Server) AorDomain() string {
	localAddr := srv.Conn.LocalAddr().String()

	if srv.AoR != "" {
		parsedAor, err := url.Parse(srv.AoR)
		if err != nil {
			return localAddr
		}
		localAddr = parsedAor.Host
	}
	return localAddr
}

func (srv *Server) packetProcessing(ctx context.Context, buf []byte, size int, addr *net.UDPAddr) {

	bufr := srv.newBufioReader(bytes.NewReader(buf[:size]))

	msg := CreateMessage(fmt.Sprintf("%v", addr))
	msg.ctx = ctx

	// For IP / socket level filter  etc...
	if err := srv.socketHandler(msg); err != nil {
		// packet ignored
		return
	}

	if err := ReadMessage(msg, bufr); err != nil {
		srv.Debugf("sip: Malformed packet - %v", err)
		// packet ignored
		return
	}

	// For SIP Message manipulation etc...
	if err := srv.ingress(msg); err != nil {
		srv.Warnf("Packet was dropped: %v", err)
		return
	}

	var transaction Transaction
	if msg.Request {
		query, err := GenerateServerTransactionKey(msg)
		if err != nil {
			//malformed packet
			srv.Infof("%v", err)
			return
		}
		transaction = srv.lookupServerTransaction(query)
		srv.Debugf("?[%v]=%v / %v", query, transaction, msg)
	} else if msg.Response {
		query, err := GenerateClientTransactionKey(msg)
		if err != nil {
			//malformed packet
			srv.Infof("%v", err)
			return
		}
		transaction = srv.lookupClientTransaction(query)
		srv.Debugf("?[%v]=%v / %v", query, transaction, msg)
	}

	if transaction == nil {
		err := srv.handleToCore(msg)
		if err != nil {
			// SIP Core return error
			srv.Warnf("%v", err)
			return
		}
	} else {
		transaction.Handle(msg)
	}
}

func (s *Server) AddServerTransaction(transaction *ServerTransaction) error {
	s.serverTransactions.Mu.Lock()
	defer s.serverTransactions.Mu.Unlock()
	if s.serverTransactions.Transactions == nil {
		s.serverTransactions.Transactions = make(map[ServerTransactionKey]*ServerTransaction)
	}
	key := transaction.Key
	_, ok := s.serverTransactions.Transactions[*key]
	if ok {
		s.Warnf("duplicated key: %v", key)
		s.Warnf("duplicated Transation: \n%v", *(s.serverTransactions.Transactions[*key]))
		// Transaction access will Racing
		return ErrTransactionDuplicated
	}
	s.serverTransactions.Transactions[*key] = transaction
	s.Debugf("Server Transaction size: %d", len(s.serverTransactions.Transactions))
	return nil
}

func (s *Server) DeleteServerTransaction(transaction *ServerTransaction) error {
	s.serverTransactions.Mu.Lock()
	defer s.serverTransactions.Mu.Unlock()
	key := transaction.Key
	if s.serverTransactions.Transactions == nil {
		return nil
	}
	_, ok := s.serverTransactions.Transactions[*key]
	if ok {
		// Transaction access will Racing
		delete(s.serverTransactions.Transactions, *key)
		s.Debugf("Server Transaction size: %d", len(s.serverTransactions.Transactions))
		return nil
	}
	return nil
}

func (s *Server) AddClientTransaction(transaction *ClientTransaction) error {
	s.clientTransactions.Mu.Lock()
	defer s.clientTransactions.Mu.Unlock()
	key := transaction.Key
	if s.clientTransactions.Transactions == nil {
		s.clientTransactions.Transactions = make(map[ClientTransactionKey]*ClientTransaction)
	}

	s.clientTransactions.Transactions[*key] = transaction
	s.Debugf("Client Transaction size: %d", len(s.clientTransactions.Transactions))
	return nil
}

func (s *Server) DeleteClientTransaction(transaction *ClientTransaction) error {
	s.clientTransactions.Mu.Lock()
	defer s.clientTransactions.Mu.Unlock()
	key := transaction.Key
	if s.clientTransactions.Transactions == nil {
		return nil
	}
	_, ok := s.clientTransactions.Transactions[*key]
	if ok {
		// Transaction access will Racing
		delete(s.clientTransactions.Transactions, *key)
		s.Debugf("client Transaction size: %d", len(s.clientTransactions.Transactions))
		return nil
	}
	return nil
}

func (s *Server) LookupServerTransaction(query *ServerTransactionKey) Transaction {
	return s.lookupServerTransaction(query)
}

func (s *Server) lookupServerTransaction(query *ServerTransactionKey) Transaction {
	s.serverTransactions.Mu.Lock()
	defer s.serverTransactions.Mu.Unlock()
	transaction, ok := s.serverTransactions.Transactions[*query]
	if !ok {
		// Transaction no found
		return nil
	}
	// Transaction found
	return transaction
}

func (s *Server) LookupClientTransaction(query *ClientTransactionKey) Transaction {
	return s.lookupClientTransaction(query)
}

func (s *Server) lookupClientTransaction(query *ClientTransactionKey) Transaction {
	s.clientTransactions.Mu.Lock()
	defer s.clientTransactions.Mu.Unlock()
	transaction, ok := s.clientTransactions.Transactions[*query]
	if !ok {
		// Transaction no found
		return nil
	}
	// Transaction found
	return transaction
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

func (srv *Server) WriteMessage(sentMsg *Message) error {
	srv.Debugf("Sent message to queue")
	localAddr := srv.Address()

	if sentMsg.Request {
		if sentMsg.Via == nil {
			sentMsg.Via = NewViaHeaders()
			if sentMsg.Via == nil {
				return ErrHeaderParseError
			}
		}
		if sentMsg.Via.TopMost() == nil || sentMsg.Via.TopMost().SentBy != localAddr {
			srv.Debugf("Via header appended : %v vs %v\n", sentMsg.Via.TopMost().SentBy, localAddr)
			v := NewViaHeaderUDP(localAddr, "branch="+GenerateBranchParam())
			sentMsg.Via.Insert(v)
		}
		if sentMsg.Contact == nil {
			sentMsg.Contact = NewContactHeaders()
			if sentMsg.Contact == nil {
				return ErrHeaderParseError
			}
		}
		if sentMsg.Contact.Length() == 0 {
			c := NewContactHeaderFromString("", "sip:"+localAddr, "")
			sentMsg.Contact.Add(c)
		}
	}

	// For SIP Message manipulation etc...
	if err := srv.egress(sentMsg); err != nil {
		srv.Warnf("Packet was dropped: %v", err)
		return err
	}

	w := new(bytes.Buffer)
	sentMsg.Write(w)
	srv.Debugf("sent to-------%v\n", sentMsg.RemoteAddr)
	srv.Debugf("msg-------\n%v\n", w)
	udpAddr, err := net.ResolveUDPAddr("udp", sentMsg.RemoteAddr)
	if err != nil {
		// Error ignore sent message
		return err
	}
	_, err = srv.Conn.WriteTo(w.Bytes(), udpAddr)
	if err != nil {
		// will raise transport error to TU
		return err
	}
	srv.Debugf("sent message done")
	return nil
}

func (srv *Server) Serve(udpLn *net.UDPConn) error {
	srv.doneChan = make(chan struct{})
	srv.sentQueue = make(chan *Message)
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
				srv.Warnf("sip: Accept error: %v; retrying in %v", err, tempDelay)
				time.Sleep(tempDelay)
				continue
			}
			return err
		}
		connCtx := ctx
		tempDelay = 0
		copiedBuf := make([]byte, n)
		n = copy(copiedBuf, buf)
		go srv.packetProcessing(connCtx, copiedBuf, n, addr)

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
	defer srv.Infof("UDP Server has been stoped")
	if err != nil {
		return err
	}
	srv.Infof("Starting UDP Server...")
	srv.Conn = ln
	return srv.Serve(ln)
}

func (s *Server) shuttingDown() bool {
	return s.inShutdown.isSet()
}

/* Server Message Helper */
func (srv *Server) CreateIniINVITE(addr, ruri string) (msg *Message) {
	msg = CreateINVITE(addr, ruri)

	localAddr := srv.Address()
	aorDomain := srv.AorDomain()

	msg.To = NewToHeaderFromString("", ruri, "")
	msg.From = NewFromHeaderFromString("", "sip:"+aorDomain, "tag="+GenerateTag())
	msg.Contact = NewContactHeaders()
	msg.Contact.Add(NewContactHeaderFromString("", "sip:"+localAddr, ""))
	msg.CSeq = NewCSeqHeader("INVITE")

	return msg
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
	ServeSIP(int, *Server, *Message) error
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
	layer int // TODO: change int to custom type
	id    string
}

// NewServeMux allocates and returns a new ServeMux.
func NewServeMux() *ServeMux { return new(ServeMux) }

var DefaultServeMux = &defaultServeMux

var defaultServeMux ServeMux

func (mux *ServeMux) ServeSIP(layer int, srv *Server, msg *Message) (err error) {
	handlers := mux.m[layer]
	if handlers == nil {
		return nil
	}
	for _, h := range handlers {
		err = h.h.ServeSIP(layer, srv, msg)
		if err != nil {
			return err
		}
	}
	return nil
}

func (mux *ServeMux) HandleFunc(layer int, id string, handler func(int, *Server, *Message) error) {
	if id == "" {
		panic("sip: invalid id")
	}
	if handler == nil {
		panic("sip: nil handler")
	}
	mux.Handle(layer, id, HandlerFunc(handler))
}

// Handle registers the handler for the given pattern.
// If a handler already exists for pattern, Handle panics.
func (mux *ServeMux) Handle(layer int, id string, handler Handler) {
	mux.mu.Lock()
	defer mux.mu.Unlock()

	/* TODO : compare iota
	if layer == "" {
		panic("sip: invalid layer")
	}
	*/
	if id == "" {
		panic("sip: invalid id")
	}
	if handler == nil {
		panic("sip: nil handler")
	}
	if stg, exist_stg := mux.m[layer]; exist_stg {
		if _, exist_id := stg[id]; exist_id {
			panic(fmt.Sprintf("sip: multiple registrations for %v/%v", layer, id))
		}
	}

	if mux.m == nil {
		mux.m = make(map[int]map[string]muxEntry)
	}
	if mux.m[layer] == nil {
		mux.m[layer] = make(map[string]muxEntry)
	}
	e := muxEntry{h: handler, layer: layer, id: id}
	mux.m[layer][id] = e
}

// Handle registers the handler for the given pattern
// in the DefaultServeMux.
// The documentation for ServeMux explains how patterns are matched.
func Handle(layer int, id string, handler Handler) { DefaultServeMux.Handle(layer, id, handler) }

// HandleFunc registers the handler function for the given condition function
// in the DefaultServeMux.
func HandleFunc(layer int, id string, handler func(int, *Server, *Message) error) {
	DefaultServeMux.HandleFunc(layer, id, handler)
}

// -----------------------------------------------------
// About HandlerFunc
// -----------------------------------------------------
// The HandlerFunc type is an adapter to allow the use of
// ordinary functions as HTTP handlers. If f is a function
// with the appropriate signature, HandlerFunc(f) is a
// Handler that calls f.
type HandlerFunc func(int, *Server, *Message) error

// ServeSIP calls f(layer, m).
func (f HandlerFunc) ServeSIP(layer int, s *Server, m *Message) error {
	return f(layer, s, m)
}
