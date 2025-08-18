package ui
import (
	"fmt"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/positronico/kkonf/internal/config"
	"github.com/positronico/kkonf/internal/models"
)
type Exporter struct {
	config *models.Config
	colors *ColorScheme
}
func NewExporter(config *models.Config, colors *ColorScheme) *Exporter {
	return &Exporter{
		config: config,
		colors: colors,
	}
}
func (e *Exporter) Export() error {
	exportType := []string{
		"Export selected items",
		"Export all",
		"Export current context and dependencies",
		"Cancel",
	}
	var selected string
	typePrompt := &survey.Select{
		Message: "Export type:",
		Options: exportType,
	}
	if err := survey.AskOne(typePrompt, &selected); err != nil {
		return err
	}
	var exportConfig *models.Config
	switch selected {
	case "Export selected items":
		exportConfig = e.selectItemsToExport()
	case "Export all":
		exportConfig = e.config
	case "Export current context and dependencies":
		exportConfig = e.exportCurrentContext()
	case "Cancel":
		return nil
	}
	if exportConfig == nil {
		return nil
	}
	var outputPath string
	pathPrompt := &survey.Input{
		Message: "Export file path:",
		Default: "kubeconfig-export.yaml",
		Suggest: func(toComplete string) []string {
			files, _ := filepath.Glob(toComplete + "*")
			return files
		},
	}
	if err := survey.AskOne(pathPrompt, &outputPath); err != nil {
		return err
	}
	writer := config.NewWriter(outputPath)
	if err := writer.Save(exportConfig); err != nil {
		e.colors.Error("Failed to export: %v", err)
		return err
	}
	e.colors.Success("Configuration exported to: %s", outputPath)
	fmt.Println("Press Enter to continue...")
	fmt.Scanln()
	return nil
}
func (e *Exporter) selectItemsToExport() *models.Config {
	exportConfig := &models.Config{
		APIVersion:  e.config.APIVersion,
		Kind:        e.config.Kind,
		Preferences: e.config.Preferences,
	}
	// Select clusters
	if len(e.config.Clusters) > 0 {
		clusterOptions := make([]string, len(e.config.Clusters))
		for i, cluster := range e.config.Clusters {
			clusterOptions[i] = cluster.Name
		}
		var selectedClusters []string
		clusterPrompt := &survey.MultiSelect{
			Message: "Select clusters to export:",
			Options: clusterOptions,
		}
		if err := survey.AskOne(clusterPrompt, &selectedClusters); err != nil {
			return nil
		}
		for _, name := range selectedClusters {
			if cluster := e.config.FindCluster(name); cluster != nil {
				exportConfig.Clusters = append(exportConfig.Clusters, *cluster)
			}
		}
	}
	// Select users
	if len(e.config.Users) > 0 {
		userOptions := make([]string, len(e.config.Users))
		for i, user := range e.config.Users {
			userOptions[i] = user.Name
		}
		var selectedUsers []string
		userPrompt := &survey.MultiSelect{
			Message: "Select users to export:",
			Options: userOptions,
		}
		if err := survey.AskOne(userPrompt, &selectedUsers); err != nil {
			return nil
		}
		for _, name := range selectedUsers {
			if user := e.config.FindUser(name); user != nil {
				exportConfig.Users = append(exportConfig.Users, *user)
			}
		}
	}
	// Select contexts
	if len(e.config.Contexts) > 0 {
		contextOptions := make([]string, len(e.config.Contexts))
		for i, context := range e.config.Contexts {
			contextOptions[i] = context.Name
		}
		var selectedContexts []string
		contextPrompt := &survey.MultiSelect{
			Message: "Select contexts to export:",
			Options: contextOptions,
		}
		if err := survey.AskOne(contextPrompt, &selectedContexts); err != nil {
			return nil
		}
		for _, name := range selectedContexts {
			if context := e.config.FindContext(name); context != nil {
				exportConfig.Contexts = append(exportConfig.Contexts, *context)
				// Check for dependencies
				if cluster := e.config.FindCluster(context.Context.Cluster); cluster != nil {
					if !e.containsCluster(exportConfig, cluster.Name) {
						exportConfig.Clusters = append(exportConfig.Clusters, *cluster)
					}
				}
				if user := e.config.FindUser(context.Context.User); user != nil {
					if !e.containsUser(exportConfig, user.Name) {
						exportConfig.Users = append(exportConfig.Users, *user)
					}
				}
			}
		}
		// Check if current context should be included
		for _, ctx := range exportConfig.Contexts {
			if ctx.Name == e.config.CurrentContext {
				exportConfig.CurrentContext = e.config.CurrentContext
				break
			}
		}
	}
	return exportConfig
}
func (e *Exporter) exportCurrentContext() *models.Config {
	if e.config.CurrentContext == "" {
		e.colors.Warning("No current context set")
		return nil
	}
	exportConfig := &models.Config{
		APIVersion:     e.config.APIVersion,
		Kind:           e.config.Kind,
		CurrentContext: e.config.CurrentContext,
		Preferences:    e.config.Preferences,
	}
	context := e.config.FindContext(e.config.CurrentContext)
	if context == nil {
		e.colors.Error("Current context not found")
		return nil
	}
	exportConfig.Contexts = append(exportConfig.Contexts, *context)
	if cluster := e.config.FindCluster(context.Context.Cluster); cluster != nil {
		exportConfig.Clusters = append(exportConfig.Clusters, *cluster)
	}
	if user := e.config.FindUser(context.Context.User); user != nil {
		exportConfig.Users = append(exportConfig.Users, *user)
	}
	fmt.Printf("Exporting current context '%s' with dependencies\n", e.config.CurrentContext)
	return exportConfig
}
func (e *Exporter) containsCluster(config *models.Config, name string) bool {
	for _, cluster := range config.Clusters {
		if cluster.Name == name {
			return true
		}
	}
	return false
}
func (e *Exporter) containsUser(config *models.Config, name string) bool {
	for _, user := range config.Users {
		if user.Name == name {
			return true
		}
	}
	return false
}