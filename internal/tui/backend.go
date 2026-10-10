// Package tui is the interactive terminal interface. It presents documented
// njulogin operations through a Backend and adds none of its own: validation,
// postconditions, and redaction stay in the Backend's commands.
package tui

import (
	"context"
	"io"

	"github.com/soundadam/njulogin/internal/portal"
)

// Backend runs documented operations against one target. Mutations write the
// same key=value records as the command line to out.
type Backend interface {
	// Name labels the target, such as "local" or "ssh my-host".
	Name() string
	// DeviceName is the default quick-login device name.
	DeviceName() string
	// HasSavedCredential reports whether password login can use the saved
	// credential without prompting.
	HasSavedCredential() bool

	Snapshot(ctx context.Context) Snapshot
	Login(ctx context.Context, out io.Writer, username, password string) error
	QR(ctx context.Context, out io.Writer) error
	Logout(ctx context.Context, out io.Writer) error
	Kick(ctx context.Context, out io.Writer, index int) error
	QuickAdd(ctx context.Context, out io.Writer, name string) error
	QuickDelete(ctx context.Context, out io.Writer, id string) error
	Setup(ctx context.Context, out io.Writer, username, password string) error
}

// Connector returns the Backend for host, or for this machine when host is "".
// It validates host before any connection is attempted.
type Connector func(host string) (Backend, error)

// Snapshot is the read-only state the dashboard shows. Each part carries its
// own error so one failed query does not hide the others.
type Snapshot struct {
	Status    portal.Status
	StatusErr error
	Online    []portal.Row
	OnlineErr error
	Quick     []portal.Row
	QuickErr  error
}
