package shared

import (
	"errors"
	"fmt"
	"strings"
)

// UserError is a sanitized, user-facing error.
// It must never contain PSK or password values.
type UserError struct {
	// Code is a stable machine-oriented category.
	Code string
	// Primary is the short status line.
	Primary string
	// Detail is optional secondary guidance.
	Detail string
}

func (e *UserError) Error() string {
	if e.Detail == "" {
		return e.Primary
	}
	return e.Primary + ": " + e.Detail
}

// NewUserError builds a sanitized, user-facing error. It is exported so the
// vpn orchestration layer and platform packages can construct UserErrors
// without importing implementation details.
func NewUserError(code, primary, detail string) *UserError {
	return &UserError{Code: code, Primary: primary, Detail: detail}
}

// AsUserError extracts a UserError if present.
func AsUserError(err error) (*UserError, bool) {
	var ue *UserError
	if errors.As(err, &ue) {
		return ue, true
	}
	return nil, false
}

// MapExecError converts process/tool failures into sanitized user messages.
func MapExecError(op string, err error, output string) error {
	if err == nil {
		return nil
	}
	msg := SanitizeOutput(output)
	lower := strings.ToLower(msg + " " + err.Error())

	switch {
	case strings.Contains(lower, "access is denied") ||
		strings.Contains(lower, "access denied") ||
		strings.Contains(lower, "elevat") ||
		strings.Contains(lower, "administrator") ||
		strings.Contains(lower, "0x80070005"):
		return NewUserError("elevation",
			"Failed to set up profile",
			"Administrator privileges may be required. Try running as a regular user first — per-user profiles are preferred.")
	case strings.Contains(lower, "734"):
		return NewUserError("ppp",
			"PPP negotiation failed (error 734)",
			"The server dropped the connection during authentication. Verify your username/password are correct and saved in Windows Credential Manager, or enter credentials in Vepeen. Also check the authentication method (MS-CHAPv2) and encryption in the Windows profile.")
	case strings.Contains(lower, "691") ||
		strings.Contains(lower, "authentication") ||
		strings.Contains(lower, "autentikasi") || // Bahasa Indonesia locale: "autentikasi" = "authentication"
		strings.Contains(lower, "logon failure") ||
		strings.Contains(lower, "username or password"):
		return NewUserError("auth",
			"Authentication failed",
			"Check your username/password or server policy. PSK is not shown.")
	case strings.Contains(lower, "623") ||
		strings.Contains(lower, "phone book") ||
		strings.Contains(lower, "cannot find") && strings.Contains(lower, "connection"):
		return NewUserError("profile",
			"Failed",
			"VPN profile not found. Try connecting again to recreate the profile.")
	case strings.Contains(lower, "789") ||
		strings.Contains(lower, "800") ||
		strings.Contains(lower, "809"):
		return NewUserError("ipsec",
			"Connection failed (L2TP/IPsec)",
			"Error 789/800/809: usually caused by NAT or IPsec settings. Vepeen attempts to set the NAT-T registry automatically (requires administrator privileges). Ensure UDP ports 500 and 4500 are not blocked by your firewall, and that the PSK is correct.")
	case strings.Contains(lower, "timeout") ||
		strings.Contains(lower, "timed out") ||
		strings.Contains(lower, "network"):
		return NewUserError("network",
			"Connection failed",
			"Check the server, your network, and UDP ports 500/4500 (L2TP/IPsec).")
	case strings.Contains(lower, "already") && strings.Contains(lower, "connect"):
		return NewUserError("already",
			"Connected",
			"Already connected.")
	default:
		detail := msg
		if detail == "" {
			detail = fmt.Sprintf("Operation %s failed.", op)
		}
		if len(detail) > 240 {
			detail = detail[:240] + "…"
		}
		return NewUserError("generic", "Failed", detail)
	}
}

// SanitizeOutput strips secret-bearing lines from tool output before it is
// surfaced to the user or logs.
func SanitizeOutput(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	// Drop lines that might echo secrets from verbose tools.
	var kept []string
	for _, line := range strings.Split(s, "\n") {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "l2tppsk") ||
			strings.Contains(lower, "-password") ||
			strings.Contains(lower, "password:") {
			continue
		}
		line = strings.TrimSpace(line)
		if line != "" {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, " ")
}

// ValidateName rejects empty or dangerous connection names.
func ValidateName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return NewUserError("validation", "Cannot connect", "Connection name is required.")
	}
	if strings.ContainsAny(name, "\r\n\x00\"'") {
		return NewUserError("validation", "Cannot connect", "Connection name contains invalid characters.")
	}
	return nil
}
