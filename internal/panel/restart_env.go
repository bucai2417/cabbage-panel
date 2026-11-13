package panel

import (
	"os"
	"runtime"
	"strings"
)

// inContainer 尽力判断当前是否运行在容器中，用于选择更简单可靠的重启方式。
func inContainer() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	if _, err := os.Stat("/run/.containerenv"); err == nil {
		return true
	}
	if value := strings.ToLower(strings.TrimSpace(os.Getenv("container"))); value == "docker" || value == "podman" {
		return true
	}
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		return true
	}
	if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		text := string(data)
		if strings.Contains(text, "docker") || strings.Contains(text, "kubepods") || strings.Contains(text, "containerd") {
			return true
		}
	}
	return false
}

// restartMode 返回当前环境的重启策略，供前端展示与后端选择实现。
func restartMode() string {
	if inContainer() {
		return "container"
	}
	if underSystemd() {
		return "systemd"
	}
	if runtime.GOOS == "windows" {
		return "windows"
	}
	return "unix"
}

// underSystemd 判断是否由 systemd 托管：此时不能用看守脚本，因为 control-group
// 会在主进程退出时连带杀掉守卫；应就地替换后退出，交给 Restart=always 重新拉起。
func underSystemd() bool {
	if runtime.GOOS == "windows" {
		return false
	}
	data, err := os.ReadFile("/proc/1/comm")
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(data)) == "systemd"
}
