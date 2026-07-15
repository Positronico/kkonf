package config

import (
	"crypto/sha256"
	"fmt"
	"os"

	"github.com/positronico/kkonf/v2/internal/models"
	"gopkg.in/yaml.v3"
)

type Loader struct {
	filePath    string
	fingerprint *Fingerprint
}

func NewLoader(filePath string) *Loader {
	return &Loader{
		filePath: filePath,
	}
}

func (l *Loader) Load() (*models.Config, error) {
	data, err := os.ReadFile(l.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if info, err := os.Stat(l.filePath); err == nil {
		l.fingerprint = &Fingerprint{
			ModTime: info.ModTime(),
			Size:    info.Size(),
			Hash:    sha256.Sum256(data),
		}
	}

	var config models.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if config.APIVersion == "" {
		config.APIVersion = "v1"
	}
	if config.Kind == "" {
		config.Kind = "Config"
	}

	return &config, nil
}

func (l *Loader) GetFilePath() string {
	return l.filePath
}

// Fingerprint returns the fingerprint of the file as of the last Load,
// or nil if no successful Load has happened yet.
func (l *Loader) Fingerprint() *Fingerprint {
	return l.fingerprint
}
