package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/positronico/kkonf/v2/internal/models"
)

type formDoneMsg struct {
	id string
	ok bool
}

// formModal wraps a huh.Form as a modal. The owning screen keeps the value
// struct the form writes into and reads it back on formDoneMsg{ok: true}.
type formModal struct {
	id       string
	form     *huh.Form
	escArmed bool
}

func newFormModal(id string, form *huh.Form) *formModal {
	return &formModal{id: id, form: form.WithShowHelp(true)}
}

func (m *formModal) init() tea.Cmd {
	return m.form.Init()
}

func (m *formModal) setSize(width, height int) {
	formWidth := width - 12
	if formWidth > 76 {
		formWidth = 76
	}
	if formWidth < 30 {
		formWidth = 30
	}
	formHeight := height - 8
	if formHeight < 8 {
		formHeight = 8
	}
	m.form = m.form.WithWidth(formWidth).WithHeight(formHeight)
}

func (m *formModal) update(msg tea.Msg) (modal, tea.Cmd) {
	// A single Esc is forwarded to huh (Selects use it to clear their '/'
	// filter); only a second consecutive Esc cancels the form, so typed
	// input is never discarded by the filter workflow.
	if keyMsg, isKey := msg.(tea.KeyMsg); isKey {
		if keyMsg.String() == "esc" {
			if m.escArmed {
				return nil, func() tea.Msg { return formDoneMsg{id: m.id, ok: false} }
			}
			m.escArmed = true
		} else {
			m.escArmed = false
		}
	}
	model, cmd := m.form.Update(msg)
	if form, isForm := model.(*huh.Form); isForm {
		m.form = form
	}
	switch m.form.State {
	case huh.StateCompleted:
		return nil, func() tea.Msg { return formDoneMsg{id: m.id, ok: true} }
	case huh.StateAborted:
		return nil, func() tea.Msg { return formDoneMsg{id: m.id, ok: false} }
	}
	return m, cmd
}

func (m *formModal) view(width int) string {
	hint := styleFooter.Render("esc esc cancel  ctrl+c cancel")
	return renderModalBox(m.form.View()+"\n"+hint, width)
}

// ---- cluster form ----

type clusterFormValues struct {
	Name          string
	Server        string
	SkipTLS       bool
	CAType        string // "embedded", "file", "none"
	CAValue       string
	TLSServerName string
	ProxyURL      string
}

func clusterForm(session *Session, values *clusterFormValues, isEdit bool) *huh.Form {
	caType := func(c models.Cluster) string {
		switch {
		case c.CertificateAuthorityData != "":
			return "embedded"
		case c.CertificateAuthority != "":
			return "file"
		}
		return "none"
	}
	_ = caType

	fields := []huh.Field{}
	if !isEdit {
		fields = append(fields, huh.NewInput().
			Title("Cluster name").
			Value(&values.Name).
			Validate(func(s string) error {
				if s == "" {
					return fmt.Errorf("name is required")
				}
				if session.Config.FindCluster(s) != nil {
					return fmt.Errorf("a cluster named %q already exists", s)
				}
				return nil
			}))
	}
	fields = append(fields,
		huh.NewInput().
			Title("Server URL").
			Value(&values.Server).
			Validate(func(s string) error {
				if s == "" {
					return fmt.Errorf("server URL is required")
				}
				return nil
			}),
		huh.NewConfirm().
			Title("Skip TLS verification?").
			Value(&values.SkipTLS),
		huh.NewSelect[string]().
			Title("Certificate authority").
			Options(
				huh.NewOption("None", "none"),
				huh.NewOption("Base64 data (embedded)", "embedded"),
				huh.NewOption("File path", "file"),
			).
			Value(&values.CAType),
		huh.NewInput().
			Title("CA value (base64 or path; blank if none)").
			Value(&values.CAValue),
		huh.NewInput().
			Title("TLS server name (optional)").
			Value(&values.TLSServerName),
		huh.NewInput().
			Title("Proxy URL (optional)").
			Value(&values.ProxyURL),
	)
	return huh.NewForm(huh.NewGroup(fields...))
}

func (v *clusterFormValues) apply(cluster *models.Cluster) {
	cluster.Server = v.Server
	cluster.InsecureSkipTLSVerify = v.SkipTLS
	cluster.TLSServerName = strings.TrimSpace(v.TLSServerName)
	cluster.ProxyURL = strings.TrimSpace(v.ProxyURL)
	cluster.CertificateAuthorityData = ""
	cluster.CertificateAuthority = ""
	switch v.CAType {
	case "embedded":
		cluster.CertificateAuthorityData = strings.TrimSpace(v.CAValue)
	case "file":
		cluster.CertificateAuthority = strings.TrimSpace(v.CAValue)
	}
}

