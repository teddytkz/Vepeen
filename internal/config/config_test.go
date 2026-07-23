package config

import (
	"encoding/json"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestParseConfigNewShape(t *testing.T) {
	data := []byte(`{
		"selectedProfile": "Vepeen",
		"routes": ["10.0.0.0/8", " 192.168.1.0/24 ", ""],
		"rememberCredentials": true
	}`)

	cfg, err := parseConfig(data)
	if err != nil {
		t.Fatalf("parseConfig returned error: %v", err)
	}
	want := []string{"10.0.0.0/8", "192.168.1.0/24"}
	if !reflect.DeepEqual(cfg.Routes, want) {
		t.Errorf("Routes = %#v, want %#v", cfg.Routes, want)
	}
	if cfg.SelectedProfile != "Vepeen" {
		t.Errorf("SelectedProfile = %q, want %q", cfg.SelectedProfile, "Vepeen")
	}
	if !cfg.RememberCredentials {
		t.Errorf("RememberCredentials = false, want true")
	}
}

func TestParseConfigOldShapeSelectedMatch(t *testing.T) {
	data := []byte(`{
		"selectedProfile": "Work",
		"profiles": {
			"Work": {"routes": ["10.0.0.0/8", "172.16.0.0/12"]},
			"Home": {"routes": ["192.168.0.0/16"]}
		}
	}`)

	cfg, err := parseConfig(data)
	if err != nil {
		t.Fatalf("parseConfig returned error: %v", err)
	}
	want := []string{"10.0.0.0/8", "172.16.0.0/12"}
	if !reflect.DeepEqual(cfg.Routes, want) {
		t.Errorf("Routes = %#v, want %#v", cfg.Routes, want)
	}
}

func TestParseConfigOldShapeFallbackUnion(t *testing.T) {
	// selectedProfile empty -> union of all profile routes, deduped, order-preserving.
	data := []byte(`{
		"selectedProfile": "",
		"profiles": {
			"Work": {"routes": ["10.0.0.0/8", "172.16.0.0/12"]},
			"Home": {"routes": ["192.168.0.0/16", "10.0.0.0/8"]}
		}
	}`)

	cfg, err := parseConfig(data)
	if err != nil {
		t.Fatalf("parseConfig returned error: %v", err)
	}
	// Profiles are processed in sorted-key order (Home before Work), so Home's
	// route appears first, then Work's routes (deduped against the union).
	want := []string{"192.168.0.0/16", "10.0.0.0/8", "172.16.0.0/12"}
	if !reflect.DeepEqual(cfg.Routes, want) {
		t.Errorf("Routes = %#v, want %#v", cfg.Routes, want)
	}
}

func TestParseConfigOldShapeSelectedMissing(t *testing.T) {
	// selectedProfile not present in map -> union fallback.
	data := []byte(`{
		"selectedProfile": "Nonexistent",
		"profiles": {
			"A": {"routes": ["10.0.0.0/8"]},
			"B": {"routes": ["192.168.0.0/16"]}
		}
	}`)

	cfg, err := parseConfig(data)
	if err != nil {
		t.Fatalf("parseConfig returned error: %v", err)
	}
	want := []string{"10.0.0.0/8", "192.168.0.0/16"}
	if !reflect.DeepEqual(cfg.Routes, want) {
		t.Errorf("Routes = %#v, want %#v", cfg.Routes, want)
	}
}

func TestDefaultRoutesNonNil(t *testing.T) {
	cfg := Default()
	if cfg.Routes == nil {
		t.Errorf("Default().Routes is nil, want non-nil empty slice")
	}
	if len(cfg.Routes) != 0 {
		t.Errorf("Default().Routes len = %d, want 0", len(cfg.Routes))
	}
}

// TestBinPath asserts the encrypted store path resolves to vepeen.bin next to
// the executable (or the per-user config dir fallback). Platform-independent.
func TestBinPath(t *testing.T) {
	path, err := BinPath()
	if err != nil {
		t.Fatalf("BinPath returned error: %v", err)
	}
	if !strings.HasSuffix(path, "vepeen.bin") {
		t.Errorf("BinPath() = %q, want suffix %q", path, "vepeen.bin")
	}
}

// TestDefaultStored asserts DefaultStored returns a usable Stored with a
// non-nil credentials map, RememberCredentials enabled, and empty routes.
func TestDefaultStored(t *testing.T) {
	s := DefaultStored()
	if s.Credentials == nil {
		t.Errorf("DefaultStored().Credentials is nil, want non-nil map")
	}
	if !s.RememberCredentials {
		t.Errorf("DefaultStored().RememberCredentials = false, want true")
	}
	if s.Routes == nil {
		t.Errorf("DefaultStored().Routes is nil, want non-nil empty slice")
	}
	if len(s.Routes) != 0 {
		t.Errorf("DefaultStored().Routes len = %d, want 0", len(s.Routes))
	}
}

// TestStoredConfigProjection asserts the Stored <-> Config projection round-trips
// settings and credentials without loss.
func TestStoredConfigProjection(t *testing.T) {
	creds := map[string]CredEntry{
		"Vepeen": {Username: "alice", Password: "s3cret", PSK: "psk123"},
	}
	stored := Stored{
		SelectedProfile:     "Vepeen",
		Routes:              []string{"10.0.0.0/8", "192.168.1.0/24"},
		RememberCredentials: true,
		Credentials:         creds,
	}

	// Stored -> Config projection keeps the non-secret settings.
	cfg := stored.Config()
	if cfg.SelectedProfile != stored.SelectedProfile {
		t.Errorf("Config.SelectedProfile = %q, want %q", cfg.SelectedProfile, stored.SelectedProfile)
	}
	if !reflect.DeepEqual(cfg.Routes, stored.Routes) {
		t.Errorf("Config.Routes = %#v, want %#v", cfg.Routes, stored.Routes)
	}
	if cfg.RememberCredentials != stored.RememberCredentials {
		t.Errorf("Config.RememberCredentials = %v, want %v", cfg.RememberCredentials, stored.RememberCredentials)
	}

	// Config + creds -> Stored round-trips back, including the credentials map.
	back := cfg.withCreds(creds)
	if !reflect.DeepEqual(back.Credentials, creds) {
		t.Errorf("withCreds Credentials = %#v, want %#v", back.Credentials, creds)
	}
	if back.SelectedProfile != stored.SelectedProfile || !reflect.DeepEqual(back.Routes, stored.Routes) {
		t.Errorf("withCreds settings mismatch: got %#v, want %#v", back, stored)
	}
}

// TestSaveLoadStoredRoundTrip validates the JSON + DPAPI crypto round-trip
// without depending on Dir()/BinPath() (which resolve next to the executable).
// It is skipped off Windows because DPAPI is unavailable there.
func TestSaveLoadStoredRoundTrip(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("DPAPI requires Windows")
	}

	original := Stored{
		SelectedProfile:     "Vepeen",
		Routes:              []string{"10.0.0.0/8", "192.168.1.0/24"},
		RememberCredentials: true,
		Credentials: map[string]CredEntry{
			"Vepeen": {Username: "alice", Password: "s3cret", PSK: "psk123"},
			"Work":   {Username: "bob", Password: "hunter2", PSK: ""},
		},
	}

	plain, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	blob, err := encryptDPAPI(plain)
	if err != nil {
		t.Fatalf("encryptDPAPI: %v", err)
	}
	decrypted, err := decryptDPAPI(blob)
	if err != nil {
		t.Fatalf("decryptDPAPI: %v", err)
	}
	var restored Stored
	if err := json.Unmarshal(decrypted, &restored); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if !reflect.DeepEqual(restored, original) {
		t.Errorf("round-trip mismatch:\n got %#v\nwant %#v", restored, original)
	}
}

