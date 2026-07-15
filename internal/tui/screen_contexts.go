package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/positronico/kkonf/internal/models"
)

type contextsScreen struct {
	session *Session
	table   entityTable

	formValues  contextFormValues
	editingName string
	renameFrom  string
	nsTarget    string
	deleteName  string
}

func newContextsScreen(session *Session) *contextsScreen {
	s := &contextsScreen{
		session: session,
		table: newEntityTable([]table.Column{
			{Title: "", Width: 2},
			{Title: "Name", Width: 32},
			{Title: "Cluster", Width: 24},
			{Title: "User", Width: 24},
			{Title: "Namespace", Width: 16},
		}),
	}
	s.refresh()
	return s
}

func (s *contextsScreen) refresh() {
	rows := make([]table.Row, len(s.session.Config.Contexts))
	for i, ctx := range s.session.Config.Contexts {
		marker := " "
		if ctx.Name == s.session.Config.CurrentContext {
			marker = styleCurrentMark.Render("●")
		}
		rows[i] = table.Row{
			marker,
			ctx.Name,
			ctx.Context.Cluster,
			ctx.Context.User,
			models.DisplayNamespace(ctx.Context.Namespace),
		}
	}
	s.table.setRows(rows)
}

func (s *contextsScreen) capturing() bool { return s.table.capturing() }

func (s *contextsScreen) help() string {
	return "enter/s switch  n namespace  a add  e edit  r rename  d delete  / filter"
}

func (s *contextsScreen) selected() *models.NamedContext {
	idx := s.table.selectedIndex()
	if idx < 0 || idx >= len(s.session.Config.Contexts) {
		return nil
	}
	return &s.session.Config.Contexts[idx]
}

func (s *contextsScreen) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if cmd, consumed := s.table.update(msg); consumed {
			return cmd
		}
		switch msg.String() {
		case "enter", "s":
			context := s.selected()
			if context == nil {
				return nil
			}
			if context.Name == s.session.Config.CurrentContext {
				return showToast(toastInfo, "Already using context %q", context.Name)
			}
			if err := s.session.Config.SetCurrentContext(context.Name); err != nil {
				return showToast(toastError, "%v", err)
			}
			s.refresh()
			return showToast(toastSuccess, "✓ Current context: %s", context.Name)

		case "n":
			context := s.selected()
			if context == nil {
				return nil
			}
			s.nsTarget = context.Name
			return openModal(newInput("ctx/ns",
				fmt.Sprintf("Namespace for %q", context.Name),
				models.DisplayNamespace(context.Context.Namespace), "default", nil))

		case "a":
			if len(s.session.Config.Clusters) == 0 || len(s.session.Config.Users) == 0 {
				return showToast(toastWarn, "Add a cluster and a user first (sections 1 and 2)")
			}
			s.formValues = contextFormValues{}
			return openModal(newFormModal("ctx/add", contextForm(s.session, &s.formValues, false)))

		case "e":
			context := s.selected()
			if context == nil {
				return nil
			}
			s.editingName = context.Name
			s.formValues = contextFormValues{
				Cluster:   context.Context.Cluster,
				User:      context.Context.User,
				Namespace: models.DisplayNamespace(context.Context.Namespace),
			}
			return openModal(newFormModal("ctx/edit", contextForm(s.session, &s.formValues, true)))

		case "r":
			context := s.selected()
			if context == nil {
				return nil
			}
			s.renameFrom = context.Name
			return openModal(newInput("ctx/rename", fmt.Sprintf("Rename context %q", context.Name),
				context.Name, "", func(v string) error {
					if v == "" {
						return fmt.Errorf("name is required")
					}
					if v != s.renameFrom && s.session.Config.FindContext(v) != nil {
						return fmt.Errorf("a context named %q already exists", v)
					}
					return nil
				}))

		case "d":
			context := s.selected()
			if context == nil {
				return nil
			}
			s.deleteName = context.Name
			body := ""
			if context.Name == s.session.Config.CurrentContext {
				body = "This is the CURRENT context."
			}
			return openModal(newConfirm("ctx/delete",
				fmt.Sprintf("Delete context %q?", context.Name), body, false))
		}

	case inputDoneMsg:
		switch msg.id {
		case "ctx/ns":
			if !msg.ok {
				return nil
			}
			if err := s.session.Config.SetNamespace(s.nsTarget, msg.value); err != nil {
				return showToast(toastError, "%v", err)
			}
			s.refresh()
			return showToast(toastSuccess, "✓ Namespace of %q set to %s", s.nsTarget,
				models.DisplayNamespace(models.NormalizeNamespace(msg.value)))
		case "ctx/rename":
			if !msg.ok || msg.value == s.renameFrom {
				return nil
			}
			if err := s.session.Config.RenameContext(s.renameFrom, msg.value); err != nil {
				return showToast(toastError, "%v", err)
			}
			s.refresh()
			return showToast(toastSuccess, "✓ Renamed context %q to %q", s.renameFrom, msg.value)
		}

	case confirmDoneMsg:
		if msg.id == "ctx/delete" && msg.ok {
			if err := s.session.Config.DeleteContext(s.deleteName); err != nil {
				return showToast(toastError, "%v", err)
			}
			s.refresh()
			return showToast(toastSuccess, "✓ Deleted context %q", s.deleteName)
		}

	case formDoneMsg:
		switch msg.id {
		case "ctx/add":
			if !msg.ok {
				return nil
			}
			newContext := models.NamedContext{
				Name: s.formValues.Name,
				Context: models.Context{
					Cluster:   s.formValues.Cluster,
					User:      s.formValues.User,
					Namespace: models.NormalizeNamespace(s.formValues.Namespace),
				},
			}
			if err := s.session.Config.AddContext(newContext); err != nil {
				return showToast(toastError, "%v", err)
			}
			if s.formValues.SetCurrent {
				_ = s.session.Config.SetCurrentContext(newContext.Name)
			}
			s.refresh()
			return showToast(toastSuccess, "✓ Added context %q", newContext.Name)
		case "ctx/edit":
			if !msg.ok {
				return nil
			}
			context := s.session.Config.FindContext(s.editingName)
			if context == nil {
				return showToast(toastError, "context %q vanished", s.editingName)
			}
			context.Context.Cluster = s.formValues.Cluster
			context.Context.User = s.formValues.User
			context.Context.Namespace = models.NormalizeNamespace(s.formValues.Namespace)
			s.refresh()
			return showToast(toastSuccess, "✓ Updated context %q", s.editingName)
		}
	}
	return nil
}

func (s *contextsScreen) view(width, height int) string {
	s.table.setSize(width, height-1)
	if len(s.session.Config.Contexts) == 0 {
		return "\n  No contexts configured. Press a to add one (needs a cluster and a user first)."
	}
	return s.table.view()
}
