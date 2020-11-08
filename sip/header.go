package sip

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// hasToken reports whether token appears with v, ASCII
// case-insensitive, with space or comma boundaries.
// token must be all lowercase.
// v may contain mixed cased.
func hasToken(v, token string) bool {
	if len(token) > len(v) || token == "" {
		return false
	}
	if v == token {
		return true
	}
	for sp := 0; sp <= len(v)-len(token); sp++ {
		// Check that first character is good.
		// The token is ASCII, so checking only a single byte
		// is sufficient. We skip this potential starting
		// position if both the first byte and its potential
		// ASCII uppercase equivalent (b|0x20) don't match.
		// False positives ('^' => '~') are caught by EqualFold.
		if b := v[sp]; b != token[0] && b|0x20 != token[0] {
			continue
		}
		// Check that start pos is on a valid token boundary.
		if sp > 0 && !isTokenBoundary(v[sp-1]) {
			continue
		}
		// Check that end pos is on a valid token boundary.
		if endPos := sp + len(token); endPos != len(v) && !isTokenBoundary(v[endPos]) {
			continue
		}
		if strings.EqualFold(v[sp:sp+len(token)], token) {
			return true
		}
	}
	return false
}

func isTokenBoundary(b byte) bool {
	return b == ' ' || b == ',' || b == '\t'
}

func validToken(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !validHeaderFieldByte(c) {
			return false
		}
	}
	return true
}

type NameAddr struct {
	DisplayName string
	Uri         *URI
}

func (addr *NameAddr) String() string {
	noDisplayName := addr.DisplayName == ""
	noUriParameters := addr.Uri.RawParameter == ""

	if noDisplayName && noUriParameters {
		return fmt.Sprintf("%s", addr.Uri)
	} else if noDisplayName {
		return fmt.Sprintf("<%s>", addr.Uri)
	}
	displayNameStr := addr.DisplayName
	if !validToken(addr.DisplayName) {
		displayNameStr = "\"" + displayNameStr + "\""
	}
	return fmt.Sprintf("%s <%s>", displayNameStr, addr.Uri)
}

/********************************
* To and From Header
********************************/
type To struct {
	Addr         *NameAddr
	RawParameter string
}
type From = To

func (t *To) String() string {
	if t.RawParameter == "" {
		return t.Addr.String()
	}
	return t.Addr.String() + ";" + t.RawParameter
}

func (t *To) Parameter() url.Values {
	v, _ := url.ParseQuery(t.RawParameter)
	return v
}

/********************************
* Via Header
********************************/
type Via struct {
	SentProtocol string
	SentBy       string
	RawParameter string
}

func (via *Via) Parameter() url.Values {
	v, _ := url.ParseQuery(via.RawParameter)
	return v
}

func (via *Via) Protocol() (name string, verMajor, verMinor int, trans string, err error) {
	sepProto := strings.SplitN(via.SentProtocol, "/", 3)
	if len(sepProto) != 3 {
		return "", 0, 0, "", ErrHeaderParseError
	}
	name = strings.ToLower(sepProto[0])
	verMajor, verMinor, ok := ParseSIPVersion(sepProto[0] + "/" + sepProto[1])
	if !ok {
		return "", 0, 0, "", ErrHeaderParseError
	}
	trans = strings.ToLower(sepProto[2])
	return name, verMajor, verMinor, trans, nil
}

func (via *Via) SetProtocol(name string, verMajor, verMinor int, trans string) (string, bool) {
	name = strings.ToUpper(name)
	trans = strings.ToUpper(trans)
	result := fmt.Sprintf("%s/%1d.%1d/%s", name, verMajor, verMinor, trans)
	via.SentProtocol = result
	return result, true
}

func (v *Via) String() string {
	param := ""
	if v.RawParameter != "" {
		param = ";" + v.RawParameter
	}
	return v.SentProtocol + " " + v.SentBy + param
}

/********************************
* CSeq Header
********************************/
type CSeq struct {
	Sequence int64
	Method   string
}

func (c *CSeq) Init() bool {
	var err error
	c.Sequence, err = GenerateInitCSeq()
	if err != nil {
		return false
	}
	return true
}

func (c *CSeq) Increment() int64 {
	c.Sequence += 1
	if c.Sequence >= 2<<31 {
		c.Sequence %= 2 << 31
	}
	return c.Sequence
}

func (c *CSeq) String() string {
	seq := c.Sequence
	if seq >= 2<<31 {
		seq %= 2 << 31
	}
	s := strconv.FormatInt(seq, 10)
	return s + " " + c.Method
}

/********************************
* CallID Header
********************************/
type CallID struct {
	Identifier string
	Host       string
}

func (c *CallID) Init() bool {
	return c.InitH("")
}

func (c *CallID) InitH(host string) bool {
	ret, err := GenerateCallID()
	if err != nil {
		return false
	}
	c.Identifier = ret
	if host != "" {
		c.Host = host
	}
	return true
}

func (c *CallID) String() string {
	if c.Host != "" {
		return c.Identifier + "@" + c.Host
	}
	return c.Identifier
}

/********************************
* MaxForwards Header
********************************/
type MaxForwards struct {
	Remains int
}

func (m *MaxForwards) Decrement() bool {
	m.Remains -= 1
	if m.Remains <= 0 {
		m.Remains = 0
		return false
	}
	return true
}

func (m *MaxForwards) String() string {
	return strconv.Itoa(m.Remains)
}

/********************************
* Contact Header
********************************/
type Contact struct {
	Star         bool
	Addr         *NameAddr
	RawParameter string
}

func (c *Contact) String() string {
	if c.Star {
		return "*"
	}
	if c.RawParameter == "" {
		return c.Addr.String()
	}
	return c.Addr.String() + ";" + c.RawParameter
}

func (c *Contact) Parameter() url.Values {
	v, _ := url.ParseQuery(c.RawParameter)
	return v
}
