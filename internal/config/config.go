package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// PollConfig defines how to poll for the current state of a toggle key.
type PollConfig struct {
	Command  string `yaml:"command"`  // shell command whose output is checked
	Interval string `yaml:"interval"` // how often to poll, e.g. "2s" (default: "2s")
	Match    string `yaml:"match"`    // substring to find in output → "on" state; omit to use exit code 0
}

// KeyConfig defines what a single Stream Deck key does.
type KeyConfig struct {
	Icon    string `yaml:"icon"`    // filename relative to icons_dir (regular keys)
	Command string `yaml:"command"` // shell command to run on press

	// Toggle/status keys: show different icons based on polled state.
	IconTrue  string      `yaml:"icon_true"`  // icon when poll match is true
	IconFalse string      `yaml:"icon_false"` // icon when poll match is false
	Poll      *PollConfig `yaml:"poll"`
}

// Config is the top-level structure of the YAML config file.
type Config struct {
	IconsDir   string            `yaml:"icons_dir"`
	Brightness int               `yaml:"brightness"`
	Device     DeviceConfig      `yaml:"device"`
	Keys       map[int]KeyConfig `yaml:"keys"`
}

// DeviceConfig allows overriding USB IDs (defaults work for Stream Deck XL v2).
type DeviceConfig struct {
	VendorID  uint16 `yaml:"vendor_id"`
	ProductID uint16 `yaml:"product_id"`
}

// Load reads and parses a YAML config file, applying sane defaults.
// icons_dir defaults to an "icons" folder next to the config file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}

	cfg := &Config{
		IconsDir:   filepath.Join(filepath.Dir(path), "icons"),
		Brightness: 70,
		Device: DeviceConfig{
			VendorID:  0x0fd9,
			ProductID: 0x00ba, // Stream Deck XL v2
		},
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Resolve relative icons_dir against the config file's directory so the
	// binary works regardless of the working directory (e.g. as a systemd service).
	if !filepath.IsAbs(cfg.IconsDir) {
		cfg.IconsDir = filepath.Join(filepath.Dir(path), cfg.IconsDir)
	}

	return cfg, nil
}
