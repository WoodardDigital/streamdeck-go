package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// KeyConfig defines what a single Stream Deck key does.
type KeyConfig struct {
	Icon    string `yaml:"icon"`    // filename relative to icons_dir
	Command string `yaml:"command"` // shell command to run on press
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
