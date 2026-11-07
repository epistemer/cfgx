//go:build unix

package loader

import (
	"os/exec"
	"syscall"
	"time"
)

// prepare makes a timeout send SIGQUIT to the schema program, which then
// dumps its goroutine stacks: they show the init that hung. If it is still
// running after WaitDelay, os/exec kills it.
func prepare(cmd *exec.Cmd) {
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGQUIT) }
	cmd.WaitDelay = 5 * time.Second
}
