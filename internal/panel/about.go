package panel

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/host"
)

// aboutInfo 返回关于页需要的运行环境信息。
func (app *App) aboutInfo() Object {
	info := Object{
		"version":              app.version,
		"commit":               app.commit,
		"go":                   runtime.Version(),
		"os":                   runtime.GOOS,
		"arch":                 runtime.GOARCH,
		"platform":             runtime.GOOS + "/" + runtime.GOARCH,
		"restartMode":          restartMode(),
		"signedUpdates":        app.updateKey != "",
		"allowUnsignedUpdates": app.store.setting("allowUnsignedUpdates", "false") == "true",
	}
	if hostInfo, err := host.Info(); err == nil {
		name := strings.TrimSpace(strings.TrimSpace(hostInfo.Platform) + " " + strings.TrimSpace(hostInfo.PlatformVersion))
		if name == "" {
			name = runtime.GOOS
		}
		info["osName"] = name
		info["hostname"] = hostInfo.Hostname
		info["kernel"] = hostInfo.KernelVersion
		info["uptime"] = hostInfo.Uptime
	} else {
		info["osName"] = runtime.GOOS
	}
	return info
}

// restartAPI 重启面板服务本身：等待当前进程退出后由看守脚本重新拉起。
// 容器与 systemd 环境下直接退出，交给外部监管者重启。
func (app *App) restartAPI(writer http.ResponseWriter, request *http.Request) {
	if request.Method != "POST" {
		fail(writer, 405, "方法不支持")
		return
	}
	if app.engine.active() > 0 {
		fail(writer, 409, "请先停止运行中的任务，再重启面板")
		return
	}
	executable, err := os.Executable()
	if err != nil {
		apiError(writer, err)
		return
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		apiError(writer, err)
		return
	}
	message := "服务正在重启"
	if !inContainer() && !underSystemd() {
		if err := selfRestart(executable, os.Args[1:]); err != nil {
			apiError(writer, err)
			return
		}
		message = "已启动看守脚本，服务退出后自动重新拉起"
	}
	ok(writer, Object{"message": message})
	app.appendSystemLog("info", "[system] 已请求重启面板")
	app.scheduleShutdown(700 * time.Millisecond)
}
