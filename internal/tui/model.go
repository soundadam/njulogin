package tui

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/soundadam/njulogin/internal/portal"
	"github.com/soundadam/njulogin/internal/redact"
)

type screen int

const (
	screenTarget screen = iota
	screenDashboard
	screenForm
	screenPick
	screenConfirm
	screenRunning
	screenResult
)

// operation is one documented command the user has chosen and confirmed.
type operation struct {
	title string
	run   func(ctx context.Context, out io.Writer) error
}

// form collects text fields and then builds the operation they feed.
type form struct {
	title  string
	labels []string
	fields []textinput.Model
	focus  int
	submit func(values []string) (operation, error)
}

// pick selects one row of the snapshot for a destructive operation.
type pick struct {
	title  string
	rows   []string
	cursor int
	choose func(index int) operation
}

type action struct {
	key   string
	label string
}

var actions = []action{
	{"l", "Password login"},
	{"q", "QR login"},
	{"o", "Log out"},
	{"k", "Kick an online device"},
	{"a", "Bind this device for quick login"},
	{"d", "Delete a quick-login device"},
	{"s", "Save credential"},
	{"r", "Refresh"},
	{"t", "Switch target"},
}

// Model is the bubbletea model for the whole interface.
type Model struct {
	connect Connector
	backend Backend
	screen  screen
	width   int
	notice  string

	targetCursor int
	host         textinput.Model
	hostActive   bool

	snapshot     *Snapshot
	loading      bool
	actionCursor int

	form    *form
	pick    *pick
	pending *operation

	spinner spinner.Model
	running *operation
	output  *liveBuffer
	cancel  context.CancelFunc
	err     error
}

// New returns a Model that starts at the target picker.
func New(connect Connector) Model {
	host := textinput.New()
	host.Prompt = "SSH host: "
	host.Placeholder = "alias from ~/.ssh/config"
	return Model{
		connect: connect,
		screen:  screenTarget,
		host:    host,
		spinner: spinner.New(spinner.WithSpinner(spinner.Dot)),
	}
}

type snapshotMsg Snapshot

type outputMsg struct{}

type doneMsg struct{ err error }

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		}
	case snapshotMsg:
		snapshot := Snapshot(msg)
		m.snapshot, m.loading = &snapshot, false
		return m, nil
	case outputMsg:
		if m.screen != screenRunning {
			return m, nil
		}
		return m, m.waitForOutput()
	case doneMsg:
		m.screen, m.err, m.cancel = screenResult, msg.err, nil
		return m, nil
	case spinner.TickMsg:
		if m.screen != screenRunning && !m.loading {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	switch m.screen {
	case screenTarget:
		return m.updateTarget(msg)
	case screenDashboard:
		return m.updateDashboard(msg)
	case screenForm:
		return m.updateForm(msg)
	case screenPick:
		return m.updatePick(msg)
	case screenConfirm:
		return m.updateConfirm(msg)
	case screenRunning:
		if key, ok := msg.(tea.KeyMsg); ok && key.String() == "esc" && m.cancel != nil {
			m.cancel()
		}
		return m, nil
	case screenResult:
		if _, ok := msg.(tea.KeyMsg); ok {
			return m.openDashboard()
		}
	}
	return m, nil
}

func (m Model) updateTarget(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if m.hostActive {
		if ok {
			switch key.String() {
			case "esc":
				m.hostActive, m.notice = false, ""
				m.host.Blur()
				return m, nil
			case "enter":
				return m.connectTo(strings.TrimSpace(m.host.Value()))
			}
		}
		var cmd tea.Cmd
		m.host, cmd = m.host.Update(msg)
		return m, cmd
	}
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "up", "k":
		m.targetCursor = 0
	case "down", "j":
		m.targetCursor = 1
	case "esc", "q":
		return m, tea.Quit
	case "enter":
		if m.targetCursor == 0 {
			return m.connectTo("")
		}
		m.hostActive = true
		return m, m.host.Focus()
	}
	return m, nil
}

func (m Model) connectTo(host string) (tea.Model, tea.Cmd) {
	backend, err := m.connect(host)
	if err != nil {
		m.notice = err.Error()
		return m, nil
	}
	m.backend, m.hostActive, m.notice = backend, false, ""
	m.host.Blur()
	m.actionCursor = 0
	return m.openDashboard()
}

