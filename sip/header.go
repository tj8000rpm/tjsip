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

func (addr *NameAddr) Clone() (cp *NameAddr) {
	cp = new(NameAddr)
	cp.DisplayName = addr.DisplayName
	if addr.Uri != nil {
		cp.Uri = addr.Uri.Clone()
	}
	return cp
}

func (addr *NameAddr) String() string {
	noDisplayName := addr.DisplayName == ""
	noUriParameters := addr.Uri.RawParameter == ""
	noSpecialCharacterInURI := validToken(addr.Uri.User.String()) && validToken(addr.Uri.Host)

	if noDisplayName && noUriParameters {
		if !noSpecialCharacterInURI {
			return fmt.Sprintf("<%s>", addr.Uri)
		}
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

func NewNameAddr(display string, uri *URI) *NameAddr {
	n := new(NameAddr)
	if n == nil {
		return nil
	}
	n.DisplayName = strings.Trim(display, " \t\r\n")
	n.Uri = uri
	return n
}

func NewNameAddrUriString(display, uristr string) *NameAddr {
	uri, err := Parse(strings.Trim(uristr, " \t\r\n"))
	if err != nil {
		return nil
	}
	return NewNameAddr(display, uri)
}

func ParseNameAddr(s string) (n *NameAddr, trail string) {
	sLen := len(s)

	display := ""
	ds := 0
	de := -1
	uriStr := ""
	us := 0
	ue := len(s)

	innerLtGt := false
	innerQuated := false
	var pc byte

	for idx := 0; idx < sLen; idx++ {
		c := s[idx]
		if c == '<' && !innerQuated && !innerLtGt {
			de = idx
			us = idx + 1
			innerLtGt = true
		} else if c == '>' && !innerQuated && innerLtGt {
			ue = idx
			innerLtGt = false
			break
		} else if c == '"' && pc != '\\' {
			innerQuated = !innerQuated
		}
		pc = c
	}

	if ds < de {
		display = strings.Trim(s[ds:de], " \"")
	}
	if us < ue {
		uriStr = strings.Trim(s[us:ue], " <>")
	}
	if innerQuated || innerLtGt {
		return
	}
	trail = strings.Trim(s[ue:], " <>")
	n = NewNameAddrUriString(display, uriStr)

	return
}

/********************************
* To and From Header
********************************/
type To struct {
	Addr         *NameAddr
	RawParameter string
}
type From = To

func (t *To) Clone() (cp *To) {
	cp = new(To)
	if t.Addr != nil {
		cp.Addr = t.Addr.Clone()
	}
	cp.RawParameter = t.RawParameter
	return cp
}

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

func NewToHeader(addr *NameAddr, rawParam string) *To {
	t := new(To)
	if t == nil {
		return nil
	}
	t.Addr = addr
	t.RawParameter = strings.Trim(rawParam, " \t\r\n;")
	return t
}
func NewFromHeader(addr *NameAddr, rawParam string) *From {
	return NewToHeader(addr, rawParam)
}

func NewToHeaderFromString(display, uri, rawParam string) *To {
	addr := NewNameAddrUriString(display, uri)
	if addr == nil {
		return nil
	}
	return NewToHeader(addr, rawParam)
}
func NewFromHeaderFromString(display, uri, rawParam string) *From {
	return NewToHeaderFromString(display, uri, rawParam)
}

func ParseFrom(s string) *From {
	nameAddr, rawParam := ParseNameAddr(s)
	rawParam = strings.Trim(rawParam, " ;")
	return NewToHeader(nameAddr, rawParam)
}

func ParseTo(s string) *To {
	return ParseFrom(s)
}

/********************************
* Via Header
********************************/
type Via struct {
	SentProtocol string
	SentBy       string
	RawParameter string
}

func (v *Via) Clone() (cp *Via) {
	cp = new(Via)
	cp.SentProtocol = v.SentProtocol
	cp.SentBy = v.SentBy
	cp.RawParameter = v.RawParameter
	return cp
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

func NewViaHeader(proto, by, param string) *Via {
	v := new(Via)
	if v == nil {
		return nil
	}
	v.SentProtocol = strings.Trim(proto, " \t\r\n")
	v.SentBy = strings.Trim(by, " \t\r\n")
	v.RawParameter = strings.Trim(param, " \t\r\n;")
	return v
}
func NewViaHeaderUDP(by, param string) *Via {
	return NewViaHeader("SIP/2.0/UDP", by, param)
}
func NewViaHeaderTCP(by, param string) *Via {
	return NewViaHeader("SIP/2.0/TCP", by, param)
}

func ParseVias(s string, v *ViaHeaders) error {
	if v == nil {
		return ErrHeaderParseError
	}
	if s == "" {
		return nil
	}
	blk := " \t\r\n"
	s = strings.Trim(s, blk)
	sSp := strings.SplitN(s, " ", 2)

	proto, s := strings.Trim(sSp[0], blk), strings.Trim(sSp[1], blk)
	i := 0
loop1:
	for i = 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case ';':
			break loop1
		case ',':
			break loop1
		}
	}
	sentby, s := s[:i], s[i:]
	sentby = strings.Trim(sentby, blk)

	quoted := false
loop2:
	for i = 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"':
			quoted = !quoted
		case ',':
			if !quoted {
				break loop2
			}
		}
	}
	rawParam := strings.Trim(s[:i], blk+";")
	trail := strings.Trim(s[i:], blk+",")
	via := NewViaHeader(proto, sentby, rawParam)
	v.Append(via)

	return ParseVias(trail, v)
}

