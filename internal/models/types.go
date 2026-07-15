package models

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync/atomic"
)

// Extra fields tagged `yaml:",inline"` collect keys not matched by any struct
// field, so unknown or future kubeconfig fields survive a load/save round-trip.

type Config struct {
	APIVersion     string                 `yaml:"apiVersion"`
	Kind           string                 `yaml:"kind"`
	CurrentContext string                 `yaml:"current-context"`
	Clusters       []NamedCluster         `yaml:"clusters"`
	Users          []NamedUser            `yaml:"users"`
	Contexts       []NamedContext         `yaml:"contexts"`
	Preferences    map[string]interface{} `yaml:"preferences,omitempty"`
	Extensions     []NamedExtension       `yaml:"extensions,omitempty"`
	Extra          map[string]interface{} `yaml:",inline"`
}

type NamedCluster struct {
	Name    string                 `yaml:"name"`
	Cluster Cluster                `yaml:"cluster"`
	Extra   map[string]interface{} `yaml:",inline"`
}

type Cluster struct {
	Server                   string                 `yaml:"server"`
	TLSServerName            string                 `yaml:"tls-server-name,omitempty"`
	CertificateAuthorityData string                 `yaml:"certificate-authority-data,omitempty"`
	CertificateAuthority     string                 `yaml:"certificate-authority,omitempty"`
	InsecureSkipTLSVerify    bool                   `yaml:"insecure-skip-tls-verify,omitempty"`
	ProxyURL                 string                 `yaml:"proxy-url,omitempty"`
	DisableCompression       bool                   `yaml:"disable-compression,omitempty"`
	Extensions               []NamedExtension       `yaml:"extensions,omitempty"`
	Extra                    map[string]interface{} `yaml:",inline"`
}

type NamedUser struct {
	Name  string                 `yaml:"name"`
	User  User                   `yaml:"user"`
	Extra map[string]interface{} `yaml:",inline"`
}

type User struct {
	ClientCertificateData string                 `yaml:"client-certificate-data,omitempty"`
	ClientCertificate     string                 `yaml:"client-certificate,omitempty"`
	ClientKeyData         string                 `yaml:"client-key-data,omitempty"`
	ClientKey             string                 `yaml:"client-key,omitempty"`
	Token                 string                 `yaml:"token,omitempty"`
	TokenFile             string                 `yaml:"tokenFile,omitempty"`
	Impersonate           string                 `yaml:"as,omitempty"`
	ImpersonateUID        string                 `yaml:"as-uid,omitempty"`
	ImpersonateGroups     []string               `yaml:"as-groups,omitempty"`
	ImpersonateUserExtra  map[string][]string    `yaml:"as-user-extra,omitempty"`
	Username              string                 `yaml:"username,omitempty"`
	Password              string                 `yaml:"password,omitempty"`
	Exec                  *ExecConfig            `yaml:"exec,omitempty"`
	AuthProvider          *AuthProvider          `yaml:"auth-provider,omitempty"`
	Extensions            []NamedExtension       `yaml:"extensions,omitempty"`
	Extra                 map[string]interface{} `yaml:",inline"`
}

type ExecConfig struct {
	APIVersion         string                 `yaml:"apiVersion,omitempty"`
	Command            string                 `yaml:"command"`
	Args               []string               `yaml:"args,omitempty"`
	Env                []ExecEnvVar           `yaml:"env,omitempty"`
	InstallHint        string                 `yaml:"installHint,omitempty"`
	ProvideClusterInfo bool                   `yaml:"provideClusterInfo,omitempty"`
	InteractiveMode    string                 `yaml:"interactiveMode,omitempty"`
	Extra              map[string]interface{} `yaml:",inline"`
}

type ExecEnvVar struct {
	Name  string                 `yaml:"name"`
	Value string                 `yaml:"value"`
	Extra map[string]interface{} `yaml:",inline"`
}

type AuthProvider struct {
	Name   string                 `yaml:"name"`
	Config map[string]interface{} `yaml:"config,omitempty"`
	Extra  map[string]interface{} `yaml:",inline"`
}

