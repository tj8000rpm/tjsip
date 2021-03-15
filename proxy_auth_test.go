package main

import (
	"crypto/md5"
	"encoding/hex"
	"net/http"
	"sip/sip"
	"testing"
)

func TestParseAuthorizationHeaderDigest(t *testing.T) {
	s := "Digest username=\"hoge, hige\", Realm=\"example.com\", " +
		"nonce=\"60b725f10c9c85c70d97880dfe8191b3\", " +
		"digest-uri=\"sip:example.com:5060\", " +
		"response=\"60b725f10c9c85c70d97880dfe8191b3\""
	res := ParseAuthorizationHeadlerDigest(s)
	if actual, expect := res["username"][0], "hoge, hige"; actual != expect {
		t.Errorf("expect %v: but '%v'", expect, actual)
	}
	if actual, expect := res["realm"][0], "example.com"; actual != expect {
		t.Errorf("expect %v: but '%v'", expect, actual)
	}
	if actual, expect := res["nonce"][0], "60b725f10c9c85c70d97880dfe8191b3"; actual != expect {
		t.Errorf("expect %v: but '%v'", expect, actual)
	}
	if actual, expect := res["digest-uri"][0], "sip:example.com:5060"; actual != expect {
		t.Errorf("expect %v: but '%v'", expect, actual)
	}
	if actual, expect := res["response"][0], "60b725f10c9c85c70d97880dfe8191b3"; actual != expect {
		t.Errorf("expect %v: but '%v'", expect, actual)
	}
}

func TestAuth(t *testing.T) {
	md5sum := md5.Sum([]byte("hoge, hige:example.com:password"))
	md5sumStr := hex.EncodeToString(md5sum[:])
	s := "Digest username=\"hoge, hige\", Realm=\"example.com\", " +
		"nonce=\"60b725f10c9c85c70d97880dfe8191b3\", " +
		"digest-uri=\"sip:example.com:5060\", " +
		"response=\"6f9fb293d6aec76321219a4beb9aee8d\""
	res := ParseAuthorizationHeadlerDigest(s)
	ok, status := auth("INVITE", res["nonce"][0],
		res["digest-uri"][0], res["response"][0], md5sumStr)
	if !ok {
		t.Errorf("expect %v: but '%v'", true, ok)
	}
	if status != 0 {
		t.Errorf("expect %v: but '%v'", 0, status)
	}
}

func TestAuthMessage(t *testing.T) {
	s := "Digest username=\"hoge, hige\", Realm=\"example.com\", " +
		"nonce=\"60b725f10c9c85c70d97880dfe8191b3\", " +
		"digest-uri=\"sip:example.com:5060\", " +
		"response=\"6f9fb293d6aec76321219a4beb9aee8d\""
	msg := &sip.Message{
		Method: sip.MethodINVITE,
		Header: make(http.Header),
	}
	lookupFunc := func(u, r string) string {
		md5sum := md5.Sum([]byte("hoge, hige:example.com:password"))
		return hex.EncodeToString(md5sum[:])
	}
	msg.Header.Add("Proxy-Authorization", s)
	ok, status := authMessage("Proxy-Authorization", msg, lookupFunc)
	if !ok {
		t.Errorf("expect %v: but '%v'", true, ok)
	}
	if status != 0 {
		t.Errorf("expect %v: but '%v'", 0, status)
	}
}
