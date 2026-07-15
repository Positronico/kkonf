package tui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/cursor"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

const testKubeconfig = `apiVersion: v1
kind: Config
current-context: prod
clusters:
  - name: prod-cluster
    cluster:
      server: https://prod:6443
users:
  - name: admin
    user:
      token: tok
contexts:
  - name: prod
    context:
      cluster: prod-cluster
      user: admin
  - name: dev
    context:
      cluster: prod-cluster
      user: admin
`

func newTestApp(t *testing.T) *App {
	t.Helper()
	toastDuration = time.Millisecond
	path := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(path, []byte(testKubeconfig), 0o600))
	session, err := NewSession(path)
	require.NoError(t, err)
	app := NewApp(session)
	app.width, app.height = 100, 30
	return app
}

func key(app *App, keys ...string) {
	for _, k := range keys {
		var msg tea.Msg
		switch k {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		drain(app, msg)
	}
}

// drain feeds a message and keeps executing returned commands until the
// command chain settles, mimicking the bubbletea runtime. Cursor-blink
// messages are dropped (they self-perpetuate) and depth is capped for safety.
func drain(app *App, msg tea.Msg) {
	drainDepth(app, msg, 0)
}

func drainDepth(app *App, msg tea.Msg, depth int) {
	if depth > 25 {
		return
	}
	if _, isBlink := msg.(cursor.BlinkMsg); isBlink {
		return
	}
	model, cmd := app.Update(msg)
	*app = *model.(*App)
	if cmd == nil {
		return
	}
	out := cmd()
	if out == nil {
		return
	}
	if batch, isBatch := out.(tea.BatchMsg); isBatch {
		for _, c := range batch {
			if c != nil {
				if inner := c(); inner != nil {
					drainDepth(app, inner, depth+1)
				}
			}
		}
		return
	}
	if _, isQuit := out.(tea.QuitMsg); isQuit {
		return
	}
	drainDepth(app, out, depth+1)
}

func TestDigitsSwitchSections(t *testing.T) {
	app := newTestApp(t)
	require.Equal(t, sectionContexts, app.section, "contexts is the landing screen")

	key(app, "1")
	require.Equal(t, sectionClusters, app.section)
	key(app, "2")
	require.Equal(t, sectionUsers, app.section)
	key(app, "5")
	require.Equal(t, sectionSettings, app.section)
	key(app, "3")
	require.Equal(t, sectionContexts, app.section)
}

func TestContextSwitchWithSKey(t *testing.T) {
	app := newTestApp(t)
	// Cursor starts on row 0 ("prod", already current). Move down to "dev".
	key(app, "j", "s")
	require.Equal(t, "dev", app.session.Config.CurrentContext)
	require.True(t, app.session.Dirty())
}

func TestRenameContextFlow(t *testing.T) {
	app := newTestApp(t)
	key(app, "r") // opens rename input pre-filled with "prod"
	require.NotNil(t, app.modal)
	key(app, "2") // digits go to the input, not section switching
	key(app, "enter")
	require.Nil(t, app.modal)
	require.NotNil(t, app.session.Config.FindContext("prod2"))
	require.Equal(t, "prod2", app.session.Config.CurrentContext, "current-context follows rename")
}

func TestDeleteContextConfirm(t *testing.T) {
	app := newTestApp(t)
	key(app, "j", "d") // select "dev", ask delete
	require.NotNil(t, app.modal)
	key(app, "y")
	require.Nil(t, app.session.Config.FindContext("dev"))
}

func TestDeleteCancelKeepsContext(t *testing.T) {
	app := newTestApp(t)
	key(app, "j", "d", "esc")
	require.NotNil(t, app.session.Config.FindContext("dev"))
}

func TestPickerDigitShortcut(t *testing.T) {
	app := newTestApp(t)
	key(app, "1") // clusters section
	key(app, "d") // delete prod-cluster → cascade picker (2 contexts use it)
	require.NotNil(t, app.modal)
	key(app, "3") // "Cancel"
	require.Nil(t, app.modal)
	require.NotNil(t, app.session.Config.FindCluster("prod-cluster"))

	key(app, "d", "1") // cascade delete
	require.Nil(t, app.session.Config.FindCluster("prod-cluster"))
	require.Empty(t, app.session.Config.Contexts, "cascade removes dependent contexts")
}

func TestQuitWithoutChangesQuits(t *testing.T) {
	app := newTestApp(t)
	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	*app = *model.(*App)
	require.NotNil(t, cmd)
	_, isQuit := cmd().(tea.QuitMsg)
	require.True(t, isQuit, "clean session quits immediately")
}

func TestQuitWithChangesAsksToSave(t *testing.T) {
	app := newTestApp(t)
	key(app, "j", "s") // make it dirty
	key(app, "q")
	require.NotNil(t, app.modal, "dirty session must prompt before quitting")
}

func TestSaveWritesFile(t *testing.T) {
	app := newTestApp(t)
	key(app, "j", "s")
	require.True(t, app.session.Dirty())
	drain(app, tea.KeyMsg{Type: tea.KeyCtrlS})
	require.False(t, app.session.Dirty(), "save must reset the dirty state")

	session, err := NewSession(app.session.Path)
	require.NoError(t, err)
	require.Equal(t, "dev", session.Config.CurrentContext)
}

func TestFilterCapturesDigits(t *testing.T) {
	app := newTestApp(t)
	key(app, "/") // open filter on contexts table
	require.True(t, app.currentScreen().capturing())
	key(app, "1") // must filter, not switch section
	require.Equal(t, sectionContexts, app.section)
	key(app, "esc")
	require.False(t, app.currentScreen().capturing())
}

func TestViewRenders(t *testing.T) {
	app := newTestApp(t)
	drain(app, tea.WindowSizeMsg{Width: 100, Height: 30})
	view := app.View()
	require.Contains(t, view, "Contexts")
	require.Contains(t, view, "prod")
	require.Contains(t, view, "ctx: prod")
}
