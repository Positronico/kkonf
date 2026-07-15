package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/positronico/kkonf/v2/internal/config"
	"github.com/stretchr/testify/require"
)

const testKubeconfig = `apiVersion: v1
kind: Config
current-context: prod
clusters:
  - name: prod-cluster
    cluster:
      server: https://prod:6443
  - name: dev-cluster
    cluster:
      server: https://dev:6443
users:
  - name: gke_p1_c1
    user:
      exec:
        command: gke-gcloud-auth-plugin
  - name: gke_p1_c2
    user:
      exec:
        command: gke-gcloud-auth-plugin
contexts:
  - name: prod
    context:
      cluster: prod-cluster
      user: gke_p1_c1
      namespace: payments
  - name: dev
    context:
      cluster: dev-cluster
      user: gke_p1_c2
`

func writeTestConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(path, []byte(testKubeconfig), 0o600))
	return path
}

func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	// Flag globals persist between Execute calls in-process; real CLI runs
	// are separate processes, so reset them to their defaults here.
	configFile = ""
	consolidateDryRun = false
	exportOutput = "kubeconfig-export.yaml"
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	return buf.String(), err
}

func TestCtxList(t *testing.T) {
	path := writeTestConfig(t)
	out, err := runCLI(t, "-f", path, "ctx")
	require.NoError(t, err)
	require.Equal(t, "* prod\n  dev\n", out)
}

func TestCtxSwitch(t *testing.T) {
	path := writeTestConfig(t)
	out, err := runCLI(t, "-f", path, "ctx", "dev")
	require.NoError(t, err)
	require.Contains(t, out, `Switched to context "dev"`)

	cfg, err := config.NewLoader(path).Load()
	require.NoError(t, err)
	require.Equal(t, "dev", cfg.CurrentContext)
}

func TestCtxSwitchUnknown(t *testing.T) {
	path := writeTestConfig(t)
	_, err := runCLI(t, "-f", path, "ctx", "nope")
	require.Error(t, err)
}

func TestNsShowAndSet(t *testing.T) {
	path := writeTestConfig(t)
	out, err := runCLI(t, "-f", path, "ns")
	require.NoError(t, err)
	require.Equal(t, "payments\n", out)

	_, err = runCLI(t, "-f", path, "ns", "default")
	require.NoError(t, err)

	cfg, err := config.NewLoader(path).Load()
	require.NoError(t, err)
	require.Equal(t, "", cfg.FindContext("prod").Context.Namespace,
		"'default' is normalized to empty")
}

func TestRenameClusterCLI(t *testing.T) {
	path := writeTestConfig(t)
	out, err := runCLI(t, "-f", path, "rename", "cluster", "prod-cluster", "prod-east")
	require.NoError(t, err)
	require.Contains(t, out, `Renamed cluster "prod-cluster" to "prod-east"`)

	cfg, err := config.NewLoader(path).Load()
	require.NoError(t, err)
	require.NotNil(t, cfg.FindCluster("prod-east"))
	require.Equal(t, "prod-east", cfg.FindContext("prod").Context.Cluster)
}

func TestRenameRejectsUnknownKind(t *testing.T) {
	path := writeTestConfig(t)
	_, err := runCLI(t, "-f", path, "rename", "gadget", "a", "b")
	require.Error(t, err)
}

func TestConsolidateDryRunAndReal(t *testing.T) {
	path := writeTestConfig(t)
	out, err := runCLI(t, "-f", path, "consolidate", "--dry-run")
	require.NoError(t, err)
	require.Contains(t, out, "Would consolidate")
	require.Contains(t, out, "gke-user")

	cfg, err := config.NewLoader(path).Load()
	require.NoError(t, err)
	require.Len(t, cfg.Users, 2, "dry run must not change the file")

	out, err = runCLI(t, "-f", path, "consolidate")
	require.NoError(t, err)
	require.Contains(t, out, "Consolidated")

	cfg, err = config.NewLoader(path).Load()
	require.NoError(t, err)
	require.Len(t, cfg.Users, 1)
	require.Equal(t, "gke-user", cfg.Users[0].Name)
	require.Equal(t, "gke-user", cfg.FindContext("prod").Context.User)
	require.Equal(t, "gke-user", cfg.FindContext("dev").Context.User)
}

func TestExportSubsetCLI(t *testing.T) {
	path := writeTestConfig(t)
	outPath := filepath.Join(t.TempDir(), "exported.yaml")
	out, err := runCLI(t, "-f", path, "export", "-o", outPath, "prod")
	require.NoError(t, err)
	require.Contains(t, out, "Exported 1 contexts, 1 clusters, 1 users")

	exported, err := config.NewLoader(outPath).Load()
	require.NoError(t, err)
	require.Len(t, exported.Contexts, 1)
	require.NotNil(t, exported.FindCluster("prod-cluster"))
	require.NotNil(t, exported.FindUser("gke_p1_c1"))
	require.Equal(t, "prod", exported.CurrentContext)
}

func TestBackupListAndRestore(t *testing.T) {
	path := writeTestConfig(t)

	// A save produces a backup of the original.
	_, err := runCLI(t, "-f", path, "ctx", "dev")
	require.NoError(t, err)

	out, err := runCLI(t, "-f", path, "backup", "list")
	require.NoError(t, err)
	require.Contains(t, out, ".bak.")

	out, err = runCLI(t, "-f", path, "backup", "restore")
	require.NoError(t, err)
	require.Contains(t, out, "Restored")

	cfg, err := config.NewLoader(path).Load()
	require.NoError(t, err)
	require.Equal(t, "prod", cfg.CurrentContext, "restore must bring back the pre-switch state")
}
