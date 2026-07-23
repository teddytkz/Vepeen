//go:build windows

package win

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// runPowerShell executes a PowerShell command string without embedding secrets in Go logs.
// Callers must not log the command if it contains secrets.
func runPowerShell(command string) (string, error) {
	cmd := exec.Command("powershell.exe",
		"-NoProfile",
		"-NonInteractive",
		"-ExecutionPolicy", "Bypass",
		"-Command", command,
	)
	// Hide console window on Windows.
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		return text, err
	}
	return text, nil
}

// PurgeOrphanScripts best-effort deletes leftover vpn-*.ps1 under %TEMP%\vepeen.
// Safe to call on app start and before writing a new script. Never logs contents.
func PurgeOrphanScripts() {
	dir := filepath.Join(os.TempDir(), "vepeen")
	matches, err := filepath.Glob(filepath.Join(dir, "vpn-*.ps1"))
	if err != nil {
		return
	}
	for _, p := range matches {
		_ = os.Remove(p)
	}
}

// runPowerShellScript writes a short-lived UTF-8 BOM script and executes it.
// Used when the script may contain PSK so we can restrict ACL and delete ASAP.
// The script content is never returned to callers for logging.
func runPowerShellScript(scriptBody string) (string, error) {
	PurgeOrphanScripts()

	dir := filepath.Join(os.TempDir(), "vepeen")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("temp dir: %w", err)
	}
	name := fmt.Sprintf("vpn-%d.ps1", time.Now().UnixNano())
	path := filepath.Join(dir, name)

	// UTF-8 BOM helps Windows PowerShell 5.1 parse Unicode.
	body := append([]byte{0xEF, 0xBB, 0xBF}, []byte(scriptBody)...)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		return "", fmt.Errorf("write script: %w", err)
	}
	defer func() {
		_ = os.Remove(path)
	}()

	// Restrict ACL to current user (best-effort; ignore failure on non-NTFS).
	_, _ = runPowerShell(fmt.Sprintf(
		`icacls %s /inheritance:r /grant:r "$($env:USERNAME):(R)" 2>$null | Out-Null`,
		psQuote(path),
	))

	cmd := exec.Command("powershell.exe",
		"-NoProfile",
		"-NonInteractive",
		"-ExecutionPolicy", "Bypass",
		"-File", path,
	)
	// Hide console window on Windows.
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		return text, err
	}
	return text, nil
}

func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func psDoubleQuote(s string) string {
	// Escape for double-quoted PowerShell strings.
	r := strings.ReplaceAll(s, "`", "``")
	r = strings.ReplaceAll(r, `"`, ``+"`"+`"`)
	r = strings.ReplaceAll(r, "$", "`$")
	return `"` + r + `"`
}
