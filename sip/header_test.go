package sip

import (
	"fmt"
	"net/url"
	"testing"
)

func TestNameAddrString(t *testing.T) {
	// Alice <sip:alice@atlanta.com>
	uri, err := Parse("sip:alice@atlanta.com")
	if err != nil {
		t.Errorf("Unexpected error on test preparing")
	}
	nameAddr := &NameAddr{
		DisplayName: "Alice",
		Uri:         uri,
	}

	actual := fmt.Sprintf("%s", nameAddr)
	expect := "Alice <sip:alice@atlanta.com>"
	if actual != expect {
		t.Errorf("Not valid NameAddrString: expect %s, but given '%s'", expect, actual)
	}
}

func TestNameAddrStringWithoutDisplayname(t *testing.T) {
	// sip:alice@atlanta.com
	uri, err := Parse("sip:alice@atlanta.com")
	if err != nil {
		t.Errorf("Unexpected error on test preparing")
	}
	nameAddr := &NameAddr{
		Uri: uri,
	}

	actual := fmt.Sprintf("%s", nameAddr)
	expect := "sip:alice@atlanta.com"
	if actual != expect {
		t.Errorf("Not valid NameAddrString: expect %s, but given '%s'", expect, actual)
	}
}

func TestNameAddrStringQuoatedDisplayname(t *testing.T) {
	// "Mr. Watson" <sip:watson@worcester.bell-telephone.com>
	uri, err := Parse("sip:watson@worcester.bell-telephone.com")
	if err != nil {
		t.Errorf("Unexpected error on test preparing")
	}
	nameAddr := &NameAddr{
		DisplayName: "Mr. Watson",
		Uri:         uri,
	}

	actual := fmt.Sprintf("%s", nameAddr)
	expect := "\"Mr. Watson\" <sip:watson@worcester.bell-telephone.com>"
	if actual != expect {
		t.Errorf("Not valid NameAddrString: expect %s, but given '%s'", expect, actual)
	}
}

func TestNameAddrStringWithUriParam(t *testing.T) {
	// <sip:alice@atlanta.com;user=phone>
	uri, err := Parse("sip:alice@atlanta.com;user=phone")
	if err != nil {
		t.Errorf("Unexpected error on test preparing")
	}
	nameAddr := &NameAddr{
		Uri: uri,
	}

	actual := fmt.Sprintf("%s", nameAddr)
	expect := "<sip:alice@atlanta.com;user=phone>"
	if actual != expect {
		t.Errorf("Not valid NameAddrString: expect %s, but given '%s'", expect, actual)
	}
}

func TestNameAddrStringWithDisplaynameAnParam(t *testing.T) {
	// "Mr. Watson" <sip:alice@atlanta.com;user=phone>
	uri, err := Parse("sip:alice@atlanta.com;user=phone")
	if err != nil {
		t.Errorf("Unexpected error on test preparing")
	}
	nameAddr := &NameAddr{
		Uri:         uri,
		DisplayName: "Mr. Watson",
	}

	actual := fmt.Sprintf("%s", nameAddr)
	expect := "\"Mr. Watson\" <sip:alice@atlanta.com;user=phone>"
	if actual != expect {
		t.Errorf("Not valid NameAddrString: expect %s, but given '%s'", expect, actual)
	}
}

func TestNameAddrStringWithDisplaynameUTF8(t *testing.T) {
	// "Mr. Watson" <sip:alice@atlanta.com;user=phone>
	uri, err := Parse("sip:alice@atlanta.com;user=phone")
	if err != nil {
		t.Errorf("Unexpected error on test preparing")
	}
	nameAddr := &NameAddr{
		Uri:         uri,
		DisplayName: "こんにちわ",
	}

	actual := fmt.Sprintf("%s", nameAddr)
	expect := "\"こんにちわ\" <sip:alice@atlanta.com;user=phone>"
	if actual != expect {
		t.Errorf("Not valid NameAddrString: expect %s, but given '%s'", expect, actual)
	}
}

func TestNameAddrStringUserInfoSpecialChar(t *testing.T) {
	// "Mr. Watson" <sip:alice@atlanta.com;user=phone>
	uri, err := Parse("sip:alice;test@atlanta.com;user=phone")
	if err != nil {
		t.Errorf("Unexpected error on test preparing")
	}
	nameAddr := &NameAddr{
		Uri: uri,
	}

	actual := fmt.Sprintf("%s", nameAddr)
	expect := "<sip:alice;test@atlanta.com;user=phone>"
	if actual != expect {
		t.Errorf("Not valid NameAddrString: expect %s, but given '%s'", expect, actual)
	}

	uri, err = Parse("sip:+81312345678;npdi;rn=+8134512345@example.ne.jp;user=phone")
	if err != nil {
		t.Errorf("Unexpected error on test preparing")
	}
	nameAddr.Uri = uri
	actual = fmt.Sprintf("%s", nameAddr)
	expect = "<sip:+81312345678;npdi;rn=+8134512345@example.ne.jp;user=phone>"
	if actual != expect {
		t.Errorf("Not valid NameAddrString: expect %s, but given '%s'", expect, actual)
	}

}

