//go:build windows

package panel

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// nativeRestart 写新程序与看守批处理，批处理在当前进程退出后替换文件并重新拉起。
func nativeRestart(exe string, data []byte, args []string) (string, error) {
	newPath := exe + ".new"
	if err := atomicWrite(newPath, data); err != nil {
		return "", err
	}
	script := exe + ".update.bat"
	pid := strconv.Itoa(os.Getpid())
	var builder strings.Builder
	builder.WriteString("@echo off\r\n")
	builder.WriteString(":wait\r\n")
	fmt.Fprintf(&builder, "tasklist /FI \"PID eq %s\" /FO CSV /NH 2>nul | find \"%s\" >nul\r\n", pid, pid)
	builder.WriteString("if not errorlevel 1 (\r\n  ping -n 2 127.0.0.1 >nul\r\n  goto wait\r\n)\r\n")
	fmt.Fprintf(&builder, "if exist %s del /f /q %s\r\n", batQuote(exe+".previous"), batQuote(exe+".previous"))
	fmt.Fprintf(&builder, "move /y %s %s >nul\r\n", batQuote(exe), batQuote(exe+".previous"))
	fmt.Fprintf(&builder, "move /y %s %s >nul\r\n", batQuote(newPath), batQuote(exe))
	builder.WriteString("start \"\" " + batQuote(exe))
	for _, arg := range args {
		builder.WriteString(" " + batQuote(arg))
	}
	builder.WriteString("\r\n")
	builder.WriteString("del %~f0\r\n")
	if err := os.WriteFile(script, []byte(builder.String()), 0644); err != nil {
		return "", err
	}
	command := exec.Command("cmd.exe", "/C", script)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	command.Stdout = nil
	command.Stderr = nil
	if err := command.Start(); err != nil {
		return "", err
	}
	return "已生成重启看守脚本，服务退出后自动替换程序并重新拉起", nil
}

// inPlaceReplace 在 Windows 上无法替换正在运行的程序，容器同样不走这条路。
func inPlaceReplace(exe string, data []byte) error {
	return fmt.Errorf("Windows 不支持就地替换正在运行的程序")
}

// batQuote 用双引号包裹参数，供批处理 start 命令安全传递含空格的路径。
func batQuote(value string) string {
	return "\"" + value + "\""
}
