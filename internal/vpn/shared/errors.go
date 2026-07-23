package shared

import (
	"errors"
	"fmt"
	"strings"
)

// UserError is a sanitized, user-facing error (Indonesian where practical).
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
			"Gagal menyiapkan profil",
			"Mungkin diperlukan hak administrator. Coba jalankan sebagai user biasa dulu; profil per-user lebih disarankan.")
	case strings.Contains(lower, "734"):
		return NewUserError("ppp",
			"Gagal negosiasi PPP (error 734)",
			"Server memutus koneksi saat verifikasi. Pastikan username/password benar dan tersimpan di Windows Credential Manager, atau isi kredensial di Vepeen. Periksa juga metode autentikasi (MS-CHAPv2) dan enkripsi di profil Windows.")
	case strings.Contains(lower, "691") ||
		strings.Contains(lower, "authentication") ||
		strings.Contains(lower, "autentikasi") ||
		strings.Contains(lower, "logon failure") ||
		strings.Contains(lower, "username or password"):
		return NewUserError("auth",
			"Gagal autentikasi",
			"Periksa username/password atau kebijakan server. PSK tidak ditampilkan.")
	case strings.Contains(lower, "623") ||
		strings.Contains(lower, "phone book") ||
		strings.Contains(lower, "cannot find") && strings.Contains(lower, "connection"):
		return NewUserError("profile",
			"Gagal",
			"Profil VPN tidak ditemukan. Coba Hubungkan lagi untuk membuat profil.")
	case strings.Contains(lower, "789") ||
		strings.Contains(lower, "800") ||
		strings.Contains(lower, "809"):
		return NewUserError("ipsec",
			"Gagal terhubung (L2TP/IPsec)",
			"Kesalahan 789/800/809: biasanya karena NAT atau pengaturan IPsec. Vepeen mencoba mengatur registri NAT-T secara otomatis (perlu hak administrator). Pastikan port UDP 500 dan 4500 tidak diblokir firewall, dan PSK sudah benar.")
	case strings.Contains(lower, "timeout") ||
		strings.Contains(lower, "timed out") ||
		strings.Contains(lower, "network"):
		return NewUserError("network",
			"Gagal terhubung",
			"Periksa server, jaringan, dan port UDP 500/4500 (L2TP/IPsec).")
	case strings.Contains(lower, "already") && strings.Contains(lower, "connect"):
		return NewUserError("already",
			"Terhubung",
			"Sudah terhubung.")
	default:
		detail := msg
		if detail == "" {
			detail = fmt.Sprintf("Operasi %s gagal.", op)
		}
		if len(detail) > 240 {
			detail = detail[:240] + "…"
		}
		return NewUserError("generic", "Gagal", detail)
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
		return NewUserError("validation", "Tidak dapat menghubungkan", "Nama koneksi wajib diisi.")
	}
	if strings.ContainsAny(name, "\r\n\x00\"'") {
		return NewUserError("validation", "Tidak dapat menghubungkan", "Nama koneksi mengandung karakter tidak valid.")
	}
	return nil
}
