//go:build windows

package win

import (
	"context"
	"encoding/binary"
	"net"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"vepeen/internal/vpn/shared"
)

// ActiveConnections returns established TCP connections whose local address
// belongs to the named VPN connection's interface. Remote hostnames are
// resolved best-effort via reverse DNS with a short timeout (falls back to IP).
// Returns an empty slice (no error) when the connection is not up or has no
// established sockets.
func ActiveConnections(name string) ([]shared.ActiveConn, error) {
	if err := shared.ValidateName(name); err != nil {
		return nil, err
	}

	ifIndex, localIPs, err := resolveVPNInterface(name)
	if err != nil {
		return nil, nil // best-effort: empty list on error
	}
	if ifIndex == 0 || len(localIPs) == 0 {
		return nil, nil
	}

	// Build a set of local IPv4 addresses (network byte order) for fast lookup.
	localSet := make(map[uint32]struct{}, len(localIPs))
	for _, ip := range localIPs {
		localSet[ipToUint32(ip)] = struct{}{}
	}

	// First call with nil buffer to learn the required table size.
	var size uint32
	r, _, _ := procGetExtendedTcpTable.Call(
		0, uintptr(unsafe.Pointer(&size)), 0, uintptr(afInet),
		uintptr(tcpTableOwnerPidConnections), 0,
	)
	if r != 0 && r != uintptr(windows.ERROR_INSUFFICIENT_BUFFER) {
		return nil, nil // best-effort: empty list on error
	}
	if size == 0 {
		return nil, nil
	}

	buf := make([]byte, size)
	r, _, _ = procGetExtendedTcpTable.Call(
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)), 0,
		uintptr(afInet), uintptr(tcpTableOwnerPidConnections), 0,
	)
	if r != 0 {
		return nil, nil // best-effort: empty list on error
	}

	if len(buf) < 4 {
		return nil, nil
	}
	numEntries := binary.LittleEndian.Uint32(buf[0:4])
	rowSize := int(unsafe.Sizeof(mibTcpRowOwnerPid{}))
	offset := 4

	var res []shared.ActiveConn
	for i := uint32(0); i < numEntries && offset+rowSize <= len(buf); i++ {
		row := (*mibTcpRowOwnerPid)(unsafe.Pointer(&buf[offset]))
		offset += rowSize
		if row.DwState != mibTcpStateEstab {
			continue
		}
		if _, ok := localSet[row.DwLocalAddr]; !ok {
			continue
		}
		ip := net.IPv4(
			byte(row.DwRemoteAddr),
			byte(row.DwRemoteAddr>>8),
			byte(row.DwRemoteAddr>>16),
			byte(row.DwRemoteAddr>>24),
		).String()
		port := strconv.FormatUint(uint64(ntohs(uint16(row.DwRemotePort))), 10)
		res = append(res, shared.ActiveConn{
			RemoteAddr: ip,
			RemotePort: port,
			Hostname:   reverseLookup(ip),
		})
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
