# PRD-005: Show Local IP + Subnet Next to "Connected" Status

**Version:** v0.1.0 (Draft) · **Status:** Draft · **Author:** Planner Agent · **Date:** 2026-07-24

## Overview
After a successful VPN connection, append the local IP and dotted-decimal subnet to the "Connected" status label, e.g. `Connected - 192.168.1.1/255.255.255.0`.

## Files (modify)
- `internal/vpn/win/netapi_windows.go` — add `InterfaceInfo(name string) (ifIndex uint32, addrs []net.IPNet, err error)`; reuse `GetAdaptersAddresses` loop, capture `ua.OnLinkPrefixLength`, build `net.IPNet{IP: ip4, Mask: net.CIDRMask(int(ua.OnLinkPrefixLength), 32)}` for IPv4 unicast addrs. Return `(0, nil, nil)` when adapter missing (graceful, like `TrafficCounters`). Add `import "net"`.
- `internal/vpn/win_exports_windows.go` — re-export: `func InterfaceInfo(name string) (uint32, []net.IPNet, error) { return win.InterfaceInfo(name) }` (+ `import "net"`).
- `internal/vpn/stub_other.go` — stub: `func InterfaceInfo(name string) (uint32, []net.IPNet, error) { return 0, nil, unsupported() }` (+ `import "net"`).
- `internal/ui/main_window.go` — add `import "net"`; add `refreshLocalIP()` goroutine (retry ~10×300ms calling `vpn.InterfaceInfo(c.profileName())`); on UI thread (`fyne.Do`) if `c.state == vpn.StatusConnected` and `c.statusPri.Text` lacks the IP, append `" - " + ip.String() + "/" + net.IP(mask).String()`. Call `c.refreshLocalIP()` in the 3 Connected paths: `onConnect` success, `onConnect` already-connected, `loadInitial` already-connected.

## Format
Exact: `Connected - 192.168.1.1/255.255.255.0` (use `net.IP(mask).String()`, NOT `Mask.String()`).

## Risks
- IP not assigned instantly after ConnectFull → mitigated by async retry loop.
- No secrets logged (consistent with `sanitizeUIErr`).

## Acceptance
- `go build ./...` and `go vet ./internal/...` pass.
- Status shows `Connected - <ip>/<dotted mask>` once adapter IP is ready; no regression when disconnected.

## Agent
Backend Developer (vpn) → Frontend Developer (ui) → Debugger/Reviewer
