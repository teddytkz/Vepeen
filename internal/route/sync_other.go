//go:build !windows && !darwin

package route

import "fmt"

// SyncRoutes is not supported outside Windows.
func SyncRoutes(connectionName string, desired []string) error {
	return fmt.Errorf("VPN route sync is only supported on Windows")
}
