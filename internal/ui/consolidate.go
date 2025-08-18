package ui
import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/positronico/kkonf/internal/models"
)
type UserConsolidator struct {
	config *models.Config
	colors *ColorScheme
}
type DuplicateGroup struct {
	Signature string
	Users     []models.NamedUser
	AuthMethod string
}
func NewUserConsolidator(config *models.Config, colors *ColorScheme) *UserConsolidator {
	return &UserConsolidator{
		config: config,
		colors: colors,
	}
}
func (c *UserConsolidator) Consolidate() (bool, error) {
	groups := c.findDuplicateGroups()
	if len(groups) == 0 {
		c.colors.Success("No duplicate users found")
		fmt.Println("Press Enter to continue...")
		fmt.Scanln()
		return false, nil
	}
	c.colors.Header("Found %d group(s) of duplicate users", len(groups))
	selectedGroups := c.selectGroupsToConsolidate(groups)
	if len(selectedGroups) == 0 {
		return false, nil
	}
	modified := false
	for _, group := range selectedGroups {
		if c.consolidateGroup(group) {
			modified = true
		}
	}
	if modified {
		c.colors.Success("User consolidation completed")
	}
	fmt.Println("Press Enter to continue...")
	fmt.Scanln()
	return modified, nil
}
func (c *UserConsolidator) findDuplicateGroups() []DuplicateGroup {
	signatures := make(map[string][]models.NamedUser)
	for _, user := range c.config.Users {
		sig := user.User.GetSignature()
		signatures[sig] = append(signatures[sig], user)
	}
	var groups []DuplicateGroup
	for sig, users := range signatures {
		if len(users) > 1 {
			groups = append(groups, DuplicateGroup{
				Signature:  sig,
				Users:      users,
				AuthMethod: users[0].User.GetAuthMethod(),
			})
		}
	}
	return groups
}
func (c *UserConsolidator) selectGroupsToConsolidate(groups []DuplicateGroup) []DuplicateGroup {
	fmt.Println()
	for i, group := range groups {
		fmt.Printf("Group %d: %s authentication (%d duplicates)\n", i+1, group.AuthMethod, len(group.Users))
		fmt.Println("  Users:")
		for _, user := range group.Users {
			contexts := c.config.GetContextsUsingUser(user.Name)
			if len(contexts) > 0 {
				fmt.Printf("    - %s (used by %d contexts)\n", user.Name, len(contexts))
			} else {
				fmt.Printf("    - %s\n", user.Name)
			}
		}
		if group.AuthMethod == "exec" && group.Users[0].User.Exec != nil {
			fmt.Printf("  Command: %s\n", group.Users[0].User.Exec.Command)
			if len(group.Users[0].User.Exec.Args) > 0 {
				fmt.Printf("  Args: %s\n", strings.Join(group.Users[0].User.Exec.Args, " "))
			}
		}
		fmt.Println()
	}
	if len(groups) == 1 {
		var consolidate bool
		prompt := &survey.Confirm{
			Message: "Consolidate this group?",
			Default: true,
		}
		if err := survey.AskOne(prompt, &consolidate); err != nil {
			return nil
		}
		if consolidate {
			return groups
		}
		return nil
	}
	var consolidateAll bool
	allPrompt := &survey.Confirm{
		Message: "Consolidate all groups?",
		Default: false,
	}
	if err := survey.AskOne(allPrompt, &consolidateAll); err != nil {
		return nil
	}
	if consolidateAll {
		return groups
	}
	options := make([]string, len(groups)+1)
	for i, group := range groups {
		options[i] = fmt.Sprintf("Group %d: %s (%d users)", i+1, group.AuthMethod, len(group.Users))
	}
	options[len(groups)] = "Done selecting"
	var selected []DuplicateGroup
	for {
		var choice string
		prompt := &survey.Select{
			Message: "Select group to consolidate:",
			Options: options,
		}
		if err := survey.AskOne(prompt, &choice); err != nil {
			return nil
		}
		if choice == "Done selecting" {
			break
		}
		for i, group := range groups {
			if choice == options[i] {
				selected = append(selected, group)
				options[i] = fmt.Sprintf("%s (selected)", options[i])
				break
			}
		}
	}
	return selected
}
func (c *UserConsolidator) consolidateGroup(group DuplicateGroup) bool {
	fmt.Printf("\nConsolidating group with %d users:\n", len(group.Users))
	for _, user := range group.Users {
		fmt.Printf("  - %s\n", user.Name)
	}
	defaultName := c.suggestConsolidatedName(group)
	var newName string
	namePrompt := &survey.Input{
		Message: "New consolidated user name:",
		Default: defaultName,
	}
	if err := survey.AskOne(namePrompt, &newName, survey.WithValidator(survey.Required)); err != nil {
		return false
	}
	existingUser := c.config.FindUser(newName)
	if existingUser != nil {
		isOneOfDuplicates := false
		for _, user := range group.Users {
			if user.Name == newName {
				isOneOfDuplicates = true
				break
			}
		}
		if !isOneOfDuplicates {
			c.colors.Error("User '%s' already exists and is not part of this duplicate group", newName)
			return false
		}
	}
	contextUpdates := make(map[string]string)
	for _, user := range group.Users {
		if user.Name != newName {
			contexts := c.config.GetContextsUsingUser(user.Name)
			for _, ctx := range contexts {
				contextUpdates[ctx] = user.Name
			}
		}
	}
	if len(contextUpdates) > 0 {
		c.colors.Info("The following contexts will be updated:")
		for ctxName, oldUser := range contextUpdates {
			fmt.Printf("  - %s: %s → %s\n", ctxName, oldUser, newName)
		}
		var confirm bool
		confirmPrompt := &survey.Confirm{
			Message: "Proceed with consolidation?",
			Default: true,
		}
		if err := survey.AskOne(confirmPrompt, &confirm); err != nil {
			return false
		}
		if !confirm {
			return false
		}
	}
	consolidatedUserExists := false
	var consolidatedUser models.NamedUser
	for _, user := range group.Users {
		if user.Name == newName {
			consolidatedUser = user
			consolidatedUserExists = true
			break
		}
	}
	if !consolidatedUserExists {
		consolidatedUser = group.Users[0]
		consolidatedUser.Name = newName
	}
	for ctxName, oldUserName := range contextUpdates {
		ctx := c.config.FindContext(ctxName)
		if ctx != nil && ctx.Context.User == oldUserName {
			ctx.Context.User = newName
		}
	}
	newUsers := []models.NamedUser{}
	
	// Always add the consolidated user first
	newUsers = append(newUsers, consolidatedUser)
	
	// Add all users that are not part of the duplicate group
	for _, user := range c.config.Users {
		isDuplicate := false
		for _, dupUser := range group.Users {
			if user.Name == dupUser.Name {
				isDuplicate = true
				break
			}
		}
		if !isDuplicate {
			newUsers = append(newUsers, user)
		}
	}
	c.config.Users = newUsers
	c.colors.Success("Consolidated %d users into '%s'", len(group.Users), newName)
	return true
}
func (c *UserConsolidator) suggestConsolidatedName(group DuplicateGroup) string {
	if group.AuthMethod == "exec" && group.Users[0].User.Exec != nil {
		command := group.Users[0].User.Exec.Command
		if strings.Contains(command, "gke-gcloud-auth-plugin") {
			return "gke-user"
		}
		if strings.Contains(command, "aws") {
			return "eks-user"
		}
		if strings.Contains(command, "az") {
			return "aks-user"
		}
		return fmt.Sprintf("%s-user", group.AuthMethod)
	}
	commonPrefix := c.findCommonPrefix(group.Users)
	if commonPrefix != "" {
		return commonPrefix + "-user"
	}
	return fmt.Sprintf("%s-user", group.AuthMethod)
}
func (c *UserConsolidator) findCommonPrefix(users []models.NamedUser) string {
	if len(users) == 0 {
		return ""
	}
	prefix := users[0].Name
	for _, user := range users[1:] {
		for !strings.HasPrefix(user.Name, prefix) && len(prefix) > 0 {
			prefix = prefix[:len(prefix)-1]
		}
		if prefix == "" {
			break
		}
	}
	prefix = strings.TrimRight(prefix, "-_")
	return prefix
}