package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/soundadam/njulogin/internal/sshtunnel"
	"github.com/soundadam/njulogin/internal/usage"
)

const qrTimeout = 60 * time.Second

// Run dispatches one invocation. With no arguments on a terminal it starts the
// interactive interface.
func (a *App) Run(ctx context.Context, args []string) error {
	root := a.rootCommand()
	root.SetArgs(args)
	return root.ExecuteContext(ctx)
}

func (a *App) rootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "njulogin",
		Short: "Authenticate to the NJU p.nju.edu.cn portal, locally or through an SSH host",
		Long: `njulogin authenticates to the NJU p.nju.edu.cn portal.

Commands print one key=value record per line. Exit status is 0 when the
documented postcondition holds, 1 on runtime failure, and 2 on invalid usage.
Run without a command in a terminal to open the interactive interface.`,
		Version: a.effectiveVersion(),
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 0 {
				return usage.Errorf("unknown command: %s", args[0])
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !a.isTerminal() {
				fmt.Fprint(a.Stderr, cmd.UsageString())
				return usage.Errorf("interactive mode requires a terminal")
			}
			return a.interactive(cmd.Context())
		},
	}
	root.SetVersionTemplate("njulogin {{.Version}}\n")
	a.configure(root)

	local := func() Target { return Local{BaseURL: a.portalURL()} }
	root.AddCommand(a.setupCommand())
	root.AddCommand(a.operationCommands(local, true)...)
	root.AddCommand(a.remoteCommand())
	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Args:  noArgs("version"),
		RunE: func(*cobra.Command, []string) error {
			fmt.Fprintf(a.Stdout, "njulogin %s\n", a.effectiveVersion())
			return nil
		},
	})
	return root
}

// configure applies the stream and error policy every command tree shares:
// results on stdout, diagnostics on stderr, and flag mistakes as usage errors.
func (a *App) configure(cmd *cobra.Command) {
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetIn(a.Stdin)
	cmd.SetOut(a.Stdout)
	cmd.SetErr(a.Stderr)
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return usage.Errorf("%v", err)
	})
}

func (a *App) setupCommand() *cobra.Command {
	var username string
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Save the username and password used by login",
		Long: `Prompt for the password without echo and save it with the username in the
local credential file (directory 0700, file 0600, atomic replacement).`,
		Args: positionalFree,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.setup(cmd.Context(), username)
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "NJU username (prompted when omitted)")
	return cmd
}

// operationCommands builds the commands shared by local and remote use. Only
// local qr accepts --timeout; remote qr uses the standard window.
func (a *App) operationCommands(target func() Target, local bool) []*cobra.Command {
	simple := func(use, short string, run func(context.Context, Target) error) *cobra.Command {
		return &cobra.Command{
			Use:   use,
			Short: short,
			Args:  noArgs(prefixOf(local) + use),
			RunE: func(cmd *cobra.Command, _ []string) error {
				return run(cmd.Context(), target())
			},
		}
	}

	var opts loginOptions
	login := &cobra.Command{
		Use:   "login",
		Short: "Log in with the saved credential or an explicit password source",
		Args:  positionalFree,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.login(cmd.Context(), target(), opts)
		},
	}
	login.Flags().StringVar(&opts.Username, "username", "", "NJU username (defaults to the saved one)")
	login.Flags().StringVar(&opts.PasswordCommand, "password-command", "", "trusted command that prints the password")
	login.Flags().BoolVar(&opts.Stdin, "password-stdin", false, "read the password from standard input")
	login.Flags().BoolVar(&opts.Ask, "ask", false, "prompt for the password without echo")
	login.Flags().StringVar(&opts.AddQuick, "add-quick", "", "bind this device for quick login under `NAME` after login")

	qr := simple("qr", "Log in by scanning a CAS QR code", func(ctx context.Context, t Target) error {
		return a.qr(ctx, t, qrTimeout)
	})
	if local {
		var timeout time.Duration
		qr.Args = positionalFree
		qr.RunE = func(cmd *cobra.Command, _ []string) error {
			return a.qr(cmd.Context(), target(), timeout)
		}
		qr.Flags().DurationVar(&timeout, "timeout", qrTimeout, "how long to wait for the scan")
	}

	var kickID string
	var kickIndex int
	kick := &cobra.Command{
		Use:   "kick",
		Short: "End one online session",
		Args:  positionalFree,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.kick(cmd.Context(), target(), kickID, kickIndex)
		},
	}
	kick.Flags().StringVar(&kickID, "id", "", "online session id")
	kick.Flags().IntVar(&kickIndex, "index", 0, fmt.Sprintf("1-based online device index from `%s info`", invocationOf(local)))

	return []*cobra.Command{
		simple("status", "Show the authentication state", a.status),
		simple("info", "Show status, online devices, and quick-login devices", a.info),
		login,
		qr,
		simple("logout", "Log out this address", a.logout),
		kick,
		a.quickCommand(target, local),
	}
}

