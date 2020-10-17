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

type Handler interface {
	ServerSIP(ResponseWriter, *Request)
}

type ResponseWriter interface {
	Header() http.Header
	Write([]byte) (int, error)
	WriteHeader(statusCode int)
}

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
}

func (s *Server) logf(format string, args ...interface{}) {
	if s.ErrorLog != nil {
		s.ErrorLog.Printf(format, args...)
	} else {
		log.Printf(format, args...)
	}
}

func (*Server) checkTransport(Addr *net.UDPAddr) {
	log.Printf("%v\n", Addr)
}

func (*Server) newBufioReader(r io.Reader) *bufio.Reader {
	return bufio.NewReader(r)
}

func (srv *Server) packetProcessing(ctx context.Context, buf []byte, size int, addr *net.UDPAddr) {

	srv.checkTransport(addr)
	bufr := srv.newBufioReader(bytes.NewReader(buf[:size]))
	req, err := ReadRequest(bufr)
	if err == nil {
		// log.Printf("msg: %v", req)

		req.RemoteAddr = fmt.Sprintf("%v", addr)
		log.Printf("remoteAddr: %v", req.RemoteAddr)
		log.Printf("URI: %v", req.RequestURI)
		// log.Printf("url: %v", req.URL)
		// log.Printf("url.Scheme: %v", req.URL.Scheme)
		// log.Printf("url.Opaque: %v", req.URL.Opaque)
		// log.Printf("url.Host: %v", req.URL.Host)
		// log.Printf("url.User: %v", req.URL.User)
		// log.Printf("url.Path: %v", req.URL.Path)
		// log.Printf("via: %v", req.Header.Get("Via"))

		//for k, v := range req.Header {
		//	for i, vv := range v {
		//		log.Printf("%v -%v: %v\n", k, i, vv)
		//	}
		//}
	}
	_ = ctx
	//log.Printf("From: %v Reciving data: %s", queue.Addr.String(), sipMessage.RawMessage)
	//:= make(chan interface{})
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
	defer log.Println("UDP Server has been stoped")
	if err != nil {
		return err
	}
	log.Println("Starting UDP Server...")
	return srv.Serve(ln)
}

func (s *Server) shuttingDown() bool {
	return s.inShutdown.isSet()
}

// ErrServerClosed is returned by the Server's Serve, ServeTLS, ListenAndServe,
// and ListenAndServeTLS methods after a call to Shutdown or Close.
var ErrServerClosed = errors.New("sip: Server closed")

func ListenAndServe(addr string, handler Handler) error {
	server := &Server{Addr: addr, Handler: handler}
	return server.ListenAndServe()
}
