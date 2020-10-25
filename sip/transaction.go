package sip

import (
	//"fmt"
	"sync"
	"time"
)

var (
	TimerA      = T1
	TimerB      = 64 * T1
	TimerD      = 32 * time.Second
	TimerE      = T1
	TimerF      = 64 * T1
	TimerG      = T1
	TimerH      = 64 * T1
	TimerI      = T4
	TimerJ      = 64 * T1
	TimerK      = T4
	Timer100Try = 200 * time.Millisecond
)

const (
	TransactionStateInit = iota
	TransactionStateTrying
	TransactionStateCalling
	TransactionStateProceeding
	TransactionStateCompleted
	TransactionStateConfirmed
	TransactionStateTerminated
	TransactionStateClosed
)

var (
	ErrTransactionDuplicated        = &ProtocolError{"transaction duplicated"}
	ErrTransactionUnexpectedMessage = &ProtocolError{"transaction recieve unexpected message"}
	ErrTransactionTransportError    = &ProtocolError{"transport error"}
	ErrTransactionClosed            = &ProtocolError{"transaction was closed"}
)

type Transaction interface {
	Handle(*Message)
	Controller()
}

type transactionState int

type clientTransactionKey struct {
	viaBranch  string
	cseqMethod string
}
type ClientTransaction struct {
	Key      *clientTransactionKey
	Server   *Server
	isInvite bool
	state    transactionState
	req      *Message
	tuChan   chan *Message
	delChan  chan bool
	err      error
}

func (t *ClientTransaction) Handle(res *Message) {
}
func (t *ClientTransaction) Controller(res *Message) {
}

type serverTransactionKey struct {
	viaBranch string
	sentBy    string
	method    string
}

type ServerTransaction struct {
	mu             sync.Mutex
	Key            *serverTransactionKey
	Server         *Server
	IsInvite       bool
	State          transactionState
	request        *Message
	provisionalRes *Message
	finalRes       *Message
	tuChan         chan *Message
	delChan        chan bool
	err            error
}

func (t *ServerTransaction) Handle(req *Message) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if (t.State == TransactionStateCompleted || t.State == TransactionStateConfirmed) &&
		req.Method == "ACK" {
		t.Server.Debugf("ACK Recived")
		t.tuChan <- req
		return
	}
	if t.finalRes != nil {
		t.Server.Debugf("[%v] retransmit final response", t.Key)
		//t.Server.Infof("%v vs %v", msg.RemoteAddr, query)
		t.WriteMessage(t.finalRes)
	} else if t.provisionalRes != nil {
		t.Server.Debugf("[%v] retransmit provisional response", t.Key)
		t.WriteMessage(t.provisionalRes)
	}
	t.Server.Infof("[%v] retrans but call be processing", t.Key)
	//t.Server.Infof("retrans Transation: \n%v", *t)
}

func (t *ServerTransaction) WriteMessage(msg *Message) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.State >= TransactionStateTerminated {
		t.Server.Warnf("[%v] Transation User channel already closed", t.Key)
		t.Server.Warnf("[%v] Transation : %v", t.Key, t)
		return
	}
	t.tuChan <- msg
}

func (t *ServerTransaction) controllerTerminated() {
	t.Server.Debugf("state to terminated in transacation %v", t.Key)
	t.State = TransactionStateTerminated
	t.Server.Debugf("[%v] Transaction was closed", t.Key)
	t.Destroy()
}
func (t *ServerTransaction) inviteControllerConfirmed() {
	t.Server.Debugf("state to confirmed in transacation %v", t.Key)
	t.State = TransactionStateCompleted
	t.State = TransactionStateConfirmed
	go func() {
		time.Sleep(TimerI)
		t.controllerTerminated()
	}()

	for {
		select {
		case <-t.delChan:
			return
		case msg := <-t.tuChan:
			if msg == nil {
				return
			}
			if msg.Request && msg.Method == "ACK" {
				continue
			}
		}
	}
}

