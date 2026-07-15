package tui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/positronico/kkonf/internal/config"
	"github.com/positronico/kkonf/internal/version"
)

type section int

const (
	sectionClusters section = iota
	sectionUsers
	sectionContexts
	sectionTools
	sectionSettings
	sectionCount
)

var sectionNames = [sectionCount]string{"Clusters", "Users", "Contexts", "Tools", "Settings"}

// screen is one main-pane view. Screens own their modals' Done messages.
type screen interface {
	// update handles a message; the returned command may open a modal via
	// openModalMsg or show a toast via toastMsg.
	update(msg tea.Msg) tea.Cmd
	view(width, height int) string
	help() string
	// refresh rebuilds the screen's rows from the session config.
	refresh()
	// capturing reports whether the screen wants ALL keys (filter/form open).
	capturing() bool
}

// Messages screens can emit through commands.
type openModalMsg struct{ m modal }
type toastMsg struct {
	text string
	kind toastKind
}
type refreshMsg struct{}
type requestSaveMsg struct{}
type requestQuitMsg struct{}

type toastKind int

const (
	toastInfo toastKind = iota
	toastSuccess
	toastWarn
	toastError
)

// toastDuration is a variable so tests can shrink it (the tick command
// sleeps synchronously when executed outside the bubbletea runtime).
var toastDuration = 4 * time.Second

type toastExpireMsg struct{ seq int }

func showToast(kind toastKind, format string, a ...interface{}) tea.Cmd {
	return func() tea.Msg { return toastMsg{text: fmt.Sprintf(format, a...), kind: kind} }
}

func openModal(m modal) tea.Cmd {
	return func() tea.Msg { return openModalMsg{m: m} }
}

func doRefresh() tea.Msg { return refreshMsg{} }

// App is the root model.
type App struct {
	session *Session
	section section
	screens [sectionCount]screen
	modal   modal

	toastText string
	toastKind toastKind
	toastSeq  int

	width  int
	height int

	quitting bool
}

func NewApp(session *Session) *App {
	app := &App{
		session: session,
		section: sectionContexts, // landing screen: quick context switching
	}
	app.screens[sectionClusters] = newClustersScreen(session)
	app.screens[sectionUsers] = newUsersScreen(session)
	app.screens[sectionContexts] = newContextsScreen(session)
	app.screens[sectionTools] = newToolsScreen(session)
	app.screens[sectionSettings] = newSettingsScreen(session)
	return app
}

func Run(configPath string) error {
	session, err := NewSession(configPath)
	if err != nil {
		return err
	}
	program := tea.NewProgram(NewApp(session), tea.WithAltScreen())
	_, err = program.Run()
	return err
}

func (a *App) Init() tea.Cmd {
	return nil
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		if a.modal != nil {
			if s, ok := a.modal.(sizeableModal); ok {
				s.setSize(a.width, a.height)
			}
		}
		return a, nil

	case openModalMsg:
		a.modal = msg.m
		if s, ok := a.modal.(sizeableModal); ok {
			s.setSize(a.width, a.height)
		}
		return a, a.modal.init()

	case toastMsg:
		a.toastText, a.toastKind = msg.text, msg.kind
		a.toastSeq++
		seq := a.toastSeq
		return a, tea.Tick(toastDuration, func(time.Time) tea.Msg { return toastExpireMsg{seq: seq} })

	case toastExpireMsg:
		if msg.seq == a.toastSeq {
			a.toastText = ""
		}
		return a, nil

	case refreshMsg:
		for _, s := range a.screens {
			s.refresh()
		}
		return a, nil

	case requestSaveMsg:
		return a, a.saveFlow()

	case requestQuitMsg:
		return a, a.quitFlow()

	case confirmDoneMsg:
		if handled, cmd := a.handleAppConfirm(msg); handled {
			return a, cmd
		}

	case tea.KeyMsg:
		if a.modal != nil {
			var cmd tea.Cmd
			a.modal, cmd = a.modal.update(msg)
			return a, cmd
		}
		if !a.currentScreen().capturing() {
			switch msg.String() {
			case "ctrl+c", "q":
				return a, a.quitFlow()
			case "ctrl+s":
				return a, a.saveFlow()
			case "1", "2", "3", "4", "5":
				a.section = section(msg.String()[0] - '1')
				a.currentScreen().refresh()
				return a, nil
			}
		}
	}

	if a.modal != nil {
		var cmd tea.Cmd
		a.modal, cmd = a.modal.update(msg)
		return a, cmd
	}
	return a, a.currentScreen().update(msg)
}

func (a *App) currentScreen() screen {
	return a.screens[a.section]
}

// ---- save / quit flows ----

const (
	confirmForceSave    = "app/force-save"
	confirmOverwriteExt = "app/overwrite-external"
	confirmQuitSave     = "app/quit-save"
	confirmQuitDiscard  = "app/quit-discard"
)

