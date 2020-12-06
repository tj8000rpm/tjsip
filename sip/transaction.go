package sip

import (
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

type transactionState int

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
	Destroy()
}

type BaseTransaction struct {
	Mu      sync.Mutex
	Key     interface{}
	Server  *Server
	Invite  bool
	State   transactionState
	TuChan  chan *Message
	DelChan chan bool
	Err     error
}

/* ****************************************************
 * Base Transaction
 * ****************************************************/

func (t *BaseTransaction) WriteMessage(msg *Message) {
	t.Mu.Lock()
	defer t.Mu.Unlock()
	if t.State >= TransactionStateTerminated {
		t.Server.Warnf("[%v] Transation User channel already closed", t.Key)
		t.Server.Warnf("[%v] Transation : %v", t.Key, t)
		return
	}
	t.TuChan <- msg
}

func (t *BaseTransaction) DestroyChanel() {
	t.Mu.Lock()
	defer t.Mu.Unlock()
	if t.State == TransactionStateClosed {
		t.Server.Debugf("[%v] transaction already closed", t.Key)
		return
	}
	t.State = TransactionStateClosed
	t.Server.Debugf("[%v] Transaction was destroyed", t.Key)
	close(t.DelChan)
	close(t.TuChan)
}

/* ****************************************************
 * Client Transaction
 * ****************************************************/

type clientTransactionKey struct {
	viaBranch  string
	cseqMethod string
}
type ClientTransaction struct {
	BaseTransaction
	Key     *clientTransactionKey
	request *Message
	ack     *Message
	resChan chan *Message
}

func (t *ClientTransaction) Destroy() {
	defer t.Server.DeleteClientTransaction(t)
	t.Mu.Lock()
	close(t.resChan)
	t.resChan = nil
	t.Mu.Unlock()
	t.DestroyChanel()
}

func (t *ClientTransaction) Handle(res *Message) {
	t.Mu.Lock()
	defer t.Mu.Unlock()
	if t.resChan == nil {
		return
	}
	t.resChan <- res
	t.Server.Debugf("handle to core in transaction")
	t.Server.HandleInTransaction(res)
}

func (t *ClientTransaction) controllerTerminated() {
	t.Server.Debugf("state to terminated in transacation %v", t.Key)
	t.State = TransactionStateTerminated
	t.Server.Debugf("[%v] Transaction was closed", t.Key)
	t.Destroy()
}

func (t *ClientTransaction) nonInviteControllerCompleted() {
	t.Server.Debugf("State to Completed")
	t.State = TransactionStateCompleted
	timerKChan := make(chan bool)
	go func() {
		t.Server.Debugf("[%v] Timer K Start", t.Key)
		time.Sleep(TimerK)
		timerKChan <- true
	}()
	for {
		select {
		case <-t.DelChan:
			t.Server.Debugf("[%v] Recived delete signal", t.Key)
			return
		case <-timerKChan:
			t.Server.Debugf("[%v] Timer K(%v) fire", t.Key, TimerK)
			t.controllerTerminated()
			return
		case msg := <-t.resChan:
			if msg == nil {
				return
			}
			continue
		}
	}
}

func (t *ClientTransaction) inviteControllerCompleted() {
	t.Server.Debugf("State to Completed")
	t.State = TransactionStateCompleted
	t.Server.WriteMessage(t.ack)
	timerDChan := make(chan bool)
	go func() {
		t.Server.Debugf("[%v] Timer D Start", t.Key)
		time.Sleep(TimerD)
		timerDChan <- true
	}()
	for {
		select {
		case <-t.DelChan:
			t.Server.Debugf("[%v] Recived delete signal", t.Key)
			return
		case <-timerDChan:
			t.Server.Debugf("[%v] Timer D(%v) fire", t.Key, TimerD)
			t.controllerTerminated()
			return
		case msg := <-t.resChan:
			if msg == nil {
				return
			}
			if msg.StatusCode >= 300 && msg.StatusCode < 700 {
				t.Server.WriteMessage(t.ack)
			}
		}
	}
}

