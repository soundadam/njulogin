package credential

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/soundadam/njulogin/internal/proc"
)

// Source supplies a password. Passwords never come from command-line values.
type Source interface {
	Secret(ctx context.Context) (string, error)
}

var (
	_ Source = Command{}
	_ Source = Line{}
	_ Source = Terminal{}
)

// Command runs a trusted local shell command and uses its stdout, without the
// trailing newline.
type Command struct {
	Script  string
	Timeout time.Duration
}

// Secret runs the command within Timeout.
func (c Command) Secret(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	output, err := proc.Command(ctx, "bash", "-lc", c.Script).Output()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "", errors.New("password command timed out")
	}
	if err != nil {
		return "", fmt.Errorf("password command failed: %w", err)
	}
	return strings.TrimRight(string(output), "\r\n"), nil
}

// Line reads one line, such as standard input.
type Line struct {
	Reader *bufio.Reader
}

// Secret reads up to the first newline or EOF.
func (l Line) Secret(context.Context) (string, error) {
	return ReadLine(l.Reader)
}

// ReadLine reads one line without its line terminator; EOF ends the line.
func ReadLine(reader *bufio.Reader) (string, error) {
	value, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimRight(value, "\r\n"), nil
}

// Terminal prompts on Prompt and reads without echo.
type Terminal struct {
	In     *os.File
	Prompt io.Writer
	Label  string
	// RequireTerminal refuses before prompting when In is not a terminal.
	RequireTerminal bool
}

// Secret prompts and reads one password.
func (t Terminal) Secret(context.Context) (string, error) {
	fd := int(t.In.Fd())
	if t.RequireTerminal && !term.IsTerminal(fd) {
		return "", errors.New("interactive password prompt requires a terminal")
	}
	fmt.Fprint(t.Prompt, t.Label)
	data, err := term.ReadPassword(fd)
	fmt.Fprintln(t.Prompt)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
