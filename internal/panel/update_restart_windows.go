//go:build windows

package panel

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unicode/utf16"
)

// nativeRestart 用 PowerShell 看守脚本替换程序并重新拉起。
// 批处理在中文路径下会因编码问题找不到文件，PowerShell 编码命令传入 Unicode 路径可避免。
func nativeRestart(exe string, data []byte, args []string) (string, error) {
	newPath := exe + ".new"
	if err := atomicWrite(newPath, data); err != nil {
		return "", err
	}
	script := strings.Join([]string{
		fmt.Sprintf("while (Get-Process -Id %d -ErrorAction SilentlyContinue) { Start-Sleep -Milliseconds 200 }", os.Getpid()),
		"Remove-Item -Force -ErrorAction SilentlyContinue " + psQuote(exe+".previous"),
		"Move-Item -Force " + psQuote(exe) + " " + psQuote(exe+".previous"),
		"Move-Item -Force " + psQuote(newPath) + " " + psQuote(exe),
		"Start-Process -FilePath " + psQuote(exe) + argsArray(args),
	}, "; ")
	if err := runPowerShellDetached(script); err != nil {
		return "", err
	}
	return "已生成重启看守脚本，服务退出后自动替换程序并重新拉起", nil
}

// selfRestart 在当前进程退出后重新拉起同一个程序（不替换文件）。
func selfRestart(exe string, args []string) error {
	script := fmt.Sprintf("while (Get-Process -Id %d -ErrorAction SilentlyContinue) { Start-Sleep -Milliseconds 200 }; Start-Process -FilePath %s%s",
		os.Getpid(), psQuote(exe), argsArray(args))
	return runPowerShellDetached(script)
}

// argsArray 把参数拼成 PowerShell 的 -ArgumentList @('a','b')；无参数时返回空串。
func argsArray(args []string) string {
	if len(args) == 0 {
		return ""
	}
	quoted := make([]string, len(args))
	for index, arg := range args {
		quoted[index] = psQuote(arg)
	}
	return " -ArgumentList @(" + strings.Join(quoted, ",") + ")"
}

// psQuote 用单引号包裹，内部单引号翻倍转义。
func psQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

// runPowerShellDetached 以 UTF-16LE Base64 编码命令启动 PowerShell，绕开控制台编码问题。
func runPowerShellDetached(script string) error {
	command := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-EncodedCommand", utf16LEBase64(script))
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	command.Stdout = nil
	command.Stderr = nil
	return command.Start()
}

func utf16LEBase64(value string) string {
	codes := utf16.Encode([]rune(value))
	buffer := make([]byte, len(codes)*2)
	for index, code := range codes {
		buffer[index*2] = byte(code)
		buffer[index*2+1] = byte(code >> 8)
	}
	return base64.StdEncoding.EncodeToString(buffer)
}

// inPlaceReplace 在 Windows 上无法替换正在运行的程序，容器同样不走这条路。
func inPlaceReplace(exe string, data []byte) error {
	return fmt.Errorf("Windows 不支持就地替换正在运行的程序")
}
