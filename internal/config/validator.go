package config

import (
	"fmt"
	"net/url"
	"reflect"
	"strings"

	"github.com/positronico/kkonf/v2/internal/models"
)

type ValidationError struct {
	Type    string
	Item    string
	Message string
}

func (v ValidationError) Error() string {
	return fmt.Sprintf("%s '%s': %s", v.Type, v.Item, v.Message)
}

type ValidationResult struct {
	Errors   []ValidationError
	Warnings []ValidationError
}

func (v *ValidationResult) IsValid() bool {
	return len(v.Errors) == 0
}

func (v *ValidationResult) AddError(typ, item, message string) {
	v.Errors = append(v.Errors, ValidationError{
		Type:    typ,
		Item:    item,
		Message: message,
	})
}

func (v *ValidationResult) AddWarning(typ, item, message string) {
	v.Warnings = append(v.Warnings, ValidationError{
		Type:    typ,
		Item:    item,
		Message: message,
	})
}

type Validator struct{}

func NewValidator() *Validator {
	return &Validator{}
}

func (v *Validator) Validate(config *models.Config) *ValidationResult {
	result := &ValidationResult{
		Errors:   []ValidationError{},
		Warnings: []ValidationError{},
	}

	v.validateClusters(config, result)
	v.validateUsers(config, result)
	v.validateContexts(config, result)
	v.validateCurrentContext(config, result)
	v.findOrphans(config, result)

	return result
}

func (v *Validator) validateClusters(config *models.Config, result *ValidationResult) {
	clusterNames := make(map[string]bool)

	for _, cluster := range config.Clusters {
		if cluster.Name == "" {
			result.AddError("Cluster", "<unnamed>", "cluster name is required")
			continue
		}

		if clusterNames[cluster.Name] {
			result.AddError("Cluster", cluster.Name, "duplicate cluster name")
		}
		clusterNames[cluster.Name] = true

		if cluster.Cluster.Server == "" {
			result.AddError("Cluster", cluster.Name, "server URL is required")
		} else if _, err := url.Parse(cluster.Cluster.Server); err != nil {
			result.AddError("Cluster", cluster.Name, fmt.Sprintf("invalid server URL: %v", err))
		}

		hasCA := cluster.Cluster.CertificateAuthority != "" || cluster.Cluster.CertificateAuthorityData != ""
		if !hasCA && !cluster.Cluster.InsecureSkipTLSVerify {
			result.AddWarning("Cluster", cluster.Name, "no certificate authority specified and TLS verification is enabled")
		}
	}
}

func (v *Validator) validateUsers(config *models.Config, result *ValidationResult) {
	userNames := make(map[string]bool)

	for _, user := range config.Users {
		if user.Name == "" {
			result.AddError("User", "<unnamed>", "user name is required")
			continue
		}

		if userNames[user.Name] {
			result.AddError("User", user.Name, "duplicate user name")
		}
		userNames[user.Name] = true

		authMethods := 0
		if user.User.ClientCertificate != "" || user.User.ClientCertificateData != "" {
			authMethods++
		}
		if user.User.Token != "" || user.User.TokenFile != "" {
			authMethods++
		}
		if user.User.Username != "" {
			authMethods++
		}
		if user.User.Exec != nil {
			authMethods++
			if user.User.Exec.Command == "" {
				result.AddError("User", user.Name, "exec command is required")
			}
		}
		if user.User.AuthProvider != nil {
			authMethods++
			if user.User.AuthProvider.Name == "" {
				result.AddError("User", user.Name, "auth provider name is required")
			}
		}

		if authMethods == 0 {
			result.AddWarning("User", user.Name, "no authentication method configured")
		}
	}
}

func (v *Validator) validateContexts(config *models.Config, result *ValidationResult) {
	contextNames := make(map[string]bool)

	for _, context := range config.Contexts {
		if context.Name == "" {
			result.AddError("Context", "<unnamed>", "context name is required")
			continue
		}

		if contextNames[context.Name] {
			result.AddError("Context", context.Name, "duplicate context name")
		}
		contextNames[context.Name] = true

		if context.Context.Cluster == "" {
			result.AddError("Context", context.Name, "cluster reference is required")
		} else if config.FindCluster(context.Context.Cluster) == nil {
			result.AddError("Context", context.Name, fmt.Sprintf("references non-existent cluster '%s'", context.Context.Cluster))
		}

		if context.Context.User == "" {
			result.AddError("Context", context.Name, "user reference is required")
		} else if config.FindUser(context.Context.User) == nil {
			result.AddError("Context", context.Name, fmt.Sprintf("references non-existent user '%s'", context.Context.User))
		}
	}
}

func (v *Validator) validateCurrentContext(config *models.Config, result *ValidationResult) {
	if config.CurrentContext != "" {
		if config.FindContext(config.CurrentContext) == nil {
			result.AddError("CurrentContext", config.CurrentContext, "references non-existent context")
		}
	}
}

func (v *Validator) findOrphans(config *models.Config, result *ValidationResult) {
	usedClusters := make(map[string]bool)
	usedUsers := make(map[string]bool)

	for _, context := range config.Contexts {
		usedClusters[context.Context.Cluster] = true
		usedUsers[context.Context.User] = true
	}

	for _, cluster := range config.Clusters {
		if !usedClusters[cluster.Name] {
			result.AddWarning("Cluster", cluster.Name, "not referenced by any context")
		}
	}

	for _, user := range config.Users {
		if !usedUsers[user.Name] {
			result.AddWarning("User", user.Name, "not referenced by any context")
		}
	}
}

func (v *Validator) ValidateImport(existing, imported *models.Config) map[string][]string {
	conflicts := make(map[string][]string)

	for _, cluster := range imported.Clusters {
		if existingCluster := existing.FindCluster(cluster.Name); existingCluster != nil {
			if !clustersEqual(existingCluster, &cluster) {
				conflicts["clusters"] = append(conflicts["clusters"], cluster.Name)
			}
		}
	}

	for _, user := range imported.Users {
		if existingUser := existing.FindUser(user.Name); existingUser != nil {
			if !existingUser.User.Equals(&user.User) {
				conflicts["users"] = append(conflicts["users"], user.Name)
			}
		}
	}

	for _, context := range imported.Contexts {
		if existingContext := existing.FindContext(context.Name); existingContext != nil {
			if !contextsEqual(existingContext, &context) {
				conflicts["contexts"] = append(conflicts["contexts"], context.Name)
			}
		}
	}

	return conflicts
}

// Equality must cover every field — hand-picked comparisons went stale when
// the model grew (tls-server-name, proxy-url, extensions, unknown extras) and
// made imports report "no conflicts" for genuinely different entries.
func clustersEqual(a, b *models.NamedCluster) bool {
	return reflect.DeepEqual(a, b)
}

func contextsEqual(a, b *models.NamedContext) bool {
	return reflect.DeepEqual(a, b)
}

func SanitizeName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, "\\", "-")
	return name
}
