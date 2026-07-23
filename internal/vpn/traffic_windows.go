//go:build windows

package vpn

import (
	"fmt"
	"strconv"
	"strings"
)

// TrafficCounters returns cumulative received and transmitted bytes for the
// named VPN connection by reading its network adapter statistics. When the
// connection is not connected (or the interface cannot be resolved), it
// returns (0, 0, nil) without error. No secrets are embedded or logged.
func TrafficCounters(name string) (rxBytes, txBytes uint64, err error) {
	if err := ValidateName(name); err != nil {
		return 0, 0, err
	}

	// Resolve the VPN interface index. Empty/0 output means not connected.
	idxScript := fmt.Sprintf(
		`Get-VpnConnection -Name %s -ErrorAction SilentlyContinue | Select-Object -ExpandProperty InterfaceIndex`,
		psQuote(name),
	)
	idxOut, idxErr := runPowerShell(idxScript)
	if idxErr != nil {
		// If the connection simply isn't up, prefer 0,0,nil over an error.
		if strings.TrimSpace(idxOut) == "" {
			return 0, 0, nil
		}
		return 0, 0, MapExecError("TrafficCounters", idxErr, idxOut)
	}
	idxStr := strings.TrimSpace(idxOut)
	if idxStr == "" || idxStr == "0" {
		return 0, 0, nil
	}
	idx, perr := strconv.ParseUint(idxStr, 10, 64)
	if perr != nil || idx == 0 {
		return 0, 0, nil
	}

	// Read cumulative bytes from the adapter statistics.
	// Use Format-List so each value appears on its own "Name : value" line.
	// The table form produced by Select-Object (header + bare numbers) is not
	// parsed by parseTrafficStats, which only matches lines starting with the
	// property name, so rx/tx would always read 0.
	statScript := fmt.Sprintf(
		`Get-NetAdapterStatistics -InterfaceIndex %d | Format-List ReceivedBytes, SentBytes`,
		idx,
	)
	statOut, statErr := runPowerShell(statScript)
	if statErr != nil {
		return 0, 0, MapExecError("TrafficCounters", statErr, statOut)
	}
	rx, tx := parseTrafficStats(statOut)
	return rx, tx, nil
}

// parseTrafficStats scans PowerShell output for ReceivedBytes and SentBytes
// numeric values. It tolerates both table form (header + separator + value)
// and list form ("ReceivedBytes : 12345"). Missing values default to 0.
func parseTrafficStats(out string) (rx, tx uint64) {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "---") {
			continue
		}
		lower := strings.ToLower(line)
		switch {
		case strings.HasPrefix(lower, "receivedbytes"):
			rx = extractNumber(line)
		case strings.HasPrefix(lower, "sentbytes"):
			tx = extractNumber(line)
		}
	}
	return rx, tx
}

// extractNumber pulls the trailing integer from a "Label : 123" or "123" line.
func extractNumber(line string) uint64 {
	line = strings.TrimSpace(line)
	if i := strings.LastIndex(line, ":"); i >= 0 {
		line = strings.TrimSpace(line[i+1:])
	}
	line = strings.ReplaceAll(line, ",", "")
	line = strings.TrimSpace(line)
	n, err := strconv.ParseUint(line, 10, 64)
	if err != nil {
		return 0
	}
	return n
}
