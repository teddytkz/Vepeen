//go:build !windows && !darwin
// +build !windows,!darwin

package ui

// pingGateway is unavailable on non-Windows builds.
func pingGateway(host string) string {
	return "not available (non-Windows)"
}
