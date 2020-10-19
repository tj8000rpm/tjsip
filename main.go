package main

import (
	//"github.com/tj8000rpm/tjsip/sip"
	"log"
	// "net"
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

func myhandler(stage int, msg *sip.Message) error {
	// remoteAddr, err := net.ResolveUDPAddr("udp", req.RemoteAddr)
	// remoteIPStr := remoteAddr.IP.String()

	if stage == sip.StageTransport {
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
	} else {
		log.Printf("ORIG: cllled in Ingress\n")
		log.Printf("ORIG: remoteAddr : %v\n", msg.RemoteAddr)
		if msg.Request {
			log.Printf("ORIG: URI : %v\n", msg.RequestURI)
			rep := msg.GenerateResponseFromRequest()
			rep.AddToTag()
			log.Printf("ORIG: RESPONSE : %v\n", rep)
		} else {
			log.Printf("ORIG: Status Code : %v\n", msg.StatusCode)
			log.Printf("ORIG: Status : %v\n", msg.Status)
		}
	}

	return nil
}

func main() {
	sip.RecieveBufSizeB = 9000
	log.SetOutput(os.Stdout)
	sip.LogLevel = sip.LogDebug
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

	sip.HandleFunc(sip.StageTransport, "odd test", myhandler)
	sip.HandleFunc(sip.StageIngress, "odd test", myhandler)
	sip.ListenAndServe("", nil)
}
