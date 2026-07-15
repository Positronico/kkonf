package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gofrs/flock"
	"github.com/positronico/kkonf/internal/models"
	"github.com/stretchr/testify/require"
)

func minimalConfig() *models.Config {
	return &models.Config{APIVersion: "v1", Kind: "Config"}
}

// Exporting to a path that doesn't exist yet must work (previously the
// unconditional pre-save backup failed on the missing file).
func TestSaveToNewPathCreatesFile(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "exported-config")

	require.NoError(t, NewWriter(dst).Save(minimalConfig()))

	info, err := os.Stat(dst)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "new files default to 0600")

	backups, err := ListBackups(dst)
	require.NoError(t, err)
	require.Empty(t, backups, "no backup should be created for a new file")
}

func TestSavePreservesExistingMode(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(dst, []byte("{}"), 0o644))

	require.NoError(t, NewWriter(dst).Save(minimalConfig()))

	info, err := os.Stat(dst)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o644), info.Mode().Perm())
}

func TestSaveThroughSymlinkKeepsSymlink(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real-config")
	link := filepath.Join(dir, "linked-config")
	require.NoError(t, os.WriteFile(real, []byte("{}"), 0o600))
	require.NoError(t, os.Symlink(real, link))

	require.NoError(t, NewWriter(link).Save(minimalConfig()))

	info, err := os.Lstat(link)
	require.NoError(t, err)
	require.NotZero(t, info.Mode()&os.ModeSymlink, "symlink must not be replaced by a regular file")

	data, err := os.ReadFile(real)
	require.NoError(t, err)
	require.Contains(t, string(data), "apiVersion: v1", "content must land in the symlink target")
}

func TestEachSaveCreatesDistinctBackup(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(dst, []byte("{}"), 0o600))

	w := NewWriter(dst)
	require.NoError(t, w.Save(minimalConfig()))
	require.NoError(t, w.Save(minimalConfig()))

	backups, err := ListBackups(dst)
	require.NoError(t, err)
	require.Len(t, backups, 2, "two saves must produce two distinct backups")
	require.NotEqual(t, backups[0], backups[1])
}

func TestSaveFailsWhenLocked(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(dst, []byte("{}"), 0o600))

	foreign := flock.New(dst + ".lock")
	locked, err := foreign.TryLock()
	require.NoError(t, err)
	require.True(t, locked)
	defer foreign.Unlock()

	err = NewWriter(dst).Save(minimalConfig())
	require.Error(t, err)
	require.Contains(t, err.Error(), "locked")
}

func TestLeftoverLockFileDoesNotBlock(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(dst, []byte("{}"), 0o600))
	// A lock FILE without a held OS lock (e.g. after a crash) must not block.
	require.NoError(t, os.WriteFile(dst+".lock", nil, 0o600))

	require.NoError(t, NewWriter(dst).Save(minimalConfig()))
}

func TestSaveReleasesLock(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(dst, []byte("{}"), 0o600))

	require.NoError(t, NewWriter(dst).Save(minimalConfig()))

	probe := flock.New(dst + ".lock")
	locked, err := probe.TryLock()
	require.NoError(t, err)
	require.True(t, locked, "lock must be released after save")
	probe.Unlock()
}

func TestSaveGuardedDetectsExternalChange(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(dst, []byte("original"), 0o600))

	fp, err := FingerprintFile(dst)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(dst, []byte("changed externally"), 0o600))

	err = NewWriter(dst).SaveGuarded(minimalConfig(), fp)
	require.ErrorIs(t, err, ErrExternalChange)

	data, err := os.ReadFile(dst)
	require.NoError(t, err)
	require.Equal(t, "changed externally", string(data), "guarded save must not clobber the external change")
}

func TestMarshalPanicBecomesError(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "config")
	cfg := minimalConfig()
	// A modeled key inside an inline Extra map makes yaml.v3 panic; Save must
	// surface that as an error, not crash.
	cfg.Extra = map[string]interface{}{"kind": "boom"}

	err := NewWriter(dst).Save(cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "marshal")
}

func TestSaveThroughDanglingSymlink(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real-config") // does not exist yet
	link := filepath.Join(dir, "linked-config")
	require.NoError(t, os.Symlink(real, link))

	require.NoError(t, NewWriter(link).Save(minimalConfig()))

	info, err := os.Lstat(link)
	require.NoError(t, err)
	require.NotZero(t, info.Mode()&os.ModeSymlink, "dangling symlink must not be replaced")
	_, err = os.Stat(real)
	require.NoError(t, err, "content must be created at the symlink target")
}

func TestChangedSince(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(dst, []byte("original"), 0o600))

	fp, err := FingerprintFile(dst)
	require.NoError(t, err)

	require.False(t, ChangedSince(dst, fp))
	require.False(t, ChangedSince(dst, nil), "nil fingerprint means no baseline to compare")

	require.NoError(t, os.WriteFile(dst, []byte("modified externally"), 0o600))
	require.True(t, ChangedSince(dst, fp))

	require.NoError(t, os.Remove(dst))
	require.True(t, ChangedSince(dst, fp), "a deleted file counts as changed")
}
