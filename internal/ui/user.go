package ui
import (
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/positronico/kkonf/internal/models"
	"github.com/olekukonko/tablewriter"
)
type UserManager struct {
	config   *models.Config
	colors   *ColorScheme
	modified bool
}
func NewUserManager(config *models.Config, colors *ColorScheme) *UserManager {
	return &UserManager{
		config: config,
		colors: colors,
	}
}
func (m *UserManager) ShowMenu() error {
	for {
		m.clearScreen()
		m.ShowUserList()
		options := []string{
			"1. Add User",
			"2. Edit User",
			"3. Rename User",
			"4. Delete User",
			"5. View Details",
			"6. Consolidate Duplicates",
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
			if err := m.addUser(); err != nil {
				return err
			}
		case options[1]:
			if err := m.editUser(); err != nil {
				return err
			}
		case options[2]:
			if err := m.renameUser(); err != nil {
				return err
			}
		case options[3]:
			if err := m.deleteUser(); err != nil {
				return err
			}
		case options[4]:
			m.viewUserDetails()
		case options[5]:
			if err := m.consolidateUsers(); err != nil {
				return err
			}
		case options[6]:
			return nil
		}
	}
}
func (m *UserManager) ShowUserList() {
	m.colors.Header("\n👤 Users")
	if len(m.config.Users) == 0 {
		m.colors.Warning("  No users configured")
		return
	}
	
	duplicates := m.findDuplicates()
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"#", "Name", "Auth Method", "Contexts", "Status"})
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
	
	for i, user := range m.config.Users {
		contexts := m.config.GetContextsUsingUser(user.Name)
		contextCount := fmt.Sprintf("%d", len(contexts))
		status := "✓"
		if _, isDup := duplicates[user.User.GetSignature()]; isDup {
			status = m.colors.warningColor.Sprint("⚠")
		}
		
		table.Append([]string{
			fmt.Sprintf("%d", i+1),
			m.colors.User(user.Name),
			user.User.GetAuthMethod(),
			contextCount,
			status,
		})
	}
	table.Render()
	fmt.Println()
}
func (m *UserManager) findDuplicates() map[string][]string {
	signatures := make(map[string][]string)
	for _, user := range m.config.Users {
		sig := user.User.GetSignature()
		signatures[sig] = append(signatures[sig], user.Name)
	}
	duplicates := make(map[string][]string)
	for sig, names := range signatures {
		if len(names) > 1 {
			duplicates[sig] = names
		}
	}
	return duplicates
}
func (m *UserManager) addUser() error {
	var name string
	namePrompt := &survey.Input{
		Message: "User name:",
	}
	if err := survey.AskOne(namePrompt, &name, survey.WithValidator(survey.Required)); err != nil {
		return err
	}
	authMethods := []string{
		"Exec (command)",
		"Token",
		"Certificate",
		"Basic (username/password)",
		"Cancel",
	}
	var authMethod string
	authPrompt := &survey.Select{
		Message: "Authentication method:",
		Options: authMethods,
	}
	if err := survey.AskOne(authPrompt, &authMethod); err != nil {
		return err
	}
	newUser := models.NamedUser{
		Name: name,
		User: models.User{},
	}
	switch authMethod {
	case "Exec (command)":
		if err := m.configureExecAuth(&newUser.User); err != nil {
			return err
		}
	case "Token":
		if err := m.configureTokenAuth(&newUser.User); err != nil {
			return err
		}
	case "Certificate":
		if err := m.configureCertAuth(&newUser.User); err != nil {
			return err
		}
	case "Basic (username/password)":
		if err := m.configureBasicAuth(&newUser.User); err != nil {
			return err
		}
	case "Cancel":
		return nil
	}
	m.config.Users = append(m.config.Users, newUser)
	m.modified = true
	m.colors.Success("User '%s' added successfully", name)
	fmt.Println("Press Enter to continue...")
	fmt.Scanln()
	return nil
}
func (m *UserManager) configureExecAuth(user *models.User) error {
	exec := &models.ExecConfig{}
	questions := []*survey.Question{
		{
			Name:     "command",
			Prompt:   &survey.Input{Message: "Command:"},
			Validate: survey.Required,
		},
		{
			Name:   "apiVersion",
			Prompt: &survey.Input{Message: "API Version:", Default: "client.authentication.k8s.io/v1beta1"},
		},
		{
			Name:   "provideClusterInfo",
			Prompt: &survey.Confirm{Message: "Provide cluster info?", Default: false},
		},
	}
	answers := struct {
		Command            string
		ApiVersion         string
		ProvideClusterInfo bool
	}{}
	if err := survey.Ask(questions, &answers); err != nil {
		return err
	}
	exec.Command = answers.Command
	exec.APIVersion = answers.ApiVersion
	exec.ProvideClusterInfo = answers.ProvideClusterInfo
	var args string
	argsPrompt := &survey.Input{
		Message: "Arguments (comma-separated, optional):",
	}
	if err := survey.AskOne(argsPrompt, &args); err != nil {
		return err
	}
	if args != "" {
		exec.Args = strings.Split(args, ",")
		for i := range exec.Args {
			exec.Args[i] = strings.TrimSpace(exec.Args[i])
		}
	}
	var installHint string
	hintPrompt := &survey.Input{
		Message: "Install hint (optional):",
	}
	if err := survey.AskOne(hintPrompt, &installHint); err != nil {
		return err
	}
	exec.InstallHint = installHint
	user.Exec = exec
	return nil
}
func (m *UserManager) configureTokenAuth(user *models.User) error {
	tokenType := []string{"Direct token", "Token file"}
	var selected string
	prompt := &survey.Select{
		Message: "Token type:",
		Options: tokenType,
	}
	if err := survey.AskOne(prompt, &selected); err != nil {
		return err
	}
	if selected == "Direct token" {
		var token string
		tokenPrompt := &survey.Input{
			Message: "Token:",
		}
		if err := survey.AskOne(tokenPrompt, &token, survey.WithValidator(survey.Required)); err != nil {
			return err
		}
		user.Token = token
	} else {
		var tokenFile string
		filePrompt := &survey.Input{
			Message: "Token file path:",
		}
		if err := survey.AskOne(filePrompt, &tokenFile, survey.WithValidator(survey.Required)); err != nil {
			return err
		}
		user.TokenFile = tokenFile
	}
	return nil
}
func (m *UserManager) configureCertAuth(user *models.User) error {
	certType := []string{"Base64 data", "File paths"}
	var selected string
	prompt := &survey.Select{
		Message: "Certificate type:",
		Options: certType,
	}
	if err := survey.AskOne(prompt, &selected); err != nil {
		return err
	}
	if selected == "Base64 data" {
		var certData, keyData string
		certPrompt := &survey.Input{
			Message: "Client certificate (base64):",
		}
		if err := survey.AskOne(certPrompt, &certData); err != nil {
			return err
		}
		keyPrompt := &survey.Input{
			Message: "Client key (base64):",
		}
		if err := survey.AskOne(keyPrompt, &keyData); err != nil {
			return err
		}
		user.ClientCertificateData = certData
		user.ClientKeyData = keyData
	} else {
		var certFile, keyFile string
		certPrompt := &survey.Input{
			Message: "Client certificate file:",
		}
		if err := survey.AskOne(certPrompt, &certFile); err != nil {
			return err
		}
		keyPrompt := &survey.Input{
			Message: "Client key file:",
		}
		if err := survey.AskOne(keyPrompt, &keyFile); err != nil {
			return err
		}
		user.ClientCertificate = certFile
		user.ClientKey = keyFile
	}
	return nil
}
func (m *UserManager) configureBasicAuth(user *models.User) error {
	var username, password string
	userPrompt := &survey.Input{
		Message: "Username:",
	}
	if err := survey.AskOne(userPrompt, &username, survey.WithValidator(survey.Required)); err != nil {
		return err
	}
	passPrompt := &survey.Password{
		Message: "Password:",
	}
	if err := survey.AskOne(passPrompt, &password); err != nil {
		return err
	}
	user.Username = username
	user.Password = password
	return nil
}
func (m *UserManager) editUser() error {
	if len(m.config.Users) == 0 {
		m.colors.Warning("No users to edit")
		fmt.Println("Press Enter to continue...")
		fmt.Scanln()
		return nil
	}
	options := make([]string, len(m.config.Users))
	for i, user := range m.config.Users {
		options[i] = user.Name
	}
	var selected string
	prompt := &survey.Select{
		Message: "Select user to edit:",
		Options: options,
	}
	if err := survey.AskOne(prompt, &selected); err != nil {
		return err
	}
	user := m.config.FindUser(selected)
	if user == nil {
		return fmt.Errorf("user not found")
	}
	m.colors.Warning("Editing user will replace all authentication settings")
	var confirm bool
	confirmPrompt := &survey.Confirm{
		Message: "Continue?",
		Default: true,
	}
	if err := survey.AskOne(confirmPrompt, &confirm); err != nil {
		return err
	}
	if !confirm {
		return nil
	}
	user.User = models.User{}
	authMethods := []string{
		"Exec (command)",
		"Token",
		"Certificate",
		"Basic (username/password)",
		"Cancel",
	}
	var authMethod string
	authPrompt := &survey.Select{
		Message: "New authentication method:",
		Options: authMethods,
	}
	if err := survey.AskOne(authPrompt, &authMethod); err != nil {
		return err
	}
	switch authMethod {
	case "Exec (command)":
		if err := m.configureExecAuth(&user.User); err != nil {
			return err
		}
	case "Token":
		if err := m.configureTokenAuth(&user.User); err != nil {
			return err
		}
	case "Certificate":
		if err := m.configureCertAuth(&user.User); err != nil {
			return err
		}
	case "Basic (username/password)":
		if err := m.configureBasicAuth(&user.User); err != nil {
			return err
		}
	case "Cancel":
		return nil
	}
	m.modified = true
	m.colors.Success("User '%s' updated", selected)
	fmt.Println("Press Enter to continue...")
	fmt.Scanln()
	return nil
}
func (m *UserManager) renameUser() error {
	if len(m.config.Users) == 0 {
		m.colors.Warning("No users to rename")
		fmt.Println("Press Enter to continue...")
		fmt.Scanln()
		return nil
	}

	options := make([]string, len(m.config.Users))
	for i, user := range m.config.Users {
		contexts := m.config.GetContextsUsingUser(user.Name)
		if len(contexts) > 0 {
			options[i] = fmt.Sprintf("%s (used by %d contexts)", user.Name, len(contexts))
		} else {
			options[i] = user.Name
		}
	}

	var selected string
	prompt := &survey.Select{
		Message: "Select user to rename:",
		Options: options,
	}
	if err := survey.AskOne(prompt, &selected); err != nil {
		return err
	}

	oldName := strings.Split(selected, " (")[0]
	user := m.config.FindUser(oldName)
	if user == nil {
		return fmt.Errorf("user not found")
	}

	var newName string
	namePrompt := &survey.Input{
		Message: "New user name:",
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
	if m.config.FindUser(newName) != nil {
		m.colors.Error("A user with name '%s' already exists", newName)
		fmt.Println("Press Enter to continue...")
		fmt.Scanln()
		return nil
	}

	// Show contexts that will be updated
	contexts := m.config.GetContextsUsingUser(oldName)
	if len(contexts) > 0 {
		m.colors.Info("The following contexts will be updated:")
		for _, ctx := range contexts {
			fmt.Printf("  - %s\n", ctx)
		}
	}

	var confirm bool
	confirmPrompt := &survey.Confirm{
		Message: fmt.Sprintf("Rename user from '%s' to '%s'?", oldName, newName),
		Default: true,
	}
	if err := survey.AskOne(confirmPrompt, &confirm); err != nil {
		return err
	}
	if !confirm {
		return nil
	}

	// Update user name
	user.Name = newName

	// Update all context references
	for i := range m.config.Contexts {
		if m.config.Contexts[i].Context.User == oldName {
			m.config.Contexts[i].Context.User = newName
		}
	}

	m.modified = true
	m.colors.Success("User renamed from '%s' to '%s'", oldName, newName)
	fmt.Println("Press Enter to continue...")
	fmt.Scanln()
	return nil
}

func (m *UserManager) deleteUser() error {
	if len(m.config.Users) == 0 {
		m.colors.Warning("No users to delete")
		fmt.Println("Press Enter to continue...")
		fmt.Scanln()
		return nil
	}
	options := make([]string, len(m.config.Users))
	for i, user := range m.config.Users {
		contexts := m.config.GetContextsUsingUser(user.Name)
		if len(contexts) > 0 {
			options[i] = fmt.Sprintf("%s (used by %d contexts)", user.Name, len(contexts))
		} else {
			options[i] = user.Name
		}
	}
	var selected string
	prompt := &survey.Select{
		Message: "Select user to delete:",
		Options: options,
	}
	if err := survey.AskOne(prompt, &selected); err != nil {
		return err
	}
	userName := strings.Split(selected, " (")[0]
	contexts := m.config.GetContextsUsingUser(userName)
	if len(contexts) > 0 {
		m.colors.Warning("This user is used by the following contexts:")
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
	if m.config.RemoveUser(userName) {
		m.modified = true
		m.colors.Success("User '%s' deleted", userName)
	}
	fmt.Println("Press Enter to continue...")
	fmt.Scanln()
	return nil
}
func (m *UserManager) viewUserDetails() {
	if len(m.config.Users) == 0 {
		m.colors.Warning("No users to view")
		fmt.Println("Press Enter to continue...")
		fmt.Scanln()
		return
	}
	options := make([]string, len(m.config.Users))
	for i, user := range m.config.Users {
		options[i] = user.Name
	}
	var selected string
	prompt := &survey.Select{
		Message: "Select user to view:",
		Options: options,
	}
	if err := survey.AskOne(prompt, &selected); err != nil {
		return
	}
	user := m.config.FindUser(selected)
	if user == nil {
		return
	}
	m.clearScreen()
	m.colors.Header("User Details: %s", user.Name)
	fmt.Printf("Auth Method: %s\n", m.colors.Info(user.User.GetAuthMethod()))
	if user.User.Exec != nil {
		fmt.Println("\nExec Configuration:")
		fmt.Printf("  Command: %s\n", m.colors.Info(user.User.Exec.Command))
		if len(user.User.Exec.Args) > 0 {
			fmt.Printf("  Args: %s\n", m.colors.Info(strings.Join(user.User.Exec.Args, " ")))
		}
		if user.User.Exec.APIVersion != "" {
			fmt.Printf("  API Version: %s\n", m.colors.Info(user.User.Exec.APIVersion))
		}
		if user.User.Exec.ProvideClusterInfo {
			fmt.Printf("  Provide Cluster Info: %s\n", m.colors.Info("Yes"))
		}
		if user.User.Exec.InstallHint != "" {
			fmt.Printf("  Install Hint: %s\n", m.colors.Info(user.User.Exec.InstallHint))
		}
	}
	if user.User.Token != "" {
		fmt.Printf("Token: %s\n", m.colors.Info("(set)"))
	}
	if user.User.TokenFile != "" {
		fmt.Printf("Token File: %s\n", m.colors.Info(user.User.TokenFile))
	}
	if user.User.ClientCertificateData != "" {
		fmt.Printf("Client Certificate: %s\n", m.colors.Info("Base64 Data (embedded)"))
	}
	if user.User.ClientCertificate != "" {
		fmt.Printf("Client Certificate: %s\n", m.colors.Info(user.User.ClientCertificate))
	}
	if user.User.ClientKeyData != "" {
		fmt.Printf("Client Key: %s\n", m.colors.Info("Base64 Data (embedded)"))
	}
	if user.User.ClientKey != "" {
		fmt.Printf("Client Key: %s\n", m.colors.Info(user.User.ClientKey))
	}
	if user.User.Username != "" {
		fmt.Printf("Username: %s\n", m.colors.Info(user.User.Username))
	}
	contexts := m.config.GetContextsUsingUser(user.Name)
	if len(contexts) > 0 {
		fmt.Printf("\nUsed by contexts:\n")
		for _, ctx := range contexts {
			fmt.Printf("  - %s\n", m.colors.Context(ctx))
		}
	}
	fmt.Println("\nPress Enter to continue...")
	fmt.Scanln()
}
func (m *UserManager) consolidateUsers() error {
	consolidator := NewUserConsolidator(m.config, m.colors)
	modified, err := consolidator.Consolidate()
	if err != nil {
		return err
	}
	m.modified = m.modified || modified
	return nil
}
func (m *UserManager) IsModified() bool {
	return m.modified
}
func (m *UserManager) clearScreen() {
	fmt.Print("\033[H\033[2J")
}