func TestNewNameAddr(t *testing.T) {
	uri := &URI{
		Scheme:       "sip",
		Host:         "atlanta.com",
		User:         url.User("alice"),
		RawParameter: "user=phone",
	}

	c := NewNameAddr("Alice", uri)

	if actual, expect := c.DisplayName, "Alice"; actual != expect {
		t.Errorf("Not valid NameAddr Constractor: expect %s, but given '%s'", expect, actual)
	}
	if actual, expect := c.Uri.Scheme, "sip"; actual != expect {
		t.Errorf("Not valid NameAddr Constractor: expect %s, but given '%s'", expect, actual)
	}
	if actual, expect := c.Uri.Host, "atlanta.com"; actual != expect {
		t.Errorf("Not valid NameAddr Constractor: expect %s, but given '%s'", expect, actual)
	}
	if actual, expect := c.Uri.User.String(), "alice"; actual != expect {
		t.Errorf("Not valid NameAddr Constractor: expect %s, but given '%s'", expect, actual)
	}
	if actual, expect := c.Uri.RawParameter, "user=phone"; actual != expect {
		t.Errorf("Not valid NameAddr Constractor: expect %s, but given '%s'", expect, actual)
	}
}

func TestNewNameAddrUriString(t *testing.T) {
	c := NewNameAddrUriString("Alice", "sip:alice@atlanta.com;user=phone")
	if actual, expect := c.DisplayName, "Alice"; actual != expect {
		t.Errorf("Not valid NameAddr Constractor: expect %s, but given '%s'", expect, actual)
	}
	if actual, expect := c.Uri.Scheme, "sip"; actual != expect {
		t.Errorf("Not valid NameAddr Constractor: expect %s, but given '%s'", expect, actual)
	}
	if actual, expect := c.Uri.Host, "atlanta.com"; actual != expect {
		t.Errorf("Not valid NameAddr Constractor: expect %s, but given '%s'", expect, actual)
	}
	if actual, expect := c.Uri.User.String(), "alice"; actual != expect {
		t.Errorf("Not valid NameAddr Constractor: expect %s, but given '%s'", expect, actual)
	}
	if actual, expect := c.Uri.RawParameter, "user=phone"; actual != expect {
		t.Errorf("Not valid NameAddr Constractor: expect %s, but given '%s'", expect, actual)
	}
}

func TestNewNameAddrStringCaseOfUserinfoSpecialCharacter(t *testing.T) {
	c := NewNameAddrUriString("", "sip:alice;foo=bar@atlanta.com")
	if actual, expect := c.String(), "<sip:alice;foo=bar@atlanta.com>"; actual != expect {
		t.Errorf("Not valid String(): expect %s, but given '%s'", expect, actual)
	}
}

func subTestParseNameAddrString(t *testing.T, s, d, u, e string) {
	n, trail := ParseNameAddr(s)
	if n == nil {
		t.Errorf("Contact still nil")
		return
	}
	if actual, expect := n.DisplayName, d; actual != expect {
		t.Errorf("Not valid DisplayName: expect %s, but given '%s'", expect, actual)
	}
	if actual, expect := n.Uri.String(), u; actual != expect {
		t.Errorf("Not valid URI: expect %s, but given '%s'", expect, actual)
	}
	if actual, expect := trail, e; actual != expect {
		t.Errorf("Not valid trail value: expect %s, but given '%s'", expect, actual)
	}
}

func TestParseNameAddr(t *testing.T) {
	s := "Alice <sip:alice;isub=123@atlanta.com:5060;user=phone>;tag=123"
	subTestParseNameAddrString(t, s, "Alice", "sip:alice;isub=123@atlanta.com:5060;user=phone", ";tag=123")
	s = "\"Alice Alice\"<sip:alice;isub=123@atlanta.com:5060;user=phone>"
	subTestParseNameAddrString(t, s, "Alice Alice", "sip:alice;isub=123@atlanta.com:5060;user=phone", "")
	s = "sip:alice@atlanta.com:5060"
	subTestParseNameAddrString(t, s, "", "sip:alice@atlanta.com:5060", "")

	s = "invalid@@ string <!!;;>>>>>!<<!"
	subTestParseNameAddrString(t, s, "invalid@@ string", "!!;;", "!<<!")

	s = "invalid@@ string !!;;>>>>>!<<<!"
	n, _ := ParseNameAddr(s)
	if n != nil {
		t.Errorf("Name Addr will be nil")
	}
}

