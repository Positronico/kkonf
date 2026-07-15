package tui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/positronico/kkonf/internal/settings"
)

type settingsScreen struct {
	session *Session
	prefs   settings.Settings
	cursor  int
}

func newSettingsScreen(session *Session) *settingsScreen {
	return &settingsScreen{session: session, prefs: settings.Load()}
}

func (s *settingsScreen) refresh()        { s.prefs = settings.Load() }
func (s *settingsScreen) capturing() bool { return false }
func (s *settingsScreen) help() string    { return "enter edit  ↑/↓ move" }

func (s *settingsScreen) entries() []struct{ name, value string } {
	return []struct{ name, value string }{
		{"Backup retention (days)", strconv.Itoa(s.prefs.BackupKeepDays)},
		{"Backups always kept (count)", strconv.Itoa(s.prefs.BackupKeepCount)},
	}
}

func (s *settingsScreen) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if s.cursor > 0 {
				s.cursor--
			}
		case "down", "j":
			if s.cursor < len(s.entries())-1 {
				s.cursor++
			}
		case "enter", "e":
			entry := s.entries()[s.cursor]
			return openModal(newInput(fmt.Sprintf("settings/%d", s.cursor),
				entry.name, entry.value, "", func(v string) error {
					n, err := strconv.Atoi(v)
					if err != nil || n < 0 {
						return fmt.Errorf("enter a non-negative number")
					}
					return nil
				}))
		}

	case inputDoneMsg:
		if !msg.ok || !strings.HasPrefix(msg.id, "settings/") {
			return nil
		}
		value, err := strconv.Atoi(msg.value)
		if err != nil {
			return showToast(toastError, "invalid number: %s", msg.value)
		}
		switch msg.id {
		case "settings/0":
			s.prefs.BackupKeepDays = value
		case "settings/1":
			s.prefs.BackupKeepCount = value
		}
		if err := s.prefs.Save(); err != nil {
			return showToast(toastError, "Failed to save settings: %v", err)
		}
		return showToast(toastSuccess, "✓ Settings saved")
	}
	return nil
}

func (s *settingsScreen) view(width, height int) string {
	var b strings.Builder
	b.WriteString("\n")
	for i, entry := range s.entries() {
		cursor := "   "
		line := styleDetailKey.Render(entry.name) + entry.value
		if i == s.cursor {
			cursor = " " + styleCurrentMark.Render("❯") + " "
		}
		b.WriteString(cursor + line + "\n")
	}
	path, err := settings.Path()
	if err == nil {
		b.WriteString("\n " + styleFooter.Render("stored at "+path) + "\n")
	}
	return b.String()
}
