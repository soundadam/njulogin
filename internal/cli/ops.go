package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/soundadam/njulogin/internal/cas"
	"github.com/soundadam/njulogin/internal/credential"
	"github.com/soundadam/njulogin/internal/portal"
	"github.com/soundadam/njulogin/internal/redact"
	"github.com/soundadam/njulogin/internal/usage"
)

// Operations run against any Target. Flag parsing lives in commands.go.

func (a *App) status(ctx context.Context, target Target) error {
	api, _, err := newAPI(target)
	if err != nil {
		return err
	}
	status, err := api.Status(ctx)
	if err != nil {
		return err
	}
	writeStatus(a.Stdout, status)
	return nil
}

func (a *App) info(ctx context.Context, target Target) error {
	api, _, err := newAPI(target)
	if err != nil {
		return err
	}
	status, err := api.Status(ctx)
	if err != nil {
		return err
	}
	writeStatus(a.Stdout, status)
	fmt.Fprintln(a.Stdout)
	online, err := api.OnlineDevices(ctx)
	if err != nil {
		return err
	}
	writeOnlineRows(a.Stdout, online)
	fmt.Fprintln(a.Stdout)
	quick, err := api.QuickDevices(ctx)
	if err != nil {
		return err
	}
	writeQuickRows(a.Stdout, quick)
	return nil
}

// loginOptions are the credential flags shared by login and remote login.
type loginOptions struct {
	Username        string
	PasswordCommand string
	Ask             bool
	Stdin           bool
	AddQuick        string
}

func (a *App) login(ctx context.Context, target Target, opts loginOptions) error {
	username, secret, err := a.resolveCredential(ctx, opts)
	if err != nil {
		return err
	}
	if strings.TrimSpace(secret) == "" {
		return usage.Errorf("empty password")
	}
	api, _, err := newAPI(target)
	if err != nil {
		return err
	}
	if err := api.Login(ctx, username, secret); err != nil {
		return err
	}
	status, err := a.Poll.AuthState(ctx, api, true)
	if err != nil {
		return err
	}
	if opts.AddQuick != "" {
		if err := a.bindQuick(ctx, target, api, opts.AddQuick); err != nil {
			return err
		}
	}
	writeStatus(a.Stdout, status)
	if opts.AddQuick != "" {
		fmt.Fprintln(a.Stdout, "quick_add_result=success")
	}
	return nil
}

func (a *App) logout(ctx context.Context, target Target) error {
	api, _, err := newAPI(target)
	if err != nil {
		return err
	}
	if err := api.Logout(ctx); err != nil {
		return err
	}
	status, err := a.Poll.AuthState(ctx, api, false)
	if err != nil {
		return err
	}
	writeStatus(a.Stdout, status)
	return nil
}

func (a *App) kick(ctx context.Context, target Target, id string, index int) error {
	if id != "" && index != 0 {
		return usage.Errorf("choose only one of --id or --index")
	}
	if id == "" && index == 0 {
		return usage.Errorf("missing --id or --index")
	}
	api, _, err := newAPI(target)
	if err != nil {
		return err
	}
	if index != 0 {
		rows, err := api.OnlineDevices(ctx)
		if err != nil {
			return err
		}
		if id, err = onlineIDAtIndex(rows, index); err != nil {
			return err
		}
	}
	if err := api.Kick(ctx, id); err != nil {
		return err
	}
	if err := a.Poll.Rows(ctx, api.OnlineDevices, portal.IDAbsent(id),
		target.Prefix()+"online-session target remained present before timeout"); err != nil {
		return err
	}
	fmt.Fprintln(a.Stdout, "result=success")
	return nil
}

func onlineIDAtIndex(rows []portal.Row, index int) (string, error) {
	if index <= 0 {
		return "", usage.Errorf("--index must be >= 1")
	}
	if index > len(rows) {
		return "", usage.Errorf("--index %d out of range; online_devices=%d", index, len(rows))
	}
	id := rows[index-1].ID()
	if id == "" || id == "<nil>" {
		return "", usage.Errorf("online device %d has no usable id", index)
	}
	return id, nil
}

func (a *App) quickList(ctx context.Context, target Target) error {
	api, _, err := newAPI(target)
	if err != nil {
		return err
	}
	rows, err := api.QuickDevices(ctx)
	if err != nil {
		return err
	}
	writeQuickRows(a.Stdout, rows)
	return nil
}

func (a *App) quickAdd(ctx context.Context, target Target, name string) error {
	if name == "" {
		return usage.Errorf("missing --name")
	}
	api, _, err := newAPI(target)
	if err != nil {
		return err
	}
	if err := a.bindQuick(ctx, target, api, name); err != nil {
		return err
	}
	fmt.Fprintln(a.Stdout, "result=success")
	return nil
}

