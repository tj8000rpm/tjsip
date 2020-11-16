package sip

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"testing"
)

func helperWriteHeader(msg *Message) string {
	msg.To = NewToHeaderFromString("Bob", "sip:bob@biloxi.com", "tag=321321")
	msg.From = NewFromHeaderFromString("Alice", "sip:alice@atlanta.com;user=phone", "tag=123123")
	msg.Via = NewViaHeaders()
	msg.Via.Insert(NewViaHeaderUDP("10.0.0.1:5060", "branch=z9hG4bKnashds8"))
	msg.Via.Insert(NewViaHeaderUDP("10.0.1.1", "branch=z9hG4bKaaaaaaa"))
	msg.MaxForwards = NewMaxForwardsHeader()
	msg.CallID = NewCallIDHeaderWithAddr("pc33.atlanta.com:5060")
	msg.CallID.Identifier = "123123"
	msg.CSeq = NewCSeqHeader("INVITE")
	msg.CSeq.Sequence = 123123
	msg.Contact = NewContactHeaders()
	msg.Contact.Add(NewContactHeaderFromString("", "sip:alice@pc33.atlanta.com:5060;user=phone", ""))
	msg.Contact.Add(NewContactHeaderStar())
	expect := "" +
		"To: Bob <sip:bob@biloxi.com>;tag=321321\r\n" +
		"From: Alice <sip:alice@atlanta.com;user=phone>;tag=123123\r\n" +
		"Via: SIP/2.0/UDP 10.0.1.1;branch=z9hG4bKaaaaaaa\r\n" +
		"Via: SIP/2.0/UDP 10.0.0.1:5060;branch=z9hG4bKnashds8\r\n" +
		"Max-Forwards: " + strconv.Itoa(InitMaxForward) + "\r\n" +
		"Call-ID: 123123@pc33.atlanta.com:5060\r\n" +
		"CSeq: 123123 INVITE\r\n" +
		"Contact: <sip:alice@pc33.atlanta.com:5060;user=phone>\r\n" +
		"Contact: *\r\n" +
		""
	return expect
}

func helperEvaluateLongString(a, b string, t *testing.T) {
	if a != b {
		t.Errorf("Unexpected SIP Message:\n"+
			" expect \n"+"---------------\n"+"%s\n"+
			"------------\n"+
			"\n"+
			"but given \n"+
			"--------------------\n"+
			"%s\n"+
			"------------------", b, a)
		for i := 0; i < len(b) && i < len(a); i++ {
			if b[i] != a[i] {
				t.Errorf("difference char index=%d\nHere:\n%s*", i, a[:i])
				break
			}
		}
	}
}

func TestMessageWriteResponse(t *testing.T) {
	out := new(bytes.Buffer)
	msg := new(Message)
	msg.Response = true
	msg.ProtoMajor = 2
	msg.ProtoMinor = 0
	msg.ReasonPhrase = "OK"
	msg.StatusCode = 200
	msg.Header = make(http.Header)
	msg.Header.Set("to", "aaaaaa")
	msg.Header.Set("from", "bbbbbb")

	msg.To = NewToHeaderFromString("Bob", "sip:bob@biloxi.com", "tag=321321")
	if msg.To == nil {
		t.Errorf("Header still nil")
	}
	msg.From = NewFromHeaderFromString("Alice", "sip:alice@atlanta.com;user=phone", "tag=123123")
	if msg.From == nil {
		t.Errorf("Header still nil")
	}
	headerStr := helperWriteHeader(msg)
	msg.Write(out)
	expect := "" +
		"SIP/2.0 200 OK\r\n" +
		headerStr +
		"\r\n"
	helperEvaluateLongString(out.String(), expect, t)
}

func TestMessageWriteRequest(t *testing.T) {
	out := new(bytes.Buffer)
	msg := new(Message)
	var err error
	msg.Request = true
	msg.ProtoMajor = 2
	msg.ProtoMinor = 0
	msg.RequestURI, err = Parse("sip:alice@atlanta.com:5060")
	if err != nil {
		t.Errorf("Unexpected error on test preparing")
	}
	msg.Method = "INVITE"
	msg.Header = make(http.Header)
	msg.Header.Set("to", "aaaaaa")
	msg.Header.Set("from", "bbbbbb")

	msg.To = NewToHeaderFromString("Bob", "sip:bob@biloxi.com", "tag=321321")
	if msg.To == nil {
		t.Errorf("Header still nil")
	}
	msg.From = NewFromHeaderFromString("Alice", "sip:alice@atlanta.com;user=phone", "tag=123123")
	if msg.From == nil {
		t.Errorf("Header still nil")
	}
	headerStr := helperWriteHeader(msg)
	msg.Write(out)
	expect := "" +
		"INVITE sip:alice@atlanta.com:5060 SIP/2.0\r\n" +
		headerStr +
		"\r\n"
	helperEvaluateLongString(out.String(), expect, t)
}

