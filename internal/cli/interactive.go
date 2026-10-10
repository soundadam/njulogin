package cli

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/soundadam/njulogin/internal/credential"
	"github.com/soundadam/njulogin/internal/usage"
)

type menuOption struct {
	Key   string
	Label string
}

func (a *App) interactive(ctx context.Context) error {
	if !a.isTerminal() {
		return errors.New("interactive mode requires a terminal")
	}
	choice, err := a.promptMenuChoice("Target", []menuOption{
		{Key: "1", Label: "local"},
		{Key: "2", Label: "ssh"},
	}, "1")
	if err != nil {
		return err
	}
	var target Target = Local{BaseURL: a.portalURL()}
	if choice == "2" {
		host, err := a.promptRequired("SSH host: ")
		if err != nil {
			return err
		}
		target = SSH{Host: host, Settings: defaultRemoteSettings(a.portalURL())}
	}
	action, err := a.promptMenuChoice("Action", []menuOption{
		{Key: "1", Label: "password-login"},
		{Key: "2", Label: "qr-login"},
		{Key: "3", Label: "info"},
		{Key: "4", Label: "logout"},
		{Key: "5", Label: "kick-online"},
		{Key: "6", Label: "update-credential-and-login"},
		{Key: "7", Label: "password-login-and-bind-quick"},
	}, "1")
	if err != nil {
		return err
	}
	if action == "6" {
		if err := a.setup(ctx, ""); err != nil {
			return err
		}
		action = "1"
	}
	return a.runAction(ctx, target, action)
}

func (a *App) runAction(ctx context.Context, target Target, action string) error {
	switch action {
	case "1":
		return a.login(ctx, target, loginOptions{})
	case "2":
		return a.qr(ctx, target, qrTimeout)
	case "3":
		return a.info(ctx, target)
	case "4":
		return a.logout(ctx, target)
	case "5":
		if err := a.info(ctx, target); err != nil {
			return err
		}
		index, err := a.promptRequired("Online device index to kick: ")
		if err != nil {
			return err
		}
		number, err := strconv.Atoi(index)
		if err != nil {
			return usage.Errorf("invalid --index %q", index)
		}
		return a.kick(ctx, target, "", number)
	case "7":
		name, err := a.promptQuickLoginName(target.DeviceName())
		if err != nil {
			return err
		}
		return a.login(ctx, target, loginOptions{AddQuick: name})
	default:
		kind := "local"
		if target.Prefix() != "" {
			kind = "remote"
		}
		return fmt.Errorf("unsupported %s action: %s", kind, action)
	}
}

func (a *App) promptLine(prompt string) (string, error) {
	fmt.Fprint(a.Stderr, prompt)
	return credential.ReadLine(a.input())
}

func (a *App) promptRequired(prompt string) (string, error) {
	value, err := a.promptLine(prompt)
	if err != nil {
		return "", err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("empty input")
	}
	return value, nil
}

func (a *App) promptMenuChoice(prompt string, options []menuOption, defaultKey string) (string, error) {
	labels := make([]string, 0, len(options))
	for _, option := range options {
		labels = append(labels, option.Key+"="+option.Label)
	}
	value, err := a.promptLine(fmt.Sprintf("%s [%s] (default %s): ", prompt, strings.Join(labels, ", "), defaultKey))
	if err != nil {
		return "", err
	}
	return resolveMenuChoice(value, defaultKey, options)
}

func resolveMenuChoice(input, defaultKey string, options []menuOption) (string, error) {
	value := strings.TrimSpace(input)
	if value == "" {
		value = defaultKey
	}
	for _, option := range options {
		if value == option.Key {
			return option.Key, nil
		}
	}
	return "", fmt.Errorf("unknown choice: %s", value)
}

func (a *App) promptQuickLoginName(defaultValue string) (string, error) {
	if defaultValue == "" {
		defaultValue = "current-device"
	}
	value, err := a.promptLine(fmt.Sprintf("Quick-login device name (default %s): ", defaultValue))
	if err != nil {
		return "", err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultValue, nil
	}
	return value, nil
}
