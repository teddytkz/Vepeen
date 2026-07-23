//go:build !windows

package route

import "fmt"

// SyncRoutes is not supported outside Windows.
func SyncRoutes(connectionName string, desired []string) error {
	return fmt.Errorf("sinkronisasi rute VPN hanya didukung di Windows")
}
