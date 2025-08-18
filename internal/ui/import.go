package ui
import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/positronico/kkonf/internal/config"
	"github.com/positronico/kkonf/internal/models"
)
type Importer struct {
	config *models.Config
	colors *ColorScheme
}
func NewImporter(config *models.Config, colors *ColorScheme) *Importer {
	return &Importer{
		config: config,
		colors: colors,
	}
}
func (i *Importer) Import(filePath string) (bool, error) {
	loader := config.NewLoader(filePath)
	importedConfig, err := loader.Load()
	if err != nil {
		i.colors.Error("Failed to load import file: %v", err)
		return false, err
	}
	i.colors.Header("Import Summary")
	fmt.Printf("Clusters: %d\n", len(importedConfig.Clusters))
	fmt.Printf("Users: %d\n", len(importedConfig.Users))
	fmt.Printf("Contexts: %d\n", len(importedConfig.Contexts))
	validator := config.NewValidator()
	conflicts := validator.ValidateImport(i.config, importedConfig)
	if len(conflicts) > 0 {
		i.colors.Warning("\nConflicts found:")
		for typ, names := range conflicts {
			fmt.Printf("  %s: %v\n", typ, names)
		}
		strategyOptions := []string{
			"Skip conflicting items",
			"Replace existing items",
			"Rename conflicting items",
			"Interactive (decide for each)",
			"Cancel import",
		}
		var strategy string
		strategyPrompt := &survey.Select{
			Message: "How to handle conflicts:",
			Options: strategyOptions,
		}
		if err := survey.AskOne(strategyPrompt, &strategy); err != nil {
			return false, err
		}
		switch strategy {
		case "Skip conflicting items":
			return i.mergeWithSkip(importedConfig), nil
		case "Replace existing items":
			return i.mergeWithReplace(importedConfig), nil
		case "Rename conflicting items":
			return i.mergeWithRename(importedConfig, conflicts), nil
		case "Interactive (decide for each)":
			return i.mergeInteractive(importedConfig, conflicts), nil
		case "Cancel import":
			return false, nil
		}
	} else {
		var confirm bool
		confirmPrompt := &survey.Confirm{
			Message: "No conflicts found. Proceed with import?",
			Default: true,
		}
		if err := survey.AskOne(confirmPrompt, &confirm); err != nil {
			return false, err
		}
		if !confirm {
			return false, nil
		}
		return i.mergeAll(importedConfig), nil
	}
	return false, nil
}
func (i *Importer) mergeAll(imported *models.Config) bool {
	count := 0
	for _, cluster := range imported.Clusters {
		i.config.Clusters = append(i.config.Clusters, cluster)
		count++
	}
	for _, user := range imported.Users {
		i.config.Users = append(i.config.Users, user)
		count++
	}
	for _, context := range imported.Contexts {
		i.config.Contexts = append(i.config.Contexts, context)
		count++
	}
	i.colors.Success("Imported %d items successfully", count)
	return count > 0
}
func (i *Importer) mergeWithSkip(imported *models.Config) bool {
	count := 0
	for _, cluster := range imported.Clusters {
		if i.config.FindCluster(cluster.Name) == nil {
			i.config.Clusters = append(i.config.Clusters, cluster)
			count++
		}
	}
	for _, user := range imported.Users {
		if i.config.FindUser(user.Name) == nil {
			i.config.Users = append(i.config.Users, user)
			count++
		}
	}
	for _, context := range imported.Contexts {
		if i.config.FindContext(context.Name) == nil {
			i.config.Contexts = append(i.config.Contexts, context)
			count++
		}
	}
	i.colors.Success("Imported %d non-conflicting items", count)
	return count > 0
}
func (i *Importer) mergeWithReplace(imported *models.Config) bool {
	count := 0
	for _, cluster := range imported.Clusters {
		existing := i.config.FindCluster(cluster.Name)
		if existing != nil {
			*existing = cluster
		} else {
			i.config.Clusters = append(i.config.Clusters, cluster)
		}
		count++
	}
	for _, user := range imported.Users {
		existing := i.config.FindUser(user.Name)
		if existing != nil {
			*existing = user
		} else {
			i.config.Users = append(i.config.Users, user)
		}
		count++
	}
	for _, context := range imported.Contexts {
		existing := i.config.FindContext(context.Name)
		if existing != nil {
			*existing = context
		} else {
			i.config.Contexts = append(i.config.Contexts, context)
		}
		count++
	}
	i.colors.Success("Imported/replaced %d items", count)
	return count > 0
}
func (i *Importer) mergeWithRename(imported *models.Config, conflicts map[string][]string) bool {
	count := 0
	for _, cluster := range imported.Clusters {
		if i.config.FindCluster(cluster.Name) != nil {
			if isInConflict(cluster.Name, conflicts["clusters"]) {
				newName := i.getNewName("cluster", cluster.Name)
				if newName != "" {
					cluster.Name = newName
					i.config.Clusters = append(i.config.Clusters, cluster)
					i.updateClusterReferences(imported, cluster.Name, newName)
					count++
				}
			}
		} else {
			i.config.Clusters = append(i.config.Clusters, cluster)
			count++
		}
	}
	for _, user := range imported.Users {
		if i.config.FindUser(user.Name) != nil {
			if isInConflict(user.Name, conflicts["users"]) {
				newName := i.getNewName("user", user.Name)
				if newName != "" {
					user.Name = newName
					i.config.Users = append(i.config.Users, user)
					i.updateUserReferences(imported, user.Name, newName)
					count++
				}
			}
		} else {
			i.config.Users = append(i.config.Users, user)
			count++
		}
	}
	for _, context := range imported.Contexts {
		if i.config.FindContext(context.Name) != nil {
			if isInConflict(context.Name, conflicts["contexts"]) {
				newName := i.getNewName("context", context.Name)
				if newName != "" {
					context.Name = newName
					i.config.Contexts = append(i.config.Contexts, context)
					count++
				}
			}
		} else {
			i.config.Contexts = append(i.config.Contexts, context)
			count++
		}
	}
	i.colors.Success("Imported %d items (with renames)", count)
	return count > 0
}
func (i *Importer) mergeInteractive(imported *models.Config, conflicts map[string][]string) bool {
	count := 0
	for _, cluster := range imported.Clusters {
		if i.config.FindCluster(cluster.Name) != nil && isInConflict(cluster.Name, conflicts["clusters"]) {
			action := i.askConflictAction("cluster", cluster.Name)
			switch action {
			case "skip":
				continue
			case "replace":
				existing := i.config.FindCluster(cluster.Name)
				*existing = cluster
				count++
			case "rename":
				newName := i.getNewName("cluster", cluster.Name)
				if newName != "" {
					cluster.Name = newName
					i.config.Clusters = append(i.config.Clusters, cluster)
					i.updateClusterReferences(imported, cluster.Name, newName)
					count++
				}
			}
		} else if i.config.FindCluster(cluster.Name) == nil {
			i.config.Clusters = append(i.config.Clusters, cluster)
			count++
		}
	}
	for _, user := range imported.Users {
		if i.config.FindUser(user.Name) != nil && isInConflict(user.Name, conflicts["users"]) {
			action := i.askConflictAction("user", user.Name)
			switch action {
			case "skip":
				continue
			case "replace":
				existing := i.config.FindUser(user.Name)
				*existing = user
				count++
			case "rename":
				newName := i.getNewName("user", user.Name)
				if newName != "" {
					user.Name = newName
					i.config.Users = append(i.config.Users, user)
					i.updateUserReferences(imported, user.Name, newName)
					count++
				}
			}
		} else if i.config.FindUser(user.Name) == nil {
			i.config.Users = append(i.config.Users, user)
			count++
		}
	}
	for _, context := range imported.Contexts {
		if i.config.FindContext(context.Name) != nil && isInConflict(context.Name, conflicts["contexts"]) {
			action := i.askConflictAction("context", context.Name)
			switch action {
			case "skip":
				continue
			case "replace":
				existing := i.config.FindContext(context.Name)
				*existing = context
				count++
			case "rename":
				newName := i.getNewName("context", context.Name)
				if newName != "" {
					context.Name = newName
					i.config.Contexts = append(i.config.Contexts, context)
					count++
				}
			}
		} else if i.config.FindContext(context.Name) == nil {
			i.config.Contexts = append(i.config.Contexts, context)
			count++
		}
	}
	i.colors.Success("Imported %d items", count)
	return count > 0
}
func (i *Importer) askConflictAction(typ, name string) string {
	options := []string{"Skip", "Replace", "Rename"}
	var selected string
	prompt := &survey.Select{
		Message: fmt.Sprintf("Conflict for %s '%s':", typ, name),
		Options: options,
	}
	if err := survey.AskOne(prompt, &selected); err != nil {
		return "skip"
	}
	return strings.ToLower(selected)
}
func (i *Importer) getNewName(typ, oldName string) string {
	var newName string
	prompt := &survey.Input{
		Message: fmt.Sprintf("New name for %s '%s':", typ, oldName),
		Default: fmt.Sprintf("%s-imported", oldName),
	}
	if err := survey.AskOne(prompt, &newName); err != nil {
		return ""
	}
	return newName
}
func (i *Importer) updateClusterReferences(config *models.Config, oldName, newName string) {
	for j := range config.Contexts {
		if config.Contexts[j].Context.Cluster == oldName {
			config.Contexts[j].Context.Cluster = newName
		}
	}
}
func (i *Importer) updateUserReferences(config *models.Config, oldName, newName string) {
	for j := range config.Contexts {
		if config.Contexts[j].Context.User == oldName {
			config.Contexts[j].Context.User = newName
		}
	}
}
func isInConflict(name string, conflicts []string) bool {
	for _, c := range conflicts {
		if c == name {
			return true
		}
	}
	return false
}