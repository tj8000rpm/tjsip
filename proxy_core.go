package main

import (
	"fmt"
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
	if len(ctxs.stToCt[st]) == 0 {
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

func route(request string) (fwdAddr, fwdDomain string, found bool) {
	found = true
	rt := translater.table.Search(request)
	if rt == nil {
		return "", "", false
	}
	fwdAddr = rt.fwd.Addr
	fwdDomain = rt.fwd.Domain
	return
}

func responseHandler(srv *sip.Server, msg *sip.Message) error {
	if msg.CSeq == nil {
		return sip.ErrMalformedMessage
	}

	callCheck := true
	switch msg.CSeq.Method {
	case sip.MethodOPTIONS,
		sip.MethodREGISTER:
		callCheck = false
	}

	info, callInstate := callStates.Get(msg.CallID.String())
	if callCheck && !callInstate {
		// Ignore response
		return nil
	}

	srv.Debugf("Lookup transaction")
	cltTxnKey_p, err := sip.GenerateClientTransactionKey(msg)
	if err != nil {
		srv.Warnf("Fail Generate Cleint Transcation Key")
		return nil
	}
	cltTxnKey := *cltTxnKey_p

	srvTxnKey, ok := responseContexts.GetStFromCt(cltTxnKey)
	if !ok || !callInstate {
		return nil
	}
	srvTxn := srv.LookupServerTransaction(&srvTxnKey)
	if srvTxn == nil || !callInstate {
		srv.Warnf("Server Transaction still nil")
		return nil
	}
	update, destroy := timerCHandler.Get(cltTxnKey)

	if msg.StatusCode == sip.StatusTrying {
		srv.Debugf("100 Trying no need forwaed")
		return nil
	} else if msg.StatusCode > 100 && msg.StatusCode < 200 {
		if update != nil {
			update <- true
		}
	} else if msg.StatusCode >= 200 {
		if destroy != nil {
			close(destroy)
			if update != nil {
				close(update)
			}
		}
		responseContexts.Remove(cltTxnKey)

		if msg.StatusCode < 300 {
			// TODO: this call was completed
			// Write CDR, recoard session, etc.
			switch msg.CSeq.Method {
			case sip.MethodINVITE:
				if !callInstate {
					info.RecordEstablishedTime()
				}
				break
			case sip.MethodBYE:
				if callInstate {
					info.RecordTerminatedTime()
					callStates.Close(info)
				}
				break
			}
		} else if msg.StatusCode >= 300 && msg.StatusCode < 400 {
			// TODO: redirect
			srv.Debugf("call will be redirected")
			return nil
		} else {
			// TODO: this call was not completed
			// Call forward? or through response to caller? etc.
			switch msg.CSeq.Method {
			case sip.MethodINVITE:
				if callInstate {
					info.RecordTerminatedTime()
					callStates.Close(info)
				}
				break
			case sip.MethodBYE:
				if !callInstate {
					info.RecordTerminatedTime()
					callStates.Close(info)
				}
				break
			}
		}
	}

	cpMsg := msg.Clone()
	if cpMsg == nil {
		srv.Warnf("Message could not copied")
		return nil
	}
	srv.Debugf("Message was copied")
	cpMsg.Via.Pop()
	topMostVia := cpMsg.Via.TopMost()
	if topMostVia == nil {
		// Message not forwarded
		return nil
	}
	if received := topMostVia.Parameter().Get("received"); received != "" {
		cpMsg.RemoteAddr = received
	} else {
		cpMsg.RemoteAddr = topMostVia.SentBy
	}
	if srvTxn != nil && callInstate {
		srvTxn.WriteMessage(cpMsg)
	} else {
		srv.Debugf("Sent Message without transaction, [%s]", cpMsg.StatusCode)
		srv.WriteMessage(cpMsg)
	}
	return nil
}

func inviteHandler(srv *sip.Server, msg *sip.Message, txn *sip.ServerTransaction) (error, int) {
	from := msg.From
	// to := msg.To

	ppi := msg.Header.Get("P-Preferred-Identity")

	if ppi != "" {
		from = sip.ParseFrom(ppi)
	}
	_ = from

	info, ok := callStates.Get(msg.CallID.String())
	if ok {
		return sip.ErrStatusError, sip.StatusLoopDetected
	}
	info = NewCallInfo()
	info.RecordCaller(msg)
	callStates.Add(info)

	var fwdMsg *sip.Message
	var err error
	var status int
	if len(msg.Header.Values("Route")) != 0 {
		// this message will re-invite
		fwdMsg, err, status = generateForwardingRequestByRouteHeader(msg)
		if err != nil {
			return sip.ErrStatusError, status
		}
	} else {
		// this message will ini-invite
		fwdMsg, err, status = generateForwardingRequest(msg)
		if err != nil {
			return sip.ErrStatusError, status
		}

		// Routing
		requestUri := msg.RequestURI
		if requestUri.Scheme != "sip" && requestUri.Scheme != "tel" {
			return sip.ErrStatusError, sip.StatusUnsupportedURIScheme
		}
		var requestService string

		switch requestUri.Scheme {
		case "sip":
			requestService = requestUri.User.Username()
		case "tel":
			requestService = requestUri.Host
		}
		fwdAddr, fwdDomain, found := route(requestService)

		if !found {
			return sip.ErrStatusError, sip.StatusNotFound
		}

		fwdMsg.RequestURI.Host = fwdDomain
		fwdMsg.RemoteAddr = fwdAddr

		// Insert Record route header
		recordRoutes := fwdMsg.Header.Values("Record-Route")
		newRR := fmt.Sprintf("<sip:%s;lr>", srv.Address())
		fwdMsg.Header.Set("Record-Route", newRR)
		for _, rr := range recordRoutes {
			fwdMsg.Header.Add("Record-Route", rr)
		}
	}

	clientTxn := sip.NewClientInviteTransaction(srv, fwdMsg, clientTransactionErrorHandler)

	// Add a new response context
	responseContexts.Add(*(txn.Key), *(clientTxn.Key))

	err = srv.AddClientTransaction(clientTxn)
	if err != nil {
		srv.Warnf("%v", err)
		clientTxn.Destroy()
		return sip.ErrStatusError, sip.StatusInternalServerError
	}
	clientTxn.WriteMessage(fwdMsg)
	info.RecordCallee(fwdMsg)

	update, destroy := timerCHandler.Add(*(clientTxn.Key))
	go func() {
		for {
			select {
			case <-time.After(TimerC):
				fireTimerC(txn, clientTxn)
				timerCHandler.Remove(*(clientTxn.Key))
				return
			case <-update:
				break
			case <-destroy:
				timerCHandler.Remove(*(clientTxn.Key))
				return
			}
		}
	}()

	// TODO: decide to handling as subsciber or other server
	switch lookupTrunkType(msg.RemoteAddr) {
	case TrunkSubscriber:
		return nil, 0
	}

	return nil, 0
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

func generateForwardingRequest(msg *sip.Message) (*sip.Message, error, int) {
	fwdMsg := msg.Clone()
	if !fwdMsg.MaxForwards.Decrement() {
		return nil, sip.ErrStatusError, sip.StatusTooManyHops
	}
	topmost := fwdMsg.Via.TopMost()
	if topmost.SentBy != msg.RemoteAddr {
		param := topmost.Parameter()
		param.Set("received", msg.RemoteAddr)
		newParam := ""
		for key, values := range param {
			for _, value := range values {
				newParam += fmt.Sprintf(";%s=%s", key, value)
			}
		}
		topmost.RawParameter = newParam[1:]
	}
	return fwdMsg, nil, 0
}

func generateForwardingRequestByRouteHeader(msg *sip.Message) (*sip.Message, error, int) {
	nextIsLR := false

	fwdMsg, err, status := generateForwardingRequest(msg)
	if err != nil {
		return nil, err, status
	}
	next := fwdMsg.RequestURI.Host
	routes := sip.NewNameAddrFormatHeaders()
	for _, route := range fwdMsg.Header.Values("Route") {
		sip.ParseNameAddrFormats(route, routes)
	}
	var headOfRouteURI *sip.URI
	if routes.Length() > 1 {
		headOfRouteURI := routes.Header[1].Addr.Uri
		if headOfRouteURI == nil {
			return nil, sip.ErrStatusError, sip.StatusInternalServerError
		}
		next = headOfRouteURI.Host
		// check a `lr` flag in top of route header
		_, nextIsLR = headOfRouteURI.Parameter()["lr"]
	}

	fwdMsg.RemoteAddr = resolveDomain(next)

	routeOffset := 2
	if nextIsLR {
		routeOffset = 1
	}

	fwdMsg.Header.Del("Route")
	for i := routeOffset; i < routes.Length(); i++ {
		fwdMsg.Header.Add("Route", routes.Header[i].String())
	}
	if headOfRouteURI != nil && !nextIsLR {
		// In case of strict routing
		fwdMsg.Header.Add("Route", fwdMsg.RequestURI.String())
		fwdMsg.RequestURI = headOfRouteURI
	}

	return fwdMsg, nil, 0
}

func ackHandler(srv *sip.Server, msg *sip.Message) (error, int) {
	srv.Debugf("Dialog was established\n")
	fwdMsg, err, status := generateForwardingRequestByRouteHeader(msg)
	if err != nil {
		return err, status
	}
	srv.WriteMessage(fwdMsg)
	return nil, 0
}

func nonInviteHandler(srv *sip.Server, msg *sip.Message, txn *sip.ServerTransaction) (error, int) {
	info, ok := callStates.Get(msg.CallID.String())
	if !ok {
		return sip.ErrStatusError, sip.StatusCallLegTransactionDoesNotExist
	}
	_ = info
	switch msg.Method {
	case sip.MethodBYE:
		srv.Debugf("Handle to BYE\n")
		break
	}
	fwdMsg, err, status := generateForwardingRequestByRouteHeader(msg)
	if err != nil {
		return err, status
	}

	clientTxn := sip.NewClientNonInviteTransaction(srv, fwdMsg, clientTransactionErrorHandler)
	responseContexts.Add(*(txn.Key), *(clientTxn.Key))

	err = srv.AddClientTransaction(clientTxn)
	if err != nil {
		srv.Warnf("%v", err)
		clientTxn.Destroy()
		return sip.ErrStatusError, sip.StatusInternalServerError
	}
	clientTxn.WriteMessage(fwdMsg)
	return nil, 0
}

func registerHandler(srv *sip.Server, msg *sip.Message, txn *sip.ServerTransaction) (error, int) {
	return nil, 0
}

func cancelHandler(srv *sip.Server, msg *sip.Message, txn *sip.ServerTransaction) (error, int) {
	lookupTxnKey, err := sip.GenerateServerTransactionKey(msg)
	if err != nil {
		return sip.ErrStatusError, sip.StatusInternalServerError
	}
	lookupTxnKey.UpdateMethod(sip.MethodINVITE)
	ctKeys, ok := responseContexts.GetCtFromSt(*lookupTxnKey)

	if !ok {
		// No matching INVITE Request
		return sip.ErrStatusError, sip.StatusCallLegTransactionDoesNotExist
	}

	rep := msg.GenerateResponseFromRequest()
	rep.StatusCode = sip.StatusOk
	txn.WriteMessage(rep)

	for ctKey, _ := range *ctKeys {
		origCt := srv.LookupClientTransaction(&ctKey).(*sip.ClientTransaction)
		origSt := srv.LookupServerTransaction(lookupTxnKey).(*sip.ServerTransaction)
		canMsg, err := sip.GenerateCancelRequest(origCt.Request)

		// - If no response from ct, ct will close after timer F
		cancelErrorHandler := func(t *sip.ClientTransaction) {
			switch t.Err {
			case sip.ErrTransactionTimedOut:
				// Send Error to UAC
				responseContextCloser(origSt, &ctKey, sip.StatusRequestTimeout)
				break
			case nil:
				break
			default:
				responseContextCloser(origSt, &ctKey, sip.StatusRequestTerminated)
			}
		}

		canCt := sip.NewClientNonInviteTransaction(srv, canMsg, cancelErrorHandler)
		err = srv.AddClientTransaction(canCt)
		if err != nil {
			srv.Warnf("%v", err)
			canCt.Destroy()
			responseContextCloser(origSt, &ctKey, sip.StatusRequestTerminated)
			return nil, 0
		}
		canCt.WriteMessage(canMsg)
	}

	return nil, 0
}

func makeErrorResponse(srv *sip.Server, msg *sip.Message,
	txn *sip.ServerTransaction, status int) error {

	rep := msg.GenerateResponseFromRequest()
	rep.StatusCode = status
	rep.AddToTag()
	txn.WriteMessage(rep)

	info, ok := callStates.Get(msg.CallID.String())
	if ok {
		info.RecordTerminatedTime()
		callStates.Close(info)
	}
	return nil
}

func clientTransactionErrorHandler(txn *sip.ClientTransaction) {
	var stKey sip.ServerTransactionKey
	var exist bool
	stKey, exist = responseContexts.GetStFromCt(*(txn.Key))
	if !exist {
		// Nothing to do
		return
	}
	srvTxn := txn.Server.LookupServerTransaction(&stKey).(*sip.ServerTransaction)
	if srvTxn == nil {
		// Nothing to do
		return
	}
	_, _, removeSt, _ := responseContexts.Remove(*txn.Key)

	if removeSt {
		switch txn.Err {
		case sip.ErrTransactionTimedOut:
			makeErrorResponse(txn.Server, srvTxn.Request, srvTxn, sip.StatusRequestTimeout)
		default:
			makeErrorResponse(txn.Server, srvTxn.Request, srvTxn, sip.StatusInternalServerError)
		}
	}
}

func requestHandler(srv *sip.Server, msg *sip.Message) error {
	txnKey, err := sip.GenerateServerTransactionKey(msg)
	if err != nil {
		return err
	}

	var txn *sip.ServerTransaction
	if msg.Method == sip.MethodINVITE {
		// Create New INVITE srever transaction
		txn = sip.NewServerInviteTransaction(srv, txnKey, msg)
	} else {
		txn = sip.NewServerNonInviteTransaction(srv, txnKey, msg)
	}
	if txn != nil {
		err = srv.AddServerTransaction(txn)
		if err != nil {
			srv.Warnf("%v", err)
			txn.Destroy()
			return err
		}
	}

	var status int
	switch msg.Method {
	case sip.MethodINVITE:
		srv.Debugf("Handle to INVITE\n")
		err, status = inviteHandler(srv, msg, txn)
		break
	case sip.MethodBYE,
		sip.MethodUPDATE,
		sip.MethodPRACK:
		err, status = nonInviteHandler(srv, msg, txn)
		break
	case sip.MethodREGISTER:
		err, status = registerHandler(srv, msg, txn)
		break
	case sip.MethodCANCEL:
		err, status = cancelHandler(srv, msg, txn)
		break
	case sip.MethodOPTIONS:
		err = sip.ErrStatusError
		status = sip.StatusOk
		break
	case sip.MethodACK:
		err, status = ackHandler(srv, msg)
		break
	default:
		err = sip.ErrStatusError
		status = sip.StatusMethodNotAllowed
		break
	}

	switch err {
	case nil:
		return nil
	case sip.ErrStatusError:
		return makeErrorResponse(srv, msg, txn, status)
	default:
		return makeErrorResponse(srv, msg, txn, sip.StatusInternalServerError)
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
