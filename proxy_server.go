package main

import (
	//"github.com/tj8000rpm/tjsip/sip"
	"encoding/csv"
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

type Fwd struct {
	Addr   string
	Domain string
}

type Routes struct {
	mu    sync.Mutex
	table map[string]*Fwd
}

var routes *Routes

func loadRoutes() bool {
	if routes == nil {
		routes = new(Routes)
		if routes == nil {
			return false
		}
		routes.table = make(map[string]*Fwd)
		if routes.table == nil {
			return false
		}
	}
	fp, err := os.Open("routes.csv")
	if err != nil {
		panic(err)
	}
	defer fp.Close()

	reader := csv.NewReader(fp)
	var line []string

	for {
		line, err = reader.Read()
		if err != nil {
			break
		}
		if len(line) != 3 {
			return false
		}
		fwd := new(Fwd)
		if fwd == nil {
			return false
		}
		fwd.Addr = line[1]
		fwd.Domain = line[2]
		routes.table[line[0]] = fwd
	}

	log.Printf("load route \n")

	for k, v := range routes.table {
		log.Printf(" %s -> (%s, %s)\n", k, v.Addr, v.Domain)
	}

	return true
}

var callGap = callGapControl{enable: false, last: time.Now()}

var dialogs *Dialogs
var earlyDialogs *EarlyDialogs
var responseCtxs *ResponseCtxs

func main() {
	sip.RecieveBufSizeB = 9000
	log.SetOutput(os.Stdout)
	sip.LogLevel = sip.LogDebug
	//sip.LogLevel = sip.LogInfo

	dialogs = NewDialogs()
	earlyDialogs = NewEarlyDialogs()
	//responseCtxs =

	if !loadRoutes() {
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
	sip.ListenAndServe("", nil)
}
