package cli

import (
	"fmt"

	"github.com/soundadam/njulogin/internal/credential"
)

// promptLine asks on standard error and reads one line from standard input.
func (a *App) promptLine(prompt string) (string, error) {
	fmt.Fprint(a.Stderr, prompt)
	return credential.ReadLine(a.input())
}