func (a *App) bindQuick(ctx context.Context, target Target, api portal.API, name string) error {
	if err := api.BindQuick(ctx, name); err != nil {
		return err
	}
	return a.Poll.Rows(ctx, api.QuickDevices, portal.DevicePresent(name),
		target.Prefix()+"quick-login binding was not observed before timeout")
}

func (a *App) quickDelete(ctx context.Context, target Target, id string) error {
	if id == "" {
		return usage.Errorf("missing --id")
	}
	api, _, err := newAPI(target)
	if err != nil {
		return err
	}
	if err := api.UnbindQuick(ctx, id); err != nil {
		return err
	}
	if err := a.Poll.Rows(ctx, api.QuickDevices, portal.IDAbsent(id),
		target.Prefix()+"quick-login binding remained present before timeout"); err != nil {
		return err
	}
	fmt.Fprintln(a.Stdout, "result=success")
	return nil
}

func (a *App) qr(ctx context.Context, target Target, timeout time.Duration) error {
	if timeout <= 0 {
		return usage.Errorf("--timeout must be positive")
	}
	api, client, err := newAPI(target)
	if err != nil {
		return err
	}
	login := cas.QRLogin{
		HTTP:         client,
		Endpoints:    a.CAS,
		Renderer:     a.QRRenderer,
		Out:          a.Stdout,
		Timeout:      timeout,
		PollInterval: a.QRPollInterval,
	}
	if err := login.Run(ctx); err != nil {
		return err
	}
	status, err := a.Poll.AuthState(ctx, api, true)
	if err != nil {
		return err
	}
	writeStatus(a.Stdout, status)
	return nil
}

func (a *App) setup(ctx context.Context, username string) error {
	if username == "" {
		value, err := a.promptLine("NJU username: ")
		if err != nil {
			return err
		}
		username = value
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return usage.Errorf("empty username")
	}
	secret, err := a.setupSecret().Secret(ctx)
	if err != nil {
		return err
	}
	if strings.TrimSpace(secret) == "" {
		return usage.Errorf("empty password")
	}
	store, err := a.store()
	if err != nil {
		return err
	}
	if err := store.Save(credential.Credential{Username: username, Password: secret}); err != nil {
		return err
	}
	fmt.Fprintln(a.Stdout, "state=configured")
	fmt.Fprintf(a.Stdout, "account=%s\n", redact.Account(username))
	fmt.Fprintln(a.Stdout, "credential_store=local_file")
	fmt.Fprintf(a.Stdout, "config_path=%s\n", redact.Field(store.Path, "unknown"))
	return nil
}

// resolveCredential picks at most one explicit password source; without one it
// uses the saved credential, which must belong to the requested username.
func (a *App) resolveCredential(ctx context.Context, opts loginOptions) (string, string, error) {
	var explicit []credential.Source
	if opts.PasswordCommand != "" {
		explicit = append(explicit, credential.Command{Script: opts.PasswordCommand, Timeout: a.PasswordCommandTimeout})
	}
	if opts.Ask {
		explicit = append(explicit, credential.Terminal{In: a.Stdin, Prompt: a.Stderr, Label: "NJU credential: "})
	}
	if opts.Stdin {
		explicit = append(explicit, credential.Line{Reader: a.input()})
	}
	if len(explicit) > 1 {
		return "", "", usage.Errorf("credential sources are mutually exclusive: choose one of --password-command, --password-stdin, or --ask")
	}
	username := opts.Username
	var saved *credential.Credential
	if username == "" || len(explicit) == 0 {
		store, err := a.store()
		if err != nil {
			return "", "", err
		}
		if saved, err = store.Load(); err != nil {
			return "", "", err
		}
	}
	if username == "" && saved != nil {
		username = saved.Username
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return "", "", usage.Errorf("missing username: run `njulogin setup --username <username>` first or pass --username")
	}
	if len(explicit) == 1 {
		secret, err := explicit[0].Secret(ctx)
		return username, secret, err
	}
	if saved == nil {
		return "", "", usage.Errorf("no saved credential: run `njulogin setup` or choose --password-command, --password-stdin, or --ask")
	}
	if saved.Username != username {
		return "", "", usage.Errorf("saved credential belongs to a different username; run `njulogin setup` or choose an explicit credential source")
	}
	if strings.TrimSpace(saved.Password) == "" {
		return "", "", errors.New("saved credential is empty")
	}
	return username, saved.Password, nil
}