func (t *ClientTransaction) nonInviteControllerProceeding() {
	t.State = TransactionStateProceeding

	timerE := TimerE
	timerF := TimerF
	for {
		t.Server.Debugf("Ready to response")
		select {
		case <-t.DelChan:
			t.Server.Debugf("[%v] Recived delete signal", t.Key)
			return
		case <-time.After(timerE):
			// retransmit request
			t.Server.Debugf("[%v] Timer E(%v) fire", t.Key, timerE)
			t.Server.WriteMessage(t.request)
			timerF -= timerE
			timerE *= 2
			if timerE > T2 {
				timerE = T2
			}
			continue
		case <-time.After(timerF):
			t.Server.Debugf("[%v] Timer F fire", t.Key)
			t.controllerTerminated()
			return
		case msg := <-t.resChan:
			if msg == nil {
				return
			}
			t.Server.Debugf("[%v] Response from TU", t.Key)
			if !msg.Response {
				t.Destroy()
				return
			}

			if msg.StatusCode >= 100 && msg.StatusCode < 700 {
				if msg.StatusCode >= 200 && msg.StatusCode < 700 {
					t.nonInviteControllerCompleted()
					return
				} else {
					t.nonInviteControllerProceeding()
					return
				}
			}
		}
	}
}

func (t *ClientTransaction) inviteControllerProceeding() {
	t.State = TransactionStateProceeding
	for {
		select {
		case <-t.DelChan:
			t.Server.Debugf("[%v] Recived delete signal", t.Key)
			return
		case msg := <-t.resChan:
			if msg == nil {
				return
			}
			t.Server.Debugf("[%v] Response from TU", t.Key)
			if !msg.Response {
				t.Destroy()
				return
			}
			if msg.StatusCode >= 100 && msg.StatusCode < 200 {
				continue
			} else if msg.StatusCode >= 200 && msg.StatusCode < 300 {
				t.controllerTerminated()
				return
			} else if msg.StatusCode >= 300 && msg.StatusCode < 700 {
				ack, err := GenerateAckFromRequestAndResponse(t.request, msg)
				if err != nil {
					t.Server.Debugf("Unable to generate ACK Request / %v", err)
					return
				}
				t.Mu.Lock()
				t.ack = ack
				t.Mu.Unlock()
				t.inviteControllerCompleted()
				return
			}
		}
	}
}

func (t *ClientTransaction) inviteController() {
	t.State = TransactionStateCalling
	// Before sent INVITE
	select {
	case <-t.DelChan:
		t.Server.Debugf("[%v] Recived delete signal", t.Key)
		return
	case msg := <-t.TuChan:
		if !msg.Request || msg.Method != "INVITE" {
			t.Err = ErrTransactionUnexpectedMessage
			t.Destroy()
			return
		}
		// transmit Initial first INVITE message
		t.Server.Debugf("[%v] Transmit INVITE request from TU", t.Key)
		t.Mu.Lock()
		t.request = msg
		t.Mu.Unlock()
		t.Server.WriteMessage(msg)
	}

	timerA := TimerA
	timerB := TimerB
	// EnterCallingState
	for {
		t.Server.Debugf("Ready to response")
		select {
		case <-t.DelChan:
			t.Server.Debugf("[%v] Recived delete signal", t.Key)
			return
		case <-time.After(timerA):
			// retransmit final response
			t.Server.Debugf("[%v] Timer A(%v) fire", t.Key, timerA)
			t.Server.WriteMessage(t.request)
			timerB -= timerA
			timerA *= 2
			continue
		case <-time.After(timerB):
			t.Server.Debugf("[%v] Timer B fire", t.Key)
			t.controllerTerminated()
			return
		case msg := <-t.resChan:
			if msg == nil {
				return
			}
			t.Server.Debugf("[%v] Response from TU", t.Key)
			if !msg.Response {
				t.Destroy()
				return
			}

			if msg.StatusCode >= 100 && msg.StatusCode < 700 {
				if msg.StatusCode >= 200 && msg.StatusCode < 300 {
					t.controllerTerminated()
					return
				} else if msg.StatusCode >= 300 && msg.StatusCode < 700 {
					ack, err := GenerateAckFromRequestAndResponse(t.request, msg)
					if err != nil {
						t.Server.Debugf("Unable to generate ACK Request / %v", err)
						return
					}
					t.Mu.Lock()
					t.ack = ack
					t.Mu.Unlock()
					t.inviteControllerCompleted()
				} else {
					t.inviteControllerProceeding()
					return
				}
			}
		}
	}
}