func (a *App) quickCommand(target func() Target, local bool) *cobra.Command {
	prefix := prefixOf(local)
	quick := &cobra.Command{
		Use:   "quick",
		Short: "List, add, or delete quick-login devices",
		Args:  cobra.ArbitraryArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 {
				return usage.Errorf("usage: %s quick [list|add|delete]", invocationOf(local))
			}
			return usage.Errorf("unknown %squick operation: %s", prefix, args[0])
		},
	}
	quick.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List quick-login devices",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 0 {
				return usage.Errorf("%squick list does not accept arguments", prefix)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.quickList(cmd.Context(), target())
		},
	})
	var name string
	add := &cobra.Command{
		Use:   "add",
		Short: "Bind this device for quick login",
		Args:  positionalFree,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.quickAdd(cmd.Context(), target(), name)
		},
	}
	add.Flags().StringVar(&name, "name", "", "quick-login device name (required)")
	var id string
	del := &cobra.Command{
		Use:   "delete",
		Short: "Remove one quick-login binding",
		Args:  positionalFree,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.quickDelete(cmd.Context(), target(), id)
		},
	}
	del.Flags().StringVar(&id, "id", "", "quick-login device id from `quick list` (required)")
	quick.AddCommand(add, del)
	return quick
}

func (a *App) remoteCommand() *cobra.Command {
	settings := defaultRemoteSettings(a.portalURL())
	cmd := &cobra.Command{
		Use:   "remote [flags] <ssh-host> <operation> [args]",
		Short: "Run an operation for an SSH host through an ssh -W tunnel",
		Long: `Run an operation as the given SSH host. Each connection is an
"ssh -T -W <portal-host>:<port> <ssh-host>" byte tunnel; HTTP, TLS, cookies,
and QR rendering stay in this process. Remote flags precede the host.

Operations: status, info, login, qr, logout, kick, quick list|add|delete.`,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) < 2 {
				return usage.Errorf("usage: njulogin remote [flags] <ssh-host> <operation> [args]")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := sshtunnel.ValidateHost(args[0]); err != nil {
				return err
			}
			if err := settings.validate(); err != nil {
				return err
			}
			if settings.Insecure {
				fmt.Fprintln(a.Stderr, "warning: TLS verification is disabled for locally processed tunneled requests")
			}
			target := SSH{Host: args[0], Settings: settings}
			ops := &cobra.Command{
				Use: "njulogin remote <ssh-host>",
				Args: func(_ *cobra.Command, args []string) error {
					if len(args) != 0 {
						return usage.Errorf("unknown remote operation: %s", args[0])
					}
					return nil
				},
				RunE: func(*cobra.Command, []string) error {
					return usage.Errorf("usage: njulogin remote [flags] <ssh-host> <operation> [args]")
				},
			}
			a.configure(ops)
			ops.CompletionOptions.DisableDefaultCmd = true
			ops.AddCommand(a.operationCommands(func() Target { return target }, false)...)
			ops.SetArgs(args[1:])
			return ops.ExecuteContext(cmd.Context())
		},
	}
	// Everything after the host belongs to the operation.
	cmd.Flags().SetInterspersed(false)
	cmd.Flags().StringVar(&settings.BaseURL, "base-url", settings.BaseURL, "portal base URL reached through SSH")
	cmd.Flags().BoolVar(&settings.Insecure, "insecure", settings.Insecure, "disable local TLS verification for tunneled requests")
	cmd.Flags().IntVar(&settings.ConnectTimeout, "connect-timeout", settings.ConnectTimeout, "SSH connect timeout in seconds (1-300)")
	cmd.Flags().IntVar(&settings.MaxTime, "max-time", settings.MaxTime, "per-request timeout in seconds (1-3600)")
	return cmd
}

func prefixOf(local bool) string {
	if local {
		return Local{}.Prefix()
	}
	return SSH{}.Prefix()
}

func invocationOf(local bool) string {
	if local {
		return Local{}.Invocation()
	}
	return SSH{}.Invocation()
}

func noArgs(name string) cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) error {
		if len(args) != 0 {
			return usage.Errorf("%s does not accept arguments", name)
		}
		return nil
	}
}

func positionalFree(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return usage.Errorf("unexpected positional arguments for %s: %s", strings.TrimPrefix(cmd.CommandPath(), "njulogin "), strings.Join(args, " "))
	}
	return nil
}

func (a *App) isTerminal() bool {
	if !term.IsTerminal(int(a.Stdin.Fd())) {
		return false
	}
	type fder interface{ Fd() uintptr }
	out, ok := a.Stdout.(fder)
	return ok && term.IsTerminal(int(out.Fd()))
}
