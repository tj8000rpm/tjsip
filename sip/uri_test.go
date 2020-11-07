package sip

import (
	"net/url"
	"testing"
)

func TestParseURIFull(t *testing.T) {
	uri := "sip:alice@atlanta.com:5060;user=phone;isub=123?query=hoge"
	actual, err := Parse(uri)
	if err != nil {
		t.Errorf("Unexpected error on test preparing")
		return
	}

	if act, exp := actual.Scheme, "sip"; act != exp {
		t.Errorf("Invalid uri scheme, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.User.Username(), "alice"; act != exp {
		t.Errorf("Invalid Username, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.Host, "atlanta.com:5060"; act != exp {
		t.Errorf("Invalid Host, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.Path, ""; act != exp {
		t.Errorf("Invalid Path, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.RawPath, ""; act != exp {
		t.Errorf("Invalid RawPath, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.ForceParameter, false; act != exp {
		t.Errorf("Invalid ForceParameter, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.RawParameter, "user=phone;isub=123"; act != exp {
		t.Errorf("Invalid RawParameter, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.ForceQuery, false; act != exp {
		t.Errorf("Invalid ForceQuery, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.RawQuery, "query=hoge"; act != exp {
		t.Errorf("Invalid RawQuery, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.Absolute, false; act != exp {
		t.Errorf("Invalid Absolute, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.String(), uri; act != exp {
		t.Errorf("Invalid uri String(), expect: %v, but actual %v", exp, act)
	}
}

func TestParseURI(t *testing.T) {
	uri := "sip:alice@atlanta.com:5060"
	actual, err := Parse(uri)
	if err != nil {
		t.Errorf("Unexpected error on test preparing")
		return
	}

	if act, exp := actual.Scheme, "sip"; act != exp {
		t.Errorf("Invalid uri scheme, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.User.Username(), "alice"; act != exp {
		t.Errorf("Invalid Username, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.Host, "atlanta.com:5060"; act != exp {
		t.Errorf("Invalid Username, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.Path, ""; act != exp {
		t.Errorf("Invalid Path, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.RawPath, ""; act != exp {
		t.Errorf("Invalid RawPath, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.ForceParameter, false; act != exp {
		t.Errorf("Invalid ForceParameter, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.RawParameter, ""; act != exp {
		t.Errorf("Invalid RawParameter, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.ForceQuery, false; act != exp {
		t.Errorf("Invalid ForceQuery, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.RawQuery, ""; act != exp {
		t.Errorf("Invalid RawQuery, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.Absolute, false; act != exp {
		t.Errorf("Invalid Absolute, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.String(), uri; act != exp {
		t.Errorf("Invalid uri String(), expect: %v, but actual %v", exp, act)
	}
}

func TestParseURIjithQuery(t *testing.T) {
	uri := "sip:alice@atlanta.com?query=hoge&second=2nd"
	actual, err := Parse(uri)
	if err != nil {
		t.Errorf("Unexpected error on test preparing")
		return
	}

	if act, exp := actual.Scheme, "sip"; act != exp {
		t.Errorf("Invalid uri scheme, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.User.Username(), "alice"; act != exp {
		t.Errorf("Invalid Username, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.Host, "atlanta.com"; act != exp {
		t.Errorf("Invalid Username, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.Path, ""; act != exp {
		t.Errorf("Invalid Path, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.RawPath, ""; act != exp {
		t.Errorf("Invalid RawPath, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.ForceParameter, false; act != exp {
		t.Errorf("Invalid ForceParameter, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.RawParameter, ""; act != exp {
		t.Errorf("Invalid RawParameter, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.ForceQuery, false; act != exp {
		t.Errorf("Invalid ForceQuery, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.RawQuery, "query=hoge&second=2nd"; act != exp {
		t.Errorf("Invalid RawQuery, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.Absolute, false; act != exp {
		t.Errorf("Invalid Absolute, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.String(), uri; act != exp {
		t.Errorf("Invalid uri String(), expect: %v, but actual %v", exp, act)
	}
}

func TestParseTelURIWithParameter(t *testing.T) {
	uri := "tel:+81312341234;isub=123"
	actual, err := Parse(uri)
	if err != nil {
		t.Errorf("Unexpected error on test preparing")
		return
	}

	if act, exp := actual.Scheme, "tel"; act != exp {
		t.Errorf("Invalid uri scheme, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.Host, "+81312341234"; act != exp {
		t.Errorf("Invalid uri Host, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.RawParameter, "isub=123"; act != exp {
		t.Errorf("Invalid uri RawParameter, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.String(), uri; act != exp {
		t.Errorf("Invalid uri String(), expect: %v, but actual %v", exp, act)
	}
}

func TestParseAbsoluteNetPath(t *testing.T) {
	uri := "sip://alice@atlanta.com/b/c;user=phone;isub=123?query=hoge"
	actual, err := Parse(uri)
	if err != nil {
		t.Errorf("Unexpected error on test preparing")
		return
	}

	if act, exp := actual.Scheme, "sip"; act != exp {
		t.Errorf("Invalid uri scheme, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.User.Username(), "alice"; act != exp {
		t.Errorf("Invalid Username, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.Host, "atlanta.com"; act != exp {
		t.Errorf("Invalid Username, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.Path, "/b/c"; act != exp {
		t.Errorf("Invalid Path, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.RawPath, ""; act != exp {
		t.Errorf("Invalid RawPath, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.ForceParameter, false; act != exp {
		t.Errorf("Invalid ForceParameter, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.RawParameter, "user=phone;isub=123"; act != exp {
		t.Errorf("Invalid RawParameter, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.ForceQuery, false; act != exp {
		t.Errorf("Invalid ForceQuery, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.RawQuery, "query=hoge"; act != exp {
		t.Errorf("Invalid RawQuery, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.Absolute, true; act != exp {
		t.Errorf("Invalid Absolute, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.String(), uri; act != exp {
		t.Errorf("Invalid uri String(), expect: %v, but actual %v", exp, act)
	}
}

func TestParseAbsoluteAbsPath(t *testing.T) {
	uri := "/b/c;user=phone;isub=123?query=hoge"
	actual, err := Parse(uri)
	if err != nil {
		t.Errorf("Unexpected error on test preparing")
		return
	}

	if act, exp := actual.Scheme, ""; act != exp {
		t.Errorf("Invalid uri scheme, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.User.Username(), ""; act != exp {
		t.Errorf("Invalid Username, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.Host, ""; act != exp {
		t.Errorf("Invalid Username, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.Path, "/b/c"; act != exp {
		t.Errorf("Invalid Path, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.RawPath, ""; act != exp {
		t.Errorf("Invalid RawPath, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.ForceParameter, false; act != exp {
		t.Errorf("Invalid ForceParameter, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.RawParameter, "user=phone;isub=123"; act != exp {
		t.Errorf("Invalid RawParameter, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.ForceQuery, false; act != exp {
		t.Errorf("Invalid ForceQuery, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.RawQuery, "query=hoge"; act != exp {
		t.Errorf("Invalid RawQuery, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.Absolute, true; act != exp {
		t.Errorf("Invalid Absolute, expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.String(), uri; act != exp {
		t.Errorf("Invalid uri String(), expect: %v, but actual %v", exp, act)
	}
}

func TestUriHostnameAndPort(t *testing.T) {
	actual := URI{Host: "atlanta.com:5060"}

	if act, exp := actual.Hostname(), "atlanta.com"; act != exp {
		t.Errorf("Invalid uri Hostname(), expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.Port(), "5060"; act != exp {
		t.Errorf("Invalid uri Port(), expect: %v, but actual %v", exp, act)
	}
}

func TestUriQuery(t *testing.T) {
	uri := URI{RawQuery: "query=hoge&second=2nd"}
	actual := uri.Query()

	if act, exp := actual.Get("query"), "hoge"; act != exp {
		t.Errorf("Invalid uri Query(), expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.Get("second"), "2nd"; act != exp {
		t.Errorf("Invalid uri Query(), expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.Get("Second"), ""; act != exp {
		t.Errorf("Invalid uri Query(), expect: %v, but actual %v", exp, act)
	}
}

func TestUriParameter(t *testing.T) {
	uri := URI{RawParameter: "user=phone;isub=123"}
	actual := uri.Parameter()

	if act, exp := actual.Get("user"), "phone"; act != exp {
		t.Errorf("Invalid uri Parameter(), expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.Get("isub"), "123"; act != exp {
		t.Errorf("Invalid uri Parameter(), expect: %v, but actual %v", exp, act)
	}
	if act, exp := actual.Get("ISUB"), ""; act != exp {
		t.Errorf("Invalid uri Parameter(), expect: %v, but actual %v", exp, act)
	}
}

func TestUriRequestURI(t *testing.T) {
	uri := URI{
		Scheme:       "sip",
		User:         url.User("alice"),
		Host:         "atlanta.com:5060",
		RawQuery:     "query=hoge",
		RawParameter: "para=meter",
	}

	actual := uri.RequestURI()

	if act, exp := actual, "sip:alice@atlanta.com:5060;para=meter?query=hoge"; act != exp {
		t.Errorf("Invalid uri RequestURI(), expect: %v, but actual %v", exp, act)
	}

	uri = URI{
		Scheme:       "sip",
		Host:         "atlanta.com",
		Path:         "/b/c/d",
		RawQuery:     "query=hoge",
		RawParameter: "para=meter",
	}

	actual = uri.RequestURI()

	if act, exp := actual, "/b/c/d;para=meter?query=hoge"; act != exp {
		t.Errorf("Invalid uri RequestURI(), expect: %v, but actual %v", exp, act)
	}
}
