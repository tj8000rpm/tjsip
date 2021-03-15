package main

import (
	"crypto/md5"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"net/http"
	"os"
	"sip/sip"
	"strings"
	"sync"
	"time"
)

type AuthRecord struct {
	username       string
	realm          string
	nonce          string
	a1md5          string
	nonceExpiredAt int64
}

func (a *AuthRecord) Expired(now int64) bool {
	return now > a.nonceExpiredAt
}

func (a *AuthRecord) A1md5() string {
	return a.a1md5
}

var authenticater *AuthController

type AuthController struct {
	mu sync.Mutex
	db *sql.DB
}

func NewAuthController() *AuthController {
	sqlitePath := "/dev/shm/tj-sip-auth.sqlite"

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

	auth := &AuthController{
		db: db,
	}
	createTable := `
		CREATE TABLE authenticate (
			username VARCHAR(255),
			realm VARCHAR(255),
			nonce VARCHAR(255),
			a1md5 VARCHAR(255),
			nonceExpiredAt INTEGER);
		`

	_, err = db.Exec(createTable)
	if err != nil {
		log.Printf("db create error")
		return nil
	}

	return auth
}

func (a *AuthController) Begin() (*sql.Tx, error) {
	return a.db.Begin()
}

func (a *AuthController) DB() *sql.DB {
	return a.db
}

func queryUser(username, realm string) *AuthRecord {
	db := authenticater.DB()
	query := "SELECT nonce, a1md5, nonceExpiredAt FROM authenticate " +
		"WHERE username = ? AND realm = ?"
	row := db.QueryRow(query, username, realm)
	var nonce, a1md5 string
	var nonceExpiredAt int64
	err := row.Scan(&nonce, &a1md5, &nonceExpiredAt)
	if err != nil {
		return nil
	}

	result := &AuthRecord{
		username:       username,
		realm:          realm,
		nonce:          nonce,
		a1md5:          a1md5,
		nonceExpiredAt: nonceExpiredAt,
	}

	return result
}

func queryA1md5(username, realm string) string {
	now := time.Now().Unix()
	res := queryUser(username, realm)
	if res == nil || res.Expired(now) {
		return ""
	}
	return res.A1md5()
}

func generateAuthRequireHeader(header, username, realm string) *http.Header {
	nonce, expire := generateNonce(username, realm)
	dbTxn, err := authenticater.Begin()
	if err != nil {
		dbTxn.Rollback()
		return nil
	}
	_, err = dbTxn.Exec("UPDATE authenticate "+
		"SET nonce = ?, onceExpiredAt = ? WHERE usernames = ? AND realm = ?",
		nonce, expire, username, realm)
	if err != nil {
		dbTxn.Rollback()
		return nil
	}

	ret := new(http.Header)
	ret.Add(header, fmt.Sprintf("DIGEST realm=\"%s\", nonce=\"%s\", stale=false, algorithm=MD5", realm, nonce))
	return ret

}

func generateNonce(username, realm string) (string, int64) {
	now := time.Now().Unix()
	generatedRandomString, err := sip.GenerateRandomString(16)
	if err != nil {
		return "", 0
	}
	noncemd5 := md5.Sum([]byte(generatedRandomString))
	return hex.EncodeToString(noncemd5[:]), now + 60
}

func importSubscriber(filepath string) bool {
	if filepath == "" {
		return false
	}
	fp, err := os.Open(filepath)
	if err != nil {
		panic(err)
	}
	defer fp.Close()

	reader := csv.NewReader(fp)
	var line []string

	authenticater.mu.Lock()
	defer authenticater.mu.Unlock()

	dbTxn, err := authenticater.Begin()
	if err != nil {
		return false
	}
	defer func() {
		if err := recover(); err != nil {
			dbTxn.Rollback()
		}
	}()

	for {
		line, err = reader.Read()
		if err != nil {
			break
		}
		if len(line) != 3 {
			log.Printf("invalid file format\n")
			dbTxn.Rollback()
			return false
		}
		username := line[0]
		password := line[1]
		realm := line[2]
		if username == "" && password == "" || realm == "" {
			log.Printf("missing mandantory value")
			dbTxn.Rollback()
			return false
		}
		a1md5 := calcA1md5(username, realm, password)
		_, err = dbTxn.Exec("INSERT INTO authenticate "+
			"(username, realm, nonce, a1md5, nonceExpiredAt) VALUES (?, ?, ?, ?, ?)",
			username, realm, "", a1md5, 0)
		if err != nil {
			log.Printf("db insertion error: %v(%v)", username, err)
			dbTxn.Rollback()
			return false
		}
	}
	err = dbTxn.Commit()
	if err != nil {
		log.Printf("err: %v", err)
		dbTxn.Rollback()
		return false
	}

	return true
}

func ParseAuthorizationHeadlerDigest(s string) map[string][]string {
	blk := ", \t\r\n"
	s = strings.Trim(s, blk)

	splitS := strings.SplitN(s, " ", 2)
	front, rear := splitS[0], splitS[1]

	result := make(map[string][]string)

	if strings.ToLower(front) != "digest" {
		return nil
	}

	cut := 0
	quoted := false
	for i, c := range rear {
		if i+1 == len(rear) || c == ',' {
			if quoted && !(quoted && i+1 == len(rear) && c == '"') {
				continue
			}
			param := strings.Trim(rear[cut:i], blk)
			kv := strings.SplitN(param, "=", 2)
			key := strings.ToLower(kv[0])
			val := strings.Trim(kv[1], "\"")
			list, ok := result[key]
			if !ok {
				list = make([]string, 0)
				result[key] = list
			}
			result[key] = append(result[key], val)

			cut = i
		} else if c == '"' {
			quoted = !quoted
		}
	}
	return result
}

func calcA1md5(username, realm, password string) string {
	a1 := md5.Sum([]byte(username + ":" + realm + ":" + password))
	return hex.EncodeToString(a1[:])
}

func auth(method, nonce, digestUri, response, a1 string) (ok bool, status int) {
	a2 := md5.Sum([]byte(method + ":" + digestUri))
	tmp := a1 + ":" + nonce + ":" + hex.EncodeToString(a2[:])
	hexExpect := md5.Sum([]byte(tmp))
	expect := hex.EncodeToString(hexExpect[:])
	if response == expect {
		return true, 0
	}
	return false, sip.StatusUnauthorized
}

func authMessage(header string, msg *sip.Message,
	lookupFunc func(string, string) string) (ok bool, status int) {
	// header "Proxy-Authorization" or "Authorization"
	UnauthErrState := sip.StatusUnauthorized
	if header == "Proxy-Authorization" {
		UnauthErrState = sip.StatusProxyAuthenticationRequired
	}
	var authorization string
	if msg.Header == nil {
		return false, UnauthErrState
	}
	authorization = msg.Header.Get(header)
	if authorization == "" {
		return false, UnauthErrState
	}
	authParam := ParseAuthorizationHeadlerDigest(authorization)
	username := authParam["username"]
	realm := authParam["realm"]
	nonce := authParam["nonce"]
	digestUri := authParam["digest-uri"]
	response := authParam["response"]
	if len(username) != 1 || len(realm) != 1 || len(nonce) != 1 ||
		len(digestUri) != 1 || len(response) != 1 || msg.Method == "" {
		return false, sip.StatusBadRequest
	}
	a1 := lookupFunc(username[0], realm[0])
	ok, status = auth(msg.Method, nonce[0], digestUri[0], response[0], a1)
	if status == sip.StatusUnauthorized && header == "Proxy-Authorization" {
		status = sip.StatusProxyAuthenticationRequired
	}
	return ok, status
}
