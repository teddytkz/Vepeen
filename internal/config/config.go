// Package config loads and saves non-secret VPN settings.
// PSK and password must never be written to the config file.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"vepeen/internal/secrets"
)

const (
	// DefaultConnectionName is the Windows VPN phonebook entry name.
	DefaultConnectionName = "Vepeen"
	appDirName            = "vepeen"
	configFileName        = "config.json"
	// binFileName is the encrypted, single-file store (DPAPI user-scope) that
	// replaces the legacy config.json + Windows Credential Manager setup.
	binFileName = "vepeen.bin"
)

// Config holds non-secret settings persisted as JSON. The app no longer creates
// VPN profiles or stores PSK; it only manages split-tunnel routing for a
// selected, pre-existing Windows VPN connection.
type Config struct {
	SelectedProfile     string   `json:"selectedProfile"`
	Routes              []string `json:"routes"`
	RememberCredentials bool     `json:"rememberCredentials"`
	RouteAllTraffic     bool     `json:"routeAllTraffic"`
}

// Stored is the full persisted state: non-secret settings plus per-profile
// secrets. It is JSON-marshaled and then DPAPI-encrypted into vepeen.bin.
type Stored struct {
	SelectedProfile     string               `json:"selectedProfile"`
	Routes              []string             `json:"routes"`
	RememberCredentials bool                 `json:"rememberCredentials"`
	RouteAllTraffic     bool                 `json:"routeAllTraffic"`
	Credentials         map[string]CredEntry `json:"credentials"` // keyed by profile name
}

// CredEntry holds the secrets for a single VPN profile. PSK is stored for
// forward compatibility even though the UI does not collect it yet.
type CredEntry struct {
	Username string `json:"username"`
	Password string `json:"password"`
	PSK      string `json:"psk"`
}

// Default returns a fresh config with product defaults.
func Default() Config {
	return Config{
		SelectedProfile:     "",
		Routes:              []string{},
		RememberCredentials: true,
		RouteAllTraffic:     false,
	}
}

// DefaultStored returns a fresh Stored with an initialized (non-nil) credentials map.
func DefaultStored() Stored {
	return Stored{
		SelectedProfile:     "",
		Routes:              []string{},
		RememberCredentials: true,
		RouteAllTraffic:     false,
		Credentials:         map[string]CredEntry{},
	}
}

// Config returns the non-secret settings projection of Stored.
func (s Stored) Config() Config {
	return Config{
		SelectedProfile:     s.SelectedProfile,
		Routes:              s.Routes,
		RememberCredentials: s.RememberCredentials,
		RouteAllTraffic:     s.RouteAllTraffic,
	}
}

// withCreds builds a Stored from a Config plus a credentials map.
func (c Config) withCreds(creds map[string]CredEntry) Stored {
	return Stored{
		SelectedProfile:     c.SelectedProfile,
		Routes:              c.Routes,
		RememberCredentials: c.RememberCredentials,
		RouteAllTraffic:     c.RouteAllTraffic,
		Credentials:         creds,
	}
}

// Dir returns the directory holding config.json. We prefer the directory of the
// running executable so the app is portable (config.json sits next to vepeen.exe).
// If the executable path cannot be determined, fall back to the per-user config dir.
func Dir() (string, error) {
	// macOS: always the per-user config dir (~/Library/Application Support/vepeen).
	// The exe-adjacent store below is a Windows portable-app choice; on macOS the
	// binary path is unstable (app bundles, `go run` temp dirs), which silently
	// lost saved routes/credentials every launch.
	if runtime.GOOS != "darwin" {
		if exe, err := os.Executable(); err == nil {
			if dir := filepath.Dir(exe); dir != "" {
				return dir, nil
			}
		}
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve config dir: %w", err)
	}
	return filepath.Join(base, appDirName), nil
}

// legacyPath returns the old %AppData%\vepeen\config.json location, used only
// for a one-time migration to the new executable-adjacent location.
func legacyPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, appDirName, configFileName), nil
}

// Path returns the full path to config.json (legacy; used only for migration).
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configFileName), nil
}

// BinPath returns the full path to the encrypted store vepeen.bin, located next
// to the executable (or in the per-user config dir as a fallback).
func BinPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, binFileName), nil
}

