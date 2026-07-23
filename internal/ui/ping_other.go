//go:build !windows
// +build !windows

package ui

// pingGateway is unavailable on non-Windows builds.
func pingGateway(host string) string {
	return "tidak tersedia (bukan Windows)"
}
