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
				Scheme:       "sip",
				Host:         "atlanta.com",
				User:         url.User("alice"),
				RawParameter: "user=phone",
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
