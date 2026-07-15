package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/positronico/kkonf/v2/internal/config"
	"github.com/positronico/kkonf/v2/internal/models"
	"github.com/positronico/kkonf/v2/internal/settings"
)

type toolsScreen struct {
	session *Session
	cursor  int

	importCfg *models.Config
	backups   []string
	exportCfg *models.Config
}

var toolEntries = []string{
	"Validate configuration",
	"Import from another kubeconfig",
	"Export contexts to a new file",
	"Restore from backup",
	"Clean old backups",
}

func newToolsScreen(session *Session) *toolsScreen {
	return &toolsScreen{session: session}
}

func (s *toolsScreen) refresh()        {}
func (s *toolsScreen) capturing() bool { return false }
func (s *toolsScreen) help() string {
	return "enter run  ↑/↓ move  (consolidation lives on the Users screen: 2 then c)"
}

func (s *toolsScreen) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if s.cursor > 0 {
				s.cursor--
			}
		case "down", "j":
			if s.cursor < len(toolEntries)-1 {
				s.cursor++
			}
		case "enter":
			return s.run(s.cursor)
		}

	case inputDoneMsg:
		switch msg.id {
		case "tools/import-path":
			if !msg.ok {
				return nil
			}
			return s.startImport(msg.value)
		case "tools/export-path":
			if !msg.ok {
				return nil
			}
			return s.finishExport(msg.value)
		}

	case pickerDoneMsg:
		switch msg.id {
		case "tools/import-all":
			if msg.index != 0 {
				s.importCfg = nil
				return nil
			}
			return s.finishImport(0)
		case "tools/import-strategy":
			return s.finishImport(msg.index)
		case "tools/export-what":
			return s.chooseExport(msg.index)
		case "tools/restore":
			return s.finishRestore(msg.index)
		}

	case confirmDoneMsg:
		if msg.id == "tools/clean-backups" && msg.ok {
			prefs := settings.Load()
			if err := config.CleanOldBackups(s.session.Path, prefs.BackupKeepDays, prefs.BackupKeepCount); err != nil {
				return showToast(toastError, "%v", err)
			}
			return showToast(toastSuccess, "✓ Cleaned backups older than %d days (kept newest %d)",
				prefs.BackupKeepDays, prefs.BackupKeepCount)
		}
	}
	return nil
}

func (s *toolsScreen) run(index int) tea.Cmd {
	switch index {
	case 0: // validate
		return openModal(newTextModal("tools/validate", "Validation", s.validationReport()))
	case 1: // import
		return openModal(newInput("tools/import-path", "Path of kubeconfig to import", "", "/path/to/kubeconfig",
			func(v string) error {
				if v == "" {
					return fmt.Errorf("path is required")
				}
				if _, err := os.Stat(v); err != nil {
					return fmt.Errorf("file not found: %s", v)
				}
				return nil
			}))
	case 2: // export
		return openModal(newPicker("tools/export-what", "What to export", []string{
			"Everything",
			"Current context and its dependencies",
			"Cancel",
		}))
	case 3: // restore
		backups, err := config.ListBackups(s.session.Path)
		if err != nil {
			return showToast(toastError, "%v", err)
		}
		if len(backups) == 0 {
			return showToast(toastInfo, "No backups found for %s", s.session.Path)
		}
		s.backups = backups
		labels := make([]string, len(backups))
		for i, b := range backups {
			labels[i] = filepath.Base(b)
			if info, err := os.Stat(b); err == nil {
				labels[i] = fmt.Sprintf("%s (%s)", filepath.Base(b), info.ModTime().Format("2006-01-02 15:04:05"))
			}
		}
		return openModal(newPicker("tools/restore", "Restore which backup? (unsaved changes will be lost)", labels))
	case 4: // clean backups
		prefs := settings.Load()
		return openModal(newConfirm("tools/clean-backups", "Clean old backups?",
			fmt.Sprintf("Removes backups older than %d days, always keeping the newest %d.",
				prefs.BackupKeepDays, prefs.BackupKeepCount), true))
	}
	return nil
}

func (s *toolsScreen) validationReport() string {
	validation := s.session.Validate()
	if validation.IsValid() && len(validation.Warnings) == 0 {
		return styleToastSuccess.Render("✓ Configuration is valid")
	}
	var b strings.Builder
	for _, e := range validation.Errors {
		b.WriteString(styleToastError.Render("✗ "+e.Error()) + "\n")
	}
	for _, w := range validation.Warnings {
		b.WriteString(styleToastWarn.Render("⚠ "+w.Error()) + "\n")
	}
	return b.String()
}

