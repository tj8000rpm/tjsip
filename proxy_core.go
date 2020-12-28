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

type ResponseCtx = map[sip.ClientTransactionKey]bool
type ResponseCtxs struct {
	stToCt map[sip.ServerTransactionKey]*ResponseCtx
	ctToSt map[sip.ClientTransactionKey]sip.ServerTransactionKey
}

func (ctxs *ResponseCtxs) GetSt(ct sip.ClientTransactionKey) sip.ServerTransactionKey {
	st, _ := ctxs.ctToSt[ct]
	return st
}

// func (ctxs *ResponseCtxs) GetSt(ct sip.ClientTransactionKey) sip.ServerTransactionKey {
// 	st, _ := ctxs.ctToSt[ct]
// 	return st
// }

func (ctxs *ResponseCtxs) Add(st sip.ServerTransactionKey, ct sip.ClientTransactionKey) bool {
	_, ok := ctxs.stToCt[st]
	if !ok {
		ctxs.stToCt[st] = new(ResponseCtx)
	}
	if ctxs.stToCt[st] == nil {
		return false
	}
	(*ctxs.stToCt[st])[ct] = true
	ctxs.ctToSt[ct] = st
	return true
}

func NewResponseCtxs() *ResponseCtxs {
	ctx := new(ResponseCtxs)
	if ctx == nil {
		return nil
	}
	ctx.stToCt = make(map[sip.ServerTransactionKey]*ResponseCtx)
	ctx.ctToSt = make(map[sip.ClientTransactionKey]sip.ServerTransactionKey)
	if ctx.stToCt == nil || ctx.ctToSt == nil {
		return nil
	}
	return ctx
}

type SipPeer struct {
	RemoteAddr     string
	RemoteContact  *sip.Contact
	InitialRequest *sip.Message
	LastSent       *sip.Message
	LastRecieved   *sip.Message
	ClientTxns     map[sip.ClientTransactionKey]sip.ServerTransactionKey
	ServerTxns     map[sip.ServerTransactionKey]map[sip.ClientTransactionKey]bool
}

func NewSipPeer(addr string, c *sip.Contact, sent, recv *sip.Message) (peer *SipPeer) {
	peer = new(SipPeer)
	if peer == nil {
		return nil
	}
	peer.RemoteAddr = addr
	peer.RemoteContact = c
	peer.LastSent = sent
	peer.LastRecieved = recv
	peer.ClientTxns = make(map[sip.ClientTransactionKey]sip.ServerTransactionKey)
	peer.ServerTxns = make(map[sip.ServerTransactionKey]map[sip.ClientTransactionKey]bool)
	return
}

func (peer *SipPeer) AddClientTxn(cTxn sip.ClientTransactionKey, sTxn sip.ServerTransactionKey) bool {
	if peer.ClientTxns == nil {
		return false
	}
	peer.ClientTxns[cTxn] = sTxn
	return true
}

func (peer *SipPeer) AddServerTxn(sTxn sip.ServerTransactionKey, cTxn sip.ClientTransactionKey) bool {
	if peer.ServerTxns == nil {
		return false
	}
	_, ok := peer.ServerTxns[sTxn]
	if !ok {
		peer.ServerTxns[sTxn] = make(map[sip.ClientTransactionKey]bool)
		if peer.ServerTxns[sTxn] == nil {
			return false
		}
	}
	peer.ServerTxns[sTxn][cTxn] = true
	return true
}

type DialogKey struct {
	FromTag string
	ToTag   string
	CallID  string
}

type Dialog struct {
	Key    DialogKey
	Caller *SipPeer
	Callee *SipPeer
}

func NewDialog(key DialogKey, caller, callee *SipPeer) (dialog *Dialog) {
	dialog = new(Dialog)
	if dialog == nil {
		return nil
	}
	dialog.Key = key
	dialog.Caller = caller
	dialog.Callee = callee
	return
}

type Dialogs struct {
	Mu      sync.Mutex
	Dialogs map[DialogKey]*Dialog
}

func (d *Dialogs) NotIn(e DialogKey) bool {
	return !d.In(e)
}

func (d *Dialogs) In(e DialogKey) bool {
	d.Mu.Lock()
	defer d.Mu.Unlock()
	_, ok := d.Dialogs[e]
	return ok
}

func (d *Dialogs) Remove(e DialogKey) bool {
	d.Mu.Lock()
	defer d.Mu.Unlock()
	_, ok := d.Dialogs[e]
	if ok {
		delete(d.Dialogs, e)
		return true
	}
	return true
}