func clusterFormFromExisting(c models.Cluster) clusterFormValues {
	values := clusterFormValues{
		Server:        c.Server,
		SkipTLS:       c.InsecureSkipTLSVerify,
		TLSServerName: c.TLSServerName,
		ProxyURL:      c.ProxyURL,
		CAType:        "none",
	}
	switch {
	case c.CertificateAuthorityData != "":
		values.CAType = "embedded"
		values.CAValue = c.CertificateAuthorityData
	case c.CertificateAuthority != "":
		values.CAType = "file"
		values.CAValue = c.CertificateAuthority
	}
	return values
}

// ---- user form ----

type userFormValues struct {
	Name   string
	Method string // exec, token, tokenfile, cert-data, cert-files, basic

	ExecCommand    string
	ExecAPIVersion string
	ExecArgs       string
	ExecEnv        string
	ExecMode       string
	ExecHint       string

	Token     string
	TokenFile string

	CertValue string
	KeyValue  string

	Username string
	Password string
}

func userForm(session *Session, values *userFormValues, isEdit bool) *huh.Form {
	first := []huh.Field{}
	if !isEdit {
		first = append(first, huh.NewInput().
			Title("User name").
			Value(&values.Name).
			Validate(func(s string) error {
				if s == "" {
					return fmt.Errorf("name is required")
				}
				if session.Config.FindUser(s) != nil {
					return fmt.Errorf("a user named %q already exists", s)
				}
				return nil
			}))
	}
	first = append(first, huh.NewSelect[string]().
		Title("Authentication method").
		Options(
			huh.NewOption("Exec (auth plugin command)", "exec"),
			huh.NewOption("Token", "token"),
			huh.NewOption("Token file", "tokenfile"),
			huh.NewOption("Client certificate (base64 data)", "cert-data"),
			huh.NewOption("Client certificate (file paths)", "cert-files"),
			huh.NewOption("Basic (username/password)", "basic"),
		).
		Value(&values.Method))

	execGroup := huh.NewGroup(
		huh.NewInput().Title("Command").Value(&values.ExecCommand).
			Validate(func(s string) error {
				if s == "" {
					return fmt.Errorf("command is required")
				}
				return nil
			}),
		huh.NewInput().Title("API version").Value(&values.ExecAPIVersion),
		huh.NewInput().Title("Arguments (comma-separated)").Value(&values.ExecArgs),
		huh.NewInput().Title("Env vars (KEY=VALUE, comma-separated)").Value(&values.ExecEnv),
		huh.NewSelect[string]().Title("Interactive mode").
			Options(
				huh.NewOption("(unset)", ""),
				huh.NewOption("Never", "Never"),
				huh.NewOption("IfAvailable", "IfAvailable"),
				huh.NewOption("Always", "Always"),
			).
			Value(&values.ExecMode),
		huh.NewInput().Title("Install hint (optional)").Value(&values.ExecHint),
	).WithHideFunc(func() bool { return values.Method != "exec" })

	tokenGroup := huh.NewGroup(
		huh.NewInput().Title("Token").EchoMode(huh.EchoModePassword).Value(&values.Token),
	).WithHideFunc(func() bool { return values.Method != "token" })

	tokenFileGroup := huh.NewGroup(
		huh.NewInput().Title("Token file path").Value(&values.TokenFile),
	).WithHideFunc(func() bool { return values.Method != "tokenfile" })

	certDataGroup := huh.NewGroup(
		huh.NewInput().Title("Client certificate (base64)").Value(&values.CertValue),
		huh.NewInput().Title("Client key (base64)").EchoMode(huh.EchoModePassword).Value(&values.KeyValue),
	).WithHideFunc(func() bool { return values.Method != "cert-data" })

	certFilesGroup := huh.NewGroup(
		huh.NewInput().Title("Client certificate file").Value(&values.CertValue),
		huh.NewInput().Title("Client key file").Value(&values.KeyValue),
	).WithHideFunc(func() bool { return values.Method != "cert-files" })

	basicGroup := huh.NewGroup(
		huh.NewInput().Title("Username").Value(&values.Username),
		huh.NewInput().Title("Password").EchoMode(huh.EchoModePassword).Value(&values.Password),
	).WithHideFunc(func() bool { return values.Method != "basic" })

	return huh.NewForm(
		huh.NewGroup(first...),
		execGroup, tokenGroup, tokenFileGroup, certDataGroup, certFilesGroup, basicGroup,
	)
}

