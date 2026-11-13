package cli

import (
	"context"
	"io"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	"github.com/soundadam/njulogin/internal/credential"
	"github.com/soundadam/njulogin/internal/sshtunnel"
	"github.com/soundadam/njulogin/internal/tui"
)

// interactive runs the terminal interface. It renders on standard error when
// that is a terminal, keeping standard output for command results.
func (a *App) interactive(ctx context.Context) error {
	out := a.Stdout
	if f, ok := a.Stderr.(interface{ Fd() uintptr }); ok && term.IsTerminal(int(f.Fd())) {
		out = a.Stderr
	}
	program := tea.NewProgram(tui.New(a.connect),
		tea.WithContext(ctx),
		tea.WithInput(a.Stdin),
		tea.WithOutput(out),
		tea.WithAltScreen(),
	)
	_, err := program.Run()
	return err
}

func (a *App) connect(host string) (tui.Backend, error) {
	if host == "" {
		return backend{app: a, target: Local{BaseURL: a.portalURL()}, name: "local"}, nil
	}
	if err := sshtunnel.ValidateHost(host); err != nil {
		return nil, err
	}
	target := SSH{Host: host, Settings: defaultRemoteSettings(a.portalURL())}
	return backend{app: a, target: target, name: "ssh " + host}, nil
}

// backend presents App operations to the TUI. Each mutation runs the same
// code path as its command, writing records to the TUI's buffer.
type backend struct {
	app    *App
	target Target
	name   string
}

var _ tui.Backend = backend{}

type fixedSecret string

func (s fixedSecret) Secret(context.Context) (string, error) { return string(s), nil }

func (b backend) to(out io.Writer) *App {
	clone := *b.app
	clone.Stdout = out
	return &clone
}

func (b backend) Name() string       { return b.name }
func (b backend) DeviceName() string { return b.target.DeviceName() }

func (b backend) HasSavedCredential() bool {
	store, err := b.app.store()
	if err != nil {
		return false
	}
	saved, err := store.Load()
	return err == nil && saved != nil
}

func (b backend) Snapshot(ctx context.Context) tui.Snapshot {
	api, _, err := newAPI(b.target)
	if err != nil {
		return tui.Snapshot{StatusErr: err, OnlineErr: err, QuickErr: err}
	}
	var s tui.Snapshot
	s.Status, s.StatusErr = api.Status(ctx)
	s.Online, s.OnlineErr = api.OnlineDevices(ctx)
	s.Quick, s.QuickErr = api.QuickDevices(ctx)
	return s
}

func (b backend) Login(ctx context.Context, out io.Writer, username, password string) error {
	opts := loginOptions{Username: username}
	if password != "" {
		opts.Source = fixedSecret(password)
	}
	return b.to(out).login(ctx, b.target, opts)
}

func (b backend) QR(ctx context.Context, out io.Writer) error {
	return b.to(out).qr(ctx, b.target, qrTimeout)
}

func (b backend) Logout(ctx context.Context, out io.Writer) error {
	return b.to(out).logout(ctx, b.target)
}

func (b backend) Kick(ctx context.Context, out io.Writer, index int) error {
	return b.to(out).kick(ctx, b.target, "", index)
}

func (b backend) QuickAdd(ctx context.Context, out io.Writer, name string) error {
	return b.to(out).quickAdd(ctx, b.target, name)
}

func (b backend) QuickDelete(ctx context.Context, out io.Writer, id string) error {
	return b.to(out).quickDelete(ctx, b.target, id)
}

func (b backend) Setup(ctx context.Context, out io.Writer, username, password string) error {
	app := b.to(out)
	app.SetupSecret = fixedSecret(password)
	return app.setup(ctx, username)
}

var _ credential.Source = fixedSecret("")
