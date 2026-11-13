//go:build !windows

package panel

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// nativeRestart 写新程序与守护脚本，脚本在当前进程退出后替换文件并重新拉起。
func nativeRestart(exe string, data []byte, args []string) (string, error) {
	newPath := exe + ".new"
	if err := atomicWrite(newPath, data); err != nil {
		return "", err
	}
	if err := os.Chmod(newPath, 0755); err != nil {
		return "", err
	}
	script := exe + ".update.sh"
	var builder strings.Builder
	builder.WriteString("#!/bin/sh\n")
	fmt.Fprintf(&builder, "while kill -0 %d 2>/dev/null; do sleep 0.3; done\n", os.Getpid())
	fmt.Fprintf(&builder, "rm -f %s\n", shellQuote(exe+".previous"))
	fmt.Fprintf(&builder, "mv -f %s %s\n", shellQuote(exe), shellQuote(exe+".previous"))
	fmt.Fprintf(&builder, "mv -f %s %s\n", shellQuote(newPath), shellQuote(exe))
	builder.WriteString("rm -f -- \"$0\"\n")
	builder.WriteString("exec " + shellQuote(exe))
	for _, arg := range args {
		builder.WriteString(" " + shellQuote(arg))
	}
	builder.WriteString("\n")
	if err := os.WriteFile(script, []byte(builder.String()), 0755); err != nil {
		return "", err
	}
	command := exec.Command("/bin/sh", script)
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	command.Stdout = nil
	command.Stderr = nil
	if err := command.Start(); err != nil {
		return "", err
	}
	return "已生成重启守护脚本，服务退出后自动替换程序并重新拉起", nil
}

// inPlaceReplace 适用于容器（Linux）环境：直接改名为运行中的可执行文件是允许的。
func inPlaceReplace(exe string, data []byte) error {
	incoming := exe + ".incoming"
	if err := atomicWrite(incoming, data); err != nil {
		return err
	}
	if err := os.Chmod(incoming, 0755); err != nil {
		return err
	}
	previous := exe + ".previous"
	_ = os.Remove(previous)
	if err := os.Rename(exe, previous); err != nil {
		return err
	}
	if err := os.Rename(incoming, exe); err != nil {
		_ = os.Rename(previous, exe)
		return err
	}
	return nil
}
