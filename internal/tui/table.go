package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// entityTable wraps bubbles/table with a '/' substring filter. Filtered rows
// keep a mapping back to the original slice index so selections stay correct.
type entityTable struct {
	tbl      table.Model
	filter   textinput.Model
	filterOn bool
	allRows  []table.Row
	indexMap []int
}

func newEntityTable(columns []table.Column) entityTable {
	styles := table.DefaultStyles()
	styles.Header = styles.Header.
		Bold(true).
		Foreground(colorAccent).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colorSubtle).
		BorderBottom(true)
	styles.Selected = styles.Selected.
		Foreground(lipgloss.AdaptiveColor{Light: "231", Dark: "231"}).
		Background(colorAccent).
		Bold(false)

	tbl := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(12),
	)
	tbl.SetStyles(styles)

	filter := textinput.New()
	filter.Placeholder = "type to filter…"
	filter.Prompt = "/ "
	filter.CharLimit = 64

	return entityTable{tbl: tbl, filter: filter}
}

// setRows replaces the row set and reapplies the current filter.
func (t *entityTable) setRows(rows []table.Row) {
	t.allRows = rows
	t.applyFilter()
}

func (t *entityTable) applyFilter() {
	needle := strings.ToLower(t.filter.Value())
	if needle == "" {
		t.indexMap = make([]int, len(t.allRows))
		for i := range t.allRows {
			t.indexMap[i] = i
		}
		t.tbl.SetRows(t.allRows)
	} else {
		var rows []table.Row
		t.indexMap = t.indexMap[:0]
		for i, row := range t.allRows {
			if strings.Contains(strings.ToLower(strings.Join(row, " ")), needle) {
				rows = append(rows, row)
				t.indexMap = append(t.indexMap, i)
			}
		}
		t.tbl.SetRows(rows)
	}
	// SetRows only clamps the cursor downward; a zero-match filter leaves it
	// at -1, which would make every selection-based key silently dead.
	if cursor := t.tbl.Cursor(); (cursor < 0 || cursor >= len(t.indexMap)) && len(t.indexMap) > 0 {
		t.tbl.SetCursor(0)
	}
}

// selectedIndex returns the index into the ORIGINAL row set, or -1.
func (t *entityTable) selectedIndex() int {
	cursor := t.tbl.Cursor()
	if cursor < 0 || cursor >= len(t.indexMap) {
		return -1
	}
	return t.indexMap[cursor]
}

// capturing reports whether the filter input is consuming keystrokes.
func (t *entityTable) capturing() bool {
	return t.filterOn
}

func (t *entityTable) setSize(width, height int) {
	t.tbl.SetWidth(width)
	t.tbl.SetHeight(height)
}

// update returns true when it consumed the message.
func (t *entityTable) update(msg tea.Msg) (tea.Cmd, bool) {
	keyMsg, isKey := msg.(tea.KeyMsg)
	if !isKey {
		return nil, false
	}
	if t.filterOn {
		switch keyMsg.String() {
		case "enter":
			t.filterOn = false
			t.filter.Blur()
			return nil, true
		case "esc":
			t.filterOn = false
			t.filter.Blur()
			t.filter.SetValue("")
			t.applyFilter()
			return nil, true
		default:
			var cmd tea.Cmd
			t.filter, cmd = t.filter.Update(msg)
			t.applyFilter()
			return cmd, true
		}
	}
	switch keyMsg.String() {
	case "/":
		t.filterOn = true
		t.filter.Focus()
		return textinput.Blink, true
	case "up", "down", "k", "j", "pgup", "pgdown", "home", "end", "g", "G":
		var cmd tea.Cmd
		t.tbl, cmd = t.tbl.Update(msg)
		return cmd, true
	}
	return nil, false
}

func (t *entityTable) view() string {
	var b strings.Builder
	if t.filterOn || t.filter.Value() != "" {
		b.WriteString(t.filter.View() + "\n")
	}
	b.WriteString(t.tbl.View())
	return b.String()
}
