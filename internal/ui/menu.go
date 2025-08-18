package ui
import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/positronico/kkonf/internal/config"
	"github.com/positronico/kkonf/internal/models"
	"github.com/positronico/kkonf/internal/version"
)
type MainMenu struct {
	configPath string
	config     *models.Config
	loader     *config.Loader
	writer     *config.Writer
	validator  *config.Validator
	colors     *ColorScheme
	modified   bool
}
func NewMainMenu(configPath string, useColors bool) *MainMenu {
	return &MainMenu{
		configPath: configPath,
		loader:     config.NewLoader(configPath),
		writer:     config.NewWriter(configPath),
		validator:  config.NewValidator(),
		colors:     NewColorScheme(useColors),
	}
}
func (m *MainMenu) Run() error {
	if err := m.loadConfig(); err != nil {
		return err
	}
	for {
		if err := m.showMainMenu(); err != nil {
			if err.Error() == "exit" {
				if m.modified {
					if m.confirmSave() {
						return m.saveConfig()
					}
				}
				return nil
			}
			return err
		}
	}
}
func (m *MainMenu) loadConfig() error {
	cfg, err := m.loader.Load()
	if err != nil {
		return err
	}
	m.config = cfg
	m.modified = false
	return nil
}
func (m *MainMenu) saveConfig() error {
	validation := m.validator.Validate(m.config)
	if !validation.IsValid() {
		m.colors.Error("Configuration has errors:")
		for _, err := range validation.Errors {
			fmt.Printf("  - %s\n", err.Error())
		}
		if !m.confirmForceSave() {
			return fmt.Errorf("save cancelled due to validation errors")
		}
	}
	if err := m.writer.Save(m.config); err != nil {
		return err
	}
	m.modified = false
	m.colors.Success("Configuration saved successfully")
	return nil
}
func (m *MainMenu) showMainMenu() error {
	m.clearScreen()
	m.showHeader()
	options := []string{
		fmt.Sprintf("1. 🏢 Clusters (%d)", len(m.config.Clusters)),
		fmt.Sprintf("2. 👤 Users (%d)", len(m.config.Users)),
		fmt.Sprintf("3. 🌐 Contexts (%d)", len(m.config.Contexts)),
		"4. 🔧 Tools",
		"5. ⚙️ Settings",
		"6. 💾 Save Configuration",
		"0. 🚪 Exit",
	}
	var selected string
	prompt := &survey.Select{
		Message: "Select option:",
		Options: options,
	}
	if err := survey.AskOne(prompt, &selected); err != nil {
		return err
	}
	switch selected {
	case options[0]: // Clusters
		return m.showClustersMenu()
	case options[1]: // Users
		return m.showUsersMenu()
	case options[2]: // Contexts
		return m.showContextsMenu()
	case options[3]: // Tools
		return m.showToolsMenu()
	case options[4]: // Settings
		return m.showSettingsMenu()
	case options[5]: // Save
		return m.saveConfigurationMenu()
	case options[6]: // Exit
		return fmt.Errorf("exit")
	}
	return nil
}
func (m *MainMenu) showHeader() {
	versionInfo := version.Get()
	m.colors.Header(fmt.Sprintf("\n⚙️ %s - kubectl Config Manager", versionInfo.String()))
	fmt.Println()
	fmt.Printf("📁 Config file: %s\n", m.colors.Info(m.configPath))
	if m.config.CurrentContext != "" {
		fmt.Printf("🎯 Current context: %s\n", m.colors.Bold(m.colors.Context(m.config.CurrentContext)))
	} else {
		m.colors.Warning("⚠️ Current context: none")
	}
	if m.modified {
		m.colors.Warning("📝 Status: Modified (unsaved)")
	} else {
		fmt.Printf("✓ Status: %s\n", m.colors.Info("Saved"))
	}
	fmt.Println()
}
func (m *MainMenu) showClustersMenu() error {
	manager := NewClusterManager(m.config, m.colors)
	if err := manager.ShowMenu(); err != nil {
		return err
	}
	m.modified = m.modified || manager.IsModified()
	return nil
}
func (m *MainMenu) showUsersMenu() error {
	manager := NewUserManager(m.config, m.colors)
	if err := manager.ShowMenu(); err != nil {
		return err
	}
	m.modified = m.modified || manager.IsModified()
	return nil
}
func (m *MainMenu) showContextsMenu() error {
	manager := NewContextManager(m.config, m.colors)
	if err := manager.ShowMenu(); err != nil {
		return err
	}
	m.modified = m.modified || manager.IsModified()
	return nil
}
func (m *MainMenu) showToolsMenu() error {
	options := []string{
		"1. ✓ Validate Configuration",
		"2. 🔄 Consolidate Duplicate Users",
		"3. 📥 Import Configuration",
		"4. 📤 Export Configuration",
		"5. ⚡ Quick Context Switch",
		"6. 🗑️ Clean Old Backups",
		"0. ← Back",
	}
	var selected string
	prompt := &survey.Select{
		Message: "Select tool:",
		Options: options,
	}
	if err := survey.AskOne(prompt, &selected); err != nil {
		return err
	}
	switch selected {
	case options[0]: // Validate
		m.validateConfig()
	case options[1]: // Consolidate
		if err := m.consolidateUsers(); err != nil {
			return err
		}
	case options[2]: // Import
		if err := m.importConfig(); err != nil {
			return err
		}
	case options[3]: // Export
		if err := m.exportConfig(); err != nil {
			return err
		}
	case options[4]: // Quick Switch
		if err := m.quickContextSwitch(); err != nil {
			return err
		}
	case options[5]: // Clean backups
		m.cleanBackups()
	}
	return nil
}
func (m *MainMenu) showSettingsMenu() error {
	fmt.Println("Settings menu - coming soon")
	fmt.Println("Press Enter to continue...")
	fmt.Scanln()
	return nil
}
func (m *MainMenu) saveConfigurationMenu() error {
	if !m.modified {
		m.colors.Info("✓ No changes to save")
		fmt.Println("Press Enter to continue...")
		fmt.Scanln()
		return nil
	}
	
	if err := m.saveConfig(); err != nil {
		m.colors.Error("Failed to save configuration: %v", err)
	} else {
		m.colors.Success("✓ Configuration saved successfully")
	}
	
	fmt.Println("Press Enter to continue...")
	fmt.Scanln()
	return nil
}
func (m *MainMenu) validateConfig() {
	validation := m.validator.Validate(m.config)
	if validation.IsValid() && len(validation.Warnings) == 0 {
		m.colors.Success("✓ Configuration is valid")
	} else {
		if len(validation.Errors) > 0 {
			m.colors.Error("Errors found:")
			for _, err := range validation.Errors {
				fmt.Printf("  ✗ %s\n", err.Error())
			}
		}
		if len(validation.Warnings) > 0 {
			m.colors.Warning("\nWarnings:")
			for _, warn := range validation.Warnings {
				fmt.Printf("  ⚠ %s\n", warn.Error())
			}
		}
	}
	fmt.Println("\nPress Enter to continue...")
	fmt.Scanln()
}
func (m *MainMenu) consolidateUsers() error {
	consolidator := NewUserConsolidator(m.config, m.colors)
	modified, err := consolidator.Consolidate()
	if err != nil {
		return err
	}
	m.modified = m.modified || modified
	return nil
}
func (m *MainMenu) importConfig() error {
	var importPath string
	prompt := &survey.Input{
		Message: "Enter path to config file to import:",
		Suggest: func(toComplete string) []string {
			files, _ := filepath.Glob(toComplete + "*")
			return files
		},
	}
	if err := survey.AskOne(prompt, &importPath); err != nil {
		return err
	}
	if _, err := os.Stat(importPath); os.IsNotExist(err) {
		m.colors.Error("File not found: %s", importPath)
		fmt.Println("Press Enter to continue...")
		fmt.Scanln()
		return nil
	}
	importer := NewImporter(m.config, m.colors)
	modified, err := importer.Import(importPath)
	if err != nil {
		return err
	}
	m.modified = m.modified || modified
	return nil
}
func (m *MainMenu) exportConfig() error {
	exporter := NewExporter(m.config, m.colors)
	return exporter.Export()
}
func (m *MainMenu) quickContextSwitch() error {
	if len(m.config.Contexts) == 0 {
		m.colors.Warning("No contexts available")
		fmt.Println("Press Enter to continue...")
		fmt.Scanln()
		return nil
	}
	options := make([]string, len(m.config.Contexts))
	for i, ctx := range m.config.Contexts {
		marker := "  "
		if ctx.Name == m.config.CurrentContext {
			marker = "* "
		}
		options[i] = fmt.Sprintf("%s%s", marker, ctx.Name)
	}
	var selected string
	prompt := &survey.Select{
		Message: "Select context to switch to:",
		Options: options,
	}
	if err := survey.AskOne(prompt, &selected); err != nil {
		return err
	}
	contextName := selected[2:] // Remove marker
	m.config.CurrentContext = contextName
	m.modified = true
	m.colors.Success("Switched to context: %s", contextName)
	fmt.Println("Press Enter to continue...")
	fmt.Scanln()
	return nil
}
func (m *MainMenu) cleanBackups() {
	if err := config.CleanOldBackups(m.configPath, 7); err != nil {
		m.colors.Error("Failed to clean backups: %v", err)
	} else {
		m.colors.Success("Old backups cleaned (kept last 7 days)")
	}
	fmt.Println("Press Enter to continue...")
	fmt.Scanln()
}
func (m *MainMenu) confirmSave() bool {
	var save bool
	prompt := &survey.Confirm{
		Message: "You have unsaved changes. Save before exiting?",
		Default: true,
	}
	survey.AskOne(prompt, &save)
	return save
}
func (m *MainMenu) confirmForceSave() bool {
	var save bool
	prompt := &survey.Confirm{
		Message: "Configuration has errors. Save anyway?",
		Default: false,
	}
	survey.AskOne(prompt, &save)
	return save
}
func (m *MainMenu) clearScreen() {
	fmt.Print("\033[H\033[2J")
}