func (m Model) openDashboard() (tea.Model, tea.Cmd) {
	m.screen, m.loading, m.form, m.pick, m.pending, m.running, m.err = screenDashboard, true, nil, nil, nil, nil, nil
	backend := m.backend
	return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
		return snapshotMsg(backend.Snapshot(context.Background()))
	})
}

func (m Model) updateDashboard(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	m.notice = ""
	switch key.String() {
	case "up":
		m.actionCursor = (m.actionCursor + len(actions) - 1) % len(actions)
		return m, nil
	case "down":
		m.actionCursor = (m.actionCursor + 1) % len(actions)
		return m, nil
	case "esc":
		return m, tea.Quit
	case "enter":
		return m.choose(actions[m.actionCursor].key)
	}
	return m.choose(key.String())
}

func (m Model) choose(key string) (tea.Model, tea.Cmd) {
	b := m.backend
	switch key {
	case "l":
		if b.HasSavedCredential() {
			return m.start(operation{"Password login", func(ctx context.Context, out io.Writer) error {
				return b.Login(ctx, out, "", "")
			}})
		}
		return m.openForm(&form{
			title:  "Password login",
			labels: []string{"Username", "Password"},
			submit: func(v []string) (operation, error) {
				return operation{"Password login", func(ctx context.Context, out io.Writer) error {
					return b.Login(ctx, out, v[0], v[1])
				}}, nil
			},
		})
	case "q":
		return m.start(operation{"QR login", b.QR})
	case "o":
		return m.confirm(operation{"Log out " + b.Name(), b.Logout})
	case "k":
		return m.openPick("Kick which online device?", m.onlineLines(), m.snapshotErr(func(s *Snapshot) error { return s.OnlineErr }),
			func(index int) operation {
				return operation{fmt.Sprintf("Kick online device %d", index+1), func(ctx context.Context, out io.Writer) error {
					return b.Kick(ctx, out, index+1)
				}}
			})
	case "a":
		f := &form{
			title:  "Bind this device for quick login",
			labels: []string{"Device name"},
			submit: func(v []string) (operation, error) {
				return operation{"Bind quick login as " + v[0], func(ctx context.Context, out io.Writer) error {
					return b.QuickAdd(ctx, out, v[0])
				}}, nil
			},
		}
		next, cmd := m.openForm(f)
		f.fields[0].SetValue(b.DeviceName())
		return next, cmd
	case "d":
		var rows []portal.Row
		if m.snapshot != nil {
			rows = m.snapshot.Quick
		}
		return m.openPick("Delete which quick-login device?", m.quickLines(), m.snapshotErr(func(s *Snapshot) error { return s.QuickErr }),
			func(index int) operation {
				id := rows[index].ID()
				return operation{fmt.Sprintf("Delete quick-login device %d", index+1), func(ctx context.Context, out io.Writer) error {
					return b.QuickDelete(ctx, out, id)
				}}
			})
	case "s":
		return m.openForm(&form{
			title:  "Save credential",
			labels: []string{"Username", "Password"},
			submit: func(v []string) (operation, error) {
				return operation{"Save credential", func(ctx context.Context, out io.Writer) error {
					return b.Setup(ctx, out, v[0], v[1])
				}}, nil
			},
		})
	case "r":
		return m.openDashboard()
	case "t":
		m.screen, m.backend, m.snapshot = screenTarget, nil, nil
		return m, nil
	}
	return m, nil
}

func (m Model) snapshotErr(part func(*Snapshot) error) error {
	if m.snapshot == nil {
		return fmt.Errorf("still loading")
	}
	return part(m.snapshot)
}

func (m Model) openForm(f *form) (tea.Model, tea.Cmd) {
	for _, label := range f.labels {
		input := textinput.New()
		input.Prompt = label + ": "
		if label == "Password" {
			input.EchoMode = textinput.EchoPassword
			input.EchoCharacter = '•'
		}
		f.fields = append(f.fields, input)
	}
	m.form, m.screen = f, screenForm
	return m, f.fields[0].Focus()
}

