package panel

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// pythonVersionPattern 从 `python --version` 输出里提取版本号。
var pythonVersionPattern = regexp.MustCompile(`(\d+\.\d+(?:\.\d+)?)`)

// pythonVersion 探测解释器版本，3 秒超时；失败返回空串。
func pythonVersion(ctx context.Context, program string) string {
	probe, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	output, err := exec.CommandContext(probe, program, "--version").CombinedOutput()
	if err != nil {
		return ""
	}
	if match := pythonVersionPattern.FindStringSubmatch(string(output)); len(match) > 1 {
		return match[1]
	}
	return ""
}

// pythonRebuildAPI 用当前配置的基础 Python 重建 venv，并重装所有 Python 依赖。
// 必须在持有 gate 读锁之前调用（由 Handler 分发），以便独占写锁。
func (app *App) pythonRebuildAPI(writer http.ResponseWriter, request *http.Request) {
	if request.Method != "POST" {
		fail(writer, 405, "方法不支持")
		return
	}
	if app.engine.active() > 0 {
		fail(writer, 409, "请先停止运行中的任务并等待队列清空")
		return
	}
	if !app.gate.TryLock() {
		fail(writer, 409, "有任务正在执行，请稍后重试")
		return
	}
	var log strings.Builder
	result, err := app.rebuildPythonEnv(request.Context(), &log)
	app.gate.Unlock()
	if err != nil {
		fail(writer, 400, err.Error())
		return
	}
	// 重建成功后重装所有 Python 依赖（type=1）。
	reinstalled := []int64{}
	if items, listErr := app.store.list("dependencies"); listErr == nil {
		for _, item := range items {
			if integer(item["type"]) != 1 {
				continue
			}
			id := integer(item["id"])
			if _, err := app.store.save("dependencies", Object{"id": id, "remove": false, "status": 1}); err != nil {
				continue
			}
			if _, err := app.engine.enqueue("dependencies", id); err != nil {
				_, _ = app.store.save("dependencies", Object{"id": id, "status": 2})
				continue
			}
			reinstalled = append(reinstalled, id)
		}
	}
	result["reinstalled"] = len(reinstalled)
	result["reinstalledIds"] = reinstalled
	result["output"] = log.String()
	ok(writer, result)
}

// rebuildPythonEnv 删除旧 venv 并用 resolvePythonBase() 重新创建。
// 调用方必须已持有 gate 写锁且确认没有运行中的任务。
func (app *App) rebuildPythonEnv(ctx context.Context, writer *strings.Builder) (Object, error) {
	base := app.resolvePythonBase()
	version := pythonVersion(ctx, base)
	if version == "" {
		return nil, fmt.Errorf("Python 解释器不可用: %s", base)
	}
	venv := filepath.Join(app.root, "packages", "venv")
	// 先备份旧环境，失败可回滚；Windows 上目录被占用时 Rename 会失败。
	backup := venv + ".bak-" + fmt.Sprint(time.Now().Unix())
	hasOld := false
	if _, err := os.Stat(venv); err == nil {
		if err := os.Rename(venv, backup); err != nil {
			return nil, fmt.Errorf("旧环境被占用，请先停止正在运行的脚本: %w", err)
		}
		hasOld = true
	}
	if err := app.process(ctx, app.root, writer, base, "-m", "venv", venv); err != nil {
		if hasOld {
			_ = os.RemoveAll(venv)
			_ = os.Rename(backup, venv)
		}
		return nil, err
	}
	_ = os.RemoveAll(backup)
	return Object{"python": base, "version": version, "path": venv}, nil
}

// cleanupPythonBackups 清扫上次重建遗留的 venv.bak-* 目录（进程崩溃或文件占用时残留）。
func (app *App) cleanupPythonBackups() {
	pattern := filepath.Join(app.root, "packages", "venv.bak-*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return
	}
	for _, target := range matches {
		_ = os.RemoveAll(target)
	}
}