/**********************************
* To and From header
**********************************/
func TestToString(t *testing.T) {
	to := &To{
		Addr: &NameAddr{
			Uri: &URI{
				Scheme:       "sip",
				Host:         "atlanta.com",
				User:         url.User("alice"),
				RawParameter: "user=phone",
			},
			DisplayName: "Alice",
		},
		RawParameter: "tag=123123",
	}

	actual := fmt.Sprintf("%s", to)
	expect := "Alice <sip:alice@atlanta.com;user=phone>;tag=123123"
	if actual != expect {
		t.Errorf("Not valid ToString: expect %s, but given '%s'", expect, actual)
	}
}

func TestFromString(t *testing.T) {
	from := &From{
		Addr: &NameAddr{
			Uri: &URI{
				Scheme:       "sip",
				Host:         "atlanta.com",
				User:         url.User("alice"),
				RawParameter: "user=phone",
			},
			DisplayName: "Alice",
		},
		RawParameter: "tag=123123",
	}

	actual := fmt.Sprintf("%s", from)
	expect := "Alice <sip:alice@atlanta.com;user=phone>;tag=123123"
	if actual != expect {
		t.Errorf("Not valid FromString: expect %s, but given '%s'", expect, actual)
	}
}

func TestToParameter(t *testing.T) {
	to := &To{
		RawParameter: "tag=123123;key=value",
	}

	if actual, expect := to.Parameter().Get("tag"), "123123"; actual != expect {
		t.Errorf("Not valid Parameter(): expect %s, but given '%s'", expect, actual)
	}
	if actual, expect := to.Parameter().Get("key"), "value"; actual != expect {
		t.Errorf("Not valid Parameter(): expect %s, but given '%s'", expect, actual)
	}
}

func TestFromParameter(t *testing.T) {
	from := &From{
		RawParameter: "tag=123123;key=value",
	}

	if actual, expect := from.Parameter().Get("tag"), "123123"; actual != expect {
		t.Errorf("Not valid Parameter(): expect %s, but given '%s'", expect, actual)
	}
	if actual, expect := from.Parameter().Get("key"), "value"; actual != expect {
		t.Errorf("Not valid Parameter(): expect %s, but given '%s'", expect, actual)
	}
}

func TestToFromConstractor(t *testing.T) {
	addr := &NameAddr{
		Uri: &URI{
			Scheme:       "sip",
			Host:         "atlanta.com",
			User:         url.User("alice"),
			RawParameter: "user=phone",
		},
		DisplayName: "Alice",
	}
	param := "tag=123123"
	to := NewToHeader(addr, param)
	from := NewFromHeader(addr, param)
	var nilTo *To = nil
	var nilFrom *From = nil

	if actual, expect := to, nilTo; actual == expect {
		t.Errorf("Not valid Constractor: unexpected %v, but given '%v'", expect, actual)
	}
	if actual, expect := to.RawParameter, "tag=123123"; actual != expect {
		t.Errorf("Not valid Constractor unseted Params: expect %s, but given '%s'", expect, actual)
	}

	if actual, expect := from, nilFrom; actual == expect {
		t.Errorf("Not valid Constractor: unexpected %v, but given '%v'", expect, actual)
	}
	if actual, expect := from.RawParameter, "tag=123123"; actual != expect {
		t.Errorf("Not valid Constractor unseted Params: expect %s, but given '%s'", expect, actual)
	}
}

func TestToFromConstractorString(t *testing.T) {
	to := NewToHeaderFromString("Bob", "sip:bob@biloxi.com", "")
	from := NewFromHeaderFromString("Alice", "sip:alice@atlanta.com;user=phone", "tag=123123")
	var nilTo *To = nil
	var nilFrom *From = nil

	if actual, expect := to, nilTo; actual == expect {
		t.Errorf("Not valid Constractor: unexpected %v, but given '%v'", expect, actual)
	}
	if actual, expect := to.RawParameter, ""; actual != expect {
		t.Errorf("Not valid Constractor unseted Params: expect %s, but given '%s'", expect, actual)
	}

	if actual, expect := from, nilFrom; actual == expect {
		t.Errorf("Not valid Constractor: unexpected %v, but given '%v'", expect, actual)
	}
	if actual, expect := from.RawParameter, "tag=123123"; actual != expect {
		t.Errorf("Not valid Constractor unseted Params: expect %s, but given '%s'", expect, actual)
	}
}

/**********************************
* Via header
**********************************/
func TestViaString(t *testing.T) {
	v := &Via{
		SentProtocol: "SIP/2.0/UDP",
		SentBy:       "127.0.0.1",
		RawParameter: "maddr=127.0.0.1;branch=z9hG4bKnashds8",
	}

	if actual, expect := v.String(), "SIP/2.0/UDP 127.0.0.1;maddr=127.0.0.1;branch=z9hG4bKnashds8"; actual != expect {
		t.Errorf("Not valid Via String(): expect %s, but given '%s'", expect, actual)
	}
}

