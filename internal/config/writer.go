package config

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
	"github.com/positronico/kkonf/v2/internal/models"
	"gopkg.in/yaml.v3"
)

// ErrExternalChange is returned by SaveGuarded when the file on disk no
// longer matches the fingerprint taken at load time.
var ErrExternalChange = errors.New("config file changed on disk since it was loaded")

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
	return w.save(config, nil)
}

// SaveGuarded saves only if the file still matches fp; the check happens
// under the write lock, so there is no window for another writer to sneak in
// between check and write.
func (w *Writer) SaveGuarded(config *models.Config, fp *Fingerprint) error {
	return w.save(config, fp)
}

func (w *Writer) save(config *models.Config, fp *Fingerprint) error {
	target := resolveWriteTarget(w.filePath)

	release, err := acquireLock(target + ".lock")
	if err != nil {
		return err
	}
	defer release()

	if fp != nil && ChangedSince(w.filePath, fp) {
		return ErrExternalChange
	}

	if _, err := w.backup.Create(); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	data, err := MarshalConfig(config)
	if err != nil {
		return err
	}

	return atomicWrite(target, data)
}

// MarshalConfig converts yaml.v3's plain-string panics (e.g. an inline Extra
// map holding a key that collides with a struct field) into errors, so a bad
// in-memory state can never crash a save — or any other caller.
func MarshalConfig(config *models.Config) (data []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			data = nil
			err = fmt.Errorf("failed to marshal config: %v", r)
		}
	}()
	data, err = yaml.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}
	return data, nil
}

// resolveWriteTarget follows symlinks (including dangling ones, which
// EvalSymlinks refuses to resolve) so saves write through the link instead of
// replacing it with a regular file.
func resolveWriteTarget(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	target := path
	for i := 0; i < 10; i++ {
		info, err := os.Lstat(target)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			return target
		}
		dest, err := os.Readlink(target)
		if err != nil {
			return target
		}
		if !filepath.IsAbs(dest) {
			dest = filepath.Join(filepath.Dir(target), dest)
		}
		target = dest
	}
	return target
}

// acquireLock serializes writers via an OS advisory lock (flock/LockFileEx).
// The kernel releases it when the holder dies, so crashed processes cannot
// leave a stale lock and there is no break-the-lock race between writers.
func acquireLock(lockPath string) (release func(), err error) {
	lock := flock.New(lockPath)
	locked, err := lock.TryLock()
	if err != nil {
		return nil, fmt.Errorf("failed to lock config file: %w", err)
	}
	if !locked {
		return nil, fmt.Errorf("config file is locked by another process (%s)", lockPath)
	}
	return func() { lock.Unlock() }, nil
}

// atomicWrite writes data via a temp file + rename, preserving an existing
// target's permissions and defaulting new files to 0600.
func atomicWrite(target string, data []byte) error {
	mode := os.FileMode(0o600)
	if info, err := os.Stat(target); err == nil {
		mode = info.Mode().Perm()
	}

	tempFile := target + ".tmp"
	temp, err := os.OpenFile(tempFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		os.Remove(tempFile)
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		os.Remove(tempFile)
		return fmt.Errorf("failed to sync temp file: %w", err)
	}
	if err := temp.Close(); err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("failed to close temp file: %w", err)
	}
	// O_CREATE honors the umask; force the intended mode.
	if err := os.Chmod(tempFile, mode); err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("failed to set temp file permissions: %w", err)
	}
	if err := os.Rename(tempFile, target); err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("failed to save config file: %w", err)
	}
	return nil
}

// LastBackupPath returns the backup made by the most recent Save, if any.
func (w *Writer) LastBackupPath() string {
	return w.backup.LastBackupPath()
}

// RestoreFromBackup copies the given backup over the config file.
func (w *Writer) RestoreFromBackup(backupPath string) error {
	return w.backup.Restore(backupPath)
}

// Fingerprint identifies a file's content state, for detecting external
// modifications (e.g. kubectl writing the kubeconfig while kkonf is open).
type Fingerprint struct {
	ModTime time.Time
	Size    int64
	Hash    [sha256.Size]byte
}

func FingerprintFile(path string) (*Fingerprint, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return &Fingerprint{
		ModTime: info.ModTime(),
		Size:    info.Size(),
		Hash:    sha256.Sum256(data),
	}, nil
}

// ChangedSince reports whether the file at path differs from the fingerprint
// taken when it was loaded. A missing file counts as changed.
func ChangedSince(path string, fp *Fingerprint) bool {
	if fp == nil {
		return false
	}
	current, err := FingerprintFile(path)
	if err != nil {
		return true
	}
	return current.Hash != fp.Hash
}
