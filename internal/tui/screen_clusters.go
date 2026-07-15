package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/positronico/kkonf/internal/models"
)

type clustersScreen struct {
	session *Session
	table   entityTable

	formValues  clusterFormValues
	editingName string
	renameFrom  string
	deleteName  string
}

func newClustersScreen(session *Session) *clustersScreen {
	s := &clustersScreen{
		session: session,
		table: newEntityTable([]table.Column{
			{Title: "Name", Width: 30},
			{Title: "Server", Width: 40},
			{Title: "TLS", Width: 5},
			{Title: "Contexts", Width: 9},
		}),
	}
	s.refresh()
	return s
}

func (s *clustersScreen) refresh() {
	rows := make([]table.Row, len(s.session.Config.Clusters))
	for i, cluster := range s.session.Config.Clusters {
		tls := "✓"
		if cluster.Cluster.InsecureSkipTLSVerify {
			tls = "✗"
		}
		rows[i] = table.Row{
			cluster.Name,
			cluster.Cluster.Server,
			tls,
			fmt.Sprintf("%d", len(s.session.Config.GetContextsUsingCluster(cluster.Name))),
		}
	}
	s.table.setRows(rows)
}

func (s *clustersScreen) capturing() bool { return s.table.capturing() }

func (s *clustersScreen) help() string {
	return "a add  e edit  r rename  d delete  v details  / filter"
}

func (s *clustersScreen) selected() *models.NamedCluster {
	idx := s.table.selectedIndex()
	if idx < 0 || idx >= len(s.session.Config.Clusters) {
		return nil
	}
	return &s.session.Config.Clusters[idx]
}

func (s *clustersScreen) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if cmd, consumed := s.table.update(msg); consumed {
			return cmd
		}
		switch msg.String() {
		case "a":
			s.formValues = clusterFormValues{CAType: "none"}
			return openModal(newFormModal("cluster/add", clusterForm(s.session, &s.formValues, false)))

		case "e", "enter":
			cluster := s.selected()
			if cluster == nil {
				return nil
			}
			s.editingName = cluster.Name
			s.formValues = clusterFormFromExisting(cluster.Cluster)
			return openModal(newFormModal("cluster/edit", clusterForm(s.session, &s.formValues, true)))

		case "r":
			cluster := s.selected()
			if cluster == nil {
				return nil
			}
			s.renameFrom = cluster.Name
			return openModal(newInput("cluster/rename", fmt.Sprintf("Rename cluster %q", cluster.Name),
				cluster.Name, "", func(v string) error {
					if v == "" {
						return fmt.Errorf("name is required")
					}
					if v != s.renameFrom && s.session.Config.FindCluster(v) != nil {
						return fmt.Errorf("a cluster named %q already exists", v)
					}
					return nil
				}))

		case "d":
			cluster := s.selected()
			if cluster == nil {
				return nil
			}
			s.deleteName = cluster.Name
			contexts := s.session.Config.GetContextsUsingCluster(cluster.Name)
			if len(contexts) == 0 {
				return openModal(newConfirm("cluster/delete-simple",
					fmt.Sprintf("Delete cluster %q?", cluster.Name), "", false))
			}
			return openModal(newPicker("cluster/delete-cascade",
				fmt.Sprintf("Cluster %q is used by: %s", cluster.Name, strings.Join(contexts, ", ")),
				[]string{
					"Delete cluster and its contexts",
					"Delete cluster only (contexts will be broken)",
					"Cancel",
				}))

		case "v":
			cluster := s.selected()
			if cluster == nil {
				return nil
			}
			return openModal(newTextModal("cluster/details",
				"Cluster: "+cluster.Name, clusterDetails(s.session.Config, cluster)))
		}

	case inputDoneMsg:
		if msg.id == "cluster/rename" {
			if !msg.ok || msg.value == s.renameFrom {
				return nil
			}
			if err := s.session.Config.RenameCluster(s.renameFrom, msg.value); err != nil {
				return showToast(toastError, "%v", err)
			}
			s.refresh()
			return tea.Batch(func() tea.Msg { return refreshMsg{} },
				showToast(toastSuccess, "✓ Renamed cluster %q to %q (contexts updated)", s.renameFrom, msg.value))
		}

	case confirmDoneMsg:
		if msg.id == "cluster/delete-simple" && msg.ok {
			return s.doDelete(false)
		}

	case pickerDoneMsg:
		if msg.id == "cluster/delete-cascade" {
			switch msg.index {
			case 0:
				return s.doDelete(true)
			case 1:
				return s.doDelete(false)
			}
		}

	case formDoneMsg:
		switch msg.id {
		case "cluster/add":
			if !msg.ok {
				return nil
			}
			newCluster := models.NamedCluster{Name: s.formValues.Name}
			s.formValues.apply(&newCluster.Cluster)
			if err := s.session.Config.AddCluster(newCluster); err != nil {
				return showToast(toastError, "%v", err)
			}
			s.refresh()
			return showToast(toastSuccess, "✓ Added cluster %q", newCluster.Name)
		case "cluster/edit":
			if !msg.ok {
				return nil
			}
			cluster := s.session.Config.FindCluster(s.editingName)
			if cluster == nil {
				return showToast(toastError, "cluster %q vanished", s.editingName)
			}
			s.formValues.apply(&cluster.Cluster)
			s.refresh()
			return showToast(toastSuccess, "✓ Updated cluster %q", s.editingName)
		}
	}
	return nil
}

func (s *clustersScreen) doDelete(cascade bool) tea.Cmd {
	removed, err := s.session.Config.DeleteCluster(s.deleteName, cascade)
	if err != nil {
		return showToast(toastError, "%v", err)
	}
	s.refresh()
	if len(removed) > 0 {
		return tea.Batch(func() tea.Msg { return refreshMsg{} },
			showToast(toastSuccess, "✓ Deleted cluster %q and contexts: %s", s.deleteName, strings.Join(removed, ", ")))
	}
	return tea.Batch(func() tea.Msg { return refreshMsg{} },
		showToast(toastSuccess, "✓ Deleted cluster %q", s.deleteName))
}

func clusterDetails(cfg *models.Config, cluster *models.NamedCluster) string {
	var b strings.Builder
	row := func(key, value string) {
		if value != "" {
			b.WriteString(styleDetailKey.Render(key) + value + "\n")
		}
	}
	row("Server", cluster.Cluster.Server)
	if cluster.Cluster.InsecureSkipTLSVerify {
		row("TLS verification", "disabled (insecure)")
	} else {
		row("TLS verification", "enabled")
	}
	switch {
	case cluster.Cluster.CertificateAuthorityData != "":
		row("Certificate authority", "base64 data (embedded)")
	case cluster.Cluster.CertificateAuthority != "":
		row("Certificate authority", cluster.Cluster.CertificateAuthority)
	default:
		row("Certificate authority", "none")
	}
	row("TLS server name", cluster.Cluster.TLSServerName)
	row("Proxy URL", cluster.Cluster.ProxyURL)
	if cluster.Cluster.DisableCompression {
		row("Compression", "disabled")
	}
	if contexts := cfg.GetContextsUsingCluster(cluster.Name); len(contexts) > 0 {
		row("Used by contexts", strings.Join(contexts, ", "))
	}
	return b.String()
}

func (s *clustersScreen) view(width, height int) string {
	s.table.setSize(width, height-1)
	if len(s.session.Config.Clusters) == 0 {
		return "\n  No clusters configured. Press a to add one."
	}
	return s.table.view()
}
