package ui
import (
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/positronico/kkonf/internal/models"
	"github.com/olekukonko/tablewriter"
)
type ClusterManager struct {
	config   *models.Config
	colors   *ColorScheme
	modified bool
}
func NewClusterManager(config *models.Config, colors *ColorScheme) *ClusterManager {
	return &ClusterManager{
		config: config,
		colors: colors,
	}
}
func (m *ClusterManager) ShowMenu() error {
	for {
		m.clearScreen()
		m.ShowClusterList()
		options := []string{
			"1. Add Cluster",
			"2. Edit Cluster",
			"3. Rename Cluster",
			"4. Delete Cluster",
			"5. View Details",
			"0. Back",
		}
		var selected string
		prompt := &survey.Select{
			Message: "Select action:",
			Options: options,
		}
		if err := survey.AskOne(prompt, &selected); err != nil {
			return err
		}
		switch selected {
		case options[0]:
			if err := m.addCluster(); err != nil {
				return err
			}
		case options[1]:
			if err := m.editCluster(); err != nil {
				return err
			}
		case options[2]:
			if err := m.renameCluster(); err != nil {
				return err
			}
		case options[3]:
			if err := m.deleteCluster(); err != nil {
				return err
			}
		case options[4]:
			m.viewClusterDetails()
		case options[5]:
			return nil
		}
	}
}
func (m *ClusterManager) ShowClusterList() {
	m.colors.Header("\n🏢 Clusters")
	if len(m.config.Clusters) == 0 {
		m.colors.Warning("  No clusters configured")
		return
	}
	
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"#", "Name", "Server", "TLS", "Contexts"})
	table.SetBorder(false)
	table.SetRowSeparator("-")
	table.SetHeaderLine(false)
	table.SetColumnSeparator("  ")
	table.SetCenterSeparator("")
	table.SetRowLine(false)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderColor(
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
	)
	
	for i, cluster := range m.config.Clusters {
		secure := "✓"
		if cluster.Cluster.InsecureSkipTLSVerify {
			secure = "✗"
		}
		contexts := m.config.GetContextsUsingCluster(cluster.Name)
		contextCount := fmt.Sprintf("%d", len(contexts))
		
		table.Append([]string{
			fmt.Sprintf("%d", i+1),
			m.colors.Cluster(cluster.Name),
			cluster.Cluster.Server,
			secure,
			contextCount,
		})
	}
	table.Render()
	fmt.Println()
}
func (m *ClusterManager) addCluster() error {
	var name, server, caData, caPath string
	var skipTLS bool
	questions := []*survey.Question{
		{
			Name:     "name",
			Prompt:   &survey.Input{Message: "Cluster name:"},
			Validate: survey.Required,
		},
		{
			Name:     "server",
			Prompt:   &survey.Input{Message: "Server URL:"},
			Validate: survey.Required,
		},
		{
			Name:   "skipTLS",
			Prompt: &survey.Confirm{Message: "Skip TLS verification?", Default: false},
		},
	}
	answers := struct {
		Name    string
		Server  string
		SkipTLS bool
	}{}
	if err := survey.Ask(questions, &answers); err != nil {
		return err
	}
	name = answers.Name
	server = answers.Server
	skipTLS = answers.SkipTLS
	if !skipTLS {
		var caType string
		caPrompt := &survey.Select{
			Message: "Certificate Authority:",
			Options: []string{"Base64 Data", "File Path", "None"},
		}
		if err := survey.AskOne(caPrompt, &caType); err != nil {
			return err
		}
		switch caType {
		case "Base64 Data":
			prompt := &survey.Input{Message: "CA Certificate (base64):"}
			survey.AskOne(prompt, &caData)
		case "File Path":
			prompt := &survey.Input{Message: "CA Certificate file path:"}
			survey.AskOne(prompt, &caPath)
		}
	}
	newCluster := models.NamedCluster{
		Name: name,
		Cluster: models.Cluster{
			Server:                   server,
			CertificateAuthorityData: caData,
			CertificateAuthority:     caPath,
			InsecureSkipTLSVerify:    skipTLS,
		},
	}
	m.config.Clusters = append(m.config.Clusters, newCluster)
	m.modified = true
	m.colors.Success("Cluster '%s' added successfully", name)
	fmt.Println("Press Enter to continue...")
	fmt.Scanln()
	return nil
}
func (m *ClusterManager) editCluster() error {
	if len(m.config.Clusters) == 0 {
		m.colors.Warning("No clusters to edit")
		fmt.Println("Press Enter to continue...")
		fmt.Scanln()
		return nil
	}
	options := make([]string, len(m.config.Clusters))
	for i, cluster := range m.config.Clusters {
		options[i] = cluster.Name
	}
	var selected string
	prompt := &survey.Select{
		Message: "Select cluster to edit:",
		Options: options,
	}
	if err := survey.AskOne(prompt, &selected); err != nil {
		return err
	}
	cluster := m.config.FindCluster(selected)
	if cluster == nil {
		return fmt.Errorf("cluster not found")
	}
	editOptions := []string{
		"Server URL",
		"Certificate Authority",
		"TLS Verification",
		"Back",
	}
	var editChoice string
	editPrompt := &survey.Select{
		Message: "What to edit:",
		Options: editOptions,
	}
	if err := survey.AskOne(editPrompt, &editChoice); err != nil {
		return err
	}
	switch editChoice {
	case "Server URL":
		var newServer string
		prompt := &survey.Input{
			Message: "New server URL:",
			Default: cluster.Cluster.Server,
		}
		if err := survey.AskOne(prompt, &newServer); err != nil {
			return err
		}
		cluster.Cluster.Server = newServer
		m.modified = true
		m.colors.Success("Server URL updated")
	case "Certificate Authority":
		var caType string
		caPrompt := &survey.Select{
			Message: "Certificate Authority type:",
			Options: []string{"Base64 Data", "File Path", "None"},
		}
		if err := survey.AskOne(caPrompt, &caType); err != nil {
			return err
		}
		cluster.Cluster.CertificateAuthorityData = ""
		cluster.Cluster.CertificateAuthority = ""
		switch caType {
		case "Base64 Data":
			var caData string
			prompt := &survey.Input{Message: "CA Certificate (base64):"}
			survey.AskOne(prompt, &caData)
			cluster.Cluster.CertificateAuthorityData = caData
		case "File Path":
			var caPath string
			prompt := &survey.Input{Message: "CA Certificate file path:"}
			survey.AskOne(prompt, &caPath)
			cluster.Cluster.CertificateAuthority = caPath
		}
		m.modified = true
		m.colors.Success("Certificate Authority updated")
	case "TLS Verification":
		var skipTLS bool
		prompt := &survey.Confirm{
			Message: "Skip TLS verification?",
			Default: cluster.Cluster.InsecureSkipTLSVerify,
		}
		if err := survey.AskOne(prompt, &skipTLS); err != nil {
			return err
		}
		cluster.Cluster.InsecureSkipTLSVerify = skipTLS
		m.modified = true
		m.colors.Success("TLS verification setting updated")
	}
	fmt.Println("Press Enter to continue...")
	fmt.Scanln()
	return nil
}
func (m *ClusterManager) renameCluster() error {
	if len(m.config.Clusters) == 0 {
		m.colors.Warning("No clusters to rename")
		fmt.Println("Press Enter to continue...")
		fmt.Scanln()
		return nil
	}

	options := make([]string, len(m.config.Clusters))
	for i, cluster := range m.config.Clusters {
		contexts := m.config.GetContextsUsingCluster(cluster.Name)
		if len(contexts) > 0 {
			options[i] = fmt.Sprintf("%s (used by %d contexts)", cluster.Name, len(contexts))
		} else {
			options[i] = cluster.Name
		}
	}

	var selected string
	prompt := &survey.Select{
		Message: "Select cluster to rename:",
		Options: options,
	}
	if err := survey.AskOne(prompt, &selected); err != nil {
		return err
	}

	oldName := strings.Split(selected, " (")[0]
	cluster := m.config.FindCluster(oldName)
	if cluster == nil {
		return fmt.Errorf("cluster not found")
	}

	var newName string
	namePrompt := &survey.Input{
		Message: "New cluster name:",
		Default: oldName,
	}
	if err := survey.AskOne(namePrompt, &newName, survey.WithValidator(survey.Required)); err != nil {
		return err
	}

	if newName == oldName {
		m.colors.Info("Name unchanged")
		fmt.Println("Press Enter to continue...")
		fmt.Scanln()
		return nil
	}

	// Check if new name already exists
	if m.config.FindCluster(newName) != nil {
		m.colors.Error("A cluster with name '%s' already exists", newName)
		fmt.Println("Press Enter to continue...")
		fmt.Scanln()
		return nil
	}

	// Show contexts that will be updated
	contexts := m.config.GetContextsUsingCluster(oldName)
	if len(contexts) > 0 {
		m.colors.Info("The following contexts will be updated:")
		for _, ctx := range contexts {
			fmt.Printf("  - %s\n", ctx)
		}
	}

	var confirm bool
	confirmPrompt := &survey.Confirm{
		Message: fmt.Sprintf("Rename cluster from '%s' to '%s'?", oldName, newName),
		Default: true,
	}
	if err := survey.AskOne(confirmPrompt, &confirm); err != nil {
		return err
	}
	if !confirm {
		return nil
	}

	// Update cluster name
	cluster.Name = newName

	// Update all context references
	for i := range m.config.Contexts {
		if m.config.Contexts[i].Context.Cluster == oldName {
			m.config.Contexts[i].Context.Cluster = newName
		}
	}

	m.modified = true
	m.colors.Success("Cluster renamed from '%s' to '%s'", oldName, newName)
	fmt.Println("Press Enter to continue...")
	fmt.Scanln()
	return nil
}

