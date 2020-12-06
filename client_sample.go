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
	if response <= 0 || response >= 700 {
		return
	}
	stat.completed++
	stat.completedPerResponse[response] += 1
}

var stat = callStat{
	//completedPerResponse: make(map[int]int),
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
		_ = key
		return nil
	}
	return nil
}

func clientMain(srv *sip.Server) {
	time.Sleep(time.Second * 1)
	client := func() {
		//log.Printf("client main")
		msg := srv.CreateIniINVITE(
			"127.0.0.1:5062",
			"sip:term@127.0.0.1:5062",
		)
		msg.AddFromTag()

		//log.Printf("%v", msg)

		newTransaction := sip.NewClientInviteTransaction(srv, msg, 10)
		//log.Printf("Transaction: %v", newTransaction)
		err := srv.AddClientTransaction(newTransaction)
		if err != nil {
			srv.Warnf("%v", err)
			newTransaction.Destroy()
			return
		}
		//log.Printf("Sent INVITE\n")
		newTransaction.WriteMessage(msg)
	loop:
		for {
			resp := newTransaction.Response()
			if resp == nil {
				log.Printf("Transaction closed and exit loop")
				newTransaction.ResponseClose()
				break
			}
			switch resp.StatusCode {
			case sip.StatusOk:
				//log.Printf("response is 200 OK")
				ack, err := sip.GenerateAckFromRequestAndResponse(msg, resp)
				if err != nil {
					log.Printf("invalid ACK Message: %v", err)
				}
				//log.Printf("Sent ACK: %v", ack)
				srv.WriteMessage(ack)
				newTransaction.ResponseClose()
				break loop
			case sip.StatusMultipleChoices:
				//log.Printf("redirect to %v\n", resp.Contact)
				newTransaction.ResponseClose()
				break loop
			default:
				continue
			}
		}
		//log.Printf("Call Established")
		stat.Increment(200)
		time.Sleep(time.Second * 10)

		bye := sip.CreateBYE("127.0.0.1:5062", msg.RequestURI.String())
		bye.CSeq = msg.CSeq
		bye.CSeq.Increment()
		bye.CSeq.Method = "BYE"
		bye.From = msg.From
		bye.To = msg.To
		bye.MaxForwards = sip.NewMaxForwardsHeader()
		bye.CallID = msg.CallID
		bye.Via = msg.Via

		newTransaction = sip.NewClientNonInviteTransaction(srv, msg, 10)
		//log.Printf("Transaction: %v", newTransaction)
		err = srv.AddClientTransaction(newTransaction)
		if err != nil {
			srv.Warnf("%v", err)
			newTransaction.Destroy()
			return
		}
		//log.Printf("Sent BYE\n")
		newTransaction.WriteMessage(bye)
	loop2:
		for {
			resp := newTransaction.Response()
			if resp == nil {
				log.Printf("Transaction closed and exit loop")
				newTransaction.ResponseClose()
				break
			}
			switch resp.StatusCode {
			case sip.StatusOk:
				//log.Printf("response is 200 OK") //log.Printf("call was terminated")
				newTransaction.ResponseClose()
				break loop2
			default:
				continue
			}
		}
		return
	}
	for i := 0; i < 3000; i++ {
		go client()
		time.Sleep(time.Millisecond * 5)
	}
}

func main() {
	sip.RecieveBufSizeB = 9000
	log.SetOutput(os.Stdout)
	sip.LogLevel = sip.LogDebug
	sip.LogLevel = sip.LogInfo
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
	server := &sip.Server{Addr: "localhost:5060"}
	go clientMain(server)
	server.ListenAndServe()
}
