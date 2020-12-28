package main

import (
	"testing"
)

func TestPrefixTrieSearch(t *testing.T) {
	p := NewPrefixTrie(nil)
	s1 := "0312341234"
	f1 := NewFwd(s1, s1)
	p.Add(s1, f1)
	s2 := "03"
	f2 := NewFwd(s2, s2)
	p.Add(s2, f2)
	s3 := "06"
	f3 := NewFwd(s3, s3)
	p.Add(s3, f3)

	r1 := p.Search("0312341234")
	if r1.fwd != f1 {
		t.Errorf("expect %v: but '%v'", f1, r1.fwd)
	}

	r2 := p.Search("031234")
	if r2.fwd != f2 {
		t.Errorf("expect %v: but '%v'", f2, r2.fwd)
	}

	r3 := p.Search("0611221212")
	if r3.fwd != f3 {
		t.Errorf("expect %v: but '%v'", f3, r3.fwd)
	}

	r4 := p.Search("07123")
	if r4 != nil {
		t.Errorf("expect %v: but '%v'", nil, r4)
	}
}

func TestPrefixTrieRemove(t *testing.T) {
	p := NewPrefixTrie(nil)
	s1 := "0312341234"
	f1 := NewFwd(s1, s1)
	p.Add(s1, f1)
	s2 := "03"
	f2 := NewFwd(s2, s2)
	p.Add(s2, f2)
	s3 := "06"
	f3 := NewFwd(s3, s3)
	p.Add(s3, f3)

	r0 := p.Search("0312341234")
	if r0.fwd != f1 {
		t.Errorf("expect %v: but '%v'", f1, r0.fwd)
	}

	r1 := p.Remove("0312341234")
	if r1 != f1 {
		t.Errorf("expect %v: but '%v'", f1, r1)
	}

	r2 := p.Remove("031234")
	if r2 != nil {
		t.Errorf("expect %v: but '%v'", nil, r2)
	}

	r3 := p.Remove("06")
	if r3 != f3 {
		t.Errorf("expect %v: but '%v'", f3, r3)
	}

	r4 := p.Search("031234")
	if r4.fwd != f2 {
		t.Errorf("expect %v: but '%v'", f2, r4.fwd)
	}

	r5 := p.Search("0312341234")
	if r5.fwd != f2 {
		t.Errorf("expect %v: but '%v'", f2, r5.fwd)
	}
}
