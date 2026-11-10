//go:build windows

package panel

import (
	"os/exec"
	"strconv"
	"syscall"
)

func configureProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x00000200}
}
func terminateProcess(command *exec.Cmd) error {
	killer := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(command.Process.Pid))
	killer.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := killer.Run(); err != nil {
		return command.Process.Kill()
	}
	return nil
}
