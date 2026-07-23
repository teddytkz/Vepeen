//go:build windows

package vpn

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// ActiveConnections returns established TCP connections whose interface is the
// named VPN connection. Remote hostnames are resolved best-effort via reverse
// DNS with a short timeout (falls back to IP). Returns an empty slice (no error)
// when the connection is not up or has no established sockets.
func ActiveConnections(name string) ([]ActiveConn, error) {
	if err := ValidateName(name); err != nil {
		return nil, err
	}
	idxScript := fmt.Sprintf(
		`Get-VpnConnection -Name %s -ErrorAction SilentlyContinue | Select-Object -ExpandProperty InterfaceIndex`,
		psQuote(name),
	)
	idxOut, idxErr := runPowerShell(idxScript)
	if idxErr != nil {
		if strings.TrimSpace(idxOut) == "" {
			return nil, nil
		}
		return nil, MapExecError("ActiveConnections", idxErr, idxOut)
	}
	idxStr := strings.TrimSpace(idxOut)
	if idxStr == "" || idxStr == "0" {
		return nil, nil
	}
	idx, perr := strconv.ParseUint(idxStr, 10, 64)
	if perr != nil || idx == 0 {
		return nil, nil
	}
	connScript := fmt.Sprintf(
		`Get-NetTCPConnection -InterfaceIndex %d -State Established -ErrorAction SilentlyContinue | ForEach-Object { "$($_.RemoteAddress)|$($_.RemotePort)" }`,
		idx,
	)
	out, err := runPowerShell(connScript)
	if err != nil {
		return nil, nil // best-effort: empty list on error
	}
	var res []ActiveConn
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 2 {
			continue
		}
		ip := strings.TrimSpace(parts[0])
		port := strings.TrimSpace(parts[1])
		res = append(res, ActiveConn{RemoteAddr: ip, RemotePort: port, Hostname: reverseLookup(ip)})
	}
	return res, nil
}

// reverseLookup resolves an IP to a hostname best-effort with a short timeout.
func reverseLookup(ip string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer cancel()
	names, err := net.DefaultResolver.LookupAddr(ctx, ip)
	if err != nil || len(names) == 0 {
		return ""
	}
	return strings.TrimSuffix(names[0], ".")
}
