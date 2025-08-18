package config

import (
	"fmt"
	"os"

	"github.com/positronico/kkonf/internal/models"
	"gopkg.in/yaml.v3"
)

type Writer struct {
	filePath string
	backup   *BackupManager
}

func NewWriter(filePath string) *Writer {
	return &Writer{
		filePath: filePath,
		backup:   NewBackupManager(filePath),
	}
}

func (w *Writer) Save(config *models.Config) error {
	if err := w.backup.Create(); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	tempFile := w.filePath + ".tmp"
	if err := os.WriteFile(tempFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tempFile, w.filePath); err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("failed to save config file: %w", err)
	}

	return nil
}

func (w *Writer) RestoreFromBackup() error {
	return w.backup.Restore()
}