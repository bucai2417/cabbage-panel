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

// terminateProcess 立即杀掉进程树。
// 先同步 Kill 主进程（让 context 尽快返回），再异步用 taskkill /T 清理子进程，
// 避免 taskkill 的启动开销拖慢停止响应。
func terminateProcess(command *exec.Cmd) error {
	if command.Process == nil {
		return nil
	}
	pid := command.Process.Pid
	// 同步强杀主进程，保证 Cancel 立刻生效。
	_ = command.Process.Kill()
	// 后台清理整棵进程树（子进程可能还在），不阻塞调用方。
	go func() {
		killer := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid))
		killer.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		_ = killer.Run()
	}()
	return nil
}