func TestViaProtocol(t *testing.T) {
	v := &Via{
		SentProtocol: "SIP/2.0/UDP",
	}

	name, major, minor, trans, err := v.Protocol()
	if err != nil {
		t.Errorf("Unexpected error on test preparing, caused by %v", err)
		return
	}

	if actual, expect := name, "sip"; actual != expect {
		t.Errorf("Not valid Via Protocol() Protocol name: expect %s, but given '%s'", expect, actual)
	}
	if actual, expect := major, 2; actual != expect {
		t.Errorf("Not valid Via Protocol() Major protocol verion: expect %d, but given '%d'", expect, actual)
	}
	if actual, expect := minor, 0; actual != expect {
		t.Errorf("Not valid Via Protocol() Minor protocol verion: expect %d, but given '%d'", expect, actual)
	}
	if actual, expect := trans, "udp"; actual != expect {
		t.Errorf("Not valid Via Protocol() transpor: expect %s, but given '%s'", expect, actual)
	}
}

func TestViaSetProtocol(t *testing.T) {
	v := &Via{}

	_, ok := v.SetProtocol("sip", 2, 0, "udp")
	if !ok {
		t.Errorf("Unexpected error on test preparing")
		return
	}

	if actual, expect := v.SentProtocol, "SIP/2.0/UDP"; actual != expect {
		t.Errorf("Not valid Via SetSentProtocol(): expect %s, but given '%s'", expect, actual)
	}
}

func TestViaConstractor(t *testing.T) {
	v := NewViaHeader("SIP/2.0/HTTP", "127.0.0.1", "key=value")

	if actual, expect := v.SentProtocol, "SIP/2.0/HTTP"; actual != expect {
		t.Errorf("Not valid Via Constractor: expect %s, but given '%s'", expect, actual)
	}
	if actual, expect := v.SentBy, "127.0.0.1"; actual != expect {
		t.Errorf("Not valid Via Constractor: expect %s, but given '%s'", expect, actual)
	}
	if actual, expect := v.RawParameter, "key=value"; actual != expect {
		t.Errorf("Not valid Via Constractor: expect %s, but given '%s'", expect, actual)
	}
}

func TestViaConstractorUDP(t *testing.T) {
	v := NewViaHeaderUDP("127.0.0.1", "key=value")
	if actual, expect := v.SentProtocol, "SIP/2.0/UDP"; actual != expect {
		t.Errorf("Not valid Via Constractor: expect %s, but given '%s'", expect, actual)
	}
}

func TestViaConstractorTCP(t *testing.T) {
	v := NewViaHeaderTCP("127.0.0.1", "key=value")
	if actual, expect := v.SentProtocol, "SIP/2.0/TCP"; actual != expect {
		t.Errorf("Not valid Via Constractor: expect %s, but given '%s'", expect, actual)
	}
}

func TestNewViaHeaders(t *testing.T) {
	v := NewViaHeaders()
	var viasNil *ViaHeaders = nil
	if actual, expect := v, viasNil; actual == expect {
		t.Errorf("Not valid ViaHeaders Constractor: unexpected %v, but given '%v'", expect, actual)
	}
	if actual, expect := len(v.Header), 0; actual != expect {
		t.Errorf("Not valid ViaHeaders Constractor: unexpected %v, but given '%v'", expect, actual)
	}
	if via := v.TopMost(); via != nil {
		t.Errorf("Invalid top most via")
	}
	if ok := v.Insert(NewViaHeaderUDP("127.0.0.1", "key=value")); !ok {
		t.Errorf("Via Header append Error")
	}
	if via := v.TopMost(); via.SentBy != "127.0.0.1" || via.RawParameter != "key=value" {
		t.Errorf("Invalid top most via")
	}
	if ok := v.Insert(NewViaHeaderUDP("10.0.0.1", "foo=bar")); !ok {
		t.Errorf("Via Header append Error")
	}
	if via := v.TopMost(); via.SentBy != "10.0.0.1" || via.RawParameter != "foo=bar" {
		t.Errorf("Invalid top most via")
	}
	if via := v.Get(1); via.SentBy != "127.0.0.1" || via.RawParameter != "key=value" {
		t.Errorf("Invalid get via")
	}
	if via := v.Get(2); via != nil {
		t.Errorf("Invalid get via")
	}
	if via := v.Get(-552); via != nil {
		t.Errorf("Invalid get via")
	}
	if via := v.Length(); via != 2 {
		t.Errorf("Invalid Length() via")
	}
	expect := "Via: SIP/2.0/UDP 10.0.0.1;foo=bar\r\nVia: SIP/2.0/UDP 127.0.0.1;key=value\r\n"
	actual := v.WriteHeader()
	if actual != expect {
		t.Errorf("Invalid WriteHeader func\n expeect-----\n '%v', but given ----\n '%v'", expect, actual)
	}
}