func (m Model) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	f := m.form
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			return m.openDashboard()
		case "tab", "down", "shift+tab", "up":
			step := 1
			if key.String() == "shift+tab" || key.String() == "up" {
				step = len(f.fields) - 1
			}
			f.fields[f.focus].Blur()
			f.focus = (f.focus + step) % len(f.fields)
			return m, f.fields[f.focus].Focus()
		case "enter":
			if f.focus < len(f.fields)-1 {
				f.fields[f.focus].Blur()
				f.focus++
				return m, f.fields[f.focus].Focus()
			}
			values := make([]string, len(f.fields))
			for i, field := range f.fields {
				values[i] = field.Value()
			}
			op, err := f.submit(values)
			if err != nil {
				m.notice = err.Error()
				return m, nil
			}
			return m.start(op)
		}
	}
	var cmd tea.Cmd
	f.fields[f.focus], cmd = f.fields[f.focus].Update(msg)
	return m, cmd
}

func (m Model) openPick(title string, rows []string, err error, choose func(int) operation) (tea.Model, tea.Cmd) {
	if err != nil {
		m.notice = err.Error()
		return m, nil
	}
	if len(rows) == 0 {
		m.notice = "nothing to choose from"
		return m, nil
	}
	m.pick, m.screen = &pick{title: title, rows: rows, choose: choose}, screenPick
	return m, nil
}

func (m Model) updatePick(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	p := m.pick
	switch key.String() {
	case "esc":
		return m.openDashboard()
	case "up", "k":
		p.cursor = (p.cursor + len(p.rows) - 1) % len(p.rows)
	case "down", "j":
		p.cursor = (p.cursor + 1) % len(p.rows)
	case "enter":
		return m.confirm(p.choose(p.cursor))
	}
	return m, nil
}

func (m Model) confirm(op operation) (tea.Model, tea.Cmd) {
	m.pending, m.screen = &op, screenConfirm
	return m, nil
}

func (m Model) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "y", "Y":
		return m.start(*m.pending)
	case "n", "N", "esc":
		return m.openDashboard()
	}
	return m, nil
}

func (m Model) start(op operation) (tea.Model, tea.Cmd) {
	ctx, cancel := context.WithCancel(context.Background())
	m.running, m.cancel, m.output = &op, cancel, newLiveBuffer()
	m.screen, m.form, m.pick, m.pending = screenRunning, nil, nil, nil
	out := m.output
	return m, tea.Batch(m.spinner.Tick, m.waitForOutput(), func() tea.Msg {
		defer cancel()
		err := op.run(ctx, out)
		out.close()
		return doneMsg{err: err}
	})
}

// waitForOutput wakes the model whenever the running operation writes, so
// streamed output such as a QR code appears immediately.
func (m Model) waitForOutput() tea.Cmd {
	if m.output == nil {
		return nil
	}
	changed := m.output.changed
	return func() tea.Msg {
		if _, open := <-changed; !open {
			return nil
		}
		return outputMsg{}
	}
}

type liveBuffer struct {
	mu      sync.Mutex
	buf     bytes.Buffer
	changed chan struct{}
}

func newLiveBuffer() *liveBuffer {
	return &liveBuffer{changed: make(chan struct{}, 1)}
}

func (l *liveBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	n, err := l.buf.Write(p)
	select {
	case l.changed <- struct{}{}:
	default:
	}
	return n, err
}

// close ends the wake-ups once the operation has returned and will not write.
func (l *liveBuffer) close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	close(l.changed)
}

func (l *liveBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

var (
	titleStyle  = lipgloss.NewStyle().Bold(true)
	okStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	badStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	dimStyle    = lipgloss.NewStyle().Faint(true)
	cursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	panelStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
)

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("NJUlogin"))
	if m.backend != nil {
		b.WriteString(dimStyle.Render("  " + m.backend.Name()))
	}
	b.WriteString("\n\n")
	switch m.screen {
	case screenTarget:
		b.WriteString(m.viewTarget())
	case screenDashboard:
		b.WriteString(m.viewDashboard())
	case screenForm:
		b.WriteString(titleStyle.Render(m.form.title) + "\n\n")
		for _, field := range m.form.fields {
			b.WriteString(field.View() + "\n")
		}
		b.WriteString(dimStyle.Render("\nenter next/submit • tab switch field • esc cancel"))
	case screenPick:
		b.WriteString(titleStyle.Render(m.pick.title) + "\n\n")
		for i, row := range m.pick.rows {
			b.WriteString(cursorLine(i == m.pick.cursor, row) + "\n")
		}
		b.WriteString(dimStyle.Render("\n↑/↓ choose • enter select • esc cancel"))
	case screenConfirm:
		b.WriteString(m.pending.title + "?\n\n" + dimStyle.Render("y confirm • n cancel"))
	case screenRunning:
		b.WriteString(m.spinner.View() + " " + m.running.title + "\n\n")
		b.WriteString(m.output.String())
		b.WriteString(dimStyle.Render("\nesc cancel"))
	case screenResult:
		b.WriteString(m.output.String())
		if m.err != nil {
			b.WriteString(badStyle.Render("error: "+m.err.Error()) + "\n")
		} else {
			b.WriteString(okStyle.Render("done") + "\n")
		}
		b.WriteString(dimStyle.Render("\npress any key to return"))
	}
	if m.notice != "" {
		b.WriteString("\n" + badStyle.Render(m.notice))
	}
	return b.String() + "\n"
}