func (t *ServerTransaction) inviteControllerCompleted() {
	t.Server.Debugf("state to complated in transacation %v", t.Key)
	t.State = TransactionStateCompleted
	timerG := TimerG
	timerH := TimerH
	for {
		select {
		case <-time.After(timerG):
			// retransmit final response
			t.Server.Debugf("[%v] Timer G(%v) fire", t.Key, timerG)
			t.Server.WriteMessage(t.finalRes)
			timerH -= timerG
			timerG *= 2
			if timerG > T2 {
				timerG = T2
			}
			continue
		case <-time.After(timerH):
			t.Server.Debugf("[%v] Timer H fire", t.Key)
			t.controllerTerminated()
			return
		case msg := <-t.tuChan:
			if msg == nil {
				return
			}
			if msg.Request && msg.Method == "ACK" {
				t.inviteControllerConfirmed()
				return
			}
		}
	}
}

func (t *ServerTransaction) inviteController() {
	t.State = TransactionStateProceeding
	select {
	case <-t.delChan:
		t.Server.Debugf("[%v] Recived delete signal", t.Key)
		return
	case <-time.After(Timer100Try):
		t.Server.Debugf("[%v] Sent 100 Trying", t.Key)
		provRes := t.request.GenerateResponseFromRequest()
		t.Server.WriteMessage(provRes)
		t.mu.Lock()
		t.provisionalRes = provRes
		t.mu.Unlock()
	case msg := <-t.tuChan:
		if msg == nil {
			return
		}
		if !msg.Response {
			t.err = ErrTransactionUnexpectedMessage
			t.Destroy()
			return
		}
		t.Server.WriteMessage(msg)
		if msg.StatusCode > 100 && msg.StatusCode < 200 {
			// transmit response state will conitunue
			t.Server.Debugf("[%v] Transmit provisional response from TU", t.Key)
			t.mu.Lock()
			t.provisionalRes = msg
			t.mu.Unlock()
		} else if msg.StatusCode >= 200 && msg.StatusCode < 300 {
			// transmit response and state to Terminated
			t.Server.Debugf("[%v] Transmit final response from TU", t.Key)
			t.mu.Lock()
			t.finalRes = msg
			t.mu.Unlock()
			t.controllerTerminated()
			return
		} else if msg.StatusCode >= 300 && msg.StatusCode < 700 {
			t.Server.Debugf("[%v] Transmit final response from TU", t.Key)
			t.mu.Lock()
			t.finalRes = msg
			t.mu.Unlock()
			t.inviteControllerCompleted()
			return
		}
	}

	t.Server.Debugf("Reply first provisional Response")

	// Stage Procesing
	for {
		t.Server.Debugf("Rediy to next response")
		select {
		case <-t.delChan:
			t.Server.Debugf("[%v] Recived delete signal", t.Key)
			return
		case msg := <-t.tuChan:
			if msg == nil {
				return
			}
			t.Server.Debugf("[%v] Response from TU", t.Key)
			if !msg.Response {
				t.Destroy()
				return
			}
			t.Server.WriteMessage(msg)
			if msg.StatusCode > 100 && msg.StatusCode < 200 {
				// transmit response state will conitunue
				t.Server.Debugf("[%v] Transmit provisional response from TU", t.Key)
				t.mu.Lock()
				t.provisionalRes = msg
				t.mu.Unlock()
			} else if msg.StatusCode >= 200 && msg.StatusCode < 300 {
				// transmit response and state to Terminated
				t.Server.Debugf("[%v] Transmit final response from TU", t.Key)
				t.mu.Lock()
				t.finalRes = msg
				t.mu.Unlock()
				t.controllerTerminated()
				return
			} else if msg.StatusCode >= 300 && msg.StatusCode <= 600 {
				t.Server.Debugf("[%v] Transmit final response(NOT 2xx) from TU", t.Key)
				t.finalRes = msg
				t.mu.Lock()
				t.mu.Unlock()
				t.inviteControllerCompleted()
				return
			} else {
				t.Server.Debugf("[%v] Missing way transaction", t.Key)
			}
		}
	}
}

