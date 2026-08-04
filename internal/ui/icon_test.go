package ui

import (
	"bytes"
	"crypto/sha256"
	"image/png"
	"os"
	"testing"
)

// The bundled icon must be byte-identical to the master PNG, and must decode as
// a real image (a truncated/escaped bundle would still compile).
func TestPenelopeIconMatchesMaster(t *testing.T) {
	want, err := os.ReadFile("../../docs/images/penelope.png")
	if err != nil {
		t.Skip("master icon not present")
	}
	got := PenelopeIcon.StaticContent
	if !bytes.Equal(got, want) {
		t.Fatalf("bundle differs from master: sha %x vs %x (len %d vs %d)",
			sha256.Sum256(got), sha256.Sum256(want), len(got), len(want))
	}
	img, err := png.Decode(bytes.NewReader(got))
	if err != nil {
		t.Fatalf("bundled icon is not decodable PNG: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 736 || b.Dy() != 730 {
		t.Fatalf("icon is %dx%d, want 736x730", b.Dx(), b.Dy())
	}
}
