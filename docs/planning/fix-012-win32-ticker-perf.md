# Fix-012: Replace PowerShell/exec in periodic tickers with Win32 syscalls

**Severity:** Medium (performance)
**Date:** 2026-07-23

## Bug Summary

While a VPN connection is active, the app spawns ~4.5 new processes per second:
- **Traffic ticker (1s):** 4× `powershell.exe` per tick — 2× `Get-VpnConnection` (interface index for traffic + connections) + 1× `Get-NetAdapterStatistics` + 1× `Get-NetTCPConnection`
- **Ping ticker (2s):** 1× `ping.exe` per tick

Each `powershell.exe` startup costs ~300ms and allocates ~30MB. These are hot-path calls that run continuously while connected.

## Root Cause

`TrafficCounters`, `ActiveConnections`, and `pingGateway` were implemented with PowerShell/exec for expedience. The project already uses Win32 syscalls extensively (`crypt32.dll`, `advapi32.dll`, `user32.dll`, `registry`) via `golang.org/x/sys/windows`, so the pattern is well-established.

## Fix Strategy

Replace all subprocess calls in the three ticker functions with native Win32 API calls from `iphlpapi.dll` (and `ws2_32.dll` for ICMP). Zero new dependencies.

### Win32 APIs (all available on Windows 7+)

| Current subprocess | Win32 replacement | DLL |
|-|-|-|
| `Get-VpnConnection -Name X \| Select InterfaceIndex` | `GetAdaptersAddresses` — match `FriendlyName` → `IfIndex` | iphlpapi.dll |
| `Get-NetAdapterStatistics -InterfaceIndex N` | `GetIfEntry2` — read `InOctets`/`OutOctets` from `MIB_IF_ROW2` | iphlpapi.dll |
| `Get-NetTCPConnection -InterfaceIndex N -State Established` | `GetExtendedTcpTable` — filter `MIB_TCP_STATE_ESTAB` rows by VPN local IPs | iphlpapi.dll |
| `ping.exe -n 1 -w 1000 host` | `IcmpCreateFile` + `IcmpSendEcho` → `ICMP_ECHO_REPLY.RoundTripTime` + `IcmpCloseHandle` | iphlpapi.dll |
| (IP string → uint32 for ICMP) | `net.ParseIP` (pure Go, no DLL needed) | — |

## Implementation Tasks

| # | Agent | File | Action | Description |
|---|-------|------|--------|-------------|
| 1 | Backend | `internal/vpn/netapi_windows.go` | **CREATE** | `//go:build windows`. Declare `iphlpapi.dll` lazy DLL + all proc handles (`GetAdaptersAddresses`, `GetIfEntry2`, `GetExtendedTcpTable`, `IcmpCreateFile`, `IcmpSendEcho`, `IcmpCloseHandle`). Define Go structs for `MIB_IF_ROW2` (subset: `InterfaceLuid`, `InterfaceIndex`, `InOctets`, `OutOctets`), `MIB_TCPROW_OWNER_PID`, `IP_ADAPTER_ADDRESSES` (linked-list walk), `ICMP_ECHO_REPLY`. Implement shared helper `resolveVPNInterfaceIndex(name string) (uint32, []net.IP, error)` — calls `GetAdaptersAddresses`, walks linked list matching `FriendlyName` (UTF-16 compare), returns `IfIndex` + unicast addresses. Returns `(0, nil, nil)` when not found (not connected). |
| 2 | Backend | `internal/vpn/traffic_windows.go` | **REWRITE** | Replace PowerShell body of `TrafficCounters(name) (uint64, uint64, error)` with: call `resolveVPNInterfaceIndex` → if index==0 return 0,0,nil → populate `MIB_IF_ROW2.InterfaceIndex` → call `GetIfEntry2` → return `InOctets`, `OutOctets`. Delete `parseTrafficStats` and `extractNumber` (dead code). Signature unchanged. |
| 3 | Backend | `internal/vpn/connections_windows.go` | **REWRITE** | Replace PowerShell body of `ActiveConnections(name) ([]ActiveConn, error)` with: call `resolveVPNInterfaceIndex` → if index==0 return nil,nil → call `GetExtendedTcpTable(AF_INET, TCP_TABLE_OWNER_PID_ALL)` → iterate `MIB_TCPROW_OWNER_PID` rows → filter `dwState == MIB_TCP_STATE_ESTAB` AND `dwLocalAddr` ∈ VPN unicast IPs → build `ActiveConn{RemoteAddr, RemotePort, Hostname: reverseLookup(ip)}`. Preserve `reverseLookup` helper unchanged. Signature unchanged. |
| 4 | Backend | `internal/ui/ping_windows.go` | **REWRITE** | Replace `exec.Command("ping.exe")` body of `pingGateway(host string) string` with: parse host via `net.ParseIP` (or `net.ResolveIPAddr`) → `IcmpCreateFile()` → `IcmpSendEcho(handle, ipv4Addr, ..., timeout=1000ms)` → read `ICMP_ECHO_REPLY.Status` + `.RoundTripTime` → `IcmpCloseHandle`. Return same Indonesian strings: `"tidak terhubung"` / `"host — Nms"` / `"host — timeout / tidak ada balasan"` / `"host — tidak dapat dijangkau"`. Remove `os/exec` and `syscall` imports (no longer needed). Signature unchanged. |
| 5 | — | `internal/vpn/stub_other.go` | **NO CHANGE** | Stubs already return `unsupported()`. No modifications. |
| 6 | — | `internal/ui/ping_other.go` | **NO CHANGE** | Stub already returns placeholder. No modifications. |
| 7 | Debugger | all modified files | **VERIFY** | `go build ./cmd/vepeen` (Windows); `go vet ./...`; manual test: connect VPN → verify traffic counters update, connections list populates, ping shows latency. Confirm zero `powershell.exe`/`ping.exe` spawns via Task Manager during connected state. |

