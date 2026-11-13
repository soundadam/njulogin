// Package usage marks errors caused by invalid invocation. The CLI maps them to
// exit status 2; every other error exits 1.
package usage

import (
	"errors"
	"fmt"
)

// Error is an invocation error: a bad flag, argument, or configuration value
// that the operator can correct without contacting any service.
type Error struct {
	message string
}

func (err Error) Error() string {
	return err.message
}

// Errorf formats an invocation error.
func Errorf(format string, args ...any) error {
	return Error{message: fmt.Sprintf(format, args...)}
}

// Is reports whether err, or an error it wraps, is an invocation error.
func Is(err error) bool {
	var target Error
	return errors.As(err, &target)
}
