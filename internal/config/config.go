package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type Device struct {
	Alias    string `json:"alias"`
	Host     string `json:"host"`
	User     string `json:"user"`
	Port     int    `json:"port"`
	Identity string `json:"identity,omitempty"`
}

type Config struct {
	Devices []Device `json:"devices"`
}

// DefaultPath returns the config path: $XDG_CONFIG_HOME/zgx/config.json,
// otherwise ~/.config/zgx/config.json.
func DefaultPath() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("find home directory: %w", err)
		}
		base = filepath.Join(homeDir, ".config")
	}
	return filepath.Join(base, "zgx", "config.json"), nil
}

// Load reads config from path. A missing file returns an empty Config.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("read config JSON: %w", err)
	}
	cfg.sortDevices()
	return &cfg, nil
}

// Save writes config to path with a private directory and file.
func Save(path string, cfg *Config) error {
	if cfg == nil {
		cfg = &Config{}
	}
	cfg.sortDevices()

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if err := os.Chmod(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("set config directory permissions: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("serialize config: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("set config file permissions: %w", err)
	}
	return nil
}

// Get finds a device by alias.
func (c *Config) Get(alias string) (Device, bool) {
	for _, device := range c.Devices {
		if device.Alias == alias {
			return device, true
		}
	}
	return Device{}, false
}

// Upsert adds or updates a device by alias.
func (c *Config) Upsert(d Device) {
	for i := range c.Devices {
		if c.Devices[i].Alias == d.Alias {
			c.Devices[i] = d
			c.sortDevices()
			return
		}
	}
	c.Devices = append(c.Devices, d)
	c.sortDevices()
}

// Remove removes a device by alias and reports whether anything was removed.
func (c *Config) Remove(alias string) bool {
	for i := range c.Devices {
		if c.Devices[i].Alias == alias {
			c.Devices = append(c.Devices[:i], c.Devices[i+1:]...)
			return true
		}
	}
	return false
}

func (c *Config) sortDevices() {
	sort.Slice(c.Devices, func(i, j int) bool {
		return c.Devices[i].Alias < c.Devices[j].Alias
	})
}
