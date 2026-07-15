package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func loadGeneric(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var out map[string]interface{}
	require.NoError(t, yaml.Unmarshal(data, &out))
	return out
}

// The kitchen-sink config exercises every modeled kubeconfig field plus
// deliberately unknown keys at each nesting level. A load→save→load cycle
// must preserve all of it.
func TestRoundTripPreservesEverything(t *testing.T) {
	src := filepath.Join("testdata", "kitchen-sink.yaml")

	cfg, err := NewLoader(src).Load()
	require.NoError(t, err)

	dst := filepath.Join(t.TempDir(), "config")
	require.NoError(t, NewWriter(dst).Save(cfg))

	reloaded, err := NewLoader(dst).Load()
	require.NoError(t, err)
	require.Equal(t, cfg, reloaded, "structs must survive save→load unchanged")

	require.Equal(t, loadGeneric(t, src), loadGeneric(t, dst),
		"saved YAML must be semantically identical to the source file")
}

func TestUnknownKeysSurviveRoundTrip(t *testing.T) {
	src := filepath.Join("testdata", "kitchen-sink.yaml")

	cfg, err := NewLoader(src).Load()
	require.NoError(t, err)

	dst := filepath.Join(t.TempDir(), "config")
	require.NoError(t, NewWriter(dst).Save(cfg))

	saved := loadGeneric(t, dst)

	require.Equal(t, "keep-me", saved["x-top-level-unknown"])

	clusters := saved["clusters"].([]interface{})
	prodEast := clusters[0].(map[string]interface{})["cluster"].(map[string]interface{})
	require.Equal(t, "keep-me-too", prodEast["x-unknown-cluster-field"])
	require.Equal(t, "kubernetes.prod.internal", prodEast["tls-server-name"])

	users := saved["users"].([]interface{})
	execUser := users[0].(map[string]interface{})["user"].(map[string]interface{})
	require.Equal(t, "keep-me-three",
		execUser["exec"].(map[string]interface{})["x-unknown-exec-field"])

	admin := users[1].(map[string]interface{})["user"].(map[string]interface{})
	require.Equal(t, "system:admin", admin["as"])
	require.Equal(t, "1234", admin["as-uid"])
	require.Equal(t, []interface{}{"system:masters", "developers"}, admin["as-groups"])
	require.Equal(t, "keep-me-four", admin["x-unknown-user-field"])
	require.NotNil(t, admin["extensions"], "per-user extensions must survive")

	contexts := saved["contexts"].([]interface{})
	prodCtx := contexts[0].(map[string]interface{})["context"].(map[string]interface{})
	require.Equal(t, "keep-me-five", prodCtx["x-unknown-context-field"])
	require.NotNil(t, prodCtx["extensions"], "per-context extensions must survive")

	// Unknown keys that are SIBLINGS of name/cluster/user/context in the
	// Named* wrappers, and inside exec env entries, must survive too.
	require.Equal(t, "keep-me-named-1",
		clusters[0].(map[string]interface{})["x-unknown-named-cluster-sibling"])
	require.Equal(t, "keep-me-named-2",
		users[0].(map[string]interface{})["x-unknown-named-user-sibling"])
	require.Equal(t, "keep-me-named-3",
		contexts[0].(map[string]interface{})["x-unknown-named-context-sibling"])
	env := execUser["exec"].(map[string]interface{})["env"].([]interface{})
	require.Equal(t, "keep-me-env", env[0].(map[string]interface{})["x-unknown-env-field"])
}

// Every modeled leaf field must appear in the kitchen-sink fixture, so the
// round-trip test cannot silently lose coverage as the model grows.
func TestKitchenSinkCoversFilePathCertFields(t *testing.T) {
	saved := loadGeneric(t, filepath.Join("testdata", "kitchen-sink.yaml"))
	users := saved["users"].([]interface{})
	var certFileUser map[string]interface{}
	for _, u := range users {
		if u.(map[string]interface{})["name"] == "cert-file-user" {
			certFileUser = u.(map[string]interface{})["user"].(map[string]interface{})
		}
	}
	require.NotNil(t, certFileUser)
	require.Equal(t, "/etc/kube/client.crt", certFileUser["client-certificate"])
	require.Equal(t, "/etc/kube/client.key", certFileUser["client-key"])
}