func (a *App) saveFlow() tea.Cmd {
	if validation := a.session.Validate(); !validation.IsValid() {
		var lines []string
		for _, e := range validation.Errors {
			lines = append(lines, "✗ "+e.Error())
		}
		return openModal(newConfirm(confirmForceSave,
			"Configuration has errors", strings.Join(lines, "\n")+"\n\nSave anyway?", false))
	}
	return a.doSave(false)
}

func (a *App) doSave(force bool) tea.Cmd {
	var err error
	if force {
		err = a.session.ForceSave()
	} else {
		err = a.session.Save()
	}
	if errors.Is(err, config.ErrExternalChange) {
		return openModal(newConfirm(confirmOverwriteExt,
			"File changed on disk",
			"The config file was modified by another tool since it was loaded.\nOverwrite it?", false))
	}
	var reloadErr *ReloadError
	if errors.As(err, &reloadErr) {
		// The data IS on disk — don't block quitting or claim the save failed.
		if a.quitting {
			return tea.Quit
		}
		return showToast(toastWarn, "Saved, but could not re-read the file: %v", reloadErr.Err)
	}
	if err != nil {
		// Keep the session alive (and cancel any pending quit) so the user's
		// in-memory edits survive a failed save.
		a.quitting = false
		return showToast(toastError, "Save failed: %v", err)
	}
	if a.quitting {
		return tea.Quit
	}
	return tea.Batch(showToast(toastSuccess, "✓ Saved %s", a.session.Path), func() tea.Msg { return refreshMsg{} })
}

func (a *App) quitFlow() tea.Cmd {
	if !a.session.Dirty() {
		return tea.Quit
	}
	return openModal(newConfirm(confirmQuitSave,
		"Unsaved changes", "Save before exiting? (No = choose discard next)", true))
}

func (a *App) handleAppConfirm(msg confirmDoneMsg) (bool, tea.Cmd) {
	switch msg.id {
	case confirmForceSave:
		if !msg.ok {
			a.quitting = false
			return true, showToast(toastWarn, "Save cancelled")
		}
		return true, a.doSave(false)
	case confirmOverwriteExt:
		if !msg.ok {
			a.quitting = false
			return true, showToast(toastWarn, "Save cancelled")
		}
		return true, a.doSave(true)
	case confirmQuitSave:
		if msg.ok {
			a.quitting = true
			return true, a.saveFlow()
		}
		return true, openModal(newConfirm(confirmQuitDiscard,
			"Discard changes?", "Exit without saving — all changes will be lost.", false))
	case confirmQuitDiscard:
		if msg.ok {
			return true, tea.Quit
		}
		return true, nil
	}
	return false, nil
}

// ---- view ----

func (a *App) View() string {
	if a.width == 0 {
		return "loading…"
	}

	header := a.renderHeader()
	tabs := a.renderTabs()
	footer := a.renderFooter()
	toast := a.renderToast()

	chromeHeight := lipgloss.Height(header) + lipgloss.Height(tabs) +
		lipgloss.Height(toast) + lipgloss.Height(footer)
	bodyHeight := a.height - chromeHeight
	if bodyHeight < 3 {
		bodyHeight = 3
	}

	var body string
	if a.modal != nil {
		body = a.modal.view(a.width)
	} else {
		body = a.currentScreen().view(a.width, bodyHeight)
	}
	body = lipgloss.NewStyle().Height(bodyHeight).MaxHeight(bodyHeight).Render(body)

	return strings.Join([]string{header, tabs, body, toast, footer}, "\n")
}

func (a *App) renderHeader() string {
	style := styleHeader
	status := "saved"
	if a.session.Dirty() {
		style = styleHeaderDirty
		status = "● modified"
	}
	current := a.session.Config.CurrentContext
	if current == "" {
		current = "(none)"
	}
	left := fmt.Sprintf(" kkonf %s │ %s │ ctx: %s │ %s ",
		version.Get().Version, a.session.Path, current, status)
	return style.Width(a.width).Render(left)
}

func (a *App) renderTabs() string {
	parts := make([]string, sectionCount)
	for i := section(0); i < sectionCount; i++ {
		label := fmt.Sprintf("%d %s", i+1, sectionNames[i])
		if i == a.section {
			parts[i] = styleTabActive.Render("[" + label + "]")
		} else {
			parts[i] = styleTabInactive.Render(" " + label + " ")
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func (a *App) renderToast() string {
	if a.toastText == "" {
		return ""
	}
	style := styleToastInfo
	switch a.toastKind {
	case toastSuccess:
		style = styleToastSuccess
	case toastWarn:
		style = styleToastWarn
	case toastError:
		style = styleToastError
	}
	return style.MaxWidth(a.width).Render(" " + a.toastText)
}

func (a *App) renderFooter() string {
	help := a.currentScreen().help()
	global := "1-5 sections  ctrl+s save  q quit"
	if a.modal != nil {
		return styleFooter.Width(a.width).Render(" (modal open)")
	}
	return styleFooter.Width(a.width).Render(" " + help + "  │  " + global)
}