func parseVias(s string) *Via {
	return nil
}

type ViaHeaders struct {
	Header []*Via
}

func (v *ViaHeaders) Clone() (cp *ViaHeaders) {
	cp = new(ViaHeaders)
	cp.Header = make([]*Via, len(v.Header))
	for i := 0; i < len(v.Header); i++ {
		if v.Header[i] != nil {
			cp.Header[i] = v.Header[i].Clone()
		}
	}
	return cp
}

func (v *ViaHeaders) Insert(via *Via) bool {
	before := len(v.Header)
	v.Header = append(v.Header, via)
	return before < len(v.Header)
}

func (v *ViaHeaders) Append(via *Via) bool {
	before := len(v.Header)
	v.Header = append([]*Via{via}, v.Header...)
	return before < len(v.Header)
}

func (v *ViaHeaders) TopMost() *Via {
	return v.Get(0)
}

func (v *ViaHeaders) Get(index int) *Via {
	lastindex := len(v.Header) - 1
	if index > lastindex || index < 0 {
		return nil
	}
	return v.Header[lastindex-index]
}

func (v *ViaHeaders) Pop() *Via {
	if v.Length() == 0 {
		return nil
	}
	via := v.Get(0)
	v.Header = v.Header[:v.Length()-1]
	return via
}

func (v *ViaHeaders) Length() int {
	return len(v.Header)
}

func (v *ViaHeaders) WriteHeader() string {
	var str string
	key := "Via: "
	if UseCompactForm {
		key = "v: "
	}
	for i := len(v.Header) - 1; i >= 0; i-- {
		str += key + v.Header[i].String() + "\r\n"
	}
	return str
}

func NewViaHeaders() *ViaHeaders {
	v := new(ViaHeaders)
	if v == nil {
		return nil
	}
	v.Header = make([]*Via, 0)
	return v
}

/********************************
* CSeq Header
********************************/
type CSeq struct {
	Sequence int64
	Method   string
}

