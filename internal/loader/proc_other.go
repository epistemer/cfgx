//go:build !unix

package loader

import (
	"os/exec"
	"time"
)

// prepare makes sure a timed-out run is killed and its output collected.
func prepare(cmd *exec.Cmd) {
	cmd.WaitDelay = 5 * time.Second
}
