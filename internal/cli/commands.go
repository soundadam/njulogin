package cli

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/soundadam/njulogin/internal/sshtunnel"
	"github.com/soundadam/njulogin/internal/usage"
	"golang.org/x/term"
)

const qrTimeout = 60 * time.Second

const usageText = `Usage:
  njulogin setup [--username USER]
  njulogin status
  njulogin info
  njulogin login [--username USER] [--password-command CMD|--password-stdin|--ask] [--add-quick NAME]
  njulogin qr
  njulogin logout
  njulogin kick --id ID|--index N
  njulogin quick list
  njulogin quick add --name NAME
  njulogin quick delete --id ID
  njulogin remote [flags] <ssh-host> <operation>
  njulogin version`

func (a *App) usage() {
	fmt.Fprintln(a.Stderr, usageText)
}

// Run dispatches one invocation. With no arguments on a terminal it starts the
// interactive menu.
func (a *App) Run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		if !a.isTerminal() {
			a.usage()
			return usage.Errorf("interactive mode requires a terminal")
		}
		return a.interactive(ctx)
	}
	local := Local{BaseURL: a.portalURL()}
	command, rest := args[0], args[1:]
	switch command {
	case "setup":
		flags := a.flagSet("setup")
		username := flags.String("username", "", "NJU username")
		if err := parse(flags, rest); err != nil {
			return err
		}
		return a.setup(ctx, *username)
	case "status", "info", "logout":
		if err := requireNoArgs(command, rest); err != nil {
			return err
		}
		return a.simple(ctx, local, command)
	case "login", "qr", "kick", "quick":
		return a.operation(ctx, local, command, rest)
	case "remote":
		return a.remote(ctx, rest)
	case "version", "--version":
		if err := requireNoArgs("version", rest); err != nil {
			return err
		}
		fmt.Fprintf(a.Stdout, "njulogin %s\n", a.effectiveVersion())
		return nil
	case "-h", "--help", "help":
		if err := requireNoArgs("help", rest); err != nil {
			return err
		}
		a.usage()
		return nil
	default:
		return usage.Errorf("unknown command: %s", command)
	}
}

func (a *App) remote(ctx context.Context, args []string) error {
	flags := a.flagSet("remote")
	settings := defaultRemoteSettings(a.portalURL())
	flags.StringVar(&settings.BaseURL, "base-url", settings.BaseURL, "portal base URL reached through SSH")
	flags.BoolVar(&settings.Insecure, "insecure", settings.Insecure, "disable local TLS verification for tunneled requests")
	flags.IntVar(&settings.ConnectTimeout, "connect-timeout", settings.ConnectTimeout, "SSH connect timeout")
	flags.IntVar(&settings.MaxTime, "max-time", settings.MaxTime, "per-request timeout")
	_ = flags.Parse(args)
	rest := flags.Args()
	if len(rest) < 2 {
		return usage.Errorf("usage: njulogin remote [flags] <ssh-host> <operation> [args]")
	}
	target := SSH{Host: rest[0], Settings: settings}
	if err := sshtunnel.ValidateHost(target.Host); err != nil {
		return err
	}
	if err := settings.validate(); err != nil {
		return err
	}
	if settings.Insecure {
		fmt.Fprintln(a.Stderr, "warning: TLS verification is disabled for locally processed tunneled requests")
	}
	op, opArgs := rest[1], rest[2:]
	switch op {
	case "status", "info", "logout", "qr":
		if err := requireNoArgs("remote "+op, opArgs); err != nil {
			return err
		}
		return a.simple(ctx, target, op)
	case "login", "kick", "quick":
		return a.operation(ctx, target, op, opArgs)
	default:
		return usage.Errorf("unknown remote operation: %s", op)
	}
}

// simple runs an operation that takes no arguments.
func (a *App) simple(ctx context.Context, target Target, op string) error {
	switch op {
	case "status":
		return a.status(ctx, target)
	case "info":
		return a.info(ctx, target)
	case "logout":
		return a.logout(ctx, target)
	case "qr":
		return a.qr(ctx, target, qrTimeout)
	}
	return fmt.Errorf("unsupported %soperation: %s", target.Prefix(), op)
}

// operation parses the flags of an operation shared by local and remote use.
func (a *App) operation(ctx context.Context, target Target, op string, args []string) error {
	prefix := target.Prefix()
	switch op {
	case "login":
		flags := a.flagSet(prefix + "login")
		var opts loginOptions
		flags.StringVar(&opts.Username, "username", "", "NJU username")
		flags.StringVar(&opts.PasswordCommand, "password-command", "", "command that prints the credential to stdout")
		flags.BoolVar(&opts.Ask, "ask", false, "read credential interactively without echo")
		flags.BoolVar(&opts.Stdin, "password-stdin", false, "read credential from stdin")
		flags.StringVar(&opts.AddQuick, "add-quick", "", "add current device as a quick-login device after login")
		if err := parse(flags, args); err != nil {
			return err
		}
		return a.login(ctx, target, opts)
	case "qr":
		flags := a.flagSet("qr")
		timeout := flags.Duration("timeout", qrTimeout, "QR login timeout")
		if err := parse(flags, args); err != nil {
			return err
		}
		return a.qr(ctx, target, *timeout)
	case "kick":
		flags := a.flagSet(prefix + "kick")
		id := flags.String("id", "", "online device id")
		index := flags.Int("index", 0, fmt.Sprintf("1-based online device index from `%s info`", target.Invocation()))
		if err := parse(flags, args); err != nil {
			return err
		}
		return a.kick(ctx, target, *id, *index)
	case "quick":
		return a.quick(ctx, target, args)
	}
	return fmt.Errorf("unsupported %soperation: %s", prefix, op)
}

func (a *App) quick(ctx context.Context, target Target, args []string) error {
	prefix := target.Prefix()
	if len(args) == 0 {
		return usage.Errorf("usage: %s quick [list|add|delete]", target.Invocation())
	}
	switch args[0] {
	case "list":
		if len(args) != 1 {
			return usage.Errorf("%squick list does not accept arguments", prefix)
		}
		return a.quickList(ctx, target)
	case "add":
		flags := a.flagSet(prefix + "quick add")
		name := flags.String("name", "", "quick-login device name")
		if err := parse(flags, args[1:]); err != nil {
			return err
		}
		return a.quickAdd(ctx, target, *name)
	case "delete":
		flags := a.flagSet(prefix + "quick delete")
		id := flags.String("id", "", "quick-login device id")
		if err := parse(flags, args[1:]); err != nil {
			return err
		}
		return a.quickDelete(ctx, target, *id)
	default:
		return usage.Errorf("unknown %squick operation: %s", prefix, args[0])
	}
}

func (a *App) flagSet(name string) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ExitOnError)
	flags.SetOutput(a.Stderr)
	return flags
}

// parse rejects positional arguments after the flags.
func parse(flags *flag.FlagSet, args []string) error {
	_ = flags.Parse(args)
	if flags.NArg() != 0 {
		return usage.Errorf("unexpected positional arguments for %s: %s", flags.Name(), strings.Join(flags.Args(), " "))
	}
	return nil
}

func requireNoArgs(command string, args []string) error {
	if len(args) != 0 {
		return usage.Errorf("%s does not accept arguments", command)
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
