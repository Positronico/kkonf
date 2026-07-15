package settings

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Settings are kkonf's own preferences, persisted separately from any
// kubeconfig at ~/.config/kkonf/config.yaml (or the platform equivalent).
type Settings struct {
	BackupKeepDays  int `yaml:"backupKeepDays"`
	BackupKeepCount int `yaml:"backupKeepCount"`
}

func Default() Settings {
	return Settings{BackupKeepDays: 7, BackupKeepCount: 10}
}

func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "kkonf", "config.yaml"), nil
}

// Load returns saved settings, falling back to defaults for missing files or
// missing fields.
func Load() Settings {
	s := Default()
	path, err := Path()
	if err != nil {
		return s
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return s
	}
	_ = yaml.Unmarshal(data, &s)
	if s.BackupKeepDays < 0 {
		s.BackupKeepDays = 0
	}
	if s.BackupKeepCount < 0 {
		s.BackupKeepCount = 0
	}
	return s
}

func (s Settings) Save() error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