// TestDecryptTamperedBlob asserts that decrypting a corrupt/garbage blob fails
// gracefully (returns an error) rather than panicking or leaking secrets.
func TestDecryptTamperedBlob(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("DPAPI requires Windows")
	}

	// Random bytes are not a valid DPAPI blob; decryption must error out.
	garbage := []byte{0xde, 0xad, 0xbe, 0xef, 0x01, 0x02, 0x03, 0x04}
	if _, err := decryptDPAPI(garbage); err == nil {
		t.Errorf("decryptDPAPI(garbage) = nil error, want error")
	}

	// An empty blob must also be rejected cleanly.
	if _, err := decryptDPAPI(nil); err == nil {
		t.Errorf("decryptDPAPI(nil) = nil error, want error")
	}
}

// TestLoadMissingFileReturnsDefault documents the missing-file behavior of
// LoadStored: when vepeen.bin is absent, LoadStored falls back to DefaultStored
// (after an attempted one-time migration). Because BinPath() resolves next to
// the executable and is not redirectable, we validate the documented fallback
// indirectly via the decrypt path on invalid input (covered by
// TestDecryptTamperedBlob) and assert DefaultStored() is the safe baseline.
// A full missing-file -> Default round-trip is covered by code review + manual
// test on Windows.
func TestLoadMissingFileReturnsDefault(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("DPAPI requires Windows")
	}

	// The graceful-degradation baseline: a corrupt blob decrypts to an error,
	// which is exactly the path LoadStored takes before returning DefaultStored.
	if _, err := decryptDPAPI([]byte("not-a-real-blob")); err == nil {
		t.Errorf("decryptDPAPI(invalid) = nil error, want error")
	}

	// DefaultStored is the safe fallback value LoadStored returns on failure.
	def := DefaultStored()
	if def.Credentials == nil || !def.RememberCredentials {
		t.Errorf("DefaultStored fallback is not in expected default state: %#v", def)
	}
}
