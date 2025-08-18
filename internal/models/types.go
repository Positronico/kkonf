package models

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
)

type Config struct {
	APIVersion     string                 `yaml:"apiVersion"`
	Kind           string                 `yaml:"kind"`
	CurrentContext string                 `yaml:"current-context"`
	Clusters       []NamedCluster         `yaml:"clusters"`
	Users          []NamedUser            `yaml:"users"`
	Contexts       []NamedContext         `yaml:"contexts"`
	Preferences    map[string]interface{} `yaml:"preferences,omitempty"`
	Extensions     []NamedExtension       `yaml:"extensions,omitempty"`
}

type NamedCluster struct {
	Name    string  `yaml:"name"`
	Cluster Cluster `yaml:"cluster"`
}

type Cluster struct {
	Server                   string `yaml:"server"`
	CertificateAuthorityData string `yaml:"certificate-authority-data,omitempty"`
	CertificateAuthority     string `yaml:"certificate-authority,omitempty"`
	InsecureSkipTLSVerify    bool   `yaml:"insecure-skip-tls-verify,omitempty"`
	ProxyURL                 string `yaml:"proxy-url,omitempty"`
	DisableCompression       bool   `yaml:"disable-compression,omitempty"`
}

type NamedUser struct {
	Name string `yaml:"name"`
	User User   `yaml:"user"`
}

type User struct {
	ClientCertificateData string        `yaml:"client-certificate-data,omitempty"`
	ClientCertificate     string        `yaml:"client-certificate,omitempty"`
	ClientKeyData         string        `yaml:"client-key-data,omitempty"`
	ClientKey             string        `yaml:"client-key,omitempty"`
	Token                 string        `yaml:"token,omitempty"`
	TokenFile             string        `yaml:"tokenFile,omitempty"`
	Username              string        `yaml:"username,omitempty"`
	Password              string        `yaml:"password,omitempty"`
	Exec                  *ExecConfig   `yaml:"exec,omitempty"`
	AuthProvider          *AuthProvider `yaml:"auth-provider,omitempty"`
}

type ExecConfig struct {
	APIVersion         string            `yaml:"apiVersion,omitempty"`
	Command            string            `yaml:"command"`
	Args               []string          `yaml:"args,omitempty"`
	Env                []ExecEnvVar      `yaml:"env,omitempty"`
	InstallHint        string            `yaml:"installHint,omitempty"`
	ProvideClusterInfo bool              `yaml:"provideClusterInfo,omitempty"`
	InteractiveMode    string            `yaml:"interactiveMode,omitempty"`
}

type ExecEnvVar struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

type AuthProvider struct {
	Name   string                 `yaml:"name"`
	Config map[string]interface{} `yaml:"config,omitempty"`
}

type NamedContext struct {
	Name    string  `yaml:"name"`
	Context Context `yaml:"context"`
}

type Context struct {
	Cluster    string           `yaml:"cluster"`
	User       string           `yaml:"user"`
	Namespace  string           `yaml:"namespace,omitempty"`
	Extensions []NamedExtension `yaml:"extensions,omitempty"`
}

type NamedExtension struct {
	Name      string                 `yaml:"name"`
	Extension map[string]interface{} `yaml:"extension,omitempty"`
}

func (u *User) GetSignature() string {
	data := make(map[string]interface{})
	
	if u.ClientCertificateData != "" {
		data["client-certificate-data"] = u.ClientCertificateData
	}
	if u.ClientCertificate != "" {
		data["client-certificate"] = u.ClientCertificate
	}
	if u.ClientKeyData != "" {
		data["client-key-data"] = u.ClientKeyData
	}
	if u.ClientKey != "" {
		data["client-key"] = u.ClientKey
	}
	if u.Token != "" {
		data["token"] = u.Token
	}
	if u.TokenFile != "" {
		data["tokenFile"] = u.TokenFile
	}
	if u.Username != "" {
		data["username"] = u.Username
	}
	if u.Password != "" {
		data["password"] = u.Password
	}
	
	if u.Exec != nil {
		execData := make(map[string]interface{})
		execData["apiVersion"] = u.Exec.APIVersion
		execData["command"] = u.Exec.Command
		execData["args"] = u.Exec.Args
		execData["env"] = u.Exec.Env
		execData["installHint"] = u.Exec.InstallHint
		execData["provideClusterInfo"] = u.Exec.ProvideClusterInfo
		execData["interactiveMode"] = u.Exec.InteractiveMode
		data["exec"] = execData
	}
	
	if u.AuthProvider != nil {
		authData := make(map[string]interface{})
		authData["name"] = u.AuthProvider.Name
		authData["config"] = u.AuthProvider.Config
		data["auth-provider"] = authData
	}
	
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	
	sortedData := make(map[string]interface{})
	for _, k := range keys {
		sortedData[k] = data[k]
	}
	
	jsonBytes, _ := json.Marshal(sortedData)
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