func TestParseVias(t *testing.T) {
	input1 := ("SIP/2.0/UDP   10.0.0.1:5060 ;foo=bar;test=\"123,123\", " +
		"SIP/2.0/UDP 192.168.0.1:sip;key2=value2   , " +
		"SIP/2.0/UDP 127.0.0.1;key=value,")
	input2 := ("SIP/2.0/UDP   172.17.0.1:5060 ;hoge=fuga")
	v := NewViaHeaders()
	err := ParseVias(input1, v)
	if err != nil {
		t.Errorf("Unexpected error on test preparing")
		return
	}
	err = ParseVias(input2, v)
	if err != nil {
		t.Errorf("Unexpected error on test preparing")
		return
	}
	if actual, expect := len(v.Header), 4; actual != expect {
		t.Errorf("Not valid Via Header length: expect %v, but given '%v'", expect, actual)
		return
	}
	if actual, expect := v.Header[3].SentBy, "10.0.0.1:5060"; actual != expect {
		t.Errorf("Not valid Via Header[0] Sent By: expect %s, but given '%s'", expect, actual)
	}
	if actual, expect := v.Header[3].RawParameter, "foo=bar;test=\"123,123\""; actual != expect {
		t.Errorf("Not valid Via Header[0] Raw Parameter: expect %s, but given '%s'", expect, actual)
	}
	if actual, expect := v.Header[2].SentBy, "192.168.0.1:sip"; actual != expect {
		t.Errorf("Not valid Via Header[1] Sent By: expect %s, but given '%s'", expect, actual)
	}
	if actual, expect := v.Header[2].RawParameter, "key2=value2"; actual != expect {
		t.Errorf("Not valid Via Header[1] Raw Parameter: expect %s, but given '%s'", expect, actual)
	}
	if actual, expect := v.Header[1].SentBy, "127.0.0.1"; actual != expect {
		t.Errorf("Not valid Via Header[2] Sent By: expect %s, but given '%s'", expect, actual)
	}
	if actual, expect := v.Header[1].RawParameter, "key=value"; actual != expect {
		t.Errorf("Not valid Via Header[2] Raw Parameter: expect %s, but given '%s'", expect, actual)
	}
	if actual, expect := v.Header[0].SentBy, "172.17.0.1:5060"; actual != expect {
		t.Errorf("Not valid Via Header[3] Sent By: expect %s, but given '%s'", expect, actual)
	}
	if actual, expect := v.Header[0].RawParameter, "hoge=fuga"; actual != expect {
		t.Errorf("Not valid Via Header[3] Raw Parameter: expect %s, but given '%s'", expect, actual)
	}
}

func TestParseViasMalformed(t *testing.T) {
	input := ("SIP/2P123,123\";key2=value2, 127.0.0.1;key=value")
	v := NewViaHeaders()
	err := ParseVias(input, v)
	if err != nil {
		t.Errorf("Unexpected error on test preparing")
		return
	}
	if actual, expect := len(v.Header), 1; actual != expect {
		t.Errorf("Not valid Via Header length: expect %v, but given '%v'", expect, actual)
		return
	}
	if actual, expect := v.Header[0].SentBy, "127.0.0.1"; actual != expect {
		t.Errorf("Not valid Via Header[0] Sent By: expect %s, but given '%s'", expect, actual)
	}
	if actual, expect := v.Header[0].RawParameter, "key=value"; actual != expect {
		t.Errorf("Not valid Via Header[0] Raw Parameter: expect %s, but given '%s'", expect, actual)
	}
}

/**********************************
* CSeq header
**********************************/
func TestCSeqString(t *testing.T) {
	c := &CSeq{
		Sequence: 1000,
		Method:   "INVITE",
	}

	if actual, expect := c.String(), "1000 INVITE"; actual != expect {
		t.Errorf("Not valid CSeq String(): expect %s, but given '%s'", expect, actual)
	}
}

func TestCSeqStringOverflow(t *testing.T) {
	c := &CSeq{
		Sequence: 2<<31 + 2,
		Method:   "INVITE",
	}

	if actual, expect := c.String(), "2 INVITE"; actual != expect {
		t.Errorf("Not valid CSeq String(): expect %s, but given '%s'", expect, actual)
	}
}

