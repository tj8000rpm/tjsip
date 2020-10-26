package main

import (
	//"github.com/tj8000rpm/tjsip/sip"
	"log"
	// "net"
	// "fmt"
	"os"
	"sip/sip"
	"sync"
	"time"
)

type lastAccess struct {
	remoteAddr string
	lastCall   time.Time
}

type callGapControl struct {
	enable         bool
	duration       time.Duration
	mu             sync.Mutex
	last           time.Time
	restrictedcall int
}

type callStat struct {
	mu                   sync.Mutex
	completed            int
	completedPerResponse [700]int
}

func (stat *callStat) Increment(response int) {
	stat.mu.Lock()
	defer stat.mu.Unlock()
	stat.completed++
	stat.completedPerResponse[response] += 1
}

var stat = callStat{
	//completedPerResponse: make(map[int]int),
}

var callGap = callGapControl{enable: false, last: time.Now()}

func myHandlerRedirect(srv *sip.Server, msg *sip.Message, trans *sip.ServerTransaction) error {
	rep := msg.GenerateResponseFromRequest()
	rep.AddToTag()
	time.Sleep(time.Millisecond * 50)

	rep.StatusCode = 300
	rep.Header.Set("Contact", "<sip:0312341234@example.com>")
	trans.WriteMessage(rep)
	stat.Increment(300)

	return nil
}

func myHandlerInvite(srv *sip.Server, msg *sip.Message, trans *sip.ServerTransaction) error {
	rep := msg.GenerateResponseFromRequest()

	rep.StatusCode = 180
	rep.AddToTag()
	trans.WriteMessage(rep)
	time.Sleep(time.Millisecond * 1000)
	rep.StatusCode = 183
	trans.WriteMessage(rep)
	time.Sleep(time.Second * 3)
	rep.StatusCode = 200
	trans.WriteMessage(rep)
	stat.Increment(200)

	return nil
}

func myHandlerNonInvite(srv *sip.Server, msg *sip.Message, trans *sip.ServerTransaction) error {
	if msg.Request && msg.Method == "ACK" {
		srv.Debugf("Dialog was established\n")
		return nil
	}
	err := srv.AddServerTransaction(msg, trans)
	if err != nil {
		srv.Warnf("%v", err)
		trans.Destroy()
		return err
	}
	rep := msg.GenerateResponseFromRequest()
	rep.StatusCode = 200
	trans.WriteMessage(rep)
	return nil
}
func myHandler(layer int, srv *sip.Server, msg *sip.Message) error {
	// remoteAddr, err := net.ResolveUDPAddr("udp", req.RemoteAddr)
	// remoteIPStr := remoteAddr.IP.String()

	if layer == sip.LayerSocket {
		if callGap.enable {
			now := time.Now()
			cur_duration := now.Sub(callGap.last)
			if callGap.duration >= cur_duration {
				callGap.mu.Lock()
				callGap.restrictedcall++
				callGap.mu.Unlock()
				return sip.ErrServerClosed
			}
			callGap.mu.Lock()
			callGap.last = now
			callGap.mu.Unlock()
		}

		log.Printf("ORIG: cllled in Transport\n")
		return nil
	} else if layer == sip.LayerParserIngress {

		log.Printf("ORIG: cllled in Ingress\n")
		log.Printf("ORIG: remoteAddr : %v\n", msg.RemoteAddr)
		/*
			if msg.Request {
				log.Printf("ORIG: URI : %v\n", msg.RequestURI)
				log.Printf("ORIG: RESPONSE : %v\n", rep)
			} else {
				log.Printf("ORIG: Status Code : %v\n", msg.StatusCode)
				log.Printf("ORIG: Status : %v\n", msg.Status)
			}
		*/
		return nil
	}
	return nil
}

func mySipCoreHandler(layer int, srv *sip.Server, msg *sip.Message) error {
	if layer != sip.LayerCore {
		return nil
	}
	if msg.Request {
		key, err := sip.GenerateServerTransactionKey(msg)
		if err != nil {
			return err
		}
		var newTransaction *sip.ServerTransaction

		if msg.Method == "INVITE" {
			newTransaction = sip.NewServerInviteTransaction(srv, key, msg)
			err := srv.AddServerTransaction(msg, newTransaction)
			if err != nil {
				srv.Warnf("%v", err)
				newTransaction.Destroy()
				return err
			}
			if msg.RemoteAddr == "127.0.0.1:5062" {
				return myHandlerRedirect(srv, msg, newTransaction)
			}
			return myHandlerInvite(srv, msg, newTransaction)
		} else {
			newTransaction = sip.NewServerNonInviteTransaction(srv, key, msg)
			return myHandlerNonInvite(srv, msg, newTransaction)
		}
		return nil
	}
	return nil
}

func main() {
	sip.RecieveBufSizeB = 9000
	log.SetOutput(os.Stdout)
	sip.LogLevel = sip.LogDebug
	sip.LogLevel = sip.LogInfo
	go func() {
		for {
			time.Sleep(time.Second * 5)
			stat.mu.Lock()
			log.Printf("Call completed: %v\n", stat.completed)
			for idx, val := range stat.completedPerResponse {
				if val == 0 {
					continue
				}
				log.Printf("Call completed[%03d]: %v\n", idx, val)
			}
			stat.mu.Unlock()
		}
	}()

	// sip.Timer100Try = 0 * time.Second

	//sip.HandleFunc(sip.LayerSocket, "odd test", myhandler)
	//sip.HandleFunc(sip.LayerParser, "odd test", myhandler)
	sip.HandleFunc(sip.LayerCore, "odd test", mySipCoreHandler)
	sip.ListenAndServe("", nil)
}
