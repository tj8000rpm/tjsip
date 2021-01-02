package main

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"net/http"
	"os"
	"sip/sip"
	"strconv"
	"strings"
	"sync"
	"time"
)

var MinimumRegisterExpiresValue = 90

const (
	REGISTRATION_QUERY = iota
	REGISTRATION_UPDATE
	REGISTRATION_DEL
	REGISTRATION_DELALL
)

type RegistrationResult struct {
	Status  int
	Contact *sip.ContactHeaders
	Header  http.Header
}

type TableReigster struct {
	Aor       string
	Bind      string
	Q         int // float value is store as int like 1.00=100 ~ 0.00=0
	ExpiredAt int64
}

func NewRegistrationResult(status int) *RegistrationResult {
	r := new(RegistrationResult)
	r.Status = status
	return r
}

type RegistrationOperation struct {
	Operation int
	BindAddr  *sip.URI
	Expires   int
	Q         int
}

type RegisterController struct {
	mu sync.Mutex
	db *sql.DB
}

func NewRegiserController() *RegisterController {
	sqlitePath := "/dev/shm/tj-sip-reg.sqlite"

	_, err := os.Stat(sqlitePath)
	if err == nil {
		err = os.Remove(sqlitePath)
		if err != nil {
			log.Printf("file remvoe error")
			return nil
		}
	}

	db, err := sql.Open("sqlite3", sqlitePath)
	if err != nil {
		log.Printf("SQL open error")
		return nil
	}

	reg := &RegisterController{
		db: db,
	}
	createTable := `
		CREATE TABLE register_seq (
			aor VARCHAR(255) PRIMARY KEY,
			callId VARCHAR(255),
			seq INTEGER);
		CREATE TABLE register (
			aor VARCHAR(255),
			bind VARCHAR(255),
			q INTEGER,
			expired_at INTEGER);
		`

	_, err = db.Exec(createTable)
	if err != nil {
		log.Printf("db create error")
		return nil
	}

	return reg
}

func (r *RegisterController) Begin() (*sql.Tx, error) {
	return r.db.Begin()
}

func (r *RegisterController) DB() *sql.DB {
	return r.db
}

func NewRegistrationOperation() *RegistrationOperation {
	return new(RegistrationOperation)
}

func queryBindingNow(dbTxn *sql.Tx, db *sql.DB, aor *sip.URI) (res []*TableReigster, err error) {
	return queryBinding(dbTxn, db, aor, time.Now().Unix())
}

