package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/soundadam/njulogin/internal/portal"
)

// fakeBackend records calls instead of contacting a portal.
type fakeBackend struct {
	saved bool
	calls []string
	fail  error
}

func (f *fakeBackend) Name() string             { return "fake" }
func (f *fakeBackend) DeviceName() string       { return "fake-device" }
func (f *fakeBackend) HasSavedCredential() bool { return f.saved }

func (f *fakeBackend) Snapshot(context.Context) Snapshot {
	return Snapshot{
		Status: portal.Status{Authenticated: true, Sessions: []portal.Session{{Username: "123456789", Fullname: "Synthetic", MAC: "02:00:00:00:00:01"}}},
		Online: []portal.Row{{"id": "online-a", "mac": "02:00:00:00:00:01"}, {"id": "online-b", "mac": "02:00:00:00:00:02"}},
		Quick:  []portal.Row{{"id": "quick-a", "device": "phone", "mac": "02:00:00:00:00:03"}},
	}
}

func (f *fakeBackend) record(out io.Writer, call string) error {
	f.calls = append(f.calls, call)
	fmt.Fprintln(out, "result=success")
	return f.fail
}

func (f *fakeBackend) Login(_ context.Context, out io.Writer, username, password string) error {
	return f.record(out, fmt.Sprintf("login %q %q", username, password))
}
func (f *fakeBackend) QR(_ context.Context, out io.Writer) error { return f.record(out, "qr") }
func (f *fakeBackend) Logout(_ context.Context, out io.Writer) error {
	return f.record(out, "logout")
}
func (f *fakeBackend) Kick(_ context.Context, out io.Writer, index int) error {
	return f.record(out, fmt.Sprintf("kick %d", index))
}
func (f *fakeBackend) QuickAdd(_ context.Context, out io.Writer, name string) error {
	return f.record(out, "quick add "+name)
}
func (f *fakeBackend) QuickDelete(_ context.Context, out io.Writer, id string) error {
	return f.record(out, "quick delete "+id)
}
func (f *fakeBackend) Setup(_ context.Context, out io.Writer, username, password string) error {
	return f.record(out, fmt.Sprintf("setup %q %q", username, password))
}

type driver struct {
	t     *testing.T
	model tea.Model
	hosts []string
}

func newDriver(t *testing.T, backend *fakeBackend) *driver {
	d := &driver{t: t}
	d.model = New(func(host string) (Backend, error) {
		d.hosts = append(d.hosts, host)
		if strings.HasPrefix(host, "-") {
			return nil, errors.New("SSH host must not begin with '-'")
		}
		return backend, nil
	})
	return d
}

// send delivers msg and then runs every resulting command to completion,
// skipping timers that would otherwise re-arm forever.
func (d *driver) send(msg tea.Msg) {
	d.t.Helper()
	queue := []tea.Msg{msg}
	for len(queue) > 0 {
		next := queue[0]
		queue = queue[1:]
		var cmd tea.Cmd
		d.model, cmd = d.model.Update(next)
		queue = append(queue, run(cmd)...)
	}
}

// run executes cmd and returns the messages it produces. Commands that block
// longer than a fake operation could, such as cursor blinks and output waits,
// are timers and are dropped.
func run(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	result := make(chan tea.Msg, 1)
	go func() { result <- cmd() }()
	var msg tea.Msg
	select {
	case msg = <-result:
	case <-time.After(100 * time.Millisecond):
		return nil
	}
	switch msg := msg.(type) {
	case nil, spinner.TickMsg:
		return nil
	case tea.BatchMsg:
		var out []tea.Msg
		for _, c := range msg {
			out = append(out, run(c)...)
		}
		return out
	default:
		return []tea.Msg{msg}
	}
}

func (d *driver) keys(keys ...string) {
	d.t.Helper()
	for _, key := range keys {
		switch key {
		case "enter":
			d.send(tea.KeyMsg{Type: tea.KeyEnter})
		case "esc":
			d.send(tea.KeyMsg{Type: tea.KeyEsc})
		case "down":
			d.send(tea.KeyMsg{Type: tea.KeyDown})
		case "tab":
			d.send(tea.KeyMsg{Type: tea.KeyTab})
		default:
			d.send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
		}
	}
}

func (d *driver) view() string { return d.model.View() }

