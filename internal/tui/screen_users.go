package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/positronico/kkonf/v2/internal/models"
)

type usersScreen struct {
	session *Session
	table   entityTable

	formValues  userFormValues
	editingName string
	renameFrom  string
	deleteName  string

	groups        []models.DuplicateGroup
	consolidating int // group index being named
}

func newUsersScreen(session *Session) *usersScreen {
	s := &usersScreen{
		session: session,
		table: newEntityTable([]table.Column{
			{Title: "Name", Width: 34},
			{Title: "Auth", Width: 14},
			{Title: "Contexts", Width: 9},
			{Title: "Dup", Width: 4},
		}),
	}
	s.refresh()
	return s
}

func (s *usersScreen) refresh() {
	duplicated := map[string]bool{}
	for _, group := range s.session.Config.DuplicateUserGroups() {
		for _, u := range group.Users {
			duplicated[u.Name] = true
		}
	}
	rows := make([]table.Row, len(s.session.Config.Users))
	for i, user := range s.session.Config.Users {
		dup := ""
		if duplicated[user.Name] {
			dup = "⚠"
		}
		rows[i] = table.Row{
			user.Name,
			user.User.GetAuthMethod(),
			fmt.Sprintf("%d", len(s.session.Config.GetContextsUsingUser(user.Name))),
			dup,
		}
	}
	s.table.setRows(rows)
}

func (s *usersScreen) capturing() bool { return s.table.capturing() }

func (s *usersScreen) help() string {
	return "a add  e edit  r rename  d delete  v details  c consolidate  / filter"
}

func (s *usersScreen) selected() *models.NamedUser {
	idx := s.table.selectedIndex()
	if idx < 0 || idx >= len(s.session.Config.Users) {
		return nil
	}
	return &s.session.Config.Users[idx]
}

func (s *usersScreen) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if cmd, consumed := s.table.update(msg); consumed {
			return cmd
		}
		switch msg.String() {
		case "a":
			s.formValues = userFormValues{Method: "exec", ExecAPIVersion: "client.authentication.k8s.io/v1beta1"}
			return openModal(newFormModal("user/add", userForm(s.session, &s.formValues, false)))

		case "e", "enter":
			user := s.selected()
			if user == nil {
				return nil
			}
			if user.User.AuthProvider != nil {
				return showToast(toastWarn,
					"%q uses a legacy auth-provider; editing would convert it — rename/delete only", user.Name)
			}
			s.editingName = user.Name
			s.formValues = userFormFromExisting(user.User)
			return openModal(newFormModal("user/edit", userForm(s.session, &s.formValues, true)))

		case "r":
			user := s.selected()
			if user == nil {
				return nil
			}
			s.renameFrom = user.Name
			return openModal(newInput("user/rename", fmt.Sprintf("Rename user %q", user.Name),
				user.Name, "", func(v string) error {
					if v == "" {
						return fmt.Errorf("name is required")
					}
					if v != s.renameFrom && s.session.Config.FindUser(v) != nil {
						return fmt.Errorf("a user named %q already exists", v)
					}
					return nil
				}))

		case "d":
			user := s.selected()
			if user == nil {
				return nil
			}
			s.deleteName = user.Name
			contexts := s.session.Config.GetContextsUsingUser(user.Name)
			if len(contexts) == 0 {
				return openModal(newConfirm("user/delete-simple",
					fmt.Sprintf("Delete user %q?", user.Name), "", false))
			}
			return openModal(newPicker("user/delete-cascade",
				fmt.Sprintf("User %q is used by: %s", user.Name, strings.Join(contexts, ", ")),
				[]string{
					"Delete user and its contexts",
					"Delete user only (contexts will be broken)",
					"Cancel",
				}))

		case "v":
			user := s.selected()
			if user == nil {
				return nil
			}
			return openModal(newTextModal("user/details", "User: "+user.Name,
				userDetails(s.session.Config, user)))

		case "c":
			s.groups = s.session.Config.DuplicateUserGroups()
			if len(s.groups) == 0 {
				return showToast(toastInfo, "No duplicate users found")
			}
			options := make([]string, len(s.groups)+1)
			for i, g := range s.groups {
				names := make([]string, len(g.Users))
				for j, u := range g.Users {
					names[j] = u.Name
				}
				options[i] = fmt.Sprintf("%s: %s", g.AuthMethod, strings.Join(names, ", "))
			}
			options[len(s.groups)] = "Consolidate ALL groups"
			return openModal(newPicker("user/consolidate-group", "Duplicate user groups", options))
		}

	case inputDoneMsg:
		switch msg.id {
		case "user/rename":
			if !msg.ok || msg.value == s.renameFrom {
				return nil
			}
			if err := s.session.Config.RenameUser(s.renameFrom, msg.value); err != nil {
				return showToast(toastError, "%v", err)
			}
			s.refresh()
			return tea.Batch(func() tea.Msg { return refreshMsg{} },
				showToast(toastSuccess, "✓ Renamed user %q to %q (contexts updated)", s.renameFrom, msg.value))
		case "user/consolidate-name":
			if !msg.ok {
				return nil
			}
			return s.consolidateGroup(s.consolidating, msg.value)
		}

	case confirmDoneMsg:
		if msg.id == "user/delete-simple" && msg.ok {
			return s.doDelete(false)
		}

	case pickerDoneMsg:
		switch msg.id {
		case "user/delete-cascade":
			switch msg.index {
			case 0:
				return s.doDelete(true)
			case 1:
				return s.doDelete(false)
			}
		case "user/consolidate-group":
			if msg.index < 0 {
				return nil
			}
			if msg.index == len(s.groups) { // consolidate all
				return s.consolidateAll()
			}
			s.consolidating = msg.index
			group := s.groups[msg.index]
			return openModal(newInput("user/consolidate-name",
				fmt.Sprintf("Merged name for %d %s users", len(group.Users), group.AuthMethod),
				models.SuggestConsolidatedName(group), "", func(v string) error {
					if v == "" {
						return fmt.Errorf("name is required")
					}
					return nil
				}))
		}

	case formDoneMsg:
		switch msg.id {
		case "user/add":
			if !msg.ok {
				return nil
			}
			newUser := models.NamedUser{Name: s.formValues.Name}
			s.formValues.apply(&newUser.User)
			if err := s.session.Config.AddUser(newUser); err != nil {
				return showToast(toastError, "%v", err)
			}
			s.refresh()
			return showToast(toastSuccess, "✓ Added user %q", newUser.Name)
		case "user/edit":
			if !msg.ok {
				return nil
			}
			user := s.session.Config.FindUser(s.editingName)
			if user == nil {
				return showToast(toastError, "user %q vanished", s.editingName)
			}
			s.formValues.apply(&user.User)
			s.refresh()
			return showToast(toastSuccess, "✓ Updated user %q", s.editingName)
		}
	}
	return nil
}