func queryBinding(dbTxn *sql.Tx, db *sql.DB, aor *sip.URI, now int64) (res []*TableReigster, err error) {
	var rows *sql.Rows
	query := "SELECT bind, q, expired_at FROM register " +
		"WHERE aor = ? AND expired_at >= ? ORDER BY q DESC"
	if dbTxn != nil {
		rows, err = dbTxn.Query(query, aor.String(), now)
	} else if db != nil {
		rows, err = db.Query(query, aor.String(), now)
	} else {
		return nil, fmt.Errorf("Either *sip.Tx or *sip.DB must not be nil")
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res = make([]*TableReigster, 0)

	for rows.Next() {
		bind := new(TableReigster)
		bind.Aor = aor.String()
		if err = rows.Scan(&bind.Bind, &bind.Q, &bind.ExpiredAt); err != nil {
			return nil, err
		}
		res = append(res, bind)
	}
	return res, nil
}

func issueTransaction(aor *sip.URI, operations []*RegistrationOperation,
	callId string, cseqNum int64) (*RegistrationResult, error) {

	dbTxn, err := register.Begin()
	if err != nil {
		log.Printf("err: %v", err)
		return nil, err
	}
	defer func() {
		if err := recover(); err != nil {
			dbTxn.Rollback()
		}
	}()

	var row *sql.Row

	var dbAor, dbCallId string
	var dbSeq int64
	row = dbTxn.QueryRow("SELECT aor, callId, seq FROM register_seq WHERE aor = ?", aor.String())
	if err != nil {
		log.Printf("err: %v", err)
		return nil, err
	}
	row.Scan(&dbAor, &dbCallId, &dbSeq)
	if dbAor != "" {
		log.Printf("%s, %s, %d", dbAor, dbCallId, dbSeq)
		if len(operations) > 1 && dbCallId == callId && dbSeq <= cseqNum {
			dbTxn.Rollback()
			log.Printf("CSeq and CallID check fail")
			return NewRegistrationResult(sip.StatusBadRequest), nil
		}
	}

	_, err = dbTxn.Exec("DELETE FROM register_seq WHERE aor = ?", aor.String())
	_, err = dbTxn.Exec("INSERT INTO register_seq (aor,callId, seq) VALUES (?, ?, ?)",
		aor.String(), callId, cseqNum)
	if err != nil {
		dbTxn.Rollback()
		return nil, err
	}

	var contacts *sip.ContactHeaders

	now := time.Now().Unix()
	for _, op := range operations {
		switch op.Operation {
		case REGISTRATION_QUERY:
			contacts = sip.NewContactHeaders()
			result, err := queryBinding(dbTxn, nil, aor, now)
			if err != nil {
				dbTxn.Rollback()
				return nil, err
			}
			for _, bind := range result {
				rawParam := fmt.Sprintf("q=%.2f;expires=%d", float64(bind.Q)/100, bind.ExpiredAt-now)
				contacts.Add(sip.NewContactHeaderFromString("", bind.Bind, rawParam))
			}
		case REGISTRATION_UPDATE:
			_, err = dbTxn.Exec("DELETE FROM register WHERE aor = ? AND bind = ?",
				aor.String(), op.BindAddr.String())
			_, err = dbTxn.Exec("INSERT INTO register (aor, bind, q, expired_at)"+
				" VALUES (?, ?, ?, ?)",
				aor.String(), op.BindAddr.String(), op.Q, int64(op.Expires)+now)
			if err != nil {
				dbTxn.Rollback()
				return nil, err
			}

		case REGISTRATION_DEL:
			_, err = dbTxn.Exec("DELETE FROM register WHERE aor = ? AND bind = ?",
				aor.String(), op.BindAddr.String())
			if err != nil {
				dbTxn.Rollback()
				return nil, err
			}
		case REGISTRATION_DELALL:
			_, err = dbTxn.Exec("DELETE FROM register WHERE aor = ?", aor.String())
			if err != nil {
				dbTxn.Rollback()
				return nil, err
			}
		}
	}
	err = dbTxn.Commit()
	if err != nil {
		log.Printf("err: %v", err)
		return nil, err
	}
	result := NewRegistrationResult(sip.StatusOk)
	result.Contact = contacts
	return result, nil
}

func determOperation(contact *sip.Contact, expires int, okE bool,
	bindAddr *sip.URI) (*RegistrationOperation, int) {

	if contact == nil {
		return nil, sip.StatusBadRequest
	}
	operation := NewRegistrationOperation()
	if contact.Star {
		if expires != 0 {
			return nil, sip.StatusBadRequest
		}
		operation.Operation = REGISTRATION_DELALL
		return operation, 0
	}
	if contact.Addr == nil || contact.Addr.Uri == nil {
		return nil, sip.StatusBadRequest
	}
	var qFloat float64
	var q, pExpires int
	var err error
	qStr := contact.Parameter().Get("q")
	okQ := qStr != ""
	if okQ && err != nil {
		return nil, sip.StatusBadRequest
	}
	qFloat, err = strconv.ParseFloat(qStr, 64)
	if err != nil {
		qFloat = 0.0
	}
	q = int(qFloat * 100)
	if q > 1 || q < 0 {
		return nil, sip.StatusBadRequest
	}
	operation.BindAddr = bindAddr

	pExpiresStr := contact.Parameter().Get("expires")
	okPE := pExpiresStr != ""
	pExpires, err = strconv.Atoi(pExpiresStr)
	if okPE && err != nil {
		return nil, sip.StatusBadRequest
	}
	if (okPE && pExpires == 0) || (okE && expires == 0 && !okPE) {
		operation.Operation = REGISTRATION_DEL
		return operation, 0
	}
	expectExpires := MinimumRegisterExpiresValue
	if okE {
		expectExpires = expires
	} else if okPE {
		expectExpires = pExpires
	}
	if expectExpires < MinimumRegisterExpiresValue {
		return nil, sip.StatusIntervalTooBrief
	}
	operation.Operation = REGISTRATION_UPDATE
	operation.Q = q
	operation.Expires = expectExpires
	return operation, 0
}

func registration(msg *sip.Message) *RegistrationResult {
	origContacts := msg.Contact
	var origToAOR *sip.URI
	if msg.To != nil && msg.To.Addr != nil {
		origToAOR = msg.To.Addr.Uri
	}

	expiresStr := msg.Header.Get("expires")
	okE := (expiresStr != "")
	expires, err := strconv.Atoi(expiresStr)
	if okE && err != nil {
		return NewRegistrationResult(sip.StatusBadRequest)
	}
	origCallID := msg.CallID
	origCSeq := msg.CSeq

	var contacts []*sip.Contact

	queryLength := 1
	if origContacts != nil {
		contacts = origContacts.Header
		queryLength += len(origContacts.Header)
	}

	operations := make([]*RegistrationOperation, queryLength)

	for idx, contact := range contacts {
		var status int
		var bindAddr *sip.URI
		if contact.Addr != nil {
			bindAddr = contact.Addr.Uri
		}
		operations[idx], status = determOperation(contact, expires, okE, bindAddr)
		if status != 0 {
			return NewRegistrationResult(status)
		}
	}
	r := NewRegistrationOperation()
	r.Operation = REGISTRATION_QUERY
	operations[len(operations)-1] = r

	result, err := issueTransaction(origToAOR, operations, origCallID.String(), origCSeq.Sequence)
	if err != nil {
		if strings.Contains(err.Error(), "syntax error") {
			return NewRegistrationResult(sip.StatusInternalServerError)
		}
		return NewRegistrationResult(sip.StatusBadRequest)
	}
	return result
}
