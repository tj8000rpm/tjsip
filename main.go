package main

import (
	"log"
	"os"
	"sip/sip"
	"sync"
	"time"
)

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

var stat = callStat{
	//completedPerResponse: make(map[int]int),
}

func myHandlerInvite(srv *sip.Server, msg *sip.Message, trans *sip.ServerTransaction) error {
	rep := msg.GenerateResponseFromRequest()

	rep.StatusCode = 180
	rep.AddToTag()
	trans.WriteMessage(rep)
	time.Sleep(time.Millisecond * 1000)
	rep.StatusCode = 183
	trans.WriteMessage(rep)
	time.Sleep(time.Second * 3)
	rep.StatusCode = 200
	trans.WriteMessage(rep)
	stat.Increment(200)

	return nil
}

func myHandlerNonInvite(srv *sip.Server, msg *sip.Message, trans *sip.ServerTransaction) error {
	if msg.Request && msg.Method == "ACK" {
		srv.Debugf("Dialog was established\n")
		return nil
	}
	err := srv.AddServerTransaction(trans)
	if err != nil {
		srv.Warnf("%v", err)
		trans.Destroy()
		return err
	}
	rep := msg.GenerateResponseFromRequest()
	rep.StatusCode = 200
	trans.WriteMessage(rep)
	return nil
}

func mySipCoreHandler(layer int, srv *sip.Server, msg *sip.Message) error {
	if layer != sip.LayerCore {
		return nil
	}
	if msg.Request {
		key, err := sip.GenerateServerTransactionKey(msg)
		if err != nil {
			return err
		}
		var newTransaction *sip.ServerTransaction

		if msg.Method == "INVITE" {
			newTransaction = sip.NewServerInviteTransaction(srv, key, msg)
			err := srv.AddServerTransaction(newTransaction)
			if err != nil {
				srv.Warnf("%v", err)
				newTransaction.Destroy()
				return err
			}
			return myHandlerInvite(srv, msg, newTransaction)
		} else {
			newTransaction = sip.NewServerNonInviteTransaction(srv, key, msg)
			return myHandlerNonInvite(srv, msg, newTransaction)
		}
		return nil
	}
	return nil
}

func clientMain(srv *sip.Server) {
	time.Sleep(time.Second * 1)
	for {
		log.Printf("client main")
		msg := srv.CreateIniINVITE(
			"127.0.0.1:5062",
			"sip:term@127.0.0.1:5062",
			"sip:orig@127.0.0.1:5060",
		)
		msg.Header.Add("From", "Hoge@127.0.0.1:5060")
		msg.AddFromTag()

		log.Printf("%v", msg)

		key, err := sip.GenerateClientTransactionKey(msg)
		if err != nil {
			log.Printf("TransactionKey is error %v", err)
			return
		}
		log.Printf("Transaction Key: %v", key)
		respChan := make(chan *sip.Message, 10)
		newTransaction := sip.NewClientInviteTransaction(srv, key, msg, respChan)
		log.Printf("Transaction: %v", newTransaction)
		err = srv.AddClientTransaction(newTransaction)
		if err != nil {
			srv.Warnf("%v", err)
			newTransaction.Destroy()
			return
		}
		log.Printf("Sent INVITE\n")
		newTransaction.TuChan <- msg
	loop:
		for {
			select {
			case <-newTransaction.TuChan:
				log.Printf("Transaction closed and exit loop")
				break loop
			case resp := <-respChan:
				log.Printf("response is Recive")
				log.Printf("%v vs %v", resp.StatusCode, sip.StatusOk)
				if resp.StatusCode == sip.StatusOk {
					log.Printf("response is 200 OK")
					ack, err := sip.GenerateAckFromRequestAndResponse(msg, resp)
					if ack != nil {
						log.Printf("invalid ACK Message: %v", err)
					}
					log.Printf("Sent ACK: %v", ack)
					srv.WriteMessage(ack)
					break loop
				} else {
					continue
				}
			}
		}
		log.Printf("Loop exited")
		time.Sleep(time.Second * 30)
		return
	}
}

func main() {
	sip.RecieveBufSizeB = 9000
	log.SetOutput(os.Stdout)
	sip.LogLevel = sip.LogDebug
	// sip.LogLevel = sip.LogInfo
	go func() {
		for {
			time.Sleep(time.Second * 5)
			stat.mu.Lock()
			log.Printf("Call completed: %v\n", stat.completed)
			for idx, val := range stat.completedPerResponse {
				if val == 0 {
					continue
				}
				log.Printf("Call completed[%03d]: %v\n", idx, val)
			}
			stat.mu.Unlock()
		}
	}()

	sip.HandleFunc(sip.LayerCore, "odd test", mySipCoreHandler)
	// server := &sip.Server{Addr: "127.0.0.1:5060"}
	server := &sip.Server{Addr: "localhost:5060"}
	go clientMain(server)
	server.ListenAndServe()
}
