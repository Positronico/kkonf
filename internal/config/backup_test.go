package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gofrs/flock"
	"github.com/stretchr/testify/require"
)

func TestCreateReturnsEmptyForMissingOriginal(t *testing.T) {
	path, err := NewBackupManager(filepath.Join(t.TempDir(), "nope")).Create()
	require.NoError(t, err)
	require.Empty(t, path)
}

func TestCreateBackupPreservesMode(t *testing.T) {
	original := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(original, []byte("data"), 0o644))

	mgr := NewBackupManager(original)
	backup, err := mgr.Create()
	require.NoError(t, err)
	require.Equal(t, backup, mgr.LastBackupPath())

	info, err := os.Stat(backup)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o644), info.Mode().Perm())

	data, err := os.ReadFile(backup)
	require.NoError(t, err)
	require.Equal(t, "data", string(data))
}

func TestRapidBackupsGetDistinctPaths(t *testing.T) {
	original := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(original, []byte("data"), 0o600))

	mgr := NewBackupManager(original)
	seen := map[string]bool{}
	for i := 0; i < 3; i++ {
		backup, err := mgr.Create()
		require.NoError(t, err)
		require.False(t, seen[backup], "backup path %s reused", backup)
		seen[backup] = true
	}
}

func TestRestoreOverwritesOriginal(t *testing.T) {
	original := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(original, []byte("current-context: v1\n"), 0o600))

	mgr := NewBackupManager(original)
	backup, err := mgr.Create()
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(original, []byte("current-context: v2\n"), 0o600))
	require.NoError(t, mgr.Restore(backup))

	data, err := os.ReadFile(original)
	require.NoError(t, err)
	require.Equal(t, "current-context: v1\n", string(data))
}

func TestRestorePreservesModeAndReleasesLock(t *testing.T) {
	original := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(original, []byte("current-context: v1\n"), 0o644))

	mgr := NewBackupManager(original)
	backup, err := mgr.Create()
	require.NoError(t, err)

	require.NoError(t, mgr.Restore(backup))

	info, err := os.Stat(original)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o644), info.Mode().Perm())

	probe := flock.New(original + ".lock")
	locked, err := probe.TryLock()
	require.NoError(t, err)
	require.True(t, locked, "restore must release its lock")
	probe.Unlock()
}

func TestRestoreFailsWhenLocked(t *testing.T) {
	original := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(original, []byte("current-context: v1\n"), 0o600))

	mgr := NewBackupManager(original)
	backup, err := mgr.Create()
	require.NoError(t, err)

	foreign := flock.New(original + ".lock")
	locked, err := foreign.TryLock()
	require.NoError(t, err)
	require.True(t, locked)
	defer foreign.Unlock()

	require.Error(t, mgr.Restore(backup), "a held foreign lock must block restore")
}

func TestRestoreRejectsGarbageBackup(t *testing.T) {
	original := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(original, []byte("apiVersion: v1\nkind: Config\n"), 0o600))

	garbage := original + ".bak.20200101-000000"
	require.NoError(t, os.WriteFile(garbage, []byte("GARBAGE NOT YAML"), 0o600))

	err := NewBackupManager(original).Restore(garbage)
	require.Error(t, err, "restoring a non-kubeconfig file must be refused")

	data, err := os.ReadFile(original)
	require.NoError(t, err)
	require.Contains(t, string(data), "apiVersion", "original must be untouched")
}

func makeBackups(t *testing.T, original string, n int, age time.Duration) []string {
	t.Helper()
	var paths []string
	for i := 0; i < n; i++ {
		p := fmt.Sprintf("%s.bak.20200101-00000%d", original, i)
		require.NoError(t, os.WriteFile(p, []byte("old"), 0o600))
		mtime := time.Now().Add(-age).Add(time.Duration(i) * time.Second)
		require.NoError(t, os.Chtimes(p, mtime, mtime))
		paths = append(paths, p)
	}
	return paths
}

func TestListBackupsNewestFirst(t *testing.T) {
	original := filepath.Join(t.TempDir(), "config")
	made := makeBackups(t, original, 3, 24*time.Hour)

	listed, err := ListBackups(original)
	require.NoError(t, err)
	require.Equal(t, []string{made[2], made[1], made[0]}, listed)
}

func TestCleanOldBackupsAlwaysKeepsNewest(t *testing.T) {
	original := filepath.Join(t.TempDir(), "config")
	makeBackups(t, original, 5, 30*24*time.Hour) // all older than any cutoff

	require.NoError(t, CleanOldBackups(original, 7, 3))

	remaining, err := ListBackups(original)
	require.NoError(t, err)
	require.Len(t, remaining, 3, "keep-count must retain the newest backups even past the age cutoff")
}

func TestCleanOldBackupsKeepsRecent(t *testing.T) {
	original := filepath.Join(t.TempDir(), "config")
	makeBackups(t, original, 4, time.Hour) // all recent

	require.NoError(t, CleanOldBackups(original, 7, 2))

	remaining, err := ListBackups(original)
	require.NoError(t, err)
	require.Len(t, remaining, 4, "recent backups must survive regardless of keep-count")
}
