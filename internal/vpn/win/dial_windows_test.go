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
		{"exit0 failed-text but exit0", nil, "connection failed but exit 0", true},
		{"exitN successfully", errors.New("exit 1"), "The command completed successfully.", true},
		{"exitN already connected", errors.New("exit 1"), "Already connected.", true},
		{"exitN succeeded", errors.New("exit 1"), "connection successfully established", true},
		{"exitN already connected variant", errors.New("exit 1"), "you are already connected", true},
		{"exitN connected", errors.New("exit 1"), "you are connected", true},
		{"exitN failed only", errors.New("exit 1"), "failed to connect", false},
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
