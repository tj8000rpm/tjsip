package sip

import (
	"fmt"
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

type NameAddr struct {
	DisplayName string
	Uri         *URI
}

func (addr *NameAddr) String() string {
	noDisplayName := addr.DisplayName == ""
	noUriParameters := !addr.Uri.ForceParameters

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

type To struct {
	Addr   NameAddr
	Params map[string]string
}

type From To

func validToken(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !validHeaderFieldByte(c) {
			return false
		}
	}
	return true
}
