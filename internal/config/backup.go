package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/positronico/kkonf/v2/internal/models"
	"gopkg.in/yaml.v3"
)

type BackupManager struct {
	originalPath string
	lastBackup   string
}

func NewBackupManager(originalPath string) *BackupManager {
	return &BackupManager{originalPath: originalPath}
}

// Create copies the original file to a fresh timestamped backup and returns
// its path. A missing original (first save to a new path) is not an error and
// returns "". Repeated saves within the same second get distinct suffixes.
func (b *BackupManager) Create() (string, error) {
	source, err := os.Open(b.originalPath)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to open original file: %w", err)
	}
	defer source.Close()

	sourceInfo, err := os.Stat(b.originalPath)
	if err != nil {
		return "", fmt.Errorf("failed to get file info: %w", err)
	}

	timestamp := time.Now().Format("20060102-150405")
	base := fmt.Sprintf("%s.bak.%s", b.originalPath, timestamp)
	backupPath := base
	var dest *os.File
	for i := 1; ; i++ {
		dest, err = os.OpenFile(backupPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, sourceInfo.Mode().Perm())
		if err == nil {
			break
		}
		if !os.IsExist(err) {
			return "", fmt.Errorf("failed to create backup file: %w", err)
		}
		backupPath = fmt.Sprintf("%s-%d", base, i)
	}
	defer dest.Close()

	if _, err := io.Copy(dest, source); err != nil {
		os.Remove(backupPath)
		return "", fmt.Errorf("failed to copy to backup: %w", err)
	}

	// O_CREATE honors the umask; force the source's exact mode.
	if err := os.Chmod(backupPath, sourceInfo.Mode().Perm()); err != nil {
		os.Remove(backupPath)
		return "", fmt.Errorf("failed to set backup permissions: %w", err)
	}

	b.lastBackup = backupPath
	return backupPath, nil
}

// LastBackupPath returns the backup created by the most recent Create call
// in this session, or "" if none was made.
func (b *BackupManager) LastBackupPath() string {
	return b.lastBackup
}

// Restore writes the backup's content over the original path with the same
// guarantees as a save: under the write lock, atomically, through symlinks,
// preserving the live file's mode. The backup must parse as a kubeconfig —
// this rejects stray or truncated files matching the .bak.* glob.
func (b *BackupManager) Restore(backupPath string) error {
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup file: %w", err)
	}

	var probe models.Config
	if err := yaml.Unmarshal(data, &probe); err != nil {
		return fmt.Errorf("refusing to restore %s: not a valid kubeconfig: %w", backupPath, err)
	}

	target := resolveWriteTarget(b.originalPath)
	release, err := acquireLock(target + ".lock")
	if err != nil {
		return err
	}
	defer release()

	if err := atomicWrite(target, data); err != nil {
		return fmt.Errorf("failed to restore from backup: %w", err)
	}
	return nil
}

// ListBackups returns the backup files for configPath, newest first.
func ListBackups(configPath string) ([]string, error) {
	files, err := filepath.Glob(configPath + ".bak.*")
	if err != nil {
		return nil, fmt.Errorf("failed to find backup files: %w", err)
	}
	sort.Slice(files, func(i, j int) bool {
		fi, errI := os.Stat(files[i])
		fj, errJ := os.Stat(files[j])
		if errI != nil || errJ != nil {
			return files[i] > files[j]
		}
		return fi.ModTime().After(fj.ModTime())
	})
	return files, nil
}

// CleanOldBackups removes backups older than keepDays, but always retains the
// newest keepCount regardless of age so a config never loses all its backups.
func CleanOldBackups(configPath string, keepDays, keepCount int) error {
	files, err := ListBackups(configPath)
	if err != nil {
		return err
	}

	cutoff := time.Now().AddDate(0, 0, -keepDays)

	for i, file := range files {
		if i < keepCount {
			continue
		}
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