type NamedContext struct {
	Name    string                 `yaml:"name"`
	Context Context                `yaml:"context"`
	Extra   map[string]interface{} `yaml:",inline"`
}

type Context struct {
	Cluster    string                 `yaml:"cluster"`
	User       string                 `yaml:"user"`
	Namespace  string                 `yaml:"namespace,omitempty"`
	Extensions []NamedExtension       `yaml:"extensions,omitempty"`
	Extra      map[string]interface{} `yaml:",inline"`
}

type NamedExtension struct {
	Name      string                 `yaml:"name"`
	Extension map[string]interface{} `yaml:"extension,omitempty"`
	Extra     map[string]interface{} `yaml:",inline"`
}

// unhashableSeq makes signatures of unmarshalable users unique per call —
// a memory address would not be unique (the GC can reuse a freed address
// mid-loop, which was shown to group different users as duplicates).
var unhashableSeq atomic.Uint64

// GetSignature hashes the entire user (json.Marshal sorts map keys, so the
// result is deterministic). Marshaling the whole struct means every field —
// including impersonation, extensions, and unknown inline extras — takes part
// in duplicate detection.
func (u *User) GetSignature() string {
	jsonBytes, err := json.Marshal(u)
	if err != nil {
		// Unmarshalable extras (e.g. non-string map keys) must never make two
		// different users hash equal — a unique per-call signature only
		// disables consolidation for them, which is the safe failure mode.
		jsonBytes = []byte(fmt.Sprintf("unhashable:%d", unhashableSeq.Add(1)))
	}
	hash := sha256.Sum256(jsonBytes)
	return fmt.Sprintf("%x", hash)
}

func (u *User) Equals(other *User) bool {
	if u == nil || other == nil {
		return u == other
	}
	return u.GetSignature() == other.GetSignature()
}

func (u *User) GetAuthMethod() string {
	if u.Exec != nil {
		return "exec"
	}
	if u.Token != "" || u.TokenFile != "" {
		return "token"
	}
	if u.ClientCertificate != "" || u.ClientCertificateData != "" {
		return "certificate"
	}
	if u.Username != "" {
		return "basic"
	}
	if u.AuthProvider != nil {
		return "auth-provider"
	}
	return "unknown"
}

func (c *Config) FindCluster(name string) *NamedCluster {
	for i := range c.Clusters {
		if c.Clusters[i].Name == name {
			return &c.Clusters[i]
		}
	}
	return nil
}

func (c *Config) FindUser(name string) *NamedUser {
	for i := range c.Users {
		if c.Users[i].Name == name {
			return &c.Users[i]
		}
	}
	return nil
}

func (c *Config) FindContext(name string) *NamedContext {
	for i := range c.Contexts {
		if c.Contexts[i].Name == name {
			return &c.Contexts[i]
		}
	}
	return nil
}

func (c *Config) RemoveCluster(name string) bool {
	for i, cluster := range c.Clusters {
		if cluster.Name == name {
			c.Clusters = append(c.Clusters[:i], c.Clusters[i+1:]...)
			return true
		}
	}
	return false
}

func (c *Config) RemoveUser(name string) bool {
	for i, user := range c.Users {
		if user.Name == name {
			c.Users = append(c.Users[:i], c.Users[i+1:]...)
			return true
		}
	}
	return false
}

func (c *Config) RemoveContext(name string) bool {
	for i, context := range c.Contexts {
		if context.Name == name {
			c.Contexts = append(c.Contexts[:i], c.Contexts[i+1:]...)
			if c.CurrentContext == name {
				c.CurrentContext = ""
			}
			return true
		}
	}
	return false
}

func (c *Config) GetContextsUsingUser(userName string) []string {
	var contexts []string
	for _, ctx := range c.Contexts {
		if ctx.Context.User == userName {
			contexts = append(contexts, ctx.Name)
		}
	}
	return contexts
}

func (c *Config) GetContextsUsingCluster(clusterName string) []string {
	var contexts []string
	for _, ctx := range c.Contexts {
		if ctx.Context.Cluster == clusterName {
			contexts = append(contexts, ctx.Name)
		}
	}
	return contexts
}
