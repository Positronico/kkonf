package config

import (
	"fmt"
	"os"

	"github.com/positronico/kkonf/internal/models"
	"gopkg.in/yaml.v3"
)

type Loader struct {
	filePath string
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