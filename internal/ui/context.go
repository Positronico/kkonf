package ui
import (
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/positronico/kkonf/internal/models"
	"github.com/olekukonko/tablewriter"
)
type ContextManager struct {
	config   *models.Config
	colors   *ColorScheme
	modified bool
}
func NewContextManager(config *models.Config, colors *ColorScheme) *ContextManager {
	return &ContextManager{
		config: config,
		colors: colors,
	}
}
func (m *ContextManager) ShowMenu() error {
	for {
		m.clearScreen()
		m.ShowContextList()
		options := []string{
			"1. Add Context",
			"2. Edit Context",
			"3. Rename Context",
			"4. Delete Context",
			"5. View Details",
			"6. Switch Current",
			"7. Set Namespace",
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
			if err := m.addContext(); err != nil {
				return err
			}
		case options[1]:
			if err := m.editContext(); err != nil {
				return err
			}
		case options[2]:
			if err := m.renameContext(); err != nil {
				return err
			}
		case options[3]:
			if err := m.deleteContext(); err != nil {
				return err
			}
		case options[4]:
			m.viewContextDetails()
		case options[5]:
			if err := m.switchContext(); err != nil {
				return err
			}
		case options[6]:
			if err := m.setNamespace(); err != nil {
				return err
			}
		case options[7]:
			return nil
		}
	}
}
func (m *ContextManager) ShowContextList() {
	m.colors.Header("\n🌐 Contexts")
	if len(m.config.Contexts) == 0 {
		m.colors.Warning("  No contexts configured")
		return
	}
	
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"", "#", "Name", "Cluster", "User", "Namespace"})
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
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
	)
	
	for i, ctx := range m.config.Contexts {
		marker := " "
		contextName := ctx.Name
		if ctx.Name == m.config.CurrentContext {
			marker = m.colors.successColor.Sprint("→")
			contextName = m.colors.Bold(m.colors.Context(ctx.Name))
		} else {
			contextName = m.colors.Context(ctx.Name)
		}
		
		namespace := ctx.Context.Namespace
		if namespace == "" {
			namespace = "default"
		}
		
		table.Append([]string{
			marker,
			fmt.Sprintf("%d", i+1),
			contextName,
			m.colors.Cluster(ctx.Context.Cluster),
			m.colors.User(ctx.Context.User),
			namespace,
		})
	}
	table.Render()
	fmt.Println()
}
func (m *ContextManager) addContext() error {
	if len(m.config.Clusters) == 0 {
		m.colors.Error("No clusters available. Please add a cluster first.")
		fmt.Println("Press Enter to continue...")
		fmt.Scanln()
		return nil
	}
	if len(m.config.Users) == 0 {
		m.colors.Error("No users available. Please add a user first.")
		fmt.Println("Press Enter to continue...")
		fmt.Scanln()
		return nil
	}
	var name, cluster, user, namespace string
	namePrompt := &survey.Input{
		Message: "Context name:",
	}
	if err := survey.AskOne(namePrompt, &name, survey.WithValidator(survey.Required)); err != nil {
		return err
	}
	clusterOptions := make([]string, len(m.config.Clusters))
	for i, c := range m.config.Clusters {
		clusterOptions[i] = c.Name
	}
	clusterPrompt := &survey.Select{
		Message: "Select cluster:",
		Options: clusterOptions,
	}
	if err := survey.AskOne(clusterPrompt, &cluster); err != nil {
		return err
	}
	userOptions := make([]string, len(m.config.Users))
	for i, u := range m.config.Users {
		userOptions[i] = u.Name
	}
	userPrompt := &survey.Select{
		Message: "Select user:",
		Options: userOptions,
	}
	if err := survey.AskOne(userPrompt, &user); err != nil {
		return err
	}
	nsPrompt := &survey.Input{
		Message: "Namespace (optional, default: 'default'):",
	}
	if err := survey.AskOne(nsPrompt, &namespace); err != nil {
		return err
	}
	newContext := models.NamedContext{
		Name: name,
		Context: models.Context{
			Cluster:   cluster,
			User:      user,
			Namespace: namespace,
		},
	}
	m.config.Contexts = append(m.config.Contexts, newContext)
	var setCurrent bool
	setCurrentPrompt := &survey.Confirm{
		Message: "Set as current context?",
		Default: false,
	}
	if err := survey.AskOne(setCurrentPrompt, &setCurrent); err != nil {
		return err
	}
	if setCurrent {
		m.config.CurrentContext = name
	}
	m.modified = true
	m.colors.Success("Context '%s' added successfully", name)
	fmt.Println("Press Enter to continue...")
	fmt.Scanln()
	return nil
}
func (m *ContextManager) editContext() error {
	if len(m.config.Contexts) == 0 {
		m.colors.Warning("No contexts to edit")
		fmt.Println("Press Enter to continue...")
		fmt.Scanln()
		return nil
	}
	options := make([]string, len(m.config.Contexts))
	for i, ctx := range m.config.Contexts {
		options[i] = ctx.Name
	}
	var selected string
	prompt := &survey.Select{
		Message: "Select context to edit:",
		Options: options,
	}
	if err := survey.AskOne(prompt, &selected); err != nil {
		return err
	}
	context := m.config.FindContext(selected)
	if context == nil {
		return fmt.Errorf("context not found")
	}
	editOptions := []string{
		"Cluster",
		"User",
		"Namespace",
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
	case "Cluster":
		clusterOptions := make([]string, len(m.config.Clusters))
		for i, c := range m.config.Clusters {
			clusterOptions[i] = c.Name
		}
		var newCluster string
		clusterPrompt := &survey.Select{
			Message: "Select new cluster:",
			Options: clusterOptions,
			Default: context.Context.Cluster,
		}
		if err := survey.AskOne(clusterPrompt, &newCluster); err != nil {
			return err
		}
		context.Context.Cluster = newCluster
		m.modified = true
		m.colors.Success("Cluster updated")
	case "User":
		userOptions := make([]string, len(m.config.Users))
		for i, u := range m.config.Users {
			userOptions[i] = u.Name
		}
		var newUser string
		userPrompt := &survey.Select{
			Message: "Select new user:",
			Options: userOptions,
			Default: context.Context.User,
		}
		if err := survey.AskOne(userPrompt, &newUser); err != nil {
			return err
		}
		context.Context.User = newUser
		m.modified = true
		m.colors.Success("User updated")
	case "Namespace":
		var newNamespace string
		nsPrompt := &survey.Input{
			Message: "New namespace:",
			Default: context.Context.Namespace,
		}
		if err := survey.AskOne(nsPrompt, &newNamespace); err != nil {
			return err
		}
		context.Context.Namespace = newNamespace
		m.modified = true
		m.colors.Success("Namespace updated")
	}
	fmt.Println("Press Enter to continue...")
	fmt.Scanln()
	return nil
}
func (m *ContextManager) renameContext() error {
	if len(m.config.Contexts) == 0 {
		m.colors.Warning("No contexts to rename")
		fmt.Println("Press Enter to continue...")
		fmt.Scanln()
		return nil
	}

	options := make([]string, len(m.config.Contexts))
	for i, ctx := range m.config.Contexts {
		if ctx.Name == m.config.CurrentContext {
			options[i] = fmt.Sprintf("%s (current)", ctx.Name)
		} else {
			options[i] = ctx.Name
		}
	}

	var selected string
	prompt := &survey.Select{
		Message: "Select context to rename:",
		Options: options,
	}
	if err := survey.AskOne(prompt, &selected); err != nil {
		return err
	}

	oldName := strings.Replace(selected, " (current)", "", 1)
	context := m.config.FindContext(oldName)
	if context == nil {
		return fmt.Errorf("context not found")
	}

	var newName string
	namePrompt := &survey.Input{
		Message: "New context name:",
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
	if m.config.FindContext(newName) != nil {
		m.colors.Error("A context with name '%s' already exists", newName)
		fmt.Println("Press Enter to continue...")
		fmt.Scanln()
		return nil
	}

	var confirm bool
	confirmPrompt := &survey.Confirm{
		Message: fmt.Sprintf("Rename context from '%s' to '%s'?", oldName, newName),
		Default: true,
	}
	if err := survey.AskOne(confirmPrompt, &confirm); err != nil {
		return err
	}
	if !confirm {
		return nil
	}

	// Update context name
	context.Name = newName

	// Update current-context if this was the current one
	if m.config.CurrentContext == oldName {
		m.config.CurrentContext = newName
	}

	m.modified = true
	m.colors.Success("Context renamed from '%s' to '%s'", oldName, newName)
	fmt.Println("Press Enter to continue...")
	fmt.Scanln()
	return nil
}

func (m *ContextManager) deleteContext() error {
	if len(m.config.Contexts) == 0 {
		m.colors.Warning("No contexts to delete")
		fmt.Println("Press Enter to continue...")
		fmt.Scanln()
		return nil
	}
	options := make([]string, len(m.config.Contexts))
	for i, ctx := range m.config.Contexts {
		if ctx.Name == m.config.CurrentContext {
			options[i] = fmt.Sprintf("%s (current)", ctx.Name)
		} else {
			options[i] = ctx.Name
		}
	}
	var selected string
	prompt := &survey.Select{
		Message: "Select context to delete:",
		Options: options,
	}
	if err := survey.AskOne(prompt, &selected); err != nil {
		return err
	}
	contextName := strings.Replace(selected, " (current)", "", 1)
	if contextName == m.config.CurrentContext {
		m.colors.Warning("This is the current context")
		var confirm bool
		confirmPrompt := &survey.Confirm{
			Message: "Delete anyway?",
			Default: false,
		}
		if err := survey.AskOne(confirmPrompt, &confirm); err != nil {
			return err
		}
		if !confirm {
			return nil
		}
	}
	if m.config.RemoveContext(contextName) {
		m.modified = true
		m.colors.Success("Context '%s' deleted", contextName)
	}
	fmt.Println("Press Enter to continue...")
	fmt.Scanln()
	return nil
}
func (m *ContextManager) viewContextDetails() {
	if len(m.config.Contexts) == 0 {
		m.colors.Warning("No contexts to view")
		fmt.Println("Press Enter to continue...")
		fmt.Scanln()
		return
	}
	options := make([]string, len(m.config.Contexts))
	for i, ctx := range m.config.Contexts {
		options[i] = ctx.Name
	}
	var selected string
	prompt := &survey.Select{
		Message: "Select context to view:",
		Options: options,
	}
	if err := survey.AskOne(prompt, &selected); err != nil {
		return
	}
	context := m.config.FindContext(selected)
	if context == nil {
		return
	}
	m.clearScreen()
	m.colors.Header("Context Details: %s", context.Name)
	if context.Name == m.config.CurrentContext {
		m.colors.Success("Status: Current Context")
	}
	fmt.Printf("Cluster: %s\n", m.colors.Cluster(context.Context.Cluster))
	cluster := m.config.FindCluster(context.Context.Cluster)
	if cluster != nil {
		fmt.Printf("  Server: %s\n", m.colors.Info(cluster.Cluster.Server))
	} else {
		m.colors.Error("  Cluster not found!")
	}
	fmt.Printf("User: %s\n", m.colors.User(context.Context.User))
	user := m.config.FindUser(context.Context.User)
	if user != nil {
		fmt.Printf("  Auth: %s\n", m.colors.Info(user.User.GetAuthMethod()))
	} else {
		m.colors.Error("  User not found!")
	}
	namespace := context.Context.Namespace
	if namespace == "" {
		namespace = "default"
	}
	fmt.Printf("Namespace: %s\n", m.colors.Info(namespace))
	fmt.Println("\nPress Enter to continue...")
	fmt.Scanln()
}
func (m *ContextManager) switchContext() error {
	if len(m.config.Contexts) == 0 {
		m.colors.Warning("No contexts available")
		fmt.Println("Press Enter to continue...")
		fmt.Scanln()
		return nil
	}
	options := make([]string, len(m.config.Contexts))
	for i, ctx := range m.config.Contexts {
		if ctx.Name == m.config.CurrentContext {
			options[i] = fmt.Sprintf("%s (current)", ctx.Name)
		} else {
			options[i] = ctx.Name
		}
	}
	var selected string
	prompt := &survey.Select{
		Message: "Select context to switch to:",
		Options: options,
	}
	if err := survey.AskOne(prompt, &selected); err != nil {
		return err
	}
	contextName := strings.Replace(selected, " (current)", "", 1)
	if contextName == m.config.CurrentContext {
		m.colors.Info("Already using this context")
	} else {
		m.config.CurrentContext = contextName
		m.modified = true
		m.colors.Success("Switched to context: %s", contextName)
	}
	fmt.Println("Press Enter to continue...")
	fmt.Scanln()
	return nil
}
func (m *ContextManager) setNamespace() error {
	if m.config.CurrentContext == "" {
		m.colors.Warning("No current context set")
		fmt.Println("Press Enter to continue...")
		fmt.Scanln()
		return nil
	}
	context := m.config.FindContext(m.config.CurrentContext)
	if context == nil {
		m.colors.Error("Current context not found")
		fmt.Println("Press Enter to continue...")
		fmt.Scanln()
		return nil
	}
	currentNs := context.Context.Namespace
	if currentNs == "" {
		currentNs = "default"
	}
	var newNamespace string
	prompt := &survey.Input{
		Message: fmt.Sprintf("New namespace for context '%s':", m.config.CurrentContext),
		Default: currentNs,
	}
	if err := survey.AskOne(prompt, &newNamespace); err != nil {
		return err
	}
	if newNamespace == "default" {
		newNamespace = ""
	}
	context.Context.Namespace = newNamespace
	m.modified = true
	displayNamespace := newNamespace
	if displayNamespace == "" {
		displayNamespace = "default"
	}
	m.colors.Success("Namespace updated to: %s", displayNamespace)
	fmt.Println("Press Enter to continue...")
	fmt.Scanln()
	return nil
}
func (m *ContextManager) IsModified() bool {
	return m.modified
}
func (m *ContextManager) clearScreen() {
	fmt.Print("\033[H\033[2J")
}