func (d *Dialogs) Add(e *Dialog) bool {
	d.Mu.Lock()
	defer d.Mu.Unlock()
	_, ok := d.Dialogs[e.Key]
	if !ok {
		d.Dialogs[e.Key] = e
		return true
	}
	return false
}

func (d *Dialogs) Get(e DialogKey) *Dialog {
	d.Mu.Lock()
	defer d.Mu.Unlock()
	dia, ok := d.Dialogs[e]
	if !ok {
		return nil
	}
	return dia
}

func NewDialogs() *Dialogs {
	d := new(Dialogs)
	d.Dialogs = map[DialogKey]*Dialog{}
	return d
}

type EarlyDialogKey struct {
	FromTag string
	CallID  string
}

type EarlyDialog struct {
	Key    EarlyDialogKey
	Caller *SipPeer
	Callee map[sip.ClientTransactionKey]*SipPeer
}

func NewEarlyDialog(key EarlyDialogKey, peer *SipPeer) (dialog *EarlyDialog) {
	dialog = new(EarlyDialog)
	if dialog == nil {
		return nil
	}
	dialog.Key = key
	dialog.Caller = peer
	dialog.Callee = make(map[sip.ClientTransactionKey]*SipPeer)
	return
}

type EarlyDialogs struct {
	Mu      sync.Mutex
	Dialogs map[EarlyDialogKey]*EarlyDialog
}

func (d *EarlyDialogs) NotIn(e EarlyDialogKey) bool {
	return !d.In(e)
}

func (d *EarlyDialogs) In(e EarlyDialogKey) bool {
	d.Mu.Lock()
	defer d.Mu.Unlock()
	_, ok := d.Dialogs[e]
	return ok
}

func (d *EarlyDialogs) Remove(e EarlyDialogKey) bool {
	d.Mu.Lock()
	defer d.Mu.Unlock()
	_, ok := d.Dialogs[e]
	if ok {
		delete(d.Dialogs, e)
		return true
	}
	return true
}

func (d *EarlyDialogs) Add(e *EarlyDialog) bool {
	d.Mu.Lock()
	defer d.Mu.Unlock()
	_, ok := d.Dialogs[e.Key]
	if !ok {
		d.Dialogs[e.Key] = e
		return true
	}
	return false
}

func (d *EarlyDialogs) Get(e EarlyDialogKey) *EarlyDialog {
	d.Mu.Lock()
	defer d.Mu.Unlock()
	dia, ok := d.Dialogs[e]
	if !ok {
		return nil
	}
	return dia
}

func NewEarlyDialogs() *EarlyDialogs {
	d := new(EarlyDialogs)
	d.Dialogs = map[EarlyDialogKey]*EarlyDialog{}
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
	var fromTag, toTag, callID string
	if msg.From == nil || msg.CallID == nil || msg.To == nil {
		srv.Debugf("Reply Bad Request")
		// TODO:return sip.StatusBadRequest
		return nil
	}

	from := msg.From
	fromTag = from.Parameter().Get("tag")
	to := msg.To
	toTag = to.Parameter().Get("tag")
	callID = msg.CallID.String()

	if fromTag == "" || callID == "" {
		srv.Debugf("Reply Bad Request")
		// TODO:return sip.StatusBadRequest
		return nil
	}

	// Handle un-established dialog
	var caller, callee *SipPeer
	edialogKey := EarlyDialogKey{FromTag: fromTag, CallID: callID}
	earlyDialog := earlyDialogs.Get(edialogKey)
	if earlyDialog == nil {
		// Handle established dialog
		if toTag == "" {
			// Invalid Discard
			srv.Debugf("No to tag")
			return nil
		}

		dialogKey := DialogKey{FromTag: fromTag, ToTag: toTag, CallID: callID}
		dialog := dialogs.Get(dialogKey)
		if dialog == nil {
			dialogKey = DialogKey{FromTag: toTag, ToTag: fromTag, CallID: callID}
			dialog = dialogs.Get(dialogKey)
			if dialog == nil {
				srv.Debugf("Invalid message discarded")
				return nil
			}
			caller = dialog.Callee
			callee = dialog.Caller
		} else {
			caller = dialog.Caller
			callee = dialog.Callee
		}
	} else {
		caller = earlyDialog.Caller
		key, err := sip.GenerateClientTransactionKey(msg)
		if err != nil {
			srv.Debugf("Invalid message")
			return nil
		}
		var ok bool
		callee, ok = earlyDialog.Callee[*key]
		if !ok {
			// invalid remote address / ignore
			srv.Debugf("Invalid remote address, message ignored")
			return nil
		}
	}
	srv.Debugf("Lookup transaction")
	cltTxnKey, err := sip.GenerateClientTransactionKey(msg)
	if err != nil {
		srv.Warnf("Fail Generate Cleint Transcation Key")
		return nil
	}
	srvTxnKey, ok := callee.ClientTxns[*cltTxnKey]
	if !ok {
		return nil
	}
	srvTxn := srv.LookupServerTransaction(&srvTxnKey)
	if srvTxn == nil {
		srv.Warnf("Server Transaction still nil")
		return nil
	}

	if msg.Contact.Length() > 0 {
		callee.RemoteContact = msg.Contact.Header[0]
	}
	if msg.StatusCode == sip.StatusTrying {
		srv.Debugf("100 Trying no need forwaed")
		return nil
	} else if msg.StatusCode >= 300 && msg.StatusCode < 400 {
		// TODO: redirect
		srv.Debugf("call will be redirected")
		return nil
	} else if msg.StatusCode >= 200 {
		earlyDialogs.Remove(edialogKey)
		if msg.StatusCode < 300 {
			dialogKey := DialogKey{FromTag: fromTag, ToTag: toTag, CallID: callID}
			dialog := NewDialog(dialogKey, caller, callee)
			dialogs.Add(dialog)
		}
	}

	callee.LastRecieved = msg
	cpMsg := msg.Clone()
	if cpMsg == nil {
		srv.Warnf("Message could not copied")
		return nil
	}
	srv.Debugf("Message was copied")
	cpMsg.RemoteAddr = caller.RemoteAddr
	caller.LastSent = cpMsg
	srvTxn.WriteMessage(cpMsg)

	return nil
}

