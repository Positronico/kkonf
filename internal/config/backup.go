package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type BackupManager struct {
	originalPath string
	backupPath   string
}

func NewBackupManager(originalPath string) *BackupManager {
	timestamp := time.Now().Format("20060102-150405")
	backupPath := fmt.Sprintf("%s.bak.%s", originalPath, timestamp)
	
	return &BackupManager{
		originalPath: originalPath,
		backupPath:   backupPath,
	}
}

func (b *BackupManager) Create() error {
	source, err := os.Open(b.originalPath)
	if err != nil {
		return fmt.Errorf("failed to open original file: %w", err)
	}
	defer source.Close()

	dest, err := os.Create(b.backupPath)
	if err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}
	defer dest.Close()

	if _, err := io.Copy(dest, source); err != nil {
		return fmt.Errorf("failed to copy to backup: %w", err)
	}

	sourceInfo, err := os.Stat(b.originalPath)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	if err := os.Chmod(b.backupPath, sourceInfo.Mode()); err != nil {
		return fmt.Errorf("failed to set backup permissions: %w", err)
	}

	return nil
}

func (b *BackupManager) Restore() error {
	source, err := os.Open(b.backupPath)
	if err != nil {
		return fmt.Errorf("failed to open backup file: %w", err)
	}
	defer source.Close()

	dest, err := os.Create(b.originalPath)
	if err != nil {
		return fmt.Errorf("failed to create restore file: %w", err)
	}
	defer dest.Close()

	if _, err := io.Copy(dest, source); err != nil {
		return fmt.Errorf("failed to restore from backup: %w", err)
	}

	return nil
}

func (b *BackupManager) GetBackupPath() string {
	return b.backupPath
}

func CleanOldBackups(configPath string, keepDays int) error {
	dir := filepath.Dir(configPath)
	base := filepath.Base(configPath)
	pattern := base + ".bak.*"

	files, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		return fmt.Errorf("failed to find backup files: %w", err)
	}

	cutoff := time.Now().AddDate(0, 0, -keepDays)

	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			if err := os.Remove(file); err != nil {
				return fmt.Errorf("failed to remove old backup %s: %w", file, err)
			}
		}
	}

	return nil
}