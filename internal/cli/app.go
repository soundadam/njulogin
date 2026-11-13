// Package cli implements the njulogin command line on top of the portal, CAS,
// credential, and transport packages.
package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"
	"time"

	appkit "github.com/soundadam/appkit/go"
	appkitcli "github.com/soundadam/appkit/go/cli"

	"github.com/soundadam/njulogin/internal/cas"
	"github.com/soundadam/njulogin/internal/credential"
	"github.com/soundadam/njulogin/internal/portal"
	"github.com/soundadam/njulogin/internal/usage"
)

// App holds the process environment and the policies commands run under.
// Tests replace fields; Main uses New.
type App struct {
	Stdin  *os.File
	Stdout io.Writer
	Stderr io.Writer
	Getenv func(string) string

	// Version is the linker-provided version, resolved against build info.
	Version string

	Poll                   portal.Poll
	CAS                    cas.Endpoints
	QRRenderer             cas.Renderer
	QRPollInterval         time.Duration
	PasswordCommandTimeout time.Duration
	// SetupSecret reads the password `setup` saves.
	SetupSecret credential.Source

	stdin *bufio.Reader
}

// New returns an App bound to the process's standard streams and environment.
func New(version string) *App {
	return &App{
		Stdin:                  os.Stdin,
		Stdout:                 os.Stdout,
		Stderr:                 os.Stderr,
		Getenv:                 os.Getenv,
		Version:                version,
		Poll:                   portal.DefaultPoll,
		CAS:                    cas.DefaultEndpoints,
		QRRenderer:             cas.TerminalQR{},
		QRPollInterval:         time.Second,
		PasswordCommandTimeout: 15 * time.Second,
	}
}

// Main runs args and returns the process exit status.
func Main(version string, args []string) int {
	credential.CreateDefaultDir()
	app := New(version)
	notice := appkitcli.StartNotice(app.identity())
	err := app.Run(context.Background(), args)
	notice.Print(os.Stderr)
	if err != nil {
		fmt.Fprintln(app.Stderr, "error:", err)
		return ExitCode(err)
	}
	return 0
}

// ExitCode is 2 for invocation errors and 1 for every other failure.
func ExitCode(err error) int {
	if usage.Is(err) {
		return 2
	}
	return 1
}

func (a *App) env(name, fallback string) string {
	if value := a.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func (a *App) portalURL() string {
	return a.env("NJU_PORTAL_BASE_URL", portal.DefaultBaseURL)
}

func (a *App) store() (credential.FileStore, error) {
	path, err := credential.DefaultPath(a.Getenv("NJULOGIN_CONFIG_DIR"))
	if err != nil {
		return credential.FileStore{}, err
	}
	return credential.FileStore{Path: path}, nil
}

// input is shared by every prompt and --password-stdin so buffered bytes are
// never lost between reads.
func (a *App) input() *bufio.Reader {
	if a.stdin == nil {
		a.stdin = bufio.NewReader(a.Stdin)
	}
	return a.stdin
}

func (a *App) setupSecret() credential.Source {
	if a.SetupSecret != nil {
		return a.SetupSecret
	}
	return credential.Terminal{In: a.Stdin, Prompt: a.Stderr, Label: "NJU credential: ", RequireTerminal: true}
}

// identity is this program's identity for appkit: release lookup, asset
// names, the Homebrew Cask, and the update command. It carries the linked
// version unresolved, so a development build never updates itself.
func (a *App) identity() appkit.App {
	return appkit.App{Name: "njulogin", Version: a.Version, Repo: "soundadam/njulogin"}
}

func (a *App) effectiveVersion() string {
	info, _ := debug.ReadBuildInfo()
	return resolveVersion(a.Version, info)
}

// resolveVersion prefers a linker-provided version, then the module version.
func resolveVersion(linked string, info *debug.BuildInfo) string {
	linked = strings.TrimSpace(linked)
	if linked != "" && linked != "dev" {
		return linked
	}
	if info != nil {
		moduleVersion := strings.TrimSpace(info.Main.Version)
		if moduleVersion != "" && moduleVersion != "(devel)" {
			return moduleVersion
		}
	}
	return "dev"
}