func inviteHandler(srv *sip.Server, msg *sip.Message, txn *sip.ServerTransaction) error {
	var fromTag, toTag, callID string
	if msg.From == nil || msg.CallID == nil || msg.To == nil { // TODO:return sip.StatusBadRequest
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
		dialog := DialogKey{FromTag: fromTag, ToTag: toTag, CallID: callID}
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
	var earlyDialog *EarlyDialog
	var caller, callee *SipPeer
	edialogKey := EarlyDialogKey{FromTag: fromTag, CallID: callID}
	if earlyDialogs.NotIn(edialogKey) {
		caller = NewSipPeer(msg.RemoteAddr, msg.Contact.Header[0], nil, msg)
		caller.InitialRequest = msg
		earlyDialog = NewEarlyDialog(edialogKey, caller)
		earlyDialogs.Add(earlyDialog)
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
	fwdMsg.MaxForwards.Decrement()
	clientTxn := sip.NewClientInviteTransaction(srv, fwdMsg, 10)
	callee = NewSipPeer(fwdAddr, nil, fwdMsg, nil)
	callee.InitialRequest = fwdMsg

	if ok := callee.AddClientTxn(*clientTxn.Key, *txn.Key); !ok {
		srv.Debugf("Fail to update clientTxn")
	}
	if ok := caller.AddServerTxn(*txn.Key, *clientTxn.Key); !ok {
		srv.Debugf("Fail to update serverTxn")
	}

	earlyDialog.Callee[*clientTxn.Key] = callee
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

	var dialog *Dialog
	dialogKey := DialogKey{FromTag: fromTag, ToTag: toTag, CallID: callID}
	dialog = dialogs.Get(dialogKey)
	if dialog == nil {
		// Invalid message discard
		return nil
	}
	callee := dialog.Callee

	fwdMsg := msg.Clone()
	fwdMsg.MaxForwards.Decrement()
	fwdMsg.RemoteAddr = callee.RemoteAddr
	fwdMsg.RequestURI = callee.InitialRequest.RequestURI
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

	var caller, callee *SipPeer
	dialogKey := DialogKey{FromTag: fromTag, ToTag: toTag, CallID: callID}
	dialog := dialogs.Get(dialogKey)
	if dialog == nil {
		dialogKey = DialogKey{FromTag: toTag, ToTag: fromTag, CallID: callID}
		dialog = dialogs.Get(dialogKey)
		if dialog == nil {
			return nil
		}
		callee = dialog.Caller
		caller = dialog.Callee
	} else {
		callee = dialog.Callee
		caller = dialog.Caller
	}

	fwdMsg := msg.Clone()
	fwdMsg.MaxForwards.Decrement()
	fwdMsg.RemoteAddr = callee.RemoteAddr

	clientTxn := sip.NewClientNonInviteTransaction(srv, fwdMsg, 10)
	callee.AddClientTxn(*clientTxn.Key, *txn.Key)
	caller.AddServerTxn(*txn.Key, *clientTxn.Key)
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
