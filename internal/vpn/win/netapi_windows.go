//go:build windows

package win

import (
	"encoding/binary"
	"fmt"
	"net"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Shared Win32 declarations and helpers for iphlpapi.dll. These back the
// traffic, connection, and ping monitoring functions so we avoid spawning
// periodic PowerShell/exec subprocesses.

var (
	modIPHelper             = windows.NewLazySystemDLL("iphlpapi.dll")
	procGetIfEntry2         = modIPHelper.NewProc("GetIfEntry2")
	procGetExtendedTcpTable = modIPHelper.NewProc("GetExtendedTcpTable")
	procIcmpCreateFile      = modIPHelper.NewProc("IcmpCreateFile")
	procIcmpSendEcho        = modIPHelper.NewProc("IcmpSendEcho")
	procIcmpCloseHandle     = modIPHelper.NewProc("IcmpCloseHandle")
)

// Win32 constants used by the iphlpapi calls below.
const (
	afInet                      = windows.AF_INET
	tcpTableOwnerPidConnections = 4 // TCP_TABLE_OWNER_PID_CONNECTIONS
	mibTcpStateEstab            = 5 // MIB_TCP_STATE_ESTAB
	icmpDefaultTimeoutMs        = 1000
)

// mibTcpRowOwnerPid mirrors MIB_TCPROW_OWNER_PID. Address/port fields are in
// network byte order; only the low 16 bits of each port are meaningful.
type mibTcpRowOwnerPid struct {
	DwState      uint32
	DwLocalAddr  uint32
	DwLocalPort  uint32
	DwRemoteAddr uint32
	DwRemotePort uint32
	DwOwningPid  uint32
}

// icmpOptions mirrors IP_OPTION_INFORMATION (only the fields we reference).
type icmpOptions struct {
	Ttl         uint8
	Tos         uint8
	Flags       uint8
	OptionsSize uint8
	OptionsData uintptr
}

// icmpEchoReply mirrors ICMP_ECHO_REPLY.
type icmpEchoReply struct {
	Address       uint32 // replying IP (network byte order)
	Status        uint32 // 0 = success (IP_SUCCESS)
	RoundTripTime uint32 // milliseconds
	DataSize      uint16
	Reserved      uint16
	Data          uintptr
	Options       icmpOptions
}

// resolveVPNInterface finds the network adapter whose FriendlyName matches
// vpnName and returns its interface index plus all unicast IPv4 addresses.
// When the adapter is not present (not connected), it returns (0, nil, nil) —
// this is not treated as an error by callers.
func resolveVPNInterface(vpnName string) (ifIndex uint32, localIPs []net.IP, err error) {
	if vpnName == "" {
		return 0, nil, nil
	}

	var bufSize uint32 = 16 * 1024
	buf := make([]byte, bufSize)
	for {
		err := windows.GetAdaptersAddresses(
			windows.AF_UNSPEC, 0, 0,
			(*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0])),
			&bufSize,
		)
		if err == windows.ERROR_BUFFER_OVERFLOW {
			buf = make([]byte, bufSize)
			continue
		}
		if err != nil {
			return 0, nil, fmt.Errorf("GetAdaptersAddresses: %w", err)
		}
		break
	}

	for addr := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0])); addr != nil; addr = addr.Next {
		if addr.FriendlyName == nil {
			continue
		}
		if windows.UTF16PtrToString(addr.FriendlyName) != vpnName {
			continue
		}
		ifIndex = addr.IfIndex
		for ua := addr.FirstUnicastAddress; ua != nil; ua = ua.Next {
			ip := ua.Address.IP()
			if ip4 := ip.To4(); ip4 != nil {
				localIPs = append(localIPs, ip4)
			}
		}
		// FriendlyName is unique enough; stop at first match.
		return ifIndex, localIPs, nil
	}
	return 0, nil, nil
}