func (c *CSeq) Clone() (cp *CSeq) {
	cp = new(CSeq)
	cp.Sequence = c.Sequence
	cp.Method = c.Method
	return cp
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

func NewCSeqHeader(method string) *CSeq {
	c := new(CSeq)
	if c == nil {
		return nil
	}
	c.Method = strings.Trim(method, " \t\r\n")
	if ok := c.Init(); !ok {
		return nil
	}
	return c
}

func ParseCSeq(s string) *CSeq {
	blk := " \t\r\n"
	s = strings.Trim(s, blk)
	temp := strings.SplitN(s, " ", 2)
	front, method := temp[0], strings.Trim(temp[1], blk)
	seq, err := strconv.ParseInt(front, 10, 64)
	if err != nil {
		return nil
	}
	c := new(CSeq)
	if c == nil {
		return nil
	}
	c.Method = method
	c.Sequence = seq
	return c
}

/********************************
* CallID Header
********************************/
type CallID struct {
	Identifier string
	Host       string
}

func (c *CallID) Clone() (cp *CallID) {
	cp = new(CallID)
	cp.Identifier = c.Identifier
	cp.Host = c.Host
	return cp
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

func NewCallIDHeader() *CallID {
	return NewCallIDHeaderWithAddr("")
}

func NewCallIDHeaderWithAddr(localAddr string) *CallID {
	c := new(CallID)
	if c == nil {
		return nil
	}
	c.InitH(strings.Trim(localAddr, " \t\r\n"))
	return c
}

func ParseCallID(s string) *CallID {
	blk := " \t\r\n"
	temp := strings.SplitN(s, "@", 2)
	var identifier, host string
	identifier = temp[0]
	if len(temp) == 2 {
		host = temp[1]
	}
	c := new(CallID)
	if c == nil {
		return nil
	}
	c.Identifier = strings.Trim(identifier, blk)
	c.Host = strings.Trim(host, blk)
	return c
}

/********************************
* MaxForwards Header
********************************/
type MaxForwards struct {
	Remains int
}

func (m *MaxForwards) Clone() (cp *MaxForwards) {
	cp = new(MaxForwards)
	cp.Remains = m.Remains
	return cp
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

func NewMaxForwardsHeader() *MaxForwards {
	m := new(MaxForwards)
	if m == nil {
		return nil
	}
	m.Remains = InitMaxForward
	return m
}

func ParseMaxForwards(s string) *MaxForwards {
	remain, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	m := new(MaxForwards)
	if m == nil {
		return nil
	}
	m.Remains = remain
	return m
}

/********************************
* Contact Header
********************************/
type Contact struct {
	Star         bool
	Addr         *NameAddr
	RawParameter string
}

func (c *Contact) Clone() (cp *Contact) {
	cp = new(Contact)
	cp.Star = c.Star
	if c.Addr != nil {
		cp.Addr = c.Addr.Clone()
	}
	cp.RawParameter = c.RawParameter
	return cp
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

func newContactHeader(nameAddr *NameAddr, param string, isStar bool) *Contact {
	c := new(Contact)
	if c == nil {
		return nil
	}
	if isStar || (nameAddr != nil && nameAddr.Uri.Path == "*") {
		c.Star = true
		return c
	}
	c.Addr = nameAddr
	c.RawParameter = strings.Trim(param, " \t\r\n;")
	return c
}

func NewContactHeader(display string, uri *URI, param string, isStar bool) *Contact {
	return newContactHeader(NewNameAddr(display, uri), param, isStar)
}

func NewContactHeaderFromString(display, uristr, param string) *Contact {
	if strings.Trim(uristr, " \t\r\n") == "*" {
		return NewContactHeader("", nil, "", true)
	}
	uri, err := Parse(uristr)
	if err != nil {
		return nil
	}
	return NewContactHeader(display, uri, param, false)
}

func NewContactHeaderStar() *Contact {
	return NewContactHeader("", nil, "", true)
}

func ParseContacts(s string, cons *ContactHeaders) error {
	if cons == nil {
		return ErrHeaderParseError
	}
	if s == "" {
		return nil
	}
	nameAddr, trail := ParseNameAddr(s)

	res := strings.SplitN(trail, ",", 2)
	rawParam := strings.Trim(res[0], " \t;")
	if len(res) == 2 {
		trail = strings.Trim(res[1], " \t")
	} else {
		trail = ""
	}

	c := newContactHeader(nameAddr, rawParam, false)
	if cons.Add(c) {
		return ParseContacts(trail, cons)
	}
	return nil
}

func ParseContact(s string) *Contact {
	nameAddr, rawParam := ParseNameAddr(s)
	rawParam = strings.Trim(rawParam, " ;")
	return newContactHeader(nameAddr, rawParam, false)
}

type ContactHeaders struct {
	Header []*Contact
}

func (c *ContactHeaders) Clone() (cp *ContactHeaders) {
	cp = new(ContactHeaders)
	cp.Header = make([]*Contact, len(c.Header))
	for i := 0; i < len(c.Header); i++ {
		if c.Header[i] != nil {
			cp.Header[i] = c.Header[i].Clone()
		}
	}
	return cp
}

func NewContactHeaders() *ContactHeaders {
	c := new(ContactHeaders)
	if c == nil {
		return nil
	}
	c.Header = make([]*Contact, 0)
	return c
}

func (c *ContactHeaders) Add(con *Contact) bool {
	before := len(c.Header)
	c.Header = append(c.Header, con)
	return before < len(c.Header)
}

func (c *ContactHeaders) WriteHeader() string {
	str := ""
	key := "Contact: "
	if UseCompactForm {
		key = "c: "
	}
	for i := 0; i < len(c.Header); i++ {
		str += key + c.Header[i].String() + "\r\n"
	}
	return str
}

func (c *ContactHeaders) Length() int {
	if c.Header == nil {
		return 0
	}
	return len(c.Header)
}

/********************************
* Name addrs header
********************************/
type NameAddrFormat struct {
	Addr         *NameAddr
	RawParameter string
}

func (c *NameAddrFormat) Clone() (cp *NameAddrFormat) {
	cp = new(NameAddrFormat)
	if c.Addr != nil {
		cp.Addr = c.Addr.Clone()
	}
	cp.RawParameter = c.RawParameter
	return cp
}

func (c *NameAddrFormat) String() string {
	if c.RawParameter == "" {
		return c.Addr.String()
	}
	return c.Addr.String() + ";" + c.RawParameter
}

func (c *NameAddrFormat) Parameter() url.Values {
	v, _ := url.ParseQuery(c.RawParameter)
	return v
}

func newNameAddrFormatHeader(nameAddr *NameAddr, param string) *NameAddrFormat {
	c := new(NameAddrFormat)
	if c == nil {
		return nil
	}
	c.Addr = nameAddr
	c.RawParameter = strings.Trim(param, " \t\r\n;")
	return c
}

func NewNameAddrFormatHeader(display string, uri *URI, param string) *NameAddrFormat {
	return newNameAddrFormatHeader(NewNameAddr(display, uri), param)
}

func NewNameAddrFormatHeaderFromString(display, uristr, param string) *NameAddrFormat {
	uri, err := Parse(uristr)
	if err != nil {
		return nil
	}
	return NewNameAddrFormatHeader(display, uri, param)
}

func ParseNameAddrFormats(s string, cons *NameAddrFormatHeaders) error {
	if cons == nil {
		return ErrHeaderParseError
	}
	if s == "" {
		return nil
	}
	nameAddr, trail := ParseNameAddr(s)

	res := strings.SplitN(trail, ",", 2)
	rawParam := strings.Trim(res[0], " \t;")
	if len(res) == 2 {
		trail = strings.Trim(res[1], " \t")
	} else {
		trail = ""
	}

	c := newNameAddrFormatHeader(nameAddr, rawParam)
	if cons.Add(c) {
		return ParseNameAddrFormats(trail, cons)
	}
	return nil
}

func ParseNameAddrFormat(s string) *NameAddrFormat {
	nameAddr, rawParam := ParseNameAddr(s)
	rawParam = strings.Trim(rawParam, " ;")
	return newNameAddrFormatHeader(nameAddr, rawParam)
}

type NameAddrFormatHeaders struct {
	Header []*NameAddrFormat
}

func (c *NameAddrFormatHeaders) Clone() (cp *NameAddrFormatHeaders) {
	cp = new(NameAddrFormatHeaders)
	cp.Header = make([]*NameAddrFormat, len(c.Header))
	for i := 0; i < len(c.Header); i++ {
		if c.Header[i] != nil {
			cp.Header[i] = c.Header[i].Clone()
		}
	}
	return cp
}

func NewNameAddrFormatHeaders() *NameAddrFormatHeaders {
	c := new(NameAddrFormatHeaders)
	if c == nil {
		return nil
	}
	c.Header = make([]*NameAddrFormat, 0)
	return c
}

func (c *NameAddrFormatHeaders) Add(con *NameAddrFormat) bool {
	before := len(c.Header)
	c.Header = append(c.Header, con)
	return before < len(c.Header)
}

func (c *NameAddrFormatHeaders) WriteHeader(full, compact string) string {
	str := ""
	key := full + ": "
	if UseCompactForm {
		key = compact + ": "
	}
	for i := 0; i < len(c.Header); i++ {
		str += key + c.Header[i].String() + "\r\n"
	}
	return str
}

func (c *NameAddrFormatHeaders) Length() int {
	if c.Header == nil {
		return 0
	}
	return len(c.Header)
}
