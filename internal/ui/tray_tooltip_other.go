//go:build !windows

package ui

// setTrayTooltip is a no-op on non-Windows platforms.
func setTrayTooltip(string) {}