func (t *ClientTransaction) nonInviteController() {
	t.State = TransactionStateTrying
	// Before State Trying
	select {
	case <-t.DelChan:
		t.Server.Debugf("[%v] Recived delete signal", t.Key)
		return
	case msg := <-t.TuChan:
		if !msg.Request || msg.Method == "INVITE" {
			t.Err = ErrTransactionUnexpectedMessage
			t.Destroy()
			return
		}
		// transmit Request message
		t.Server.Debugf("[%v] Transmit request from TU", t.Key)
		t.Mu.Lock()
		t.request = msg
		t.Mu.Unlock()
		t.Server.WriteMessage(msg)
	}

	timerE := TimerE
	timerF := TimerF
	// Enter Trying State
	for {
		t.Server.Debugf("Ready to response")
		select {
		case <-t.DelChan:
			t.Server.Debugf("[%v] Recived delete signal", t.Key)
			return
		case <-time.After(timerE):
			// retransmit request
			t.Server.Debugf("[%v] Timer E(%v) fire", t.Key, timerE)
			t.Server.WriteMessage(t.request)
			timerF -= timerE
			timerE *= 2
			if timerE > T2 {
				timerE = T2
			}
			continue
		case <-time.After(timerF):
			t.Server.Debugf("[%v] Timer F fire", t.Key)
			t.controllerTerminated()
			return
		case msg := <-t.resChan:
			if msg == nil {
				return
			}
			t.Server.Debugf("[%v] Response from TU", t.Key)
			if !msg.Response {
				t.Destroy()
				return
			}

			if msg.StatusCode >= 100 && msg.StatusCode < 700 {
				if msg.StatusCode >= 200 && msg.StatusCode < 700 {
					t.nonInviteControllerCompleted()
					return
				} else {
					t.nonInviteControllerProceeding()
					return
				}
			}
		}
	}
}

func (t *ClientTransaction) Controller() {
	if t.Invite {
		t.inviteController()
	} else {
		t.nonInviteController()
	}
}

func NewClientInviteTransaction(srv *Server, msg *Message, chanSize int) *ClientTransaction {
	respChan := make(chan *Message, chanSize)
	if msg.Via == nil {
		msg.Via = NewViaHeaders()
	}
	branch := "branch=" + GenerateBranchParam()
	v := NewViaHeaderUDP(srv.Address(), branch)
	msg.Via.Insert(v)
	key, err := GenerateClientTransactionKey(msg)
	if err != nil {
		return nil
	}
	return newClientTransaction(srv, true, key, msg, respChan)
}

func NewClientNonInviteTransaction(srv *Server, msg *Message, chanSize int) *ClientTransaction {
	respChan := make(chan *Message, chanSize)
	if msg.Via == nil {
		msg.Via = NewViaHeaders()
	}
	branch := "branch=" + GenerateBranchParam()
	v := NewViaHeaderUDP(srv.Address(), branch)
	msg.Via.Insert(v)
	key, err := GenerateClientTransactionKey(msg)
	if err != nil {
		return nil
	}
	return newClientTransaction(srv, false, key, msg, respChan)
}

func newClientTransaction(srv *Server, isInvite bool, key *clientTransactionKey, msg *Message, respChan chan *Message) *ClientTransaction {
	trans := new(ClientTransaction)
	trans.State = TransactionStateInit
	trans.Invite = isInvite
	trans.Server = srv
	trans.TuChan = make(chan *Message)
	trans.Err = nil
	trans.DelChan = make(chan bool)
	trans.Key = key
	trans.request = msg
	trans.resChan = make(chan *Message)
	go trans.Controller()
	return trans
}

func GenerateClientTransactionKey(msg *Message) (*clientTransactionKey, error) {
	var key *clientTransactionKey
	_, _, params, err := msg.GetTopMostVia()
	if err != nil {
		// Malformed topmost via header
		return nil, err
	}
	viaBranch, ok := params["branch"]
	if !ok || len(viaBranch) == 0 {
		// Branch parameter not found
		return nil, ErrHeaderParseError
	}
	method, _, err := msg.GetCSeq()
	if err != nil {
		// Malformed cseq header
		return nil, err
	}

	key = &clientTransactionKey{viaBranch: viaBranch[0], cseqMethod: method}
	return key, nil
}

type clientMessageReciever struct {
	reciever chan *Message
}

func NewClientMessageReciever(qsize int) *clientMessageReciever {
	cmr := new(clientMessageReciever)
	cmr.reciever = make(chan *Message, qsize)
	return cmr
}

func (cmr *clientMessageReciever) Reciver() chan *Message {
	return cmr.reciever
}

func (cmr *clientMessageReciever) Recive() *Message {
	return <-cmr.reciever
}

/* ****************************************************
 * Server Transaction
 * ****************************************************/
type serverTransactionKey struct {
	viaBranch string
	sentBy    string
	method    string
}

type ServerTransaction struct {
	BaseTransaction
	Key            *serverTransactionKey
	request        *Message
	provisionalRes *Message
	finalRes       *Message
}

func (t *ServerTransaction) Destroy() {
	t.DestroyChanel()
	t.Server.DeleteServerTransaction(t)
}