func TestCSeqIncrement(t *testing.T) {
	c := &CSeq{
		Sequence: 1000,
		Method:   "INVITE",
	}

	c.Increment()

	if actual, expect := c.Sequence, int64(1001); actual != expect {
		t.Errorf("Not valid CSeq Incrementation: expect %d, but given '%d'", expect, actual)
	}
}

func TestCSeqIncrementOverflow(t *testing.T) {
	c := &CSeq{
		Sequence: 2<<31 - 1,
		Method:   "INVITE",
	}

	c.Increment()

	if actual, expect := c.Sequence, int64(0); actual != expect {
		t.Errorf("Not valid CSeq Incrementation overflowed: expect %d, but given '%d'", expect, actual)
	}
}

func TestCSeqInit(t *testing.T) {
	c := &CSeq{
		Sequence: 1000,
		Method:   "INVITE",
	}

	if ok := c.Init(); !ok {
		t.Errorf("Unexpected error on test preparing")
	}

	if actual, expect := c.Sequence, int64(1000); actual == expect {
		t.Errorf("Not valid CSeq Incrementation: unexpected %d, but given '%d'", expect, actual)
	}
}

func TestCSeqConstractor(t *testing.T) {
	c := NewCSeqHeader("OPTIONS")
	if actual := c.Sequence; !(int64(0) <= actual && actual < int64(2<<31)) {
		t.Errorf("Not valid CSeq Constractor: unexpected range sequence must be 0 <= %d < 2**31", actual)
	}
	if actual, expect := c.Method, "OPTIONS"; actual != expect {
		t.Errorf("Not valid CSeq Constractor: unexpected %s, but given '%s'", expect, actual)
	}
}

/********************************
* CallID Header
********************************/
func TestCallIDString(t *testing.T) {
	c := &CallID{
		Identifier: "abc-def-ghi",
		Host:       "localhost:5060",
	}

	if actual, expect := c.String(), "abc-def-ghi@localhost:5060"; actual != expect {
		t.Errorf("Not valid CallID String(): expect %s, but given '%s'", expect, actual)
	}
}

func TestCallIDStringWithoutHost(t *testing.T) {
	c := &CallID{
		Identifier: "abc-def-ghi",
	}

	if actual, expect := c.String(), "abc-def-ghi"; actual != expect {
		t.Errorf("Not valid CallID String(): expect %s, but given '%s'", expect, actual)
	}
}

func TestCallIDInit(t *testing.T) {
	c := &CallID{
		Identifier: "abc-def-ghi",
		Host:       "localhost:5060",
	}

	if ok := c.Init(); !ok {
		t.Errorf("Unexpected error on test preparing")
	}

	if actual, expect := c.Identifier, "abc-def-ghi"; actual == expect {
		t.Errorf("Not valid CallID Init: unexpected %s, but given '%s'", expect, actual)
	}

	if actual, expect := c.Host, "localhost:5060"; actual != expect {
		t.Errorf("Not valid CallID Host through Init(): expect %s, but given '%s'", expect, actual)
	}
}

func TestCallIDInitH(t *testing.T) {
	c := &CallID{
		Identifier: "abc-def-ghi",
		Host:       "localhost:5060",
	}

	if ok := c.InitH("127.0.0.1"); !ok {
		t.Errorf("Unexpected error on test preparing")
	}

	if actual, expect := c.Identifier, "abc-def-ghi"; actual == expect {
		t.Errorf("Not valid CallID Init: unexpected %s, but given '%s'", expect, actual)
	}

	if actual, expect := c.Host, "127.0.0.1"; actual != expect {
		t.Errorf("Not valid CallID Host through Init(): expect %s, but given '%s'", expect, actual)
	}
}

func TestCallIDConstractor(t *testing.T) {
	c := NewCallIDHeaderWithAddr("127.0.0.1")
	if actual, expect := c.Host, "127.0.0.1"; actual != expect {
		t.Errorf("Not valid CallID Constractor: expected %s, but given '%s'", expect, actual)
	}
	if actual, expect := c.Identifier, ""; actual == expect {
		t.Errorf("Not valid CallID Constractor: unexpected %s, but given '%s'", expect, actual)
	}
}

/**********************************
* MaxForwards header
**********************************/
func TestMaxForwardsString(t *testing.T) {
	m := &MaxForwards{
		Remains: 1000,
	}

	if actual, expect := m.String(), "1000"; actual != expect {
		t.Errorf("Not valid MaxForwards String(): expect %s, but given '%s'", expect, actual)
	}
}

func TestMaxForwardsDecrement(t *testing.T) {
	m := &MaxForwards{
		Remains: 1000,
	}

	if ok := m.Decrement(); !ok {
		t.Errorf("the value could not decremented")
	}

	if actual, expect := m.Remains, 999; actual != expect {
		t.Errorf("Not valid MaxForwards decrementation: expect %d, but given '%d'", expect, actual)
	}
}

