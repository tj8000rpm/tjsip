package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sip/sip"
	"time"
)

func inviteRouted(fwdMsg *sip.Message) (error, int) {
	// Routing
	requestUri := fwdMsg.RequestURI
	if requestUri == nil {
		return sip.ErrStatusError, sip.StatusBadRequest
	}
	if requestUri.Scheme != "sip" && requestUri.Scheme != "tel" {
		return sip.ErrStatusError, sip.StatusUnsupportedURIScheme
	}
	fwdAddr, fwdDomain, found := route(requestUri)

	if !found {
		return sip.ErrStatusError, sip.StatusNotFound
	}

	fwdMsg.RequestURI.Host = fwdDomain
	fwdMsg.RemoteAddr = fwdAddr
	return nil, 0
}

func inviteHandler(srv *sip.Server, msg *sip.Message, txn *sip.ServerTransaction) (error, int, *http.Header) {
	from := msg.From
	// to := msg.To

	ppi := msg.Header.Get("P-Preferred-Identity")

	if ppi != "" {
		from = sip.ParseFrom(ppi)
	}
	_ = from

	info, ok := callStates.Get(msg.CallID.String())
	if ok {
		// This is established call will Reinvite etc
		//return sip.ErrStatusError, sip.StatusLoopDetected
	} else {
		/*
			authorized, _ := authMessage("Proxy-Authorization", msg, queryA1md5)
			if !authorized {
				generateAuthRequireHeader("Proxy-Authenticate")
				return sip.ErrStatusError, sip.StatusProxyAuthenticationRequired, nil
			}
		*/
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
			return sip.ErrStatusError, status, nil
		}
	} else {
		// this message will ini-invite
		fwdMsg, err, status = generateForwardingRequest(msg)
		if err != nil {
			return sip.ErrStatusError, status, nil
		}

		err, status = inviteRouted(fwdMsg)
		if err != nil {
			return err, status, nil
		}

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
		return sip.ErrStatusError, sip.StatusInternalServerError, nil
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
		return nil, 0, nil
	}

	return nil, 0, nil
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
	_, ok := callStates.Get(msg.CallID.String())
	if !ok {
		return sip.ErrStatusError, sip.StatusCallLegTransactionDoesNotExist
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
	result := registration(msg)
	if result == nil {
		return sip.ErrStatusError, sip.StatusInternalServerError
	}

	rep := msg.GenerateResponseFromRequest()
	rep.StatusCode = result.Status
	rep.Contact = result.Contact
	rep.AddToTag()
	if rep.Header == nil {
		rep.Header = result.Header
	} else if result.Header != nil {
		for key, values := range result.Header {
			for _, value := range values {
				result.Header.Add(key, value)
			}
		}
	}

	txn.WriteMessage(rep)
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
	txn *sip.ServerTransaction, status int, header *http.Header) error {

	rep := msg.GenerateResponseFromRequest()
	rep.StatusCode = status
	rep.AddToTag()
	txn.WriteMessage(rep)

	info, ok := callStates.Get(msg.CallID.String())
	if ok {
		info.RecordTerminated(msg, TERM_INTERNAL)
		callStates.Close(info)
	}
	return nil
}

func clientTransactionErrorHandler(txn *sip.ClientTransaction) {
	stKey, exist := responseContexts.GetStFromCt(*(txn.Key))
	if !exist {
		// Nothing to do
		return
	}

	_, destroy := timerCHandler.Get(*txn.Key)
	if destroy != nil {
		close(destroy)
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
			makeErrorResponse(txn.Server, srvTxn.Request, srvTxn, sip.StatusRequestTimeout, nil)
		default:
			makeErrorResponse(txn.Server, srvTxn.Request, srvTxn, sip.StatusInternalServerError, nil)
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
	} else if msg.Method != sip.MethodACK {
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

	err = sip.ErrStatusError
	status := sip.StatusMethodNotAllowed
	var addHeader *http.Header
	switch msg.Method {
	case sip.MethodINVITE:
		srv.Debugf("Handle to INVITE\n")
		err, status, addHeader = inviteHandler(srv, msg, txn)
	case sip.MethodBYE,
		sip.MethodUPDATE,
		sip.MethodPRACK:
		err, status = nonInviteHandler(srv, msg, txn)
	case sip.MethodREGISTER:
		err, status = registerHandler(srv, msg, txn)
	case sip.MethodCANCEL:
		err, status = cancelHandler(srv, msg, txn)
	case sip.MethodOPTIONS:
		err = sip.ErrStatusError
		status = sip.StatusOk
	case sip.MethodACK:
		err, status = ackHandler(srv, msg)
	}

	switch err {
	case nil:
		return nil
	case sip.ErrStatusError:
		return makeErrorResponse(srv, msg, txn, status, addHeader)
	default:
		return makeErrorResponse(srv, msg, txn, sip.StatusInternalServerError, addHeader)
	}
}

func route(requestUri *sip.URI) (fwdAddr, fwdDomain string, found bool) {
	var requestService string

	candidates := make([]*sip.URI, 0)

	binds, err := queryBindingNow(nil, register.DB(), requestUri)
	if err == nil && len(binds) > 0 {
		lastQvalue := 0
		i := 0
		for _, bind := range binds {
			if lastQvalue > bind.Q {
				break
			}
			uri, err := sip.Parse(bind.Bind)
			if bind.Aor == "" || bind.Bind == "" || err != nil {
				log.Printf("Unexpected Status %v", bind)
				log.Printf("But try cotinue")
				continue
			}
			candidates = append(candidates, uri)
			lastQvalue = bind.Q
			i++
		}
		candidates = candidates[:i]
	}

	if len(candidates) > 0 {
		choosen := candidates[rand.Intn(len(candidates))]
		fwdAddr = choosen.Host
		fwdDomain = choosen.Host
		found = true
		return
	}

	switch requestUri.Scheme {
	case "sip":
		requestService = requestUri.User.Username()
	case "tel":
		requestService = requestUri.Host
	}
	found = true
	rt := translater.table.Search(requestService)
	if rt == nil {
		return "", "", false
	}
	fwdAddr = rt.fwd.Addr
	fwdDomain = rt.fwd.Domain
	return
}
