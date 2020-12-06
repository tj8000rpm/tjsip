package main

import (
	"fmt"
	"sip/sip"
	"strings"
	"sync"
	//"time"
)

const (
	TrunkSubscriber = iota
	TrunkServer
	TrunkUntrustedServer
)

type Dialog struct {
	FromTag string
	ToTag   string
	CallID  string
}

type Dialogs struct {
	Mu      sync.Mutex
	Dialogs map[Dialog]bool
}

func (d *Dialogs) NotIn(e Dialog) bool {
	return !d.In(e)
}

func (d *Dialogs) In(e Dialog) bool {
	d.Mu.Lock()
	defer d.Mu.Unlock()
	_, ok := d.Dialogs[e]
	return ok
}

func (d *Dialogs) Remove(e Dialog) bool {
	d.Mu.Lock()
	defer d.Mu.Unlock()
	_, ok := d.Dialogs[e]
	if ok {
		delete(d.Dialogs, e)
		fmt.Printf("Dialogs - %v\n", d.Dialogs)
		return true
	}
	return true
}

func (d *Dialogs) Add(e Dialog) bool {
	d.Mu.Lock()
	defer d.Mu.Unlock()
	_, ok := d.Dialogs[e]
	if !ok {
		d.Dialogs[e] = true
		fmt.Printf("Dialogs - %v\n", d.Dialogs)
		return true
	}
	return false
}

func NewDialogs() *Dialogs {
	d := new(Dialogs)
	d.Dialogs = map[Dialog]bool{}
	return d
}

type EarlyDialog struct {
	FromTag string
	CallID  string
}

type EarlyDialogs struct {
	Mu      sync.Mutex
	Dialogs map[EarlyDialog]bool
}

func (d *EarlyDialogs) NotIn(e EarlyDialog) bool {
	return !d.In(e)
}

func (d *EarlyDialogs) In(e EarlyDialog) bool {
	d.Mu.Lock()
	defer d.Mu.Unlock()
	_, ok := d.Dialogs[e]
	return ok
}

func (d *EarlyDialogs) Remove(e EarlyDialog) bool {
	d.Mu.Lock()
	defer d.Mu.Unlock()
	_, ok := d.Dialogs[e]
	if ok {
		delete(d.Dialogs, e)
		fmt.Printf("EarlyDialogs - %v\n", d.Dialogs)
		return true
	}
	return true
}

func (d *EarlyDialogs) Add(e EarlyDialog) bool {
	d.Mu.Lock()
	defer d.Mu.Unlock()
	_, ok := d.Dialogs[e]
	if !ok {
		d.Dialogs[e] = true
		fmt.Printf("EarlyDialogs - %v\n", d.Dialogs)
		return true
	}
	return false
}

func NewEarlyDialogs() *EarlyDialogs {
	d := new(EarlyDialogs)
	d.Dialogs = map[EarlyDialog]bool{}
	return d
}

func lookupTrunkType(addr string) int {
	addrPort := strings.SplitN(addr, ":", 2)
	if addrPort[1] == "" {
		addrPort[1] = "5060"
	}
	return TrunkSubscriber
}

func route(request string) (fwdAddr, fwdDomain string) {
	fwdAddr = "127.0.0.1:5062"
	fwdDomain = "localhost:5062"
	return
}

func responseHandler(srv *sip.Server, msg *sip.Message) error {
	fmt.Printf("*******************\n")
	fmt.Printf("*******************Response\n")
	fmt.Printf("*******************\n")

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

	if fromTag == "" || callID == "" {
		// TODO:return sip.StatusBadRequest
		return nil
	}
	ppi := msg.Header.Get("P-Preferred-Identity")

	if ppi != "" {
		from = sip.ParseFrom(ppi)
	}

	// Handle established dialog
	if toTag != "" {
		dialog := Dialog{FromTag: fromTag, ToTag: toTag, CallID: callID}
		if dialogs.In(dialog) {
			// TODO: this message will be re-INVITE
			// sessionUpdate(disalog, msg)
			return nil
		} else {
			// TODO: reply sip.StatusBadRequest
			return nil
		}
	}

	return nil
}

func inviteHandler(srv *sip.Server, msg *sip.Message, txn *sip.ServerTransaction) error {
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

	if fromTag == "" || callID == "" {
		// TODO:return sip.StatusBadRequest
		return nil
	}
	ppi := msg.Header.Get("P-Preferred-Identity")

	if ppi != "" {
		from = sip.ParseFrom(ppi)
	}

	// Handle established dialog
	if toTag != "" {
		dialog := Dialog{FromTag: fromTag, ToTag: toTag, CallID: callID}
		if dialogs.In(dialog) {
			// TODO: this message will be re-INVITE
			// sessionUpdate(disalog, msg)
			return nil
		} else {
			// TODO: reply sip.StatusBadRequest
			return nil
		}
	}

	// Handle un-established dialog
	edialog := EarlyDialog{FromTag: fromTag, CallID: callID}
	if earlyDialogs.NotIn(edialog) {
		earlyDialogs.Add(edialog)
	} else {
		//TODO: reply sip.StatusLoopDetected
		return nil
	}

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
	fwdAddr, fwdDomain := route(requestService)

	fmt.Printf("Forwad to request : %s@%s / %s\n", requestService, fwdDomain, fwdAddr)

	fwdMsg := msg.Clone()
	fwdMsg.RequestURI.Host = fwdDomain
	fwdMsg.RemoteAddr = fwdAddr
	clientTxn := sip.NewClientInviteTransaction(srv, fwdMsg, 10)
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
	return nil
}

func cancelHandler(srv *sip.Server, msg *sip.Message, txn *sip.ServerTransaction) error {
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
	if msg.Method == sip.MethodACK {
		return ackHandler(srv, msg)
	}

	var txn *sip.ServerTransaction
	txnKey, err := sip.GenerateServerTransactionKey(msg)
	if err != nil {
		return err
	}
	// Create New INVITE srever transaction
	txn = sip.NewServerInviteTransaction(srv, txnKey, msg)
	err = srv.AddServerTransaction(txn)
	if err != nil {
		srv.Warnf("%v", err)
		txn.Destroy()
		return err
	}

	switch msg.Method {
	case sip.MethodINVITE:
		return inviteHandler(srv, msg, txn)
	case sip.MethodBYE:
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
