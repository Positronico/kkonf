package models

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func sampleConfig() *Config {
	return &Config{
		APIVersion:     "v1",
		Kind:           "Config",
		CurrentContext: "prod",
		Clusters: []NamedCluster{
			{Name: "prod-cluster", Cluster: Cluster{Server: "https://prod:6443"}},
			{Name: "dev-cluster", Cluster: Cluster{Server: "https://dev:6443"}},
		},
		Users: []NamedUser{
			{Name: "admin", User: User{Token: "tok-a"}},
			{Name: "dev", User: User{Token: "tok-b"}},
		},
		Contexts: []NamedContext{
			{Name: "prod", Context: Context{Cluster: "prod-cluster", User: "admin"}},
			{Name: "dev", Context: Context{Cluster: "dev-cluster", User: "dev"}},
		},
	}
}

func TestAddRejectsDuplicates(t *testing.T) {
	cfg := sampleConfig()
	require.Error(t, cfg.AddCluster(NamedCluster{Name: "prod-cluster"}))
	require.Error(t, cfg.AddUser(NamedUser{Name: "admin"}))
	require.Error(t, cfg.AddContext(NamedContext{Name: "prod"}))
	require.Error(t, cfg.AddCluster(NamedCluster{Name: ""}))
	require.Len(t, cfg.Clusters, 2)

	require.NoError(t, cfg.AddCluster(NamedCluster{Name: "new-cluster"}))
	require.Len(t, cfg.Clusters, 3)
}

func TestRenameClusterRewritesContexts(t *testing.T) {
	cfg := sampleConfig()
	require.NoError(t, cfg.RenameCluster("prod-cluster", "prod-east"))
	require.Nil(t, cfg.FindCluster("prod-cluster"))
	require.NotNil(t, cfg.FindCluster("prod-east"))
	require.Equal(t, "prod-east", cfg.FindContext("prod").Context.Cluster)
	require.Equal(t, "dev-cluster", cfg.FindContext("dev").Context.Cluster, "unrelated contexts untouched")
}

func TestRenameUserRewritesContexts(t *testing.T) {
	cfg := sampleConfig()
	require.NoError(t, cfg.RenameUser("admin", "root"))
	require.Equal(t, "root", cfg.FindContext("prod").Context.User)
}

func TestRenameContextFollowsCurrentContext(t *testing.T) {
	cfg := sampleConfig()
	require.NoError(t, cfg.RenameContext("prod", "production"))
	require.Equal(t, "production", cfg.CurrentContext)
}

func TestRenameRejectsCollisionAndMissing(t *testing.T) {
	cfg := sampleConfig()
	require.Error(t, cfg.RenameCluster("prod-cluster", "dev-cluster"))
	require.Error(t, cfg.RenameCluster("nope", "x"))
	require.Error(t, cfg.RenameCluster("prod-cluster", ""))
	require.NoError(t, cfg.RenameCluster("prod-cluster", "prod-cluster"), "same-name rename is a no-op")
}

func TestDeleteClusterCascade(t *testing.T) {
	cfg := sampleConfig()
	removed, err := cfg.DeleteCluster("prod-cluster", true)
	require.NoError(t, err)
	require.Equal(t, []string{"prod"}, removed)
	require.Nil(t, cfg.FindContext("prod"))
	require.Empty(t, cfg.CurrentContext, "current-context cleared when its context is cascade-deleted")
}

func TestDeleteClusterWithoutCascadeLeavesContexts(t *testing.T) {
	cfg := sampleConfig()
	removed, err := cfg.DeleteCluster("prod-cluster", false)
	require.NoError(t, err)
	require.Empty(t, removed)
	require.NotNil(t, cfg.FindContext("prod"), "context left dangling by explicit choice")
}

func TestDeleteUserCascade(t *testing.T) {
	cfg := sampleConfig()
	removed, err := cfg.DeleteUser("dev", true)
	require.NoError(t, err)
	require.Equal(t, []string{"dev"}, removed)
	require.Nil(t, cfg.FindUser("dev"))
}

func TestDeleteMissingEntities(t *testing.T) {
	cfg := sampleConfig()
	_, err := cfg.DeleteCluster("nope", true)
	require.Error(t, err)
	_, err = cfg.DeleteUser("nope", false)
	require.Error(t, err)
	require.Error(t, cfg.DeleteContext("nope"))
}

func TestSetCurrentContextValidates(t *testing.T) {
	cfg := sampleConfig()
	require.Error(t, cfg.SetCurrentContext("nope"))
	require.NoError(t, cfg.SetCurrentContext("dev"))
	require.Equal(t, "dev", cfg.CurrentContext)
}

func TestNamespaceHelpers(t *testing.T) {
	require.Equal(t, "", NormalizeNamespace("default"))
	require.Equal(t, "payments", NormalizeNamespace("payments"))
	require.Equal(t, "default", DisplayNamespace(""))
	require.Equal(t, "payments", DisplayNamespace("payments"))

	cfg := sampleConfig()
	require.NoError(t, cfg.SetNamespace("prod", "default"))
	require.Equal(t, "", cfg.FindContext("prod").Context.Namespace,
		"the literal namespace 'default' is stored as empty")
}

