package main

import (
	"log"
	"sip/sip"
	"strings"
	"sync"
	"time"
)

const (
	TrunkSubscriber = iota
	TrunkServer
	TrunkUntrustedServer
)

var (
	TimerC = 180 * time.Second
	//TimerC = 5 * time.Second
)

type TimerCHandlers struct {
	mu      sync.Mutex
	update  map[sip.ClientTransactionKey](chan bool)
	destroy map[sip.ClientTransactionKey](chan bool)
}

func (t *TimerCHandlers) Remove(txn sip.ClientTransactionKey) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	var okU, okD bool
	var closeChan chan bool
	if closeChan, okU = t.update[txn]; okU && closeChan != nil {
		delete(t.update, txn)
	}
	if closeChan, okD = t.destroy[txn]; okD && closeChan != nil {
		delete(t.destroy, txn)
	}
	return okU || okD
}

func (t *TimerCHandlers) Get(txn sip.ClientTransactionKey) (chanU, chanD chan bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	var ok bool
	if chanU, ok = t.update[txn]; !ok {
		return nil, nil
	}
	if chanD, ok = t.destroy[txn]; !ok {
		return nil, nil
	}
	return
}

func (t *TimerCHandlers) Add(txn sip.ClientTransactionKey) (chan bool, chan bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.update[txn] = make(chan bool)
	if t.update[txn] == nil {
		return nil, nil
	}
	t.destroy[txn] = make(chan bool)
	if t.destroy[txn] == nil {
		delete(t.update, txn)
		return nil, nil
	}
	return t.update[txn], t.destroy[txn]
}

func NewTimerCHandlers() *TimerCHandlers {
	t := &TimerCHandlers{
		update:  make(map[sip.ClientTransactionKey](chan bool)),
		destroy: make(map[sip.ClientTransactionKey](chan bool)),
	}
	return t
}

type ResponseCtx = map[sip.ClientTransactionKey]bool
type ResponseCtxs struct {
	mu     sync.Mutex
	stToCt map[sip.ServerTransactionKey]ResponseCtx
	ctToSt map[sip.ClientTransactionKey]sip.ServerTransactionKey
}

func (ctxs *ResponseCtxs) GetStFromCt(ct sip.ClientTransactionKey) (sip.ServerTransactionKey, bool) {
	ctxs.mu.Lock()
	defer ctxs.mu.Unlock()
	st, ok := ctxs.ctToSt[ct]
	return st, ok
}

func (ctxs *ResponseCtxs) GetCtFromSt(st sip.ServerTransactionKey) (*ResponseCtx, bool) {
	ctxs.mu.Lock()
	defer ctxs.mu.Unlock()
	cts, ok := ctxs.stToCt[st]
	return &cts, ok
}

func (ctxs *ResponseCtxs) Add(st sip.ServerTransactionKey, ct sip.ClientTransactionKey) bool {
	ctxs.mu.Lock()
	defer ctxs.mu.Unlock()
	_, ok := ctxs.stToCt[st]
	if !ok {
		ctxs.stToCt[st] = make(ResponseCtx)
	}
	if ctxs.stToCt[st] == nil {
		return false
	}
	ctxs.stToCt[st][ct] = true
	ctxs.ctToSt[ct] = st
	return true
}

func (ctxs *ResponseCtxs) Remove(ct sip.ClientTransactionKey) (complete, found,
	removeServerTransaction bool, st sip.ServerTransactionKey) {

	ctxs.mu.Lock()
	defer ctxs.mu.Unlock()
	st, found = ctxs.ctToSt[ct]
	if !found {
		return
	}

	// Delete ct from ct to st map
	delete(ctxs.ctToSt, ct)
	complete = true

	delete(ctxs.stToCt[st], ct)

	// if st has no children, delete st from st to ct map

	removeServerTransaction = len(ctxs.stToCt[st]) == 0
	if removeServerTransaction {
		delete(ctxs.stToCt, st)
	}
	return
}

func NewResponseCtxs() *ResponseCtxs {
	ctx := new(ResponseCtxs)
	if ctx == nil {
		return nil
	}
	ctx.stToCt = make(map[sip.ServerTransactionKey]ResponseCtx)
	ctx.ctToSt = make(map[sip.ClientTransactionKey]sip.ServerTransactionKey)
	if ctx.stToCt == nil || ctx.ctToSt == nil {
		return nil
	}
	return ctx
}

func lookupTrunkType(addr string) int {
	addrPort := strings.SplitN(addr, ":", 2)
	if addrPort[1] == "" {
		addrPort[1] = "5060"
	}
	return TrunkSubscriber
}

func responseContextCloser(st *sip.ServerTransaction, ctKey *sip.ClientTransactionKey, status int) {
	// - Unbind responseContexts
	_, _, deleteSt, _ := responseContexts.Remove(*ctKey)
	if !deleteSt {
		// Nothing to do
		return
	}
	errRes := st.Request.GenerateResponseFromRequest()
	errRes.StatusCode = status
	provRes := st.ProvisionalRes
	if provRes != nil {
		errRes.To = provRes.To
	}
	errRes.AddToTag()
	st.WriteMessage(errRes)
}

func fireTimerC(st *sip.ServerTransaction, ct *sip.ClientTransaction) {
	log.Printf("Timer C was fired")

	recivedProvisonalResponse := ct.ProvisionalRes != nil
	if recivedProvisonalResponse {
		// - If no response from ct, ct will close after timer F
		cancelErrorHandler := func(t *sip.ClientTransaction) {
			switch t.Err {
			case sip.ErrTransactionTimedOut:
				// Send Error to UAC
				responseContextCloser(st, ct.Key, sip.StatusRequestTimeout)
				break
			case nil:
				break
			default:
				responseContextCloser(st, ct.Key, sip.StatusRequestTerminated)
			}
		}

		// If the client transaction has received a provisional response,
		// the proxy MUST generate a CANCEL request matching that transaction.
		canMsg, err := sip.GenerateCancelRequest(ct.Request)
		if err != nil {
			// Error
			ct.Destroy()
			responseContextCloser(st, ct.Key, sip.StatusRequestTimeout)
			return
		}
		srv := ct.Server
		canCt := sip.NewClientNonInviteTransaction(srv, canMsg, cancelErrorHandler)
		err = srv.AddClientTransaction(canCt)
		if err != nil {
			srv.Warnf("%v", err)
			canCt.Destroy()
			return
		}
		canCt.WriteMessage(canMsg)
	} else {
		// If the client transaction has not received a provisional response,
		// the proxy MUST behave as if the transaction received
		// a 408 (Request Timeout) response.

		// Send Error to UAC
		responseContextCloser(st, ct.Key, sip.StatusRequestTimeout)
	}

}

func proxyCoreHandler(layer int, srv *sip.Server, msg *sip.Message) error {
	if layer != sip.LayerCore && layer != sip.LayerTransaction {
		return nil
	}
	if msg.Request {
		return requestHandler(srv, msg)
	} else if msg.Response {
		return responseHandler(srv, msg)
	}
	return nil
}
