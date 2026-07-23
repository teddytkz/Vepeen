// Package route parses selective VPN destinations and syncs profile routes.
package route

import (
	"context"
	"fmt"
	"net"
	"strings"
)

// ParseError describes an invalid line in a multi-line IP/CIDR list.
type ParseError struct {
	Line    int
	Snippet string
	Msg     string
}

func (e *ParseError) Error() string {
	snip := e.Snippet
	if len(snip) > 40 {
		snip = snip[:40] + "…"
	}
	if e.Msg != "" {
		return fmt.Sprintf("Baris %d tidak valid: %q. %s", e.Line, snip, e.Msg)
	}
	return fmt.Sprintf("Baris %d tidak valid: %q. Gunakan format IP atau CIDR (contoh 10.0.0.0/24).", e.Line, snip)
}

// ParseLines parses multi-line text into normalized IPv4 CIDR prefixes.
// Blank lines and lines starting with # are ignored.
// Bare IPv4 addresses become /32. IPv6 is rejected in v1.
// On the first invalid line, returns a *ParseError (and any prefixes parsed so far are discarded).
func ParseLines(text string) ([]string, error) {
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	seen := make(map[string]struct{})

	for i, raw := range lines {
		lineNo := i + 1
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Strip inline comment after whitespace + #
		if idx := strings.Index(line, " #"); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
		}

		prefix, err := normalizeEntry(line)
		if err != nil {
			return nil, &ParseError{Line: lineNo, Snippet: line, Msg: err.Error()}
		}
		if _, ok := seen[prefix]; ok {
			continue
		}
		seen[prefix] = struct{}{}
		out = append(out, prefix)
	}
	return out, nil
}

// NormalizeList normalizes a slice of IP/CIDR strings (already split).
func NormalizeList(items []string) ([]string, error) {
	return ParseLines(strings.Join(items, "\n"))
}

func normalizePrefix(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("kosong")
	}

	// Reject IPv6 early (contains ':' and is not IPv4-mapped we care about for v1).
	if strings.Contains(s, ":") {
		return "", fmt.Errorf("IPv6 belum didukung")
	}

	if strings.Contains(s, "/") {
		ip, ipNet, err := net.ParseCIDR(s)
		if err != nil {
			return "", fmt.Errorf("Gunakan format IP atau CIDR (contoh 10.0.0.0/24)")
		}
		if ip.To4() == nil {
			return "", fmt.Errorf("IPv6 belum didukung")
		}
		ones, bits := ipNet.Mask.Size()
		if bits != 32 || ones < 0 || ones > 32 {
			return "", fmt.Errorf("panjang prefiks tidak valid")
		}
		// Canonical form: network address + prefix length.
		return fmt.Sprintf("%s/%d", ipNet.IP.String(), ones), nil
	}

	ip := net.ParseIP(s)
	if ip == nil || ip.To4() == nil {
		return "", fmt.Errorf("Gunakan format IP atau CIDR (contoh 10.0.0.0/24)")
	}
	return ip.To4().String() + "/32", nil
}

// normalizeEntry accepts an IPv4 IP/CIDR prefix OR a domain name.
// IP/CIDR is canonicalized via normalizePrefix; a domain is returned
// lowercased as-is for later resolution to IPs. Invalid input errors.
func normalizeEntry(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("kosong")
	}
	// IPv6 (contains ':') is unsupported in v1.
	if strings.Contains(s, ":") {
		return "", fmt.Errorf("IPv6 belum didukung")
	}
	if norm, err := normalizePrefix(s); err == nil {
		return norm, nil
	}
	if !isDomainName(s) {
		return "", fmt.Errorf("Gunakan format IP, CIDR, atau nama domain (contoh 10.0.0.0/24 atau example.com)")
	}
	return strings.ToLower(s), nil
}

// isDomainName reports whether s is a plausible domain name (no wildcard,
// path, port, or spaces). Final validity is confirmed at DNS resolution.
func isDomainName(s string) bool {
	if strings.ContainsAny(s, " /\\#@:%") || strings.Contains(s, "..") {
		return false
	}
	if len(s) > 253 {
		return false
	}
	labels := strings.Split(s, ".")
	if len(labels) < 2 {
		return false
	}
	for _, l := range labels {
		if l == "" || len(l) > 63 {
			return false
		}
		if strings.HasPrefix(l, "-") || strings.HasSuffix(l, "-") {
			return false
		}
		for _, r := range l {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-') {
				return false
			}
		}
	}
	return true
}

// lookupHost resolves a host to IP addresses. Overridable in tests.
var lookupHost = func(ctx context.Context, host string) ([]string, error) {
	return net.DefaultResolver.LookupHost(ctx, host)
}

// ResolveRoutes expands domain entries to their IPv4 addresses (as /32
// prefixes) and returns a deduplicated list of IPv4 CIDR prefixes. IP/CIDR
// entries pass through unchanged. A domain that fails to resolve, or resolves
// to no IPv4 address, yields an error so the caller can surface it.
func ResolveRoutes(ctx context.Context, entries []string) ([]string, error) {
	out := make([]string, 0, len(entries))
	seen := make(map[string]struct{})
	add := func(p string) {
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	for _, e := range entries {
		if norm, err := normalizePrefix(e); err == nil {
			add(norm)
			continue
		}
		domain := strings.ToLower(strings.TrimSpace(e))
		addrs, err := lookupHost(ctx, domain)
		if err != nil {
			return nil, fmt.Errorf("gagal mengresolve domain %q: %w", domain, err)
		}
		resolved := 0
		for _, a := range addrs {
			ip := net.ParseIP(a)
			if ip == nil || ip.To4() == nil {
				continue // skip IPv6 for v1
			}
			add(ip.To4().String() + "/32")
			resolved++
		}
		if resolved == 0 {
			return nil, fmt.Errorf("domain %q tidak memiliki alamat IPv4", domain)
		}
	}
	return out, nil
}
