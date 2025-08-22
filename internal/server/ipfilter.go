// Package server provides TCP server components including IP filtering.
package server

import (
	"fmt"
	"net"
	"strings"
)

// IPFilter provides allowlist/blocklist based filtering for remote IPs.
//
// Rules:
// - If blocklist matches, the IP is rejected regardless of allowlist.
// - If allowlist is empty, all IPs are allowed (unless blocked).
// - If allowlist is non-empty, only IPs inside at least one allowed network are permitted.
type IPFilter struct {
	allow []*net.IPNet
	block []*net.IPNet
}

// NewIPFilterFromStrings constructs an IPFilter from string slices.
// Each entry can be a CIDR (e.g., "10.0.0.0/8", "2001:db8::/32") or a single IP
// (e.g., "192.168.1.10", "2001:db8::1"). Single IPs are normalized to /32 or /128.
func NewIPFilterFromStrings(allow, block []string) (*IPFilter, error) {
	allowNets, err := parseCIDRList(allow)
	if err != nil {
		return nil, fmt.Errorf("invalid allowlist: %w", err)
	}

	blockNets, err := parseCIDRList(block)
	if err != nil {
		return nil, fmt.Errorf("invalid blocklist: %w", err)
	}

	return &IPFilter{allow: allowNets, block: blockNets}, nil
}

// Allow reports whether the provided IP is permitted by the filter.
func (f *IPFilter) Allow(ip net.IP) bool {
	if f == nil {
		return true
	}
	if ip == nil {
		return false
	}

	nip := normalizeIP(ip)

	// Blocklist takes precedence
	for _, n := range f.block {
		if n.Contains(nip) {
			return false
		}
	}

	// If no allowlist configured, allow by default
	if len(f.allow) == 0 {
		return true
	}

	for _, n := range f.allow {
		if n.Contains(nip) {
			return true
		}
	}
	return false
}

func parseCIDRList(items []string) ([]*net.IPNet, error) {
	var nets []*net.IPNet
	for _, raw := range items {
		s := strings.TrimSpace(raw)
		if s == "" {
			continue
		}
		// CIDR notation
		if strings.Contains(s, "/") {
			ip, n, err := net.ParseCIDR(s)
			if err != nil {
				return nil, err
			}
			// Normalize IPv4 addresses
			if v4 := ip.To4(); v4 != nil {
				ip = v4
				n.IP = v4
			}
			nets = append(nets, n)
			continue
		}

		// Single IP -> normalize to /32 or /128
		ip := net.ParseIP(s)
		if ip == nil {
			return nil, fmt.Errorf("invalid IP: %s", s)
		}
		if v4 := ip.To4(); v4 != nil {
			_, n, _ := net.ParseCIDR(v4.String() + "/32")
			nets = append(nets, n)
		} else {
			_, n, _ := net.ParseCIDR(ip.String() + "/128")
			nets = append(nets, n)
		}
	}
	return nets, nil
}

func normalizeIP(ip net.IP) net.IP {
	if v4 := ip.To4(); v4 != nil {
		return v4
	}
	return ip
}