// LoadStored reads the encrypted store (vepeen.bin). A missing file triggers a
// one-time migration from legacy sources (config.json + Credential Manager) and
// returns the migrated Stored. A corrupt or unreadable blob degrades gracefully
// to DefaultStored() — the app keeps running, just without persisted secrets.
func LoadStored() (Stored, error) {
	path, err := BinPath()
	if err != nil {
		return DefaultStored(), err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// No encrypted store yet: attempt one-time migration from legacy sources.
			stored, migrated, merr := migrateLegacy()
			if merr != nil {
				log.Printf("config: legacy migration skipped: %v", merr)
			}
			if migrated {
				return stored, nil
			}
			return DefaultStored(), nil
		}
		return DefaultStored(), fmt.Errorf("read config: %w", err)
	}
	plain, derr := decryptDPAPI(data)
	if derr != nil {
		// Corrupt/unreadable blob: degrade gracefully (never panic, never log secrets).
		log.Printf("config: failed to decrypt %s; using defaults", binFileName)
		return DefaultStored(), nil
	}
	var stored Stored
	if uerr := json.Unmarshal(plain, &stored); uerr != nil {
		log.Printf("config: failed to parse %s; using defaults", binFileName)
		return DefaultStored(), nil
	}
	if stored.Credentials == nil {
		stored.Credentials = map[string]CredEntry{}
	}
	return stored, nil
}

// Load reads the non-secret settings projection of the encrypted store.
// Retained for backward compatibility with callers that only need settings
// (e.g. the UI before it migrates to the Stored API in Phase 3).
func Load() (Config, error) {
	stored, err := LoadStored()
	return stored.Config(), err
}

// parseConfig unmarshals config JSON, migrating legacy shapes when needed.
// configWire mirrors Config but uses *bool for RememberCredentials so a missing
// key can be distinguished from an explicit false (missing defaults to true).
type configWire struct {
	SelectedProfile     string   `json:"selectedProfile"`
	Routes              []string `json:"routes"`
	RememberCredentials *bool    `json:"rememberCredentials"`
}

// oldShapeWire detects the old per-profile config shape during migration.
type oldShapeWire struct {
	SelectedProfile     string                     `json:"selectedProfile"`
	Profiles            map[string]oldProfileEntry `json:"profiles"`
	RememberCredentials *bool                      `json:"rememberCredentials"`
}

// oldProfileEntry is the legacy per-profile struct used only during migration.
type oldProfileEntry struct {
	Routes []string `json:"routes"`
}

// parseConfig unmarshals config JSON, migrating legacy shapes when needed.
func parseConfig(data []byte) (Config, error) {
	// Try new shape first: top-level "routes" field.
	var w configWire
	if err := json.Unmarshal(data, &w); err == nil && w.Routes != nil {
		cfg := Config{
			SelectedProfile:     w.SelectedProfile,
			Routes:              cleanRoutes(w.Routes),
			RememberCredentials: true,
		}
		if w.RememberCredentials != nil {
			cfg.RememberCredentials = *w.RememberCredentials
		}
		return cfg, nil
	}

	// Try old shape: per-profile routes map.
	var old oldShapeWire
	if err := json.Unmarshal(data, &old); err == nil && old.Profiles != nil {
		cfg := Config{
			SelectedProfile:     old.SelectedProfile,
			RememberCredentials: true,
		}
		if old.RememberCredentials != nil {
			cfg.RememberCredentials = *old.RememberCredentials
		}
		cfg.Routes = collapseProfileRoutes(old.Profiles, old.SelectedProfile)
		return cfg, nil
	}

	// Try legacy shape: { connectionName, serverAddress, username, routes, rememberUsername }.
	var legacy struct {
		ConnectionName string   `json:"connectionName"`
		ServerAddress  string   `json:"serverAddress"`
		Username       string   `json:"username"`
		Routes         []string `json:"routes"`
	}
	if err := json.Unmarshal(data, &legacy); err != nil {
		// Not legacy either; fall back to default to avoid data loss of nothing.
		return Default(), nil
	}
	cfg := Default()
	if strings.TrimSpace(legacy.ConnectionName) != "" {
		cfg.SelectedProfile = strings.TrimSpace(legacy.ConnectionName)
	}
	cfg.Routes = cleanRoutes(legacy.Routes)
	return cfg, nil
}

