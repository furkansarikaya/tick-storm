package server

import (
	"net"
	"testing"
)

func TestIPFilter_NoLists_AllowsAll(t *testing.T) {
	f, err := NewIPFilterFromStrings(nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cases := []string{"1.2.3.4", "8.8.8.8", "2001:db8::1"}
	for _, c := range cases {
		if !f.Allow(net.ParseIP(c)) {
			t.Errorf("expected %s to be allowed", c)
		}
	}
}

func TestIPFilter_AllowlistOnly(t *testing.T) {
	f, err := NewIPFilterFromStrings([]string{"10.0.0.0/8"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !f.Allow(net.ParseIP("10.1.2.3")) {
		t.Errorf("expected 10.1.2.3 to be allowed")
	}
	if f.Allow(net.ParseIP("192.168.1.1")) {
		t.Errorf("expected 192.168.1.1 to be denied")
	}
}

func TestIPFilter_BlocklistOnly(t *testing.T) {
	f, err := NewIPFilterFromStrings(nil, []string{"192.168.0.0/16"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Allow(net.ParseIP("192.168.1.1")) {
		t.Errorf("expected 192.168.1.1 to be blocked")
	}
	if !f.Allow(net.ParseIP("8.8.8.8")) {
		t.Errorf("expected 8.8.8.8 to be allowed")
	}
}

func TestIPFilter_BlockPrecedence(t *testing.T) {
	f, err := NewIPFilterFromStrings([]string{"0.0.0.0/0"}, []string{"1.2.3.4"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Allow(net.ParseIP("1.2.3.4")) {
		t.Errorf("expected 1.2.3.4 to be blocked by precedence")
	}
	if !f.Allow(net.ParseIP("1.2.3.5")) {
		t.Errorf("expected 1.2.3.5 to be allowed")
	}
}

func TestIPFilter_SingleIPParsing(t *testing.T) {
	f, err := NewIPFilterFromStrings([]string{"192.168.1.10"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !f.Allow(net.ParseIP("192.168.1.10")) {
		t.Errorf("expected exact ip to be allowed")
	}
	if f.Allow(net.ParseIP("192.168.1.11")) {
		t.Errorf("expected other ip to be denied")
	}
}

func TestIPFilter_IPv6(t *testing.T) {
	f, err := NewIPFilterFromStrings([]string{"2001:db8::/32"}, []string{"2001:db8::dead"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !f.Allow(net.ParseIP("2001:db8::1")) {
		t.Errorf("expected IPv6 in range to be allowed")
	}
	if f.Allow(net.ParseIP("2001:dead::1")) {
		t.Errorf("expected IPv6 outside range to be denied")
	}
	if f.Allow(net.ParseIP("2001:db8::dead")) {
		t.Errorf("expected blocked IPv6 to be denied")
	}
}

func TestIPFilter_InvalidConfig(t *testing.T) {
	if _, err := NewIPFilterFromStrings([]string{"bogus"}, nil); err == nil {
		t.Errorf("expected error for invalid allowlist entry")
	}
}
