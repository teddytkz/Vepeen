package ui

import (
	"bytes"
	"crypto/sha256"
	"image/png"
	"os"
	"testing"
)

// The bundled icon must be byte-identical to the generated master PNG, and must
// decode as a real image (a truncated/escaped bundle would still compile).
func TestVepeenIconMatchesMaster(t *testing.T) {
	want, err := os.ReadFile("../../assets/brand/icons/icon_1024.png")
	if err != nil {
		t.Skip("master icon not present")
	}
	got := VepeenIcon.StaticContent
	if !bytes.Equal(got, want) {
		t.Fatalf("bundle differs from master: sha %x vs %x (len %d vs %d)",
			sha256.Sum256(got), sha256.Sum256(want), len(got), len(want))
	}
	img, err := png.Decode(bytes.NewReader(got))
	if err != nil {
		t.Fatalf("bundled icon is not decodable PNG: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 1024 || b.Dy() != 1024 {
		t.Fatalf("icon is %dx%d, want 1024x1024", b.Dx(), b.Dy())
	}
}