func TestMessageWriteHeaderOverWritten(t *testing.T) {
	out := new(bytes.Buffer)
	msg := new(Message)
	msg.Header = make(http.Header)
	msg.Header.Set("to", "aaaaaa")
	msg.Header.Set("from", "bbbbbb")
	msg.Header.Set("Call-id", "cccccc")
	msg.Header.Set("CSeq", "dddddd")
	msg.Header.Set("Via", "eeeeeee")
	msg.Header.Set("Via", "fffffff")
	msg.Header.Set("Via", "ggggggg")
	msg.Header.Set("Max-forwards", "hhhhhhhh")
	msg.Header.Set("contact", "iiiiiiiiiii")

	expect := helperWriteHeader(msg)

	writeHeader(out, msg)
	helperEvaluateLongString(out.String(), expect, t)
}

func TestMessageWriteHeader(t *testing.T) {
	out := new(bytes.Buffer)
	msg := new(Message)
	msg.Header = make(http.Header)
	msg.Header.Set("to", "aaaaaa")
	msg.Header.Set("from", "bbbbbb")
	msg.Header.Set("Call-id", "cccccc")
	msg.Header.Set("CSeq", "dddddd")
	msg.Header.Set("Via", "eeeeeee")
	msg.Header.Add("Via", "fffffff")
	msg.Header.Add("Via", "ggggggg")
	msg.Header.Set("Max-forwards", "hhhhhhhh")
	msg.Header.Set("contact", "iiiiiiiiiii")

	writeHeader(out, msg)
	expect := "" +
		"Call-ID: cccccc\r\n" +
		"Contact: iiiiiiiiiii\r\n" +
		"CSeq: dddddd\r\n" +
		"From: bbbbbb\r\n" +
		"Max-Forwards: hhhhhhhh\r\n" +
		"To: aaaaaa\r\n" +
		"Via: eeeeeee\r\n" +
		"Via: fffffff\r\n" +
		"Via: ggggggg\r\n" +
		""
	helperEvaluateLongString(out.String(), expect, t)
}

func TestReadMessage(t *testing.T) {
	in := new(bytes.Buffer)
	fmt.Fprintf(in, "INVITE sip:alice@atlanta.com SIP/2.0\r\n")
	fmt.Fprintf(in, "Via: SIP/2.0/UDP 10.0.0.1;branch=z9hG4bKnashds8\r\n")
	fmt.Fprintf(in, "Via: SIP/2.0/UDP 10.0.0.2;branch=z9hG4bKaaaaaaa\r\n")
	fmt.Fprintf(in, "To: Bob <sip:bob@biloxi.com>\r\n")
	fmt.Fprintf(in, "From: Alice <sip:alice@atlanta.com;user=phone>\r\n")
	fmt.Fprintf(in, "Contact: Alice <sip:alice@atlanta.com;user=phone>\r\n")
	fmt.Fprintf(in, "Call-ID: aaaaaaa-bbbbbbb-cccccccc@pc33.atlanta.com\r\n")
	fmt.Fprintf(in, "CSeq: 100 INVITE\r\n")
	fmt.Fprintf(in, "Max-Forwards: 69\r\n")
	fmt.Fprintf(in, "\r\n")
	fmt.Fprintf(in, "Content-Type: application/sdp\r\n")
	fmt.Fprintf(in, "\r\n")
	fmt.Fprintf(in, "v=0\n")
	fmt.Fprintf(in, "o=alice 53655765 2353687637 IN IP4 pc33.atlanta.com\n")
	fmt.Fprintf(in, "s=Session SDP\n")
	fmt.Fprintf(in, "t=0 0\n")
	fmt.Fprintf(in, "c=IN IP4 pc33.atlanta.com\n")
	fmt.Fprintf(in, "m=audio 3456 RTP/AVP 0 1 3 99\n")
	fmt.Fprintf(in, "a=rtpmap:0 PCMU/8000\n")
	r := bufio.NewReader(bytes.NewReader(in.Bytes()))

	msg := CreateMessage("127.0.0.1:5060")
	ReadMessage(msg, r)

	if msg.Contact == nil {
		t.Errorf("Header still nil")
		return
	}
	c := msg.Contact.Header[0]
	if c.Addr.DisplayName != "Alice" || c.Addr.Uri.String() != "sip:alice@atlanta.com;user=phone" {
		t.Errorf("Invald contact addr %v", c.Addr)
	}
	if c.RawParameter != "" {
		t.Errorf("Invald contact Parameter")
	}

}