func TestMaxForwardsDecrementNotOK(t *testing.T) {
	m := &MaxForwards{
		Remains: 0,
	}

	if ok := m.Decrement(); ok {
		t.Errorf("un expected return")
	}

	if actual, expect := m.Remains, 0; actual != expect {
		t.Errorf("Not valid MaxForwards decrementation: expect %d, but given '%d'", expect, actual)
	}
}

func TestMaxForwardsConstractor(t *testing.T) {
	c := NewMaxForwardsHeader()
	if actual, expect := c.Remains, InitMaxForward; actual != expect {
		t.Errorf("Not valid MaxForwards Constractor: expected %v, but given '%v'", expect, actual)
	}
}

/********************************
* Contact Header
********************************/
func TestContactString(t *testing.T) {
	c := &Contact{
		Star: false,
		Addr: &NameAddr{
			Uri: &URI{
				Scheme:       "sip",
				Host:         "atlanta.com",
				User:         url.User("alice"),
				RawParameter: "user=phone",
			},
			DisplayName: "Alice",
		},
		RawParameter: "expires=3600",
	}

	if actual, expect := c.String(), "Alice <sip:alice@atlanta.com;user=phone>;expires=3600"; actual != expect {
		t.Errorf("Not valid Contact String(): expect %s, but given '%s'", expect, actual)
	}
}

func TestContactStringStar(t *testing.T) {
	c := &Contact{
		Star: true,
		Addr: &NameAddr{
			Uri: &URI{
				Scheme: "sip",
				Host:   "atlanta.com", User: url.User("alice"), RawParameter: "user=phone",
			},
			DisplayName: "Alice",
		},
		RawParameter: "expires=3600",
	}

	if actual, expect := c.String(), "*"; actual != expect {
		t.Errorf("Not valid Contact String(): expect %s, but given '%s'", expect, actual)
	}
}

func TestContactParameter(t *testing.T) {
	c := &Contact{
		Star: true,
		Addr: &NameAddr{
			Uri: &URI{
				Scheme:       "sip",
				Host:         "atlanta.com",
				User:         url.User("alice"),
				RawParameter: "user=phone",
			},
			DisplayName: "Alice",
		},
		RawParameter: "expires=3600;q=1.000",
	}

	if actual, expect := c.Parameter().Get("expires"), "3600"; actual != expect {
		t.Errorf("Not valid Parameter(): expect %s, but given '%s'", expect, actual)
	}
	if actual, expect := c.Parameter().Get("q"), "1.000"; actual != expect {
		t.Errorf("Not valid Parameter(): expect %s, but given '%s'", expect, actual)
	}
}

func TestNewContactHeader(t *testing.T) {
	uri := &URI{
		Scheme:       "sip",
		Host:         "atlanta.com",
		User:         url.User("alice"),
		RawParameter: "user=phone",
	}
	c := NewContactHeader("Alice", uri, ";expires=3600;q=1.000  ", false)
	if actual, expect := c.String(), "Alice <sip:alice@atlanta.com;user=phone>;expires=3600;q=1.000"; actual != expect {
		t.Errorf("Not valid Contact String(): expect %s, but given '%s'", expect, actual)
	}
}

func TestNewContactHeaderFrmString(t *testing.T) {
	c := NewContactHeaderFromString("Alice", "sip:alice@atlanta.com;user=phone", "expires=3600;q=1.000")
	if actual, expect := c.String(), "Alice <sip:alice@atlanta.com;user=phone>;expires=3600;q=1.000"; actual != expect {
		t.Errorf("Not valid Contact String(): expect %s, but given '%s'", expect, actual)
	}
}

func TestNewContactHeaderFrmStringStar(t *testing.T) {
	c := NewContactHeaderFromString("Alice", "*", "expires=3600;q=1.000")
	if actual, expect := c.String(), "*"; actual != expect {
		t.Errorf("Not valid Contact String(): expect %s, but given '%s'", expect, actual)
	}
	c = NewContactHeaderFromString("Alice", " * ", "expires=3600;q=1.000")
	if actual, expect := c.String(), "*"; actual != expect {
		t.Errorf("Not valid Contact String(): expect %s, but given '%s'", expect, actual)
	}
}

func TestNewContactHeaderStar(t *testing.T) {
	c := NewContactHeaderStar()
	if actual, expect := c.String(), "*"; actual != expect {
		t.Errorf("Not valid Contact String(): expect %s, but given '%s'", expect, actual)
	}
}