func consolidationConfig() *Config {
	dup := User{Exec: &ExecConfig{Command: "gke-gcloud-auth-plugin"}}
	return &Config{
		Users: []NamedUser{
			{Name: "keep-order", User: User{Token: "unique"}},
			{Name: "gke_p1_c1", User: dup},
			{Name: "gke_p1_c2", User: dup},
			{Name: "gke_p2_c1", User: dup},
		},
		Contexts: []NamedContext{
			{Name: "c1", Context: Context{User: "gke_p1_c1"}},
			{Name: "c2", Context: Context{User: "gke_p1_c2"}},
			{Name: "c3", Context: Context{User: "gke_p2_c1"}},
		},
	}
}

func TestDuplicateUserGroups(t *testing.T) {
	groups := consolidationConfig().DuplicateUserGroups()
	require.Len(t, groups, 1)
	require.Len(t, groups[0].Users, 3)
	require.Equal(t, "exec", groups[0].AuthMethod)
}

func TestConsolidateUsersToFreshName(t *testing.T) {
	cfg := consolidationConfig()
	updated, err := cfg.ConsolidateUsers([]string{"gke_p1_c1", "gke_p1_c2", "gke_p2_c1"}, "gke-user")
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"c1", "c2", "c3"}, updated)
	require.Len(t, cfg.Users, 2)
	require.Equal(t, "keep-order", cfg.Users[0].Name, "non-group users keep their positions")
	require.Equal(t, "gke-user", cfg.Users[1].Name, "consolidated user takes the first group slot")
	for _, ctx := range cfg.Contexts {
		require.Equal(t, "gke-user", ctx.Context.User)
	}
}

func TestConsolidateUsersIntoGroupMember(t *testing.T) {
	cfg := consolidationConfig()
	updated, err := cfg.ConsolidateUsers([]string{"gke_p1_c1", "gke_p1_c2", "gke_p2_c1"}, "gke_p1_c2")
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"c1", "c3"}, updated, "contexts already on the kept name are untouched")
	require.Len(t, cfg.Users, 2)
}

func TestConsolidateUsersRejectsForeignExistingName(t *testing.T) {
	cfg := consolidationConfig()
	_, err := cfg.ConsolidateUsers([]string{"gke_p1_c1", "gke_p1_c2"}, "keep-order")
	require.Error(t, err)
}

func importedConfig() *Config {
	return &Config{
		Clusters: []NamedCluster{
			{Name: "prod-cluster", Cluster: Cluster{Server: "https://other:6443"}},
			{Name: "fresh-cluster", Cluster: Cluster{Server: "https://fresh:6443"}},
		},
		Users: []NamedUser{
			{Name: "admin", User: User{Token: "other-token"}},
		},
		Contexts: []NamedContext{
			{Name: "imported-ctx", Context: Context{Cluster: "prod-cluster", User: "admin"}},
		},
	}
}

func TestMergeSkipConflicts(t *testing.T) {
	cfg := sampleConfig()
	res := cfg.Merge(importedConfig(), MergeOptions{})
	require.Equal(t, 2, res.Added, "fresh cluster + imported context")
	require.Equal(t, 2, res.Skipped)
	require.Equal(t, "https://prod:6443", cfg.FindCluster("prod-cluster").Cluster.Server, "existing untouched")
}

func TestMergeReplaceConflicts(t *testing.T) {
	cfg := sampleConfig()
	res := cfg.Merge(importedConfig(), MergeOptions{
		OnConflict: func(string, string) MergeAction { return MergeReplace },
	})
	require.Equal(t, 2, res.Replaced)
	require.Equal(t, "https://other:6443", cfg.FindCluster("prod-cluster").Cluster.Server)
	require.Len(t, cfg.Clusters, 3, "replace must not duplicate entries")
}

// The historical bug: renamed imports left their dependent contexts pointing
// at the pre-rename (conflicting) names. Contexts must follow the rename.
func TestMergeRenameRewritesImportedReferences(t *testing.T) {
	cfg := sampleConfig()
	res := cfg.Merge(importedConfig(), MergeOptions{
		OnConflict: func(string, string) MergeAction { return MergeRename },
		Rename:     func(kind, old string) string { return old + "-imported" },
	})
	require.Equal(t, 2, res.Renamed)

	imported := cfg.FindContext("imported-ctx")
	require.NotNil(t, imported)
	require.Equal(t, "prod-cluster-imported", imported.Context.Cluster,
		"imported context must reference the renamed cluster, not the existing one")
	require.Equal(t, "admin-imported", imported.Context.User)

	require.Equal(t, "prod-cluster", cfg.FindContext("prod").Context.Cluster,
		"pre-existing contexts must not be touched by import renames")
}

func TestMergeRenameCollisionSkips(t *testing.T) {
	cfg := sampleConfig()
	res := cfg.Merge(importedConfig(), MergeOptions{
		OnConflict: func(string, string) MergeAction { return MergeRename },
		Rename:     func(kind, old string) string { return "dev-cluster" }, // collides for clusters
	})
	require.GreaterOrEqual(t, res.Skipped, 1)
	require.Equal(t, "https://dev:6443", cfg.FindCluster("dev-cluster").Cluster.Server)
}