## File Summary

| File | Status |
|------|--------|
| `internal/vpn/netapi_windows.go` | NEW — shared DLL procs, structs, `resolveVPNInterfaceIndex` |
| `internal/vpn/traffic_windows.go` | MODIFY — rewrite body, delete `parseTrafficStats`/`extractNumber` |
| `internal/vpn/connections_windows.go` | MODIFY — rewrite PowerShell body, keep `reverseLookup` |
| `internal/ui/ping_windows.go` | MODIFY — rewrite to ICMP API |
| `internal/vpn/stub_other.go` | UNCHANGED |
| `internal/ui/ping_other.go` | UNCHANGED |

## Architecture Notes

- **Shared resolver:** `resolveVPNInterfaceIndex` is called by both `TrafficCounters` and `ActiveConnections`, replacing the duplicated `Get-VpnConnection` PowerShell in each. It also returns the adapter's unicast IPs (needed by `ActiveConnections` to filter TCP rows by VPN interface).
- **DLL loading pattern:** Follow existing project convention — `windows.NewLazySystemDLL("iphlpapi.dll")` + `dll.NewProc(...)` at package level. Same as `crypt32.dll` in `dpapi_windows.go`.
- **Struct layout:** `MIB_IF_ROW2` is large (1352 bytes). Only declare the full struct with padding so `GetIfEntry2` writes into a correctly-sized buffer. Read only `InOctets`/`OutOctets` (offset 0x2B0/0x2B8, both uint64).
- **`IP_ADAPTER_ADDRESSES`:** Variable-length linked list. Use the two-call pattern: first call with `bufLen=0` to get `ERROR_BUFFER_OVERFLOW` + required size, allocate `[]byte`, second call to fill.
- **TCP table:** `GetExtendedTcpTable` also uses two-call size pattern. `dwLocalAddr` is a `uint32` in network byte order — compare against VPN adapter IPs converted to `uint32`.
- **ICMP:** `IcmpSendEcho` is synchronous with a timeout parameter, perfect for a 2s ticker. No need for async I/O.
- **Error handling:** Return `MapExecError` or `newUserError` where the current code does. For Win32 failures, use `windows.GetLastError()` or the NTSTATUS return value. Return `(0, 0, nil)` / `(nil, nil)` for "not connected" cases (matching current behavior).
- **Build tags:** All new/modified files: `//go:build windows`.

## Acceptance Criteria

- [ ] Zero `powershell.exe` or `ping.exe` processes spawned while VPN is connected
- [ ] `TrafficCounters` returns correct rx/tx bytes (matches `Get-NetAdapterStatistics` output)
- [ ] `ActiveConnections` returns established TCP connections routed through VPN interface
- [ ] `pingGateway` returns latency in same format as before (e.g. `"10.0.0.1 — 24 ms"`)
- [ ] All three functions return gracefully when VPN is not connected (no error)
- [ ] `go build ./cmd/vepeen` succeeds on Windows with `CGO_ENABLED=1`
- [ ] `go vet ./...` clean
- [ ] Non-Windows build (`GOOS=linux go build ./cmd/vepeen`) still compiles via stubs
- [ ] No new dependencies added to `go.mod`

## Regression Risk

- **Low:** Function signatures are unchanged. Callers in `main_window.go` are untouched. Stubs are untouched. The only risk is Win32 struct layout mismatches causing incorrect data reads — mitigated by comparing output against PowerShell baseline during testing.