// collapseProfileRoutes migrates the old per-profile routes map into a single
// global routes list. If selectedProfile resolves to an existing entry, that
// entry's routes are used; otherwise the union (deduped, order-preserving) of
// routes across all entries is used.
func collapseProfileRoutes(profiles map[string]oldProfileEntry, selected string) []string {
	if entry, ok := profiles[selected]; ok {
		return cleanRoutes(entry.Routes)
	}
	seen := make(map[string]struct{})
	var union []string
	// Iterate profile keys in sorted order for a deterministic, order-preserving union.
	names := make([]string, 0, len(profiles))
	for name := range profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		for _, r := range profiles[name].Routes {
			r = strings.TrimSpace(r)
			if r == "" {
				continue
			}
			if _, dup := seen[r]; dup {
				continue
			}
			seen[r] = struct{}{}
			union = append(union, r)
		}
	}
	return union
}

// SaveStored writes the full persisted state to vepeen.bin: JSON-marshal, then
// DPAPI-encrypt, then atomically replace the file (tmp + rename).
func SaveStored(s Stored) error {
	if s.Credentials == nil {
		s.Credentials = map[string]CredEntry{}
	}
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	path, err := BinPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	blob, err := encryptDPAPI(data)
	if err != nil {
		return fmt.Errorf("encrypt config: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, blob, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}

// Save writes non-secret settings, preserving any credentials already present in
// vepeen.bin so the legacy Config-only save path does not wipe secrets before the
// UI migrates to the Stored API (Phase 3).
func Save(cfg Config) error {
	existing, err := LoadStored()
	creds := existing.Credentials
	if err != nil || creds == nil {
		creds = map[string]CredEntry{}
	}
	return SaveStored(cfg.withCreds(creds))
}

// cleanRoutes trims and drops empty entries.
func cleanRoutes(in []string) []string {
	cleaned := make([]string, 0, len(in))
	for _, r := range in {
		r = strings.TrimSpace(r)
		if r != "" {
			cleaned = append(cleaned, r)
		}
	}
	return cleaned
}

// migrateLegacy imports settings from the legacy config.json (executable-adjacent
// or %AppData%\vepeen\config.json) and credentials from the Windows Credential
// Manager into a Stored value, persists them to vepeen.bin, and purges the old
// sources. It is invoked from LoadStored only when vepeen.bin is absent.
//
// Returns (stored, migrated, error). migrated is true when legacy data was found
// and successfully written to vepeen.bin. On a write failure the old sources are
// intentionally left in place (safe fallback, no data loss).
func migrateLegacy() (Stored, bool, error) {
	stored := DefaultStored()
	found := false

	// 1. Legacy config.json: executable-adjacent first, then %AppData%\vepeen.
	exePath, _ := Path()
	legPath, _ := legacyPath()
	for _, candidate := range []string{exePath, legPath} {
		data, rerr := os.ReadFile(candidate)
		if rerr != nil {
			continue
		}
		cfg, perr := parseConfig(data)
		if perr != nil {
			continue
		}
		stored.SelectedProfile = cfg.SelectedProfile
		stored.Routes = cfg.Routes
		stored.RememberCredentials = cfg.RememberCredentials
		found = true
	}

	// 2. Credential Manager entries for the selected profile and the default
	// connection name. CredMan has no list API here, so we read the profiles we
	// can infer. PSK is not present in CredMan today, so it stays empty.
	profileNames := map[string]struct{}{}
	if stored.SelectedProfile != "" {
		profileNames[stored.SelectedProfile] = struct{}{}
	}
	profileNames[DefaultConnectionName] = struct{}{}
	for name := range profileNames {
		user, okU := secrets.ReadLegacy(name, secrets.KindUsername)
		pass, okP := secrets.ReadLegacy(name, secrets.KindPassword)
		if okU || okP {
			stored.Credentials[name] = CredEntry{Username: user, Password: pass}
			found = true
		}
	}

	if !found {
		return DefaultStored(), false, nil
	}

	// 3. Persist to vepeen.bin (idempotency anchor: once it exists, LoadStored
	// will never re-enter migration).
	if err := SaveStored(stored); err != nil {
		return stored, false, fmt.Errorf("migrate save: %w", err)
	}

	// 4. Purge old sources only after a successful write.
	for _, candidate := range []string{exePath, legPath} {
		_ = os.Remove(candidate)
	}
	for name := range profileNames {
		if _, okU := secrets.ReadLegacy(name, secrets.KindUsername); okU {
			_ = secrets.DeleteLegacy(name, secrets.KindUsername)
		}
		if _, okP := secrets.ReadLegacy(name, secrets.KindPassword); okP {
			_ = secrets.DeleteLegacy(name, secrets.KindPassword)
		}
	}

	return stored, true, nil
}
