package main

import (
	//"github.com/tj8000rpm/tjsip/sip"
	"log"
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

var stat = callStat{}

var callGap = callGapControl{enable: false, last: time.Now()}

var responseContexts *ResponseCtxs

func main() {
	sip.RecieveBufSizeB = 9000
	log.SetOutput(os.Stdout)
	sip.LogLevel = sip.LogDebug
	//sip.LogLevel = sip.LogInfo

	responseContexts = NewResponseCtxs()

	if !loadRoutes(sip.LogLevel >= sip.LogDebug) {
		return
	}

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
	sip.HandleFunc(sip.LayerParserIngress, "module sip message manipulation", messageManipulationHandler)
	sip.HandleFunc(sip.LayerParserEgress, "module sip message manipulation", messageManipulationHandler)
	sip.HandleFunc(sip.LayerCore, "module sip core(proxy)", proxyCoreHandler)
	sip.HandleFunc(sip.LayerTransaction, "module sip core-transaction(proxy)", proxyCoreHandler)
	sip.ListenAndServe("192.168.0.51:5060", nil)
}
