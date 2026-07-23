//go:build windows
// +build windows

package ui

import (
	"strconv"

	"vepeen/internal/vpn"
)

// pingGateway pings the VPN gateway once using the native ICMP Win32 API and
// returns a human-readable status string in Indonesian. It never logs the host
// or any credentials.
func pingGateway(host string) string {
	if host == "" {
		return "tidak terhubung"
	}
	rtt, err := vpn.PingHost(host, 1000)
	if err != nil {
		return host + " — timeout / tidak ada balasan"
	}
	if rtt == 0 {
		return host + " — <1 ms"
	}
	return host + " — " + strconv.FormatUint(uint64(rtt), 10) + " ms"
}
