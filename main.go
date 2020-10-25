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

var callGap = callGapControl{enable: false, last: time.Now()}

func myHandlerInvite(srv *sip.Server, msg *sip.Message, trans *sip.ServerTransaction) error {
	time.Sleep(time.Millisecond * 100)
	rep := msg.GenerateResponseFromRequest()

	/*/
	rep.StatusCode = 180
	rep.Status = "180 Ringing"
	rep.AddToTag()
	trans.WriteMessage(rep)
	// fmt.Println("Ringing!!!")
	time.Sleep(time.Millisecond * 1000)
	rep.StatusCode = 183
	rep.Status = "183 Session Progess"
	trans.WriteMessage(rep)
	// fmt.Println("Progress!!!!!")
	time.Sleep(time.Second * 3)
	rep.StatusCode = 200
	rep.Status = "200 OK"
	trans.WriteMessage(rep)

	/**/
	time.Sleep(time.Second * 3)
	rep.StatusCode = 300
	rep.Status = "300 Multiple Choices"
	trans.WriteMessage(rep)
	/**/

	return nil
}

func myHandlerNonInvite(srv *sip.Server, msg *sip.Message, trans *sip.ServerTransaction) error {
	// log.Printf("Non Invite Message was Recieved\n")
	// log.Printf("!-!-!-!-!-!-!-!\n%v\n!-!-!-!-!-!-!-!", msg)
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
	rep.Status = "200 OK"
	trans.WriteMessage(rep)
	// fmt.Println("OK!!!!")
	return nil
}

func myhandler(layer int, srv *sip.Server, msg *sip.Message) error {
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
	} else if layer == sip.LayerParser {

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
	} else if layer == sip.LayerCore {
		// log.Printf("ORIG: cllled in SIP Core\n")
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
				return myHandlerInvite(srv, msg, newTransaction)
			} else {
				newTransaction = sip.NewServerNonInviteTransaction(srv, key, msg)
				return myHandlerNonInvite(srv, msg, newTransaction)
			}
			return nil
		}
		return nil
	}
	return nil
}

func main() {
	sip.RecieveBufSizeB = 9000
	log.SetOutput(os.Stdout)
	sip.LogLevel = sip.LogDebug
	//sip.LogLevel = sip.LogInfo
	go func() {
		time.Sleep(time.Second * 5)
		callGap.mu.Lock()
		defer callGap.mu.Unlock()

		log.Println("call restricted")
		callGap.enable = true
		callGap.duration = time.Second * 1
	}()
	go func() {
		time.Sleep(time.Second * 30)
		callGap.mu.Lock()
		defer callGap.mu.Unlock()

		callGap.enable = false
	}()

	// sip.Timer100Try = 0 * time.Second

	//sip.HandleFunc(sip.LayerSocket, "odd test", myhandler)
	//sip.HandleFunc(sip.LayerParser, "odd test", myhandler)
	sip.HandleFunc(sip.LayerCore, "odd test", myhandler)
	sip.ListenAndServe("", nil)
}
