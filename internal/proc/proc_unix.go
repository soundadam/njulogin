//go:build darwin || linux

package proc

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// Command starts name in its own process group and kills the whole group when
// ctx ends, so helpers spawned by the child cannot outlive it.
func Command(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	return cmd
}
