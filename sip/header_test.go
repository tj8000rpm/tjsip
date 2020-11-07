package sip

import (
	"fmt"
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
