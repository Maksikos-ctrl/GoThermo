package main

import (
	"context"
	"errors"
	"net"
	"testing"
)

func TestEmailRegex(t *testing.T) {
	good := []string{"a@b.co", "ivan.petrov+x@mail.example.com", "x_y-z@sub-domain.example.org", "maksikos973@gmail.com"}
	bad := []string{"", "a@b", "a@b.c", "@x.com", "a b@x.com", "a@-x.com", "a@x..com", "a@x.com.", "a@@x.com", "plain", "a@x.c0m"}
	for _, e := range good {
		if !emailRe.MatchString(e) {
			t.Errorf("should accept %q", e)
		}
	}
	for _, e := range bad {
		if emailRe.MatchString(e) {
			t.Errorf("should reject %q", e)
		}
	}
}

func withLookups(mx func(string) ([]*net.MX, error), host func(string) ([]string, error), fn func()) {
	om, oh := lookupMX, lookupHost
	lookupMX = func(_ context.Context, d string) ([]*net.MX, error) { return mx(d) }
	lookupHost = func(_ context.Context, d string) ([]string, error) { return host(d) }
	defer func() { lookupMX, lookupHost = om, oh }()
	fn()
}

var nf = &net.DNSError{Err: "no such host", IsNotFound: true}

func TestMailDomain(t *testing.T) {
	cases := []struct {
		name    string
		mx      func(string) ([]*net.MX, error)
		host    func(string) ([]string, error)
		wantErr bool
	}{
		{"has MX", func(string) ([]*net.MX, error) { return []*net.MX{{Host: "mx.x."}}, nil }, nil, false},
		{"null MX", func(string) ([]*net.MX, error) { return []*net.MX{{Host: "."}}, nil }, nil, true},
		{"no MX but A", func(string) ([]*net.MX, error) { return nil, nf }, func(string) ([]string, error) { return []string{"1.2.3.4"}, nil }, false},
		{"nothing", func(string) ([]*net.MX, error) { return nil, nf }, func(string) ([]string, error) { return nil, nf }, true},
		{"mx timeout fails open", func(string) ([]*net.MX, error) { return nil, &net.DNSError{Err: "i/o timeout", IsTimeout: true} }, nil, false},
		{"host timeout fails open", func(string) ([]*net.MX, error) { return nil, nf }, func(string) ([]string, error) { return nil, errors.New("network down") }, false},
	}
	for _, c := range cases {
		withLookups(c.mx, c.host, func() {
			err := checkMailDomain("x.test")
			if (err != nil) != c.wantErr {
				t.Errorf("%s: err=%v wantErr=%v", c.name, err, c.wantErr)
			}
		})
	}
}

func TestValidateNewEmail(t *testing.T) {
	withLookups(func(string) ([]*net.MX, error) { return []*net.MX{{Host: "mx."}}, nil }, nil, func() {
		if err := ValidateNewEmail("ok@example.com"); err != nil {
			t.Error(err)
		}
		if err := ValidateNewEmail("bad@nodot"); err == nil {
			t.Error("expected format error")
		}
	})
}
