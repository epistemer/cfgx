//go:build unix

package loader

import (
	"os/exec"
	"syscall"
	"time"
)

// prepare runs the command in its own process group. On timeout the whole
// group gets SIGQUIT, so the schema program dumps its goroutine stacks, which
// show the init that hung; anything still running is killed shortly after.
func prepare(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGQUIT)
	}
	cmd.WaitDelay = 5 * time.Second
}
