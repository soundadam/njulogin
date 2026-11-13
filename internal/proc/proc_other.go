//go:build !darwin && !linux

package proc

import (
	"context"
	"os/exec"
)

// Command starts name and kills it when ctx ends.
func Command(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}
