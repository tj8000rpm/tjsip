package main

import (
	"encoding/csv"
	"log"
	"os"
	"sync"
)

type Fwd struct {
	Addr   string
	Domain string
}

func NewFwd(addr, domain string) (fwd *Fwd) {
	fwd = new(Fwd)
	if fwd == nil {
		return nil
	}
	fwd.Addr = addr
	fwd.Domain = domain
	return
}

type PrefixTrie struct {
	prefix   string
	parent   *PrefixTrie
	children map[rune]*PrefixTrie
	fwd      *Fwd
}

func (p *PrefixTrie) add(prefix string, s []rune, fwd *Fwd) *Fwd {
	if len(s) == 0 {
		p.prefix = prefix
		p.fwd = fwd
		return fwd
	}
	r := s[0]
	children, ok := p.children[r]
	if !ok {
		children = NewPrefixTrie(p)
		p.children[r] = children
	}
	return children.add(prefix, s[1:], fwd)
}

func (p *PrefixTrie) Add(prefix string, fwd *Fwd) *Fwd {
	return p.add(prefix, []rune(prefix), fwd)
}

func (p *PrefixTrie) remove(s []rune) *Fwd {
	pp := p.search(s)
	if pp.prefix != string(s) {
		return nil
	}
	fwd := pp.fwd
	pp.fwd = nil
	if len(pp.children) == 0 {
		pp.children = nil
		pp = pp.parent
		delete(pp.children, s[len(s)-1])
	}

	return fwd
}
func (p *PrefixTrie) Remove(s string) *Fwd {
	return p.remove([]rune(s))
}

func (p *PrefixTrie) SearchToTop() *PrefixTrie {
	pp := p
	for pp.fwd == nil {
		if pp.parent == nil {
			return nil
		}
		pp = pp.parent
	}
	return pp
}

func (p *PrefixTrie) search(s []rune) *PrefixTrie {
	if len(s) == 0 {
		return p.SearchToTop()
	}
	r := s[0]
	children, ok := p.children[r]
	if !ok {
		return p.SearchToTop()
	}
	return children.search(s[1:])
}

func (p *PrefixTrie) Search(s string) *PrefixTrie {
	return p.search([]rune(s))
}

func (p *PrefixTrie) Dump() {
	if p.fwd != nil {
		log.Printf("%s -> (%v, %v)", p.prefix, p.fwd.Addr, p.fwd.Domain)
	}
	for _, v := range p.children {
		v.Dump()
	}
}

func NewPrefixTrie(parent *PrefixTrie) (p *PrefixTrie) {
	p = new(PrefixTrie)
	p.children = make(map[rune]*PrefixTrie)
	p.parent = parent
	return p
}

type Routes struct {
	mu    sync.Mutex
	table *PrefixTrie
}

var routes *Routes

func loadRoutes() bool {
	if routes == nil {
		routes = new(Routes)
		if routes == nil {
			return false
		}
		routes.table = NewPrefixTrie(nil)
		if routes.table == nil {
			return false
		}
	}
	fp, err := os.Open("routes.csv")
	if err != nil {
		panic(err)
	}
	defer fp.Close()

	reader := csv.NewReader(fp)
	var line []string

	routes.mu.Lock()
	defer routes.mu.Unlock()
	for {
		line, err = reader.Read()
		if err != nil {
			break
		}
		if len(line) != 3 {
			return false
		}
		fwd := NewFwd(line[1], line[2])
		if fwd == nil {
			return false
		}
		routes.table.Add(line[0], fwd)
	}

	log.Printf("load route \n")

	routes.table.Dump()
	return true
}