func TestLocalDashboardShowsRedactedSnapshot(t *testing.T) {
	d := newDriver(t, &fakeBackend{saved: true})
	d.keys("enter")
	view := d.view()
	for _, want := range []string{"● authenticated", "123***789", "02:00:xx:xx:xx:02", "phone", "Password login"} {
		if !strings.Contains(view, want) {
			t.Fatalf("dashboard missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "123456789") || strings.Contains(view, "02:00:00:00:00:01") {
		t.Fatalf("dashboard leaked an unredacted field:\n%s", view)
	}
	if len(d.hosts) != 1 || d.hosts[0] != "" {
		t.Fatalf("hosts = %q", d.hosts)
	}
}

func TestSSHHostIsValidatedBeforeConnecting(t *testing.T) {
	backend := &fakeBackend{}
	d := newDriver(t, backend)
	d.keys("down", "enter", "-bad", "enter")
	if !strings.Contains(d.view(), "must not begin with '-'") {
		t.Fatalf("view:\n%s", d.view())
	}
	d.keys("esc", "enter")
	if !strings.Contains(d.view(), "SSH host: ") {
		t.Fatalf("host prompt did not reopen:\n%s", d.view())
	}
}

func TestSavedCredentialLoginRunsWithoutPrompt(t *testing.T) {
	backend := &fakeBackend{saved: true}
	d := newDriver(t, backend)
	d.keys("enter", "l")
	if len(backend.calls) != 1 || backend.calls[0] != `login "" ""` {
		t.Fatalf("calls = %q", backend.calls)
	}
	if view := d.view(); !strings.Contains(view, "result=success") || !strings.Contains(view, "done") {
		t.Fatalf("result view:\n%s", view)
	}
	d.keys("x")
	if !strings.Contains(d.view(), "Quick-login devices") {
		t.Fatal("any key did not return to the dashboard")
	}
}

func TestPasswordFormFeedsLoginAndMasksInput(t *testing.T) {
	backend := &fakeBackend{}
	d := newDriver(t, backend)
	d.keys("enter", "l", "synthetic-user", "enter", "s3cret")
	if view := d.view(); strings.Contains(view, "s3cret") || !strings.Contains(view, "••••••") {
		t.Fatalf("password echoed:\n%s", view)
	}
	d.keys("enter")
	if len(backend.calls) != 1 || backend.calls[0] != `login "synthetic-user" "s3cret"` {
		t.Fatalf("calls = %q", backend.calls)
	}
}

func TestDestructiveOperationsRequireSelectionAndConfirmation(t *testing.T) {
	backend := &fakeBackend{}
	d := newDriver(t, backend)
	d.keys("enter", "k", "down", "enter")
	if !strings.Contains(d.view(), "Kick online device 2?") {
		t.Fatalf("confirm view:\n%s", d.view())
	}
	d.keys("n")
	if len(backend.calls) != 0 {
		t.Fatalf("declined kick ran: %q", backend.calls)
	}
	d.keys("k", "down", "enter", "y", "x", "d", "enter", "y", "x", "o", "y")
	want := []string{"kick 2", "quick delete quick-a", "logout"}
	if strings.Join(backend.calls, ",") != strings.Join(want, ",") {
		t.Fatalf("calls = %q", backend.calls)
	}
}

func TestQuickAddDefaultsToDeviceNameAndSetupSavesCredential(t *testing.T) {
	backend := &fakeBackend{}
	d := newDriver(t, backend)
	d.keys("enter", "a", "enter", "x", "s", "synthetic-user", "tab", "pw", "enter")
	want := []string{"quick add fake-device", `setup "synthetic-user" "pw"`}
	if strings.Join(backend.calls, ",") != strings.Join(want, ",") {
		t.Fatalf("calls = %q", backend.calls)
	}
}

func TestOperationErrorIsShown(t *testing.T) {
	backend := &fakeBackend{fail: errors.New("portal operation rejected with reply_code=1")}
	d := newDriver(t, backend)
	d.keys("enter", "q")
	if !strings.Contains(d.view(), "error: portal operation rejected with reply_code=1") {
		t.Fatalf("view:\n%s", d.view())
	}
}

func TestEscapeQuitsFromDashboard(t *testing.T) {
	d := newDriver(t, &fakeBackend{})
	d.keys("enter")
	_, cmd := d.model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil || fmt.Sprintf("%T", cmd()) != "tea.QuitMsg" {
		t.Fatal("esc on the dashboard did not quit")
	}
}