func (s *toolsScreen) startImport(path string) tea.Cmd {
	imported, err := config.NewLoader(path).Load()
	if err != nil {
		return showToast(toastError, "Failed to load %s: %v", path, err)
	}
	s.importCfg = imported
	conflicts := config.NewValidator().ValidateImport(s.session.Config, imported)
	summary := fmt.Sprintf("%d clusters, %d users, %d contexts",
		len(imported.Clusters), len(imported.Users), len(imported.Contexts))
	if len(conflicts) == 0 {
		// Distinct id: this picker's indexes must not be decoded with the
		// 4-option conflict-strategy mapping.
		return openModal(newPicker("tools/import-all",
			fmt.Sprintf("Import %s — no name conflicts", summary),
			[]string{"Import everything", "Cancel"}))
	}
	var conflictList []string
	for kind, names := range conflicts {
		conflictList = append(conflictList, fmt.Sprintf("%s: %s", kind, strings.Join(names, ", ")))
	}
	return openModal(newPicker("tools/import-strategy",
		fmt.Sprintf("Import %s — conflicts: %s", summary, strings.Join(conflictList, " · ")),
		[]string{
			"Skip conflicting items",
			"Replace existing items",
			"Rename conflicting items (adds -imported suffix)",
			"Cancel",
		}))
}

func (s *toolsScreen) finishImport(choice int) tea.Cmd {
	if s.importCfg == nil || choice < 0 {
		return nil
	}
	imported := s.importCfg
	s.importCfg = nil

	var opts models.MergeOptions
	switch choice {
	case 0: // skip (or "import everything" when no conflicts)
	case 1:
		opts.OnConflict = func(string, string) models.MergeAction { return models.MergeReplace }
	case 2:
		opts.OnConflict = func(string, string) models.MergeAction { return models.MergeRename }
		opts.Rename = func(kind, oldName string) string { return oldName + "-imported" }
	default:
		return nil
	}
	res := s.session.Config.Merge(imported, opts)
	return tea.Batch(func() tea.Msg { return refreshMsg{} },
		showToast(toastSuccess, "✓ Imported %d items (%d added, %d replaced, %d renamed, %d skipped)",
			res.Total(), res.Added, res.Replaced, res.Renamed, res.Skipped))
}

func (s *toolsScreen) chooseExport(choice int) tea.Cmd {
	switch choice {
	case 0:
		subset, err := s.session.Config.ExportSubset(nil)
		if err != nil {
			return showToast(toastError, "%v", err)
		}
		s.exportCfg = subset
	case 1:
		if s.session.Config.CurrentContext == "" {
			return showToast(toastWarn, "No current context set")
		}
		subset, err := s.session.Config.ExportSubset([]string{s.session.Config.CurrentContext})
		if err != nil {
			return showToast(toastError, "%v", err)
		}
		s.exportCfg = subset
	default:
		return nil
	}
	return openModal(newInput("tools/export-path", "Export to file", "kubeconfig-export.yaml", "", func(v string) error {
		if v == "" {
			return fmt.Errorf("path is required")
		}
		return nil
	}))
}

func (s *toolsScreen) finishExport(path string) tea.Cmd {
	if s.exportCfg == nil {
		return nil
	}
	subset := s.exportCfg
	s.exportCfg = nil
	if err := config.NewWriter(path).Save(subset); err != nil {
		return showToast(toastError, "Export failed: %v", err)
	}
	return showToast(toastSuccess, "✓ Exported %d contexts, %d clusters, %d users to %s",
		len(subset.Contexts), len(subset.Clusters), len(subset.Users), path)
}

func (s *toolsScreen) finishRestore(index int) tea.Cmd {
	if index < 0 || index >= len(s.backups) {
		return nil
	}
	backup := s.backups[index]
	mgr := config.NewBackupManager(s.session.Path)
	// Preserve the pre-restore state so the restore is itself undoable —
	// and abort if that safety copy cannot be made.
	if _, err := mgr.Create(); err != nil {
		return showToast(toastError, "Restore aborted, could not back up current state: %v", err)
	}
	if err := mgr.Restore(backup); err != nil {
		return showToast(toastError, "Restore failed: %v", err)
	}
	if err := s.session.Reload(); err != nil {
		return showToast(toastError, "Restored, but reload failed: %v", err)
	}
	return tea.Batch(func() tea.Msg { return refreshMsg{} },
		showToast(toastSuccess, "✓ Restored from %s", filepath.Base(backup)))
}

func (s *toolsScreen) view(width, height int) string {
	var b strings.Builder
	b.WriteString("\n")
	for i, entry := range toolEntries {
		cursor := "   "
		line := entry
		if i == s.cursor {
			cursor = " " + styleCurrentMark.Render("❯") + " "
			line = styleTabActive.Render(line)
		}
		b.WriteString(cursor + line + "\n")
	}
	return b.String()
}