// apply replaces only credential fields, preserving impersonation,
// extensions, and unknown fields on the target user.
func (v *userFormValues) apply(user *models.User) {
	user.ClientCertificateData = ""
	user.ClientCertificate = ""
	user.ClientKeyData = ""
	user.ClientKey = ""
	user.Token = ""
	user.TokenFile = ""
	user.Username = ""
	user.Password = ""
	user.Exec = nil
	user.AuthProvider = nil

	switch v.Method {
	case "exec":
		exec := &models.ExecConfig{
			Command:         strings.TrimSpace(v.ExecCommand),
			APIVersion:      strings.TrimSpace(v.ExecAPIVersion),
			InstallHint:     v.ExecHint,
			InteractiveMode: v.ExecMode,
		}
		if args := strings.TrimSpace(v.ExecArgs); args != "" {
			for _, arg := range strings.Split(args, ",") {
				exec.Args = append(exec.Args, strings.TrimSpace(arg))
			}
		}
		if env := strings.TrimSpace(v.ExecEnv); env != "" {
			for _, pair := range strings.Split(env, ",") {
				key, value, found := strings.Cut(strings.TrimSpace(pair), "=")
				if found && key != "" {
					exec.Env = append(exec.Env, models.ExecEnvVar{Name: key, Value: value})
				}
			}
		}
		user.Exec = exec
	case "token":
		user.Token = strings.TrimSpace(v.Token)
	case "tokenfile":
		user.TokenFile = strings.TrimSpace(v.TokenFile)
	case "cert-data":
		user.ClientCertificateData = strings.TrimSpace(v.CertValue)
		user.ClientKeyData = strings.TrimSpace(v.KeyValue)
	case "cert-files":
		user.ClientCertificate = strings.TrimSpace(v.CertValue)
		user.ClientKey = strings.TrimSpace(v.KeyValue)
	case "basic":
		user.Username = strings.TrimSpace(v.Username)
		user.Password = v.Password
	}
}

func userFormFromExisting(u models.User) userFormValues {
	values := userFormValues{Method: "token", ExecAPIVersion: "client.authentication.k8s.io/v1beta1"}
	switch {
	case u.Exec != nil:
		values.Method = "exec"
		values.ExecCommand = u.Exec.Command
		values.ExecAPIVersion = u.Exec.APIVersion
		values.ExecArgs = strings.Join(u.Exec.Args, ",")
		values.ExecMode = u.Exec.InteractiveMode
		values.ExecHint = u.Exec.InstallHint
		var env []string
		for _, e := range u.Exec.Env {
			env = append(env, e.Name+"="+e.Value)
		}
		values.ExecEnv = strings.Join(env, ",")
	case u.Token != "":
		values.Method = "token"
		values.Token = u.Token
	case u.TokenFile != "":
		values.Method = "tokenfile"
		values.TokenFile = u.TokenFile
	case u.ClientCertificateData != "":
		values.Method = "cert-data"
		values.CertValue = u.ClientCertificateData
		values.KeyValue = u.ClientKeyData
	case u.ClientCertificate != "":
		values.Method = "cert-files"
		values.CertValue = u.ClientCertificate
		values.KeyValue = u.ClientKey
	case u.Username != "":
		values.Method = "basic"
		values.Username = u.Username
		values.Password = u.Password
	}
	return values
}

// ---- context form ----

type contextFormValues struct {
	Name       string
	Cluster    string
	User       string
	Namespace  string
	SetCurrent bool
}

func contextForm(session *Session, values *contextFormValues, isEdit bool) *huh.Form {
	clusterOptions := make([]huh.Option[string], len(session.Config.Clusters))
	for i, c := range session.Config.Clusters {
		clusterOptions[i] = huh.NewOption(c.Name, c.Name)
	}
	userOptions := make([]huh.Option[string], len(session.Config.Users))
	for i, u := range session.Config.Users {
		userOptions[i] = huh.NewOption(u.Name, u.Name)
	}

	fields := []huh.Field{}
	if !isEdit {
		fields = append(fields, huh.NewInput().
			Title("Context name").
			Value(&values.Name).
			Validate(func(s string) error {
				if s == "" {
					return fmt.Errorf("name is required")
				}
				if session.Config.FindContext(s) != nil {
					return fmt.Errorf("a context named %q already exists", s)
				}
				return nil
			}))
	}
	fields = append(fields,
		huh.NewSelect[string]().Title("Cluster").Options(clusterOptions...).Value(&values.Cluster),
		huh.NewSelect[string]().Title("User").Options(userOptions...).Value(&values.User),
		huh.NewInput().Title("Namespace (blank = default)").Value(&values.Namespace),
	)
	if !isEdit {
		fields = append(fields, huh.NewConfirm().Title("Set as current context?").Value(&values.SetCurrent))
	}
	return huh.NewForm(huh.NewGroup(fields...))
}
