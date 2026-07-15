package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Modals are layered above the active screen. When a modal finishes it
// returns nil from update() and emits a *DoneMsg that the owning screen
// consumes; the id says which flow it belongs to.

type modal interface {
	init() tea.Cmd
	update(msg tea.Msg) (modal, tea.Cmd)
	view(width int) string
}

// sizeableModal is implemented by modals that adapt to the terminal size
// (currently huh forms, whose group viewport needs a height to scroll).
type sizeableModal interface {
	setSize(width, height int)
}

type confirmDoneMsg struct {
	id string
	ok bool
}

type pickerDoneMsg struct {
	id    string
	index int // -1 on cancel
}

type inputDoneMsg struct {
	id    string
	value string
	ok    bool
}

// ---- confirm ----

type confirmModal struct {
	id       string
	title    string
	body     string
	yesLabel string
	noLabel  string
	yes      bool
}

func newConfirm(id, title, body string, defaultYes bool) *confirmModal {
	return &confirmModal{id: id, title: title, body: body, yesLabel: "Yes", noLabel: "No", yes: defaultYes}
}

func (m *confirmModal) init() tea.Cmd { return nil }

func (m *confirmModal) update(msg tea.Msg) (modal, tea.Cmd) {
	keyMsg, isKey := msg.(tea.KeyMsg)
	if !isKey {
		return m, nil
	}
	switch keyMsg.String() {
	case "left", "right", "tab", "h", "l":
		m.yes = !m.yes
		return m, nil
	case "y", "Y":
		return nil, func() tea.Msg { return confirmDoneMsg{id: m.id, ok: true} }
	case "n", "N":
		return nil, func() tea.Msg { return confirmDoneMsg{id: m.id, ok: false} }
	case "enter":
		ok := m.yes
		return nil, func() tea.Msg { return confirmDoneMsg{id: m.id, ok: ok} }
	case "esc", "0":
		return nil, func() tea.Msg { return confirmDoneMsg{id: m.id, ok: false} }
	}
	return m, nil
}

func (m *confirmModal) view(width int) string {
	var b strings.Builder
	b.WriteString(styleModalTitle.Render(m.title) + "\n")
	if m.body != "" {
		b.WriteString(m.body + "\n")
	}
	b.WriteString("\n")
	yes, no := "  "+m.yesLabel+"  ", "  "+m.noLabel+"  "
	if m.yes {
		yes = styleHeader.Render(m.yesLabel)
		no = styleTabInactive.Render(m.noLabel)
	} else {
		yes = styleTabInactive.Render(m.yesLabel)
		no = styleHeader.Render(m.noLabel)
	}
	b.WriteString(yes + "   " + no + "\n\n")
	b.WriteString(styleFooter.Render("y/n  ←/→ switch  enter confirm  esc cancel"))
	return renderModalBox(b.String(), width)
}

// ---- picker (numbered list, digits 1-9 jump-select, 0/esc cancel) ----

type pickerModal struct {
	id      string
	title   string
	options []string
	cursor  int
}

func newPicker(id, title string, options []string) *pickerModal {
	return &pickerModal{id: id, title: title, options: options}
}

func (m *pickerModal) init() tea.Cmd { return nil }

func (m *pickerModal) update(msg tea.Msg) (modal, tea.Cmd) {
	keyMsg, isKey := msg.(tea.KeyMsg)
	if !isKey {
		return m, nil
	}
	done := func(idx int) (modal, tea.Cmd) {
		return nil, func() tea.Msg { return pickerDoneMsg{id: m.id, index: idx} }
	}
	s := keyMsg.String()
	switch s {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case "down", "j":
		if m.cursor < len(m.options)-1 {
			m.cursor++
		}
		return m, nil
	case "enter":
		return done(m.cursor)
	case "esc", "0", "q":
		return done(-1)
	}
	// Menu numbers as quick shortcuts: 1-9 select directly.
	if len(s) == 1 && s[0] >= '1' && s[0] <= '9' {
		idx := int(s[0] - '1')
		if idx < len(m.options) {
			return done(idx)
		}
	}
	return m, nil
}

func (m *pickerModal) view(width int) string {
	var b strings.Builder
	b.WriteString(styleModalTitle.Render(m.title) + "\n\n")
	for i, opt := range m.options {
		cursor := "  "
		line := fmt.Sprintf("%d. %s", i+1, opt)
		if i > 8 {
			line = fmt.Sprintf("   %s", opt)
		}
		if i == m.cursor {
			cursor = styleCurrentMark.Render("❯ ")
			line = lipgloss.NewStyle().Bold(true).Render(line)
		}
		b.WriteString(cursor + line + "\n")
	}
	b.WriteString("\n" + styleFooter.Render("1-9 quick select  ↑/↓ move  enter select  0/esc cancel"))
	return renderModalBox(b.String(), width)
}

// ---- input ----

type inputModal struct {
	id       string
	title    string
	input    textinput.Model
	validate func(string) error
	errText  string
}

func newInput(id, title, initial, placeholder string, validate func(string) error) *inputModal {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.SetValue(initial)
	ti.CursorEnd()
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 40
	return &inputModal{id: id, title: title, input: ti, validate: validate}
}

func (m *inputModal) init() tea.Cmd { return textinput.Blink }

func (m *inputModal) update(msg tea.Msg) (modal, tea.Cmd) {
	if keyMsg, isKey := msg.(tea.KeyMsg); isKey {
		switch keyMsg.String() {
		case "enter":
			value := m.input.Value()
			if m.validate != nil {
				if err := m.validate(value); err != nil {
					m.errText = err.Error()
					return m, nil
				}
			}
			return nil, func() tea.Msg { return inputDoneMsg{id: m.id, value: value, ok: true} }
		case "esc":
			return nil, func() tea.Msg { return inputDoneMsg{id: m.id, ok: false} }
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	// Only keystrokes clear the error — cursor-blink ticks arrive every
	// ~500ms and would wipe it before the user can read it.
	if _, isKey := msg.(tea.KeyMsg); isKey {
		m.errText = ""
	}
	return m, cmd
}

func (m *inputModal) view(width int) string {
	var b strings.Builder
	b.WriteString(styleModalTitle.Render(m.title) + "\n\n")
	b.WriteString(m.input.View() + "\n")
	if m.errText != "" {
		b.WriteString(styleToastError.Render(m.errText) + "\n")
	}
	b.WriteString("\n" + styleFooter.Render("enter confirm  esc cancel"))
	return renderModalBox(b.String(), width)
}

// ---- static text viewer (validation output, details) ----

type textModal struct {
	id    string
	title string
	body  string
}

func newTextModal(id, title, body string) *textModal {
	return &textModal{id: id, title: title, body: body}
}

func (m *textModal) init() tea.Cmd { return nil }

func (m *textModal) update(msg tea.Msg) (modal, tea.Cmd) {
	if _, isKey := msg.(tea.KeyMsg); isKey {
		return nil, nil
	}
	return m, nil
}

func (m *textModal) view(width int) string {
	body := styleModalTitle.Render(m.title) + "\n\n" + m.body +
		"\n\n" + styleFooter.Render("any key to close")
	return renderModalBox(body, width)
}

func renderModalBox(content string, width int) string {
	box := styleModalBox.MaxWidth(width - 2).Render(content)
	return lipgloss.Place(width, lipgloss.Height(box)+2, lipgloss.Center, lipgloss.Center, box)
}