func (t *ServerTransaction) Handle(req *Message) {
	t.Mu.Lock()
	defer t.Mu.Unlock()
	if req.Method == "ACK" {
		t.Server.Debugf("ACK Recived")
		t.TuChan <- req
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
		case <-t.DelChan:
			return
		case msg := <-t.TuChan:
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
		case msg := <-t.TuChan:
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
	case <-t.DelChan:
		t.Server.Debugf("[%v] Recived delete signal", t.Key)
		return
	case <-time.After(Timer100Try):
		t.Server.Debugf("[%v] Sent 100 Trying", t.Key)
		provRes := t.request.GenerateResponseFromRequest()
		t.Mu.Lock()
		t.provisionalRes = provRes
		t.Mu.Unlock()
		t.Server.WriteMessage(provRes)
	case msg := <-t.TuChan:
		if msg == nil {
			return
		}
		if !msg.Response {
			t.Err = ErrTransactionUnexpectedMessage
			t.Destroy()
			return
		}
		if msg.StatusCode > 100 && msg.StatusCode < 200 {
			// transmit response state will conitunue
			t.Server.Debugf("[%v] Transmit provisional response from TU", t.Key)
			t.Mu.Lock()
			t.provisionalRes = msg
			t.Mu.Unlock()
			t.Server.WriteMessage(msg)
		} else if msg.StatusCode >= 200 && msg.StatusCode < 300 {
			// transmit response and state to Terminated
			t.Server.Debugf("[%v] Transmit final response from TU", t.Key)
			t.Mu.Lock()
			t.finalRes = msg
			t.Mu.Unlock()
			t.Server.WriteMessage(msg)
			t.controllerTerminated()
			return
		} else if msg.StatusCode >= 300 && msg.StatusCode < 700 {
			t.Server.Debugf("[%v] Transmit final response from TU", t.Key)
			t.Mu.Lock()
			t.finalRes = msg
			t.Mu.Unlock()
			t.Server.WriteMessage(msg)
			t.inviteControllerCompleted()
			return
		}
	}

	t.Server.Debugf("Reply first provisional Response")

	// Stage Procesing
	for {
		t.Server.Debugf("Ready to next response")
		select {
		case <-t.DelChan:
			t.Server.Debugf("[%v] Recived delete signal", t.Key)
			return
		case msg := <-t.TuChan:
			if msg == nil {
				return
			}
			t.Server.Debugf("[%v] Response from TU", t.Key)
			if !msg.Response {
				t.Destroy()
				return
			}
			if msg.StatusCode > 100 && msg.StatusCode < 200 {
				// transmit response state will conitunue
				t.Server.Debugf("[%v] Transmit provisional response from TU", t.Key)
				t.Mu.Lock()
				t.provisionalRes = msg
				t.Mu.Unlock()
				t.Server.WriteMessage(msg)
			} else if msg.StatusCode >= 200 && msg.StatusCode < 300 {
				// transmit response and state to Terminated
				t.Server.Debugf("[%v] Transmit final response from TU", t.Key)
				t.Mu.Lock()
				t.finalRes = msg
				t.Mu.Unlock()
				t.Server.WriteMessage(msg)
				t.controllerTerminated()
				return
			} else if msg.StatusCode >= 300 && msg.StatusCode <= 600 {
				t.Server.Debugf("[%v] Transmit final response(NOT 2xx) from TU", t.Key)
				t.Mu.Lock()
				t.finalRes = msg
				t.Mu.Unlock()
				t.Server.WriteMessage(msg)
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
		case <-t.DelChan:
			t.Server.Debugf("[%v] Recived delete signal", t.Key)
			return
		case msg := <-t.TuChan:
			if msg == nil {
				return
			}
			if !msg.Response {
				t.Err = ErrTransactionUnexpectedMessage
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
	case <-t.DelChan:
		t.Server.Debugf("[%v] Recived delete signal", t.Key)
		return
	}
}

func (t *ServerTransaction) nonInviteController() {
	t.State = TransactionStateTrying
	select {
	case <-t.DelChan:
		t.Server.Debugf("[%v] Recived delete signal", t.Key)
		return
	case msg := <-t.TuChan:
		if msg == nil {
			return
		}
		if !msg.Response {
			t.Err = ErrTransactionUnexpectedMessage
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
	if t.Invite {
		t.inviteController()
	} else {
		t.nonInviteController()
	}
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
	trans.Invite = isInvite
	trans.Server = srv
	trans.TuChan = make(chan *Message)
	trans.Err = nil
	trans.DelChan = make(chan bool)
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
	if !ok || len(viaBranch) == 0 {
		// Branch parameter not found
		return nil, ErrHeaderParseError
	}
	method := msg.Method
	if method == "ACK" {
		method = "INVITE"
	}
	key = &serverTransactionKey{viaBranch: viaBranch[0], sentBy: sentBy, method: method}
	return key, nil
}
