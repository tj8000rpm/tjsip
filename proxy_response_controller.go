package main

import (
	"sip/sip"
	"time"
)

func forwardRedirection(srv *sip.Server, msg *sip.Message, info *callInfo, srvTxn *sip.ServerTransaction) error {
	srv.Debugf("call will be redirected")
	contacts := msg.Contact
	if contacts == nil || contacts.Length() == 0 {
		info.RecordTerminated(msg, TERM_INTERNAL)
		return makeErrorResponse(srv, srvTxn.Request,
			srvTxn, sip.StatusNotFound, nil)
	}
	contact := contacts.Header[0]
	if contact.Star || contact.Addr == nil || contact.Addr.Uri == nil {
		info.RecordTerminated(msg, TERM_INTERNAL)
		return makeErrorResponse(srv, srvTxn.Request,
			srvTxn, sip.StatusNotFound, nil)
	}

	fwdMsg := info.SentRequest().Clone()
	if !fwdMsg.MaxForwards.Decrement() {
		return makeErrorResponse(srv, srvTxn.Request, srvTxn, sip.StatusTooManyHops, nil)
	}
	fwdMsg.RequestURI = contact.Addr.Uri
	fwdMsg.Via.Pop()
	err, status := inviteRouted(fwdMsg)
	if err != nil {
		return makeErrorResponse(srv, srvTxn.Request, srvTxn, status, nil)
	}

	newCT := sip.NewClientInviteTransaction(srv, fwdMsg, clientTransactionErrorHandler)
	responseContexts.Add(*(srvTxn.Key), *(newCT.Key))

	err = srv.AddClientTransaction(newCT)
	if err != nil {
		srv.Warnf("%v", err)
		newCT.Destroy()
		return makeErrorResponse(srv, srvTxn.Request,
			srvTxn, sip.StatusInternalServerError, nil)
	}
	newCT.WriteMessage(fwdMsg)
	info.RecordCallee(fwdMsg)

	update, destroy := timerCHandler.Add(*(newCT.Key))
	go func() {
		for {
			select {
			case <-time.After(TimerC):
				fireTimerC(srvTxn, newCT)
				timerCHandler.Remove(*(newCT.Key))
				return
			case <-update:
				break
			case <-destroy:
				timerCHandler.Remove(*(newCT.Key))
				return
			}
		}
	}()
	return nil
}

func forwardingResponse(srv *sip.Server, msg *sip.Message, srvTxn *sip.ServerTransaction) error {
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
	if srvTxn != nil {
		srvTxn.WriteMessage(cpMsg)
	} else {
		srv.Debugf("Sent Message without transaction, [%s]", cpMsg.StatusCode)
		srv.WriteMessage(cpMsg)
	}
	return nil
}

func byeResponseHandler(srv *sip.Server, msg *sip.Message, srvTxn *sip.ServerTransaction,
	ctKey *sip.ClientTransactionKey, info *callInfo) error {

	cltTxnKey := *ctKey
	if msg.StatusCode == sip.StatusTrying {
		srv.Debugf("100 Trying no need forwarded")
		return nil
	} else if msg.StatusCode >= 200 {
		responseContexts.Remove(cltTxnKey)
		if msg.StatusCode < 300 {
			info.RecordTerminated(msg, TERM_REMOTE)
			callStates.Close(info)
		}
	}
	return forwardingResponse(srv, msg, srvTxn)
}

func invaiteResponseHandler(srv *sip.Server, msg *sip.Message, srvTxn *sip.ServerTransaction,
	ctKey *sip.ClientTransactionKey, info *callInfo) error {
	cltTxnKey := *ctKey

	update, destroy := timerCHandler.Get(cltTxnKey)

	if msg.StatusCode == sip.StatusTrying {
		srv.Debugf("100 Trying no need forwarded")
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
			info.RecordEstablished(msg)
		} else if msg.StatusCode >= 300 && msg.StatusCode < 400 {
			return forwardRedirection(srv, msg, info, srvTxn)
		} else {
			info.RecordTerminated(msg, TERM_REMOTE)
			callStates.Close(info)
		}
	}
	return forwardingResponse(srv, msg, srvTxn)
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

	switch msg.CSeq.Method {
	case sip.MethodINVITE:
		return invaiteResponseHandler(srv, msg, srvTxn.(*sip.ServerTransaction), cltTxnKey_p, info)
	case sip.MethodBYE:
		return byeResponseHandler(srv, msg, srvTxn.(*sip.ServerTransaction), cltTxnKey_p, info)
	}

	if msg.StatusCode == sip.StatusTrying {
		srv.Debugf("100 Trying no need forwarded")
		return nil
	} else if msg.StatusCode >= 200 {
		responseContexts.Remove(cltTxnKey)
	}
	return forwardingResponse(srv, msg, srvTxn.(*sip.ServerTransaction))
}