func (s *usersScreen) doDelete(cascade bool) tea.Cmd {
	removed, err := s.session.Config.DeleteUser(s.deleteName, cascade)
	if err != nil {
		return showToast(toastError, "%v", err)
	}
	s.refresh()
	if len(removed) > 0 {
		return tea.Batch(func() tea.Msg { return refreshMsg{} },
			showToast(toastSuccess, "✓ Deleted user %q and contexts: %s", s.deleteName, strings.Join(removed, ", ")))
	}
	return tea.Batch(func() tea.Msg { return refreshMsg{} },
		showToast(toastSuccess, "✓ Deleted user %q", s.deleteName))
}

func (s *usersScreen) consolidateGroup(index int, newName string) tea.Cmd {
	group := s.groups[index]
	names := make([]string, len(group.Users))
	for i, u := range group.Users {
		names[i] = u.Name
	}
	updated, err := s.session.Config.ConsolidateUsers(names, newName)
	if err != nil {
		return showToast(toastError, "%v", err)
	}
	s.refresh()
	return tea.Batch(func() tea.Msg { return refreshMsg{} },
		showToast(toastSuccess, "✓ Consolidated %d users into %q (%d contexts updated)",
			len(names), newName, len(updated)))
}

func (s *usersScreen) consolidateAll() tea.Cmd {
	total, contexts := 0, 0
	taken := map[string]bool{}
	for _, group := range s.groups {
		names := make([]string, len(group.Users))
		inGroup := map[string]bool{}
		for i, u := range group.Users {
			names[i] = u.Name
			inGroup[u.Name] = true
		}
		base := models.SuggestConsolidatedName(group)
		name := base
		for i := 2; (s.session.Config.FindUser(name) != nil && !inGroup[name]) || taken[name]; i++ {
			name = fmt.Sprintf("%s-%d", base, i)
		}
		taken[name] = true
		updated, err := s.session.Config.ConsolidateUsers(names, name)
		if err != nil {
			return showToast(toastError, "%v", err)
		}
		total += len(names)
		contexts += len(updated)
	}
	s.refresh()
	return tea.Batch(func() tea.Msg { return refreshMsg{} },
		showToast(toastSuccess, "✓ Consolidated %d users across %d groups (%d contexts updated)",
			total, len(s.groups), contexts))
}

func userDetails(cfg *models.Config, user *models.NamedUser) string {
	var b strings.Builder
	row := func(key, value string) {
		if value != "" {
			b.WriteString(styleDetailKey.Render(key) + value + "\n")
		}
	}
	row("Auth method", user.User.GetAuthMethod())
	if exec := user.User.Exec; exec != nil {
		row("Command", exec.Command)
		row("Args", strings.Join(exec.Args, " "))
		row("API version", exec.APIVersion)
		var env []string
		for _, e := range exec.Env {
			env = append(env, e.Name+"="+e.Value)
		}
		row("Env", strings.Join(env, ", "))
		row("Interactive mode", exec.InteractiveMode)
		row("Install hint", exec.InstallHint)
	}
	if user.User.Token != "" {
		row("Token", "(set)")
	}
	row("Token file", user.User.TokenFile)
	if user.User.ClientCertificateData != "" {
		row("Client certificate", "base64 data (embedded)")
	}
	row("Client certificate", user.User.ClientCertificate)
	if user.User.ClientKeyData != "" {
		row("Client key", "base64 data (embedded)")
	}
	row("Client key", user.User.ClientKey)
	row("Username", user.User.Username)
	if user.User.AuthProvider != nil {
		row("Auth provider", user.User.AuthProvider.Name)
	}
	row("Impersonate", user.User.Impersonate)
	row("Impersonate groups", strings.Join(user.User.ImpersonateGroups, ", "))
	if contexts := cfg.GetContextsUsingUser(user.Name); len(contexts) > 0 {
		row("Used by contexts", strings.Join(contexts, ", "))
	}
	return b.String()
}

func (s *usersScreen) view(width, height int) string {
	s.table.setSize(width, height-1)
	if len(s.session.Config.Users) == 0 {
		return "\n  No users configured. Press a to add one."
	}
	return s.table.view()
}