func (t *ServerTransaction) nonInviteControllerProceeding() {
	t.Server.Debugf("state to proceeding in transacation %v", t.Key)
	t.State = TransactionStateProceeding
	for {
		select {
		case <-t.delChan:
			t.Server.Debugf("[%v] Recived delete signal", t.Key)
			return
		case msg := <-t.tuChan:
			if msg == nil {
				return
			}
			if !msg.Response {
				t.err = ErrTransactionUnexpectedMessage
				t.Destroy()
				return
			}
			t.Server.WriteMessage(msg)
			if msg.StatusCode > 100 && msg.StatusCode < 200 {
				// transmit response state will conitunue
				t.Server.Debugf("[%v] Transmit provisional response from TU", t.Key)
				t.provisionalRes = msg
				continue
			} else if msg.StatusCode >= 200 && msg.StatusCode < 700 {
				// transmit response and state to Terminated
				t.Server.Debugf("[%v] Transmit final response from TU", t.Key)
				t.finalRes = msg
				t.nonInviteControllerCompleted()
				return
			}
		}
	}
}

func (t *ServerTransaction) nonInviteControllerCompleted() {
	t.Server.Debugf("state to complated in transacation %v", t.Key)
	t.State = TransactionStateCompleted
	timerJ := TimerJ
	select {
	case <-time.After(timerJ):
		t.Server.Debugf("[%v] Timer J fire", t.Key)
		t.controllerTerminated()
		return
	case <-t.delChan:
		t.Server.Debugf("[%v] Recived delete signal", t.Key)
		return
	}
}

func (t *ServerTransaction) nonInviteController() {
	t.State = TransactionStateTrying
	select {
	case <-t.delChan:
		t.Server.Debugf("[%v] Recived delete signal", t.Key)
		return
	case msg := <-t.tuChan:
		if msg == nil {
			return
		}
		if !msg.Response {
			t.err = ErrTransactionUnexpectedMessage
			t.Destroy()
			return
		}
		t.Server.WriteMessage(msg)
		if msg.StatusCode > 100 && msg.StatusCode < 200 {
			// transmit response state will conitunue
			t.Server.Debugf("[%v] Transmit provisional response from TU", t.Key)
			t.provisionalRes = msg
			t.nonInviteControllerProceeding()
		} else if msg.StatusCode >= 200 && msg.StatusCode < 700 {
			// transmit response and state to Terminated
			t.Server.Debugf("[%v] Transmit final response from TU", t.Key)
			t.finalRes = msg
			t.nonInviteControllerCompleted()
			return
		}
	}

	t.Server.Debugf("Reply first provisional Response")

}

func (t *ServerTransaction) Controller() {
	if t.IsInvite {
		t.inviteController()
	} else {
		t.nonInviteController()
	}
}
func (t *ServerTransaction) Destroy() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.State == TransactionStateClosed {
		t.Server.Debugf("[%v] transaction already closed", t.Key)
		return
	}
	t.State = TransactionStateClosed
	t.Server.Debugf("[%v] Transaction was destroyed", t.Key)
	close(t.delChan)
	close(t.tuChan)
	t.Server.DeleteServerTransaction(t)
}

func NewServerInviteTransaction(srv *Server, key *serverTransactionKey, msg *Message) *ServerTransaction {
	return newServerTransaction(srv, true, key, msg)
}

func NewServerNonInviteTransaction(srv *Server, key *serverTransactionKey, msg *Message) *ServerTransaction {
	return newServerTransaction(srv, false, key, msg)
}

func newServerTransaction(srv *Server, isInvite bool, key *serverTransactionKey, msg *Message) *ServerTransaction {
	trans := new(ServerTransaction)
	trans.State = TransactionStateInit
	trans.IsInvite = isInvite
	trans.Server = srv
	trans.tuChan = make(chan *Message)
	trans.err = nil
	trans.delChan = make(chan bool)
	trans.Key = key
	trans.request = msg
	go trans.Controller()
	return trans
}

func GenerateServerTransactionKey(msg *Message) (*serverTransactionKey, error) {
	var key *serverTransactionKey
	_, sentBy, params, err := msg.GetTopMostVia()
	if err != nil {
		// Malformed topmost via header
		return nil, err
	}
	viaBranch, ok := params["branch"]
	if !ok {
		// Branch parameter not found
		return nil, ErrHeaderParseError
	}
	method := msg.Method
	if method == "ACK" {
		method = "INVITE"
	}
	key = &serverTransactionKey{viaBranch: viaBranch, sentBy: sentBy, method: method}
	return key, nil
}

func GenerateClientTransactionKey(msg *Message) (*clientTransactionKey, error) {
	return nil, nil
}