// InterfaceInfo finds the network adapter whose FriendlyName matches name and
// returns its interface index plus all IPv4 unicast addresses with their on-link
// prefix length expressed as a net.IPNet (IP + subnet mask). When name is empty
// or the adapter is not present (not connected), it returns (0, nil, nil) — this
// is not treated as an error by callers, mirroring resolveVPNInterface and
// TrafficCounters graceful degradation.
func InterfaceInfo(name string) (ifIndex uint32, addrs []net.IPNet, err error) {
	if name == "" {
		return 0, nil, nil
	}

	var bufSize uint32 = 16 * 1024
	buf := make([]byte, bufSize)
	for {
		err := windows.GetAdaptersAddresses(
			windows.AF_UNSPEC, 0, 0,
			(*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0])),
			&bufSize,
		)
		if err == windows.ERROR_BUFFER_OVERFLOW {
			buf = make([]byte, bufSize)
			continue
		}
		if err != nil {
			return 0, nil, fmt.Errorf("GetAdaptersAddresses: %w", err)
		}
		break
	}

	for addr := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0])); addr != nil; addr = addr.Next {
		if addr.FriendlyName == nil {
			continue
		}
		if windows.UTF16PtrToString(addr.FriendlyName) != name {
			continue
		}
		ifIndex = addr.IfIndex
		for ua := addr.FirstUnicastAddress; ua != nil; ua = ua.Next {
			ip := ua.Address.IP()
			if ip4 := ip.To4(); ip4 != nil {
				addrs = append(addrs, net.IPNet{
					IP:   ip4,
					Mask: net.CIDRMask(int(ua.OnLinkPrefixLength), 32),
				})
			}
		}
		// FriendlyName is unique enough; stop at first match.
		return ifIndex, addrs, nil
	}
	return 0, nil, nil
}

// getIfEntry2 fills a MIB_IF_ROW2 for the given interface index and returns the
// cumulative received and transmitted octets. It uses the raw GetIfEntry2 proc
// (GetIfEntry2Ex is wrapped by x/sys but GetIfEntry2 is not).
func getIfEntry2(ifIndex uint32) (inOctets, outOctets uint64, err error) {
	var row windows.MibIfRow2
	row.InterfaceIndex = ifIndex
	r, _, e := procGetIfEntry2.Call(uintptr(unsafe.Pointer(&row)))
	if r != 0 {
		return 0, 0, fmt.Errorf("GetIfEntry2: %w", e)
	}
	return row.InOctets, row.OutOctets, nil
}

// ipToUint32 converts an IPv4 address to the uint32 representation used by the
// native iphlpapi structs. Windows stores IPv4 addresses in network byte order
// in memory; on a little-endian host Go therefore reads the field as a native
// (little-endian) uint32. We replicate that exact interpretation so values
// produced here match what GetExtendedTcpTable / IcmpSendEcho expect.
func ipToUint32(ip net.IP) uint32 {
	v4 := ip.To4()
	if v4 == nil {
		return 0
	}
	return binary.LittleEndian.Uint32(v4)
}

// ntohs converts a 16-bit value from network to host byte order by swapping
// its high and low bytes.
func ntohs(v uint16) uint16 {
	return (v >> 8) | (v << 8)
}

// PingHost sends a single ICMP echo to host and returns the round-trip time in
// milliseconds. A zero RTT with a nil error means the host replied in under one
// millisecond. Any failure (unreachable, timeout, bad status) returns an error.
func PingHost(host string, timeoutMs uint32) (rttMs uint32, err error) {
	if timeoutMs == 0 {
		timeoutMs = icmpDefaultTimeoutMs
	}
	ra, err := net.ResolveIPAddr("ip4", host)
	if err != nil {
		return 0, fmt.Errorf("resolve %q: %w", host, err)
	}
	dest := ipToUint32(ra.IP)
	if dest == 0 {
		return 0, fmt.Errorf("invalid IPv4 address for %q", host)
	}

	handle, _, e := procIcmpCreateFile.Call()
	if handle == 0 || handle == ^uintptr(0) { // INVALID_HANDLE_VALUE
		return 0, fmt.Errorf("IcmpCreateFile: %w", e)
	}
	defer procIcmpCloseHandle.Call(handle)

	sendData := []byte("vepeen-ping")
	replySize := int(unsafe.Sizeof(icmpEchoReply{})) + len(sendData) + 8
	replyBuf := make([]byte, replySize)

	r, _, e := procIcmpSendEcho.Call(
		handle,
		uintptr(dest),
		uintptr(unsafe.Pointer(&sendData[0])),
		uintptr(len(sendData)),
		0, // no IP options
		uintptr(unsafe.Pointer(&replyBuf[0])),
		uintptr(replySize),
		uintptr(timeoutMs),
	)
	if r == 0 {
		return 0, fmt.Errorf("IcmpSendEcho: %w", e)
	}

	reply := (*icmpEchoReply)(unsafe.Pointer(&replyBuf[0]))
	if reply.Status != 0 {
		return 0, fmt.Errorf("icmp status %d", reply.Status)
	}
	return reply.RoundTripTime, nil
}