func (m *ClusterManager) deleteCluster() error {
	if len(m.config.Clusters) == 0 {
		m.colors.Warning("No clusters to delete")
		fmt.Println("Press Enter to continue...")
		fmt.Scanln()
		return nil
	}
	options := make([]string, len(m.config.Clusters))
	for i, cluster := range m.config.Clusters {
		contexts := m.config.GetContextsUsingCluster(cluster.Name)
		if len(contexts) > 0 {
			options[i] = fmt.Sprintf("%s (used by %d contexts)", cluster.Name, len(contexts))
		} else {
			options[i] = cluster.Name
		}
	}
	var selected string
	prompt := &survey.Select{
		Message: "Select cluster to delete:",
		Options: options,
	}
	if err := survey.AskOne(prompt, &selected); err != nil {
		return err
	}
	clusterName := strings.Split(selected, " (")[0]
	contexts := m.config.GetContextsUsingCluster(clusterName)
	if len(contexts) > 0 {
		m.colors.Warning("This cluster is used by the following contexts:")
		for _, ctx := range contexts {
			fmt.Printf("  - %s\n", ctx)
		}
		var confirm bool
		confirmPrompt := &survey.Confirm{
			Message: "Delete anyway? (contexts will be broken)",
			Default: false,
		}
		if err := survey.AskOne(confirmPrompt, &confirm); err != nil {
			return err
		}
		if !confirm {
			return nil
		}
	}
	if m.config.RemoveCluster(clusterName) {
		m.modified = true
		m.colors.Success("Cluster '%s' deleted", clusterName)
	}
	fmt.Println("Press Enter to continue...")
	fmt.Scanln()
	return nil
}
func (m *ClusterManager) viewClusterDetails() {
	if len(m.config.Clusters) == 0 {
		m.colors.Warning("No clusters to view")
		fmt.Println("Press Enter to continue...")
		fmt.Scanln()
		return
	}
	options := make([]string, len(m.config.Clusters))
	for i, cluster := range m.config.Clusters {
		options[i] = cluster.Name
	}
	var selected string
	prompt := &survey.Select{
		Message: "Select cluster to view:",
		Options: options,
	}
	if err := survey.AskOne(prompt, &selected); err != nil {
		return
	}
	cluster := m.config.FindCluster(selected)
	if cluster == nil {
		return
	}
	m.clearScreen()
	m.colors.Header("Cluster Details: %s", cluster.Name)
	fmt.Printf("Server: %s\n", m.colors.Info(cluster.Cluster.Server))
	if cluster.Cluster.InsecureSkipTLSVerify {
		m.colors.Warning("TLS Verification: Disabled")
	} else {
		fmt.Printf("TLS Verification: %s\n", m.colors.Info("Enabled"))
	}
	if cluster.Cluster.CertificateAuthorityData != "" {
		fmt.Printf("Certificate Authority: %s\n", m.colors.Info("Base64 Data (embedded)"))
	} else if cluster.Cluster.CertificateAuthority != "" {
		fmt.Printf("Certificate Authority: %s\n", m.colors.Info(cluster.Cluster.CertificateAuthority))
	} else {
		m.colors.Warning("Certificate Authority: None")
	}
	if cluster.Cluster.ProxyURL != "" {
		fmt.Printf("Proxy URL: %s\n", m.colors.Info(cluster.Cluster.ProxyURL))
	}
	if cluster.Cluster.DisableCompression {
		m.colors.Warning("Compression: Disabled")
	}
	contexts := m.config.GetContextsUsingCluster(cluster.Name)
	if len(contexts) > 0 {
		fmt.Printf("\nUsed by contexts:\n")
		for _, ctx := range contexts {
			fmt.Printf("  - %s\n", m.colors.Context(ctx))
		}
	}
	fmt.Println("\nPress Enter to continue...")
	fmt.Scanln()
}
func (m *ClusterManager) IsModified() bool {
	return m.modified
}
func (m *ClusterManager) clearScreen() {
	fmt.Print("\033[H\033[2J")
}