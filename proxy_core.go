package main

import (
	"fmt"
	"sip/sip"
	"strings"
	"sync"
)

const (
	TrunkSubscriber = iota
	TrunkServer
	TrunkUntrustedServer
)

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

func (ctxs *ResponseCtxs) Remove(ct sip.ClientTransactionKey) (complete, found bool) {
	ctxs.mu.Lock()
	defer ctxs.mu.Unlock()
	st, ok := ctxs.ctToSt[ct]
	if !ok {
		return false, false
	}

	// Delete ct from ct to st map
	delete(ctxs.ctToSt, ct)

	// Delete ct from st to ct map
	_, ok = ctxs.stToCt[st][ct]
	if !ok {
		// abnormal case but still continue
		return true, false
	}
	delete(ctxs.stToCt[st], ct)

	// if st has no children, delete st from st to ct map
	if len(ctxs.stToCt[st]) == 0 {
		delete(ctxs.stToCt, st)
	}
	return true, true
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
	rt := routes.table.Search(request)
	if rt == nil {
		return "", "", false
	}
	fwdAddr = rt.fwd.Addr
	fwdDomain = rt.fwd.Domain
	return
}

func responseHandler(srv *sip.Server, msg *sip.Message) error {

	srv.Debugf("Lookup transaction")
	cltTxnKey_p, err := sip.GenerateClientTransactionKey(msg)
	if err != nil {
		srv.Warnf("Fail Generate Cleint Transcation Key")
		return nil
	}
	cltTxnKey := *cltTxnKey_p

	srvTxnKey, ok := responseContexts.GetStFromCt(cltTxnKey)
	if !ok {
		return nil
	}
	srvTxn := srv.LookupServerTransaction(&srvTxnKey)
	if srvTxn == nil {
		srv.Warnf("Server Transaction still nil")
		return nil
	}

	if msg.StatusCode == sip.StatusTrying {
		srv.Debugf("100 Trying no need forwaed")
		return nil
	} else if msg.StatusCode >= 300 && msg.StatusCode < 400 {
		// TODO: redirect
		srv.Debugf("call will be redirected")
		responseContexts.Remove(cltTxnKey)
		return nil
	} else if msg.StatusCode >= 200 {
		responseContexts.Remove(cltTxnKey)
		if msg.StatusCode < 300 {
			// TODO: this call was completed
			// Write CDR, recoard session, etc.
		} else {
			// TODO: this call was not completed
			// Call forward? or through response to caller? etc.
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
	cpMsg.RemoteAddr = topMostVia.SentBy
	srvTxn.WriteMessage(cpMsg)

	return nil
}

func inviteHandler(srv *sip.Server, msg *sip.Message, txn *sip.ServerTransaction) error {
	from := msg.From
	// to := msg.To

	ppi := msg.Header.Get("P-Preferred-Identity")

	if ppi != "" {
		from = sip.ParseFrom(ppi)
	}
	_ = from

	fwdMsg := msg.Clone()
	topmost := fwdMsg.Via.TopMost()
	//if topmost.SentBy != msg.RemoteAddr {
	if true {
		topmost.RawParam = fmt.Sprintf("recieved=%s;", msg.RemoteAddr) + topmost.RawParam
	}
	routes := fwdMsg.Header.Values("Route")
	if len(routes) != 0 {
		// this message will ini-invite
		var next string
		next = fwdMsg.RequestURI.Host
		if len(routes) > 1 {
			uri, err := sip.Parse(routes[1])
			if err != nil {
				return nil
			}
			next = uri.Host
		}
		fwdMsg.RemoteAddr = resolveDomain(next)
		fwdMsg.Header.Del("Route")
		for _, route := range routes[1:] {
			fwdMsg.Header.Add("Route", route)
		}
	} else {
		// Routing
		requestUri := msg.RequestURI
		if requestUri.Scheme != "sip" && requestUri.Scheme != "tel" {
			// TODO: Unexpected RequestURI
			// sip.StatusUnsupportedURIScheme
			fmt.Printf("Unexpected RequestURI Scheme\n")
			return nil
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
			// TODO: Return not found
		}

		// fmt.Printf("Forward to request : %s@%s / %s\n", requestService, fwdDomain, fwdAddr)
		fwdMsg.RequestURI.Host = fwdDomain
		fwdMsg.RemoteAddr = fwdAddr

		// Insert Record route header
		recordRoutes := fwdMsg.Header.Values("Record-Route")
		newRR := fmt.Sprintf("<sip:%s;lr>", srv.Addr)
		fwdMsg.Header.Set("Record-Route", newRR)
		for _, rr := range recordRoutes {
			fwdMsg.Header.Add("Record-Route", rr)
		}
	}

	fwdMsg.MaxForwards.Decrement()
	clientTxn := sip.NewClientInviteTransaction(srv, fwdMsg, 10)

	// Add a new response context
	responseContexts.Add(*(txn.Key), *(clientTxn.Key))

	err := srv.AddClientTransaction(clientTxn)
	if err != nil {
		srv.Warnf("%v", err)
		clientTxn.Destroy()
		// TODO: Return sip.StatusInternalServerError
		return nil
	}
	//log.Printf("Sent INVITE\n")
	clientTxn.WriteMessage(fwdMsg)

	// TODO: decide to handling as subsciber or other server
	switch lookupTrunkType(msg.RemoteAddr) {
	case TrunkSubscriber:
		return nil
	}

	return nil
}

func ackHandler(srv *sip.Server, msg *sip.Message) error {
	srv.Debugf("Dialog was established\n")

	// from := msg.From
	fwdMsg := msg.Clone()
	fwdMsg.MaxForwards.Decrement()

	routes := fwdMsg.Header.Values("Route")
	if len(routes) == 0 {
		return nil
	}
	var next string
	next = fwdMsg.RequestURI.Host
	if len(routes) > 1 {
		uri, err := sip.Parse(routes[1])
		if err != nil {
			return nil
		}
		next = uri.Host
	}

	fwdMsg.RemoteAddr = resolveDomain(next)
	fwdMsg.Header.Del("Route")
	for _, route := range routes[1:] {
		fwdMsg.Header.Add("Route", route)
	}
	srv.WriteMessage(fwdMsg)

	return nil
}

func prackHandler(srv *sip.Server, msg *sip.Message, txn *sip.ServerTransaction) error {
	rep := msg.GenerateResponseFromRequest()
	rep.StatusCode = 200
	txn.WriteMessage(rep)
	return nil
}

func registerHandler(srv *sip.Server, msg *sip.Message, txn *sip.ServerTransaction) error {
	return nil
}

func byeHandler(srv *sip.Server, msg *sip.Message, txn *sip.ServerTransaction) error {
	srv.Debugf("Dialog was established\n")
	var fromTag, toTag, callID string
	if msg.From == nil || msg.CallID == nil || msg.To == nil {
		// TODO:return sip.StatusBadRequest
		return nil
	}

	from := msg.From
	fromTag = from.Parameter().Get("tag")
	to := msg.To
	toTag = to.Parameter().Get("tag")
	callID = msg.CallID.String()

	if fromTag == "" || callID == "" || toTag == "" {
		// TODO:return sip.StatusBadRequest
		return nil
	}

	fwdMsg := msg.Clone()
	fwdMsg.MaxForwards.Decrement()

	routes := fwdMsg.Header.Values("Route")
	if len(routes) == 0 {
		return nil
	}
	var next string
	next = fwdMsg.RequestURI.Host
	if len(routes) > 1 {
		uri, err := sip.Parse(routes[1])
		if err != nil {
			return nil
		}
		next = uri.Host
	}

	fwdMsg.RemoteAddr = next
	fwdMsg.Header.Del("Route")
	for _, route := range routes[1:] {
		fwdMsg.Header.Add("Route", route)
	}

	clientTxn := sip.NewClientNonInviteTransaction(srv, fwdMsg, 10)
	responseContexts.Add(*(txn.Key), *(clientTxn.Key))

	err := srv.AddClientTransaction(clientTxn)
	if err != nil {
		srv.Warnf("%v", err)
		clientTxn.Destroy()
		// TODO: Return sip.StatusInternalServerError
		return nil
	}
	clientTxn.WriteMessage(fwdMsg)

	return nil
}

func cancelHandler(srv *sip.Server, msg *sip.Message, txn *sip.ServerTransaction) error {
	rep := msg.GenerateResponseFromRequest()
	rep.StatusCode = 200
	txn.WriteMessage(rep)
	params := msg.Via.TopMost().Parameter()
	params.Get("branch")
	return nil
}

func optionsHandler(srv *sip.Server, msg *sip.Message, txn *sip.ServerTransaction) error {
	rep := msg.GenerateResponseFromRequest()
	rep.StatusCode = 200
	txn.WriteMessage(rep)
	return nil
}

func updateHandler(srv *sip.Server, msg *sip.Message, txn *sip.ServerTransaction) error {
	return nil
}

func methodNotAllowdHandler(srv *sip.Server, msg *sip.Message, txn *sip.ServerTransaction) error {
	return nil
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

	switch msg.Method {
	case sip.MethodINVITE:
		srv.Debugf("Handle to INVITE\n")
		return inviteHandler(srv, msg, txn)
	case sip.MethodBYE:
		srv.Debugf("Handle to BYE\n")
		return byeHandler(srv, msg, txn)
	case sip.MethodPRACK:
		return prackHandler(srv, msg, txn)
	case sip.MethodREGISTER:
		return registerHandler(srv, msg, txn)
	case sip.MethodCANCEL:
		return cancelHandler(srv, msg, txn)
	case sip.MethodOPTIONS:
		return optionsHandler(srv, msg, txn)
	case sip.MethodUPDATE:
		return updateHandler(srv, msg, txn)
	case sip.MethodACK:
		return ackHandler(srv, msg)
	default:
		return methodNotAllowdHandler(srv, msg, txn)
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
