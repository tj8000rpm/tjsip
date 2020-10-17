package sip

import (
	"fmt"
	//"context"
	"bufio"
	"bytes"
	"io"
	"log"
	"net"
	"os"
	"sync"
	//"time"
)

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

	mu sync.Mutex
	//listeners  map[*net.Listener]struct{}
	//activeConn map[*conn]struct{}
	doneChan   chan struct{}
	onShutdown []func()
}

var recieveBufSizeB int

type RecieveBufs struct {
	Addr *net.UDPAddr
	Size int
	Buf  []byte
}

func (*Server) checkTransport(Addr *net.UDPAddr) {
	log.Printf("%v\n", Addr)
}

func (*Server) newBufioReader(r io.Reader) *bufio.Reader {
	return bufio.NewReader(r)
}

func (srv *Server) packetProcessing(done <-chan interface{}, queue *RecieveBufs, wg *sync.WaitGroup) {
	defer (*wg).Done()

	srv.checkTransport(queue.Addr)
	bufr := srv.newBufioReader(bytes.NewReader(queue.Buf[:queue.Size]))
	req, err := ReadRequest(bufr)
	if err == nil {
		// log.Printf("msg: %v", req)

		req.RemoteAddr = fmt.Sprintf("%v", queue.Addr)
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
	//log.Printf("From: %v Reciving data: %s", queue.Addr.String(), sipMessage.RawMessage)
	//:= make(chan interface{})
}

func (srv *Server) recieveQueues(done <-chan interface{}, queue <-chan RecieveBufs) {
	var recieveBuf RecieveBufs
	var wg sync.WaitGroup
loop:
	for {
		select {
		case recieveBuf = <-queue:
			// check udp layer filtering
			//log.Printf("From: %v Reciving data: %s", recieveBuf.Addr.String(), string(recieveBuf.Buf[:recieveBuf.Size]))
			wg.Add(1)
			go srv.packetProcessing(done, &recieveBuf, &wg)
		case <-done:
			log.Printf("Interupt break")
			break loop
		}
	}
	wg.Wait()
}

func (*Server) Serve(listenHost string, listenPort int) <-chan interface{} {

	srv.doneChan = make(chan struct{})

	go func() {
		udpAddr := &net.UDPAddr{
			IP:   net.ParseIP(listenHost),
			Port: listenPort,
		}

		udpLn, err := net.ListenUDP("udp", udpAddr)
		if err != nil {
			log.Fatalln(err)
		}
		defer log.Println("UDP Server has been stoped")
		defer udpLn.Close()

		buf := make([]byte, recieveBufSizeB)
		log.Println("Starting UDP Server...")

		recieveQueue := make(chan RecieveBufs)

		go srv.recieveQueues(doneChan, recieveQueue)

		for {
			n, addr, err := udpLn.ReadFromUDP(buf)
			if err != nil {
				log.Fatalln(err)
				break
			}
			recieveBuf := RecieveBufs{Addr: addr, Size: n, Buf: buf}
			recieveQueue <- recieveBuf
		}
	}()

	return doneChan
}
