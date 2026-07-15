package models

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func execUser() User {
	return User{
		Exec: &ExecConfig{
			APIVersion: "client.authentication.k8s.io/v1beta1",
			Command:    "gke-gcloud-auth-plugin",
			Args:       []string{"--use_application_default_credentials"},
		},
	}
}

func TestSignatureEqualForIdenticalUsers(t *testing.T) {
	a, b := execUser(), execUser()
	require.Equal(t, a.GetSignature(), b.GetSignature())
	require.True(t, a.Equals(&b))
}

func TestSignatureDiffersOnImpersonation(t *testing.T) {
	a, b := execUser(), execUser()
	b.ImpersonateGroups = []string{"system:masters"}
	require.NotEqual(t, a.GetSignature(), b.GetSignature(),
		"users differing only in as-groups must not be treated as duplicates")

	c, d := execUser(), execUser()
	d.Impersonate = "system:admin"
	require.NotEqual(t, c.GetSignature(), d.GetSignature())
}

func TestSignatureDiffersOnUnknownExtras(t *testing.T) {
	a, b := execUser(), execUser()
	b.Extra = map[string]interface{}{"x-custom": "value"}
	require.NotEqual(t, a.GetSignature(), b.GetSignature(),
		"users differing in unknown fields must not be consolidated together")
}

func TestSignatureDiffersOnExtensions(t *testing.T) {
	a, b := execUser(), execUser()
	b.Extensions = []NamedExtension{{Name: "ext/v1"}}
	require.NotEqual(t, a.GetSignature(), b.GetSignature())
}

func TestUnhashableUsersNeverCompareEqual(t *testing.T) {
	// Non-string map keys make json.Marshal fail; the fail-safe must yield
	// unique signatures so such users are never wrongly consolidated.
	bad := map[string]interface{}{"k": map[interface{}]interface{}{1: "v"}}
	a, b := execUser(), execUser()
	a.Extra = bad
	b.Extra = bad
	require.NotEqual(t, a.GetSignature(), b.GetSignature())
	require.False(t, a.Equals(&b))
}