func subTestParseContactString(t *testing.T, s, d, p string) {
	c := ParseContact(s)
	if c == nil {
		t.Errorf("Contact still nil")
		return
	}
	if actual, expect := c.Star, false; actual != expect {
		t.Errorf("Not valid Contact Star: expect %v, but given '%v'", expect, actual)
	}
	if actual, expect := c.Addr.DisplayName, d; actual != expect {
		t.Errorf("Not valid Contact DisplayName: expect %s, but given '%s'", expect, actual)
	}
	if actual, expect := c.RawParameter, p; actual != expect {
		t.Errorf("Not valid Contact RawParameter: expect %s, but given '%s'", expect, actual)
	}
}

func TestParseContact(t *testing.T) {
	s := "Alice <sip:alice;isub=123@atlanta.com:5060;usre=phone>;expires=3600;q=1.000"
	subTestParseContactString(t, s, "Alice", "expires=3600;q=1.000")
	s = "\"Alice Alice\"<sip:alice;isub=123@atlanta.com:5060;usre=phone>"
	subTestParseContactString(t, s, "Alice Alice", "")
	s = "sip:alice@atlanta.com:5060"
	subTestParseContactString(t, s, "", "")
	c := ParseContact("*")
	if actual, expect := c.Star, true; actual != expect {
		t.Errorf("Not valid Contact Star: expect %v, but given '%v'", expect, actual)
	}
}

func TestParseContacts(t *testing.T) {
	c := NewContactHeaders()
	s := ("\"Mr. Watson\" <sip:watson@worcester.bell-telephone.com> ;q=0.7; " +
		"expires=3600, \"Mr. Watson\" <mailto:watson@bell-telephone.com> ;q=0.1")
	ParseContacts(s, c)
	if c == nil {
		t.Errorf("Contacts still nil")
		return
	}
	if len(c.Header) != 2 {
		t.Errorf("Contacts length is not valid")
		return
	}
	sipc := c.Header[0]
	if actual, expect := sipc.Addr.DisplayName, "Mr. Watson"; actual != expect {
		t.Errorf("Not valid Contact DisplayName: expect %v, but given '%v'", expect, actual)
	}
	if actual, expect := sipc.Addr.Uri.String(), "sip:watson@worcester.bell-telephone.com"; actual != expect {
		t.Errorf("Not valid Contact URI: expect %v, but given '%v'", expect, actual)
	}
	if actual, expect := sipc.RawParameter, "q=0.7; expires=3600"; actual != expect {
		t.Errorf("Not valid Contact RawParameter: expect %v, but given '%v'", expect, actual)
	}
	mailc := c.Header[1]
	if actual, expect := mailc.Addr.DisplayName, "Mr. Watson"; actual != expect {
		t.Errorf("Not valid Contact DisplayName: expect %v, but given '%v'", expect, actual)
	}
	if actual, expect := mailc.Addr.Uri.String(), "mailto:watson@bell-telephone.com"; actual != expect {
		t.Errorf("Not valid Contact URI: expect %v, but given '%v'", expect, actual)
	}
	if actual, expect := mailc.RawParameter, "q=0.1"; actual != expect {
		t.Errorf("Not valid Contact RawParameter: expect %v, but given '%v'", expect, actual)
	}
}

func TestParseContactsOnlyOneEntry(t *testing.T) {
	c := NewContactHeaders()
	s := ("\"Mr. Watson\" <sip:watson@worcester.bell-telephone.com> ;q=0.7; expires=3600")
	ParseContacts(s, c)
	if c == nil {
		t.Errorf("Contacts still nil")
		return
	}
	if len(c.Header) != 1 {
		t.Errorf("Contacts length is not valid")
		return
	}
	sipc := c.Header[0]
	if actual, expect := sipc.Addr.DisplayName, "Mr. Watson"; actual != expect {
		t.Errorf("Not valid Contact DisplayName: expect %v, but given '%v'", expect, actual)
	}
	if actual, expect := sipc.Addr.Uri.String(), "sip:watson@worcester.bell-telephone.com"; actual != expect {
		t.Errorf("Not valid Contact URI: expect %v, but given '%v'", expect, actual)
	}
	if actual, expect := sipc.RawParameter, "q=0.7; expires=3600"; actual != expect {
		t.Errorf("Not valid Contact RawParameter: expect %v, but given '%v'", expect, actual)
	}
}

func TestNewContactHeaders(t *testing.T) {
	c := NewContactHeaders()
	if actual := c; actual == nil {
		t.Errorf("ContactHeaders still nil")
		return
	}
	if actual := c.Header; actual == nil {
		t.Errorf("ContactHeaders Header still nil")
	}
	if actual, expect := len(c.Header), 0; actual != expect {
		t.Errorf("ContactHeaders Header length is not 0")
	}
	if ok := c.Add(NewContactHeaderStar()); !ok {
		t.Errorf("ContactHeaders Header Add fail")
	}
	if actual, expect := len(c.Header), 1; actual != expect {
		t.Errorf("ContactHeaders Header length is not 1")
	}
}
