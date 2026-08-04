//go:build !windows

package ui

// IsRunOnStartup is unsupported on non-Windows platforms in v1.
func IsRunOnStartup() bool { return false }

// SetRunOnStartup is unsupported on non-Windows platforms in v1.
func SetRunOnStartup(enabled bool) error { return nil }
