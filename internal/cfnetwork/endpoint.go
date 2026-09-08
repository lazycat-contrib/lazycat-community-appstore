// Package cfnetwork connects to preferred Cloudflare edges while preserving origin identity.
package cfnetwork

import (
	"errors"
	"net/netip"
	"strings"
)

const DefaultEndpoint = "saas.sin.fan"

// Normalize accepts a bare public IP or DNS name, never a URL or port.
func Normalize(value string) (string, error) {
	value = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(value), "."))
	if ip, err := netip.ParseAddr(value); err == nil {
		if !publicIP(ip) {
			return "", errors.New("preferred IP must be public")
		}
		return ip.String(), nil
	}
	if len(value) > 253 || !strings.Contains(value, ".") {
		return "", errors.New("enter a public IP or domain without a URL scheme, path or port")
	}
	for label := range strings.SplitSeq(value, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", errors.New("invalid preferred domain")
		}
		for _, ch := range label {
			if (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') && ch != '-' {
				return "", errors.New("invalid preferred domain")
			}
		}
	}
	// Numeric-looking invalid IPv4 addresses must not be treated as domains.
	if strings.Trim(value, "0123456789.") == "" {
		return "", errors.New("invalid preferred IP")
	}
	return value, nil
}

func ParseList(value string) ([]string, error) {
	if len(value) > 8192 {
		return nil, errors.New("preferred endpoints list is too long")
	}
	out := []string{}
	seen := map[string]bool{}
	for line := range strings.SplitSeq(strings.ReplaceAll(value, ",", "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		endpoint, err := Normalize(line)
		if err != nil {
			return nil, err
		}
		if !seen[endpoint] {
			out = append(out, endpoint)
			seen[endpoint] = true
		}
		if len(out) > 32 {
			return nil, errors.New("at most 32 preferred endpoints are allowed")
		}
	}
	return out, nil
}

func publicIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, prefix := range blockedPrefixes {
		if prefix.Contains(ip) {
			return false
		}
	}
	return true
}

var blockedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"), netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001:db8::/32"), netip.MustParsePrefix("64:ff9b::/96"),
}
