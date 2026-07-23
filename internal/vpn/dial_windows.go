//go:build windows

package vpn

import (
	"os/exec"
	"strings"
	"syscall"
)

// Connect dials the VPN via rasdial. Username and Password are optional: when
// both are empty/whitespace, rasdial uses the Windows-saved credentials; if
// either is provided, both are required. Password is passed as an argument (OS
// limitation) and is never logged by this package.
func Connect(p ConnectParams) error {
	if err := ValidateName(p.Name); err != nil {
		return err
	}
	user := strings.TrimSpace(p.Username)
	pass := p.Password

	// rasdial <entry> [<user> <pass>]
	var cmd *exec.Cmd
	if user == "" && pass == "" {
		cmd = exec.Command("rasdial.exe", p.Name)
	} else {
		if user == "" || pass == "" {
			return newUserError("validation", "Tidak dapat menghubungkan", "Username dan password wajib diisi bersama-sama.")
		}
		cmd = exec.Command("rasdial.exe", p.Name, user, pass)
	}
	// Hide console window on Windows.
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	text := string(out)
	// Exit code is authoritative: 0 means success, do not scan text.
	if err == nil {
		return nil
	}
	// Non-zero exit: treat as success only if output contains a known marker
	// (English or Indonesian), otherwise map the error.
	if evaluateRasdialResult(err, text) {
		return nil
	}
	return MapExecError("Connect", err, text)
}

// evaluateRasdialResult reports whether a rasdial invocation succeeded.
// A nil exit error means success. Otherwise success is detected only via known
// success markers (English or Indonesian) in the output.
func evaluateRasdialResult(exitErr error, text string) bool {
	if exitErr == nil {
		return true
	}
	lower := strings.ToLower(text)
	for _, marker := range []string{
		"successfully",
		"already connected",
		"berhasil",
		"sudah terhubung",
		"connected",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

// Disconnect hangs up the VPN via rasdial /DISCONNECT.
func Disconnect(name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	cmd := exec.Command("rasdial.exe", name, "/DISCONNECT")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	text := string(out)
	if err != nil {
		// Already disconnected is often a non-zero exit — treat soft.
		lower := strings.ToLower(text + " " + err.Error())
		if strings.Contains(lower, "not connected") ||
			strings.Contains(lower, "623") ||
			strings.Contains(lower, "no connections") {
			return nil
		}
		return MapExecError("Disconnect", err, text)
	}
	return nil
}
