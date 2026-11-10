//go:build !windows

package panel

import (
	"os/exec"
	"syscall"
)

func configureProcess(command *exec.Cmd) { command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }
func terminateProcess(command *exec.Cmd) error {
	return syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
}
