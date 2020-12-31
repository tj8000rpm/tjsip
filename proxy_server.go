package main

import (
	//"github.com/tj8000rpm/tjsip/sip"
	"fmt"
	"log"
	"os"
	"runtime"
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

type callInfo struct {
	mu                sync.Mutex
	setupTime         time.Time
	establishedTime   time.Time
	terminatedTime    time.Time
	callerAddress     string
	calleeAddress     string
	callId            string
	from              *sip.From
	to                *sip.To
	recivedRequestURI string
	sentRequestURI    string
	closed            bool
}

func (c *callInfo) String() string {
	complete := 0
	duration := time.Second * 0
	if !(c.establishedTime.IsZero() || c.terminatedTime.IsZero()) {
		complete, duration = 1, c.terminatedTime.Sub(c.establishedTime)
	}
	eTimeS := c.establishedTime.String()
	if c.establishedTime.IsZero() {
		eTimeS = ""
	}
	return fmt.Sprintf("%d,\"%s\",\"%s\",\"%s\",%d,\"%s\",\"%s\",\"%s\",\"%s\",\"%s\",\"%s\",\"%s\"",
		complete,
		c.setupTime,
		eTimeS,
		c.terminatedTime,
		duration*time.Second,
		c.callerAddress,
		c.calleeAddress,
		c.callId,
		c.from.String(),
		c.to.String(),
		c.recivedRequestURI,
		c.sentRequestURI,
	)
}

func (c *callInfo) RecordCaller(msg *sip.Message) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.recivedRequestURI = msg.RequestURI.String()
	c.from = msg.From
	c.to = msg.To
	c.setupTime = time.Now()
	c.callerAddress = msg.RemoteAddr
	c.callId = msg.CallID.String()
}

func (c *callInfo) RecordCallee(msg *sip.Message) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.sentRequestURI = msg.RequestURI.String()
	c.calleeAddress = msg.RemoteAddr
}

func (c *callInfo) RecordEstablishedTime() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.establishedTime = time.Now()
}

func (c *callInfo) RecordTerminatedTime() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.terminatedTime = time.Now()
}

func (c *callInfo) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
}

func (c *callInfo) Closed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

func (c *callInfo) Id() string {
	return c.callId
}

func NewCallInfo() (c *callInfo) {
	c = new(callInfo)
	return c
}

type CallStates struct {
	mu    sync.Mutex
	calls map[string]*callInfo
}

func (c *CallStates) Add(info *callInfo) bool {
	if info == nil {
		return false
	}
	id := info.Id()
	if id == "" {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	c.calls[id] = info
	return true
}

func (c *CallStates) Remove(info *callInfo) bool {
	if info == nil {
		return false
	}
	id := info.Id()
	if id == "" {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.calls[id]; !ok {
		return false
	}

	delete(c.calls, id)
	return true
}

func (c *CallStates) Close(info *callInfo) bool {
	if info == nil {
		return false
	}
	info.Close()
	// Write CDR
	log.Printf("%s\n", info.String())
	go func() {
		time.Sleep(sip.TimerA * 32)
		c.Remove(info)
	}()
	return true
}

func (c *CallStates) Get(id string) (info *callInfo, ok bool) {
	if id == "" {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	info, ok = c.calls[id]
	return
}

func (c *CallStates) Length() int {
	return len(c.calls)
}

func NewCallStates() (c *CallStates) {
	c = &CallStates{
		calls: make(map[string]*callInfo),
	}
	return c
}

var callGap = callGapControl{enable: false, last: time.Now()}

var responseContexts *ResponseCtxs
var timerCHandler *TimerCHandlers
var callStates *CallStates

func main() {
	listenAddr, ok := os.LookupEnv("LISTEN")
	if !ok {
		listenAddr = ""
	}
	logLevel, ok := os.LookupEnv("LOGLEVEL")
	if !ok {
		logLevel = "INFO"
	}
	filepath, ok := os.LookupEnv("TRANSFILE")
	if !ok {
		filepath = "routes.csv"
	}

	sip.RecieveBufSizeB = 9000
	log.SetOutput(os.Stdout)
	switch logLevel {
	case "INFO":
		sip.LogLevel = sip.LogInfo
		break
	case "DEBUG":
		sip.LogLevel = sip.LogDebug
		break
	default:
		sip.LogLevel = sip.LogInfo
	}

	responseContexts = NewResponseCtxs()
	timerCHandler = NewTimerCHandlers()
	callStates = NewCallStates()

	if !loadTranslater(filepath, sip.LogLevel >= sip.LogDebug) {
		return
	}

	go func() {
		for {
			time.Sleep(time.Second * 5)
			stat.mu.Lock()
			log.Printf("Current goroutines: %d\n", runtime.NumGoroutine())
			log.Printf("Call completed: %v\n", stat.completed)
			log.Printf("Current Call: %v\n", callStates.Length())

			log.Printf("Response Context Size st: %d / ct: %v\n", len(responseContexts.stToCt), len(responseContexts.ctToSt))

			// if runtime.NumGoroutine() > 2 {
			// 	pprof.Lookup("goroutine").WriteTo(os.Stdout, 2)
			// }
			if len(responseContexts.ctToSt) <= 10 {
				for key, _ := range responseContexts.stToCt {
					log.Printf("--- %v\n", key)
				}
			}
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
	sip.ListenAndServe(listenAddr, nil)
}