func (m Model) viewTarget() string {
	var b strings.Builder
	b.WriteString("Authenticate which machine?\n\n")
	b.WriteString(cursorLine(m.targetCursor == 0, "This machine") + "\n")
	b.WriteString(cursorLine(m.targetCursor == 1, "An SSH host (ssh -W tunnel)") + "\n")
	if m.hostActive {
		b.WriteString("\n" + m.host.View() + "\n")
		b.WriteString(dimStyle.Render("\nenter connect • esc back"))
	} else {
		b.WriteString(dimStyle.Render("\n↑/↓ choose • enter select • esc quit"))
	}
	return b.String()
}

func (m Model) viewDashboard() string {
	var status strings.Builder
	switch {
	case m.snapshot == nil:
		status.WriteString(m.spinner.View() + " loading")
	case m.snapshot.StatusErr != nil:
		status.WriteString(badStyle.Render("status unavailable: " + m.snapshot.StatusErr.Error()))
	case m.snapshot.Status.Authenticated:
		status.WriteString(okStyle.Render("● authenticated"))
		for _, s := range m.snapshot.Status.Sessions {
			status.WriteString(fmt.Sprintf("\n  %s %s  %s  since %s",
				redact.Account(s.Username), redact.Text(s.Fullname),
				redact.Field(s.ServiceName, "unknown"), redact.Unix(s.AcctStartTime)))
		}
	default:
		status.WriteString(badStyle.Render("○ not authenticated"))
	}

	var menu strings.Builder
	for i, a := range actions {
		menu.WriteString(cursorLine(i == m.actionCursor, fmt.Sprintf("%s  %s", cursorStyle.Render(a.key), a.label)) + "\n")
	}

	var b strings.Builder
	b.WriteString(panelStyle.Render(status.String()) + "\n")
	if m.snapshot != nil {
		b.WriteString(section("Online devices", m.onlineLines(), m.snapshot.OnlineErr))
		b.WriteString(section("Quick-login devices", m.quickLines(), m.snapshot.QuickErr))
	}
	b.WriteString("\n" + menu.String())
	b.WriteString(dimStyle.Render("\n↑/↓ + enter or press a key • esc quit"))
	return b.String()
}

func (m Model) onlineLines() []string {
	if m.snapshot == nil {
		return nil
	}
	lines := make([]string, 0, len(m.snapshot.Online))
	for i, row := range m.snapshot.Online {
		lines = append(lines, fmt.Sprintf("%d. since %s  mac %s  ipv6 %s", i+1,
			redact.AnyUnix(row["acctstarttime"]), redact.MAC(row.Field("mac")), redact.Presence(row["user_ipv6"])))
	}
	return lines
}

func (m Model) quickLines() []string {
	if m.snapshot == nil {
		return nil
	}
	lines := make([]string, 0, len(m.snapshot.Quick))
	for i, row := range m.snapshot.Quick {
		lines = append(lines, fmt.Sprintf("%d. %s  mac %s", i+1,
			redact.Field(row.Field("device"), "unknown"), redact.MAC(row.Field("mac"))))
	}
	return lines
}

func section(title string, lines []string, err error) string {
	var b strings.Builder
	b.WriteString("\n" + titleStyle.Render(title) + "\n")
	switch {
	case err != nil:
		b.WriteString(dimStyle.Render("  unavailable: "+err.Error()) + "\n")
	case len(lines) == 0:
		b.WriteString(dimStyle.Render("  none") + "\n")
	default:
		for _, line := range lines {
			b.WriteString("  " + line + "\n")
		}
	}
	return b.String()
}

func cursorLine(selected bool, text string) string {
	if selected {
		return cursorStyle.Render("› ") + text
	}
	return "  " + text
}
