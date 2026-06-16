package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFileReturnsEmptyConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "zgx", "config.json")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error for missing file: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load returned nil config")
	}
	if len(cfg.Devices) != 0 {
		t.Fatalf("Load missing file gave %d devices, want 0", len(cfg.Devices))
	}
}

func TestSaveLoadRoundTripAndFilePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "zgx", "config.json")
	want := &Config{Devices: []Device{
		{Alias: "zgx-a", Host: "zgx-a.local", User: "root", Port: 22},
		{Alias: "zgx-b", Host: "zgx-b.local", User: "hp", Port: 2222, Identity: "/tmp/id"},
	}}

	if err := Save(path, want); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat config file failed: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("file perms = %o, want 600", got)
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("Stat config directory failed: %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("directory perms = %o, want 700", got)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(got.Devices) != 2 {
		t.Fatalf("Load gave %d devices, want 2", len(got.Devices))
	}
	if got.Devices[0] != want.Devices[0] || got.Devices[1] != want.Devices[1] {
		t.Fatalf("round-trip mismatch: got %#v want %#v", got.Devices, want.Devices)
	}
}

func TestLoadCorruptJSONReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{ not json"), 0o600); err != nil {
		t.Fatalf("write corrupt config: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("Load returned nil error for corrupt JSON")
	}
}

func TestUpsertSameAliasUpdatesWithoutDuplicate(t *testing.T) {
	cfg := &Config{}
	cfg.Upsert(Device{Alias: "nano", Host: "old.local", User: "hp", Port: 22})
	cfg.Upsert(Device{Alias: "nano", Host: "new.local", User: "root", Port: 2222, Identity: "/tmp/id"})

	if len(cfg.Devices) != 1 {
		t.Fatalf("Upsert gave %d devices, want 1", len(cfg.Devices))
	}
	got, ok := cfg.Get("nano")
	if !ok {
		t.Fatal("Get did not find updated alias")
	}
	if got.Host != "new.local" || got.User != "root" || got.Port != 2222 || got.Identity != "/tmp/id" {
		t.Fatalf("Upsert did not update device: %#v", got)
	}
}

func TestRemove(t *testing.T) {
	cfg := &Config{}
	cfg.Upsert(Device{Alias: "nano", Host: "nano.local", User: "hp", Port: 22})

	if !cfg.Remove("nano") {
		t.Fatal("Remove existing alias returned false")
	}
	if _, ok := cfg.Get("nano"); ok {
		t.Fatal("Remove did not remove alias")
	}
	if cfg.Remove("missing") {
		t.Fatal("Remove non-existing alias returned true")
	}
}
