//go:build windows

package win

import (
	"errors"
	"testing"
)

func TestEvaluateRasdialResult(t *testing.T) {
	cases := []struct {
		name    string
		exitErr error
		text    string
		want    bool
	}{
		{"exit0 any text", nil, "command completed successfully", true},
		{"exit0 empty", nil, "", true},
		{"exit0 gagal text", nil, "koneksi gagal tapi exit 0", true},
		{"exitN successfully", errors.New("exit 1"), "The command completed successfully.", true},
		{"exitN already connected", errors.New("exit 1"), "Already connected.", true},
		{"exitN berhasil", errors.New("exit 1"), "koneksi berhasil dibuat", true},
		{"exitN sudah terhubung", errors.New("exit 1"), "anda sudah terhubung", true},
		{"exitN connected", errors.New("exit 1"), "you are connected", true},
		{"exitN gagal only", errors.New("exit 1"), "gagal menghubungkan", false},
		{"exitN empty", errors.New("exit 1"), "", false},
		{"exitN random", errors.New("exit 1"), "some other failure", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := evaluateRasdialResult(c.exitErr, c.text); got != c.want {
				t.Errorf("evaluateRasdialResult(%v, %q) = %v, want %v", c.exitErr, c.text, got, c.want)
			}
		})
	}
}
