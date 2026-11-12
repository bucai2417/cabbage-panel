package panel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
	"golang.org/x/crypto/bcrypt"
)

func verifyPassword(hashed, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password))
}

func (app *App) systemAPI(writer http.ResponseWriter, request *http.Request, tail string) {
	if (tail == "" || tail == "stats") && request.Method == "GET" {
		stats := Object{"version": app.version, "commit": app.commit, "uptime": int(time.Since(app.started).Seconds()), "platform": runtime.GOOS + "/" + runtime.GOARCH, "goVersion": runtime.Version(), "active": app.engine.active(), "mcp": "/mcp", "timezone": time.Now().Location().String()}
		var tasks, today, success, failed, running, queued int
		_ = app.store.db.QueryRow("SELECT COUNT(*) FROM records WHERE kind='crons'").Scan(&tasks)
		_ = app.store.db.QueryRow("SELECT COUNT(*) FROM runs WHERE kind='crons' AND substr(created,1,10)=?", time.Now().UTC().Format("2006-01-02")).Scan(&today)
		_ = app.store.db.QueryRow("SELECT COUNT(*) FROM runs WHERE kind='crons' AND status='success'").Scan(&success)
		_ = app.store.db.QueryRow("SELECT COUNT(*) FROM runs WHERE kind='crons' AND status IN ('failed','timeout','interrupted')").Scan(&failed)
		_ = app.store.db.QueryRow("SELECT COUNT(*) FROM runs WHERE status='running'").Scan(&running)
		_ = app.store.db.QueryRow("SELECT COUNT(*) FROM runs WHERE status='queued'").Scan(&queued)
		stats["tasks"] = tasks
		stats["today"] = today
		stats["success"] = success
		stats["failed"] = failed
		stats["running"] = running
		stats["queued"] = queued
		stats["successRate"] = 0.0
		if success+failed > 0 {
			stats["successRate"] = float64(success) * 100 / float64(success+failed)
		}
		if values, err := cpu.Percent(0, false); err == nil && len(values) > 0 {
			stats["cpu"] = values[0]
		}
		if memory, err := mem.VirtualMemory(); err == nil {
			stats["memory"] = memory.UsedPercent
			stats["memoryTotal"] = memory.Total
			stats["memoryUsed"] = memory.Used
		}
		if storage, err := disk.Usage(app.root); err == nil {
			stats["disk"] = storage.UsedPercent
			stats["diskFree"] = storage.Free
		}
		var usage runtime.MemStats
		runtime.ReadMemStats(&usage)
		stats["heapBytes"] = usage.Alloc
		series := []Object{}
		for offset := 6; offset >= 0; offset-- {
			day := time.Now().UTC().AddDate(0, 0, -offset).Format("2006-01-02")
			var good, bad int
			_ = app.store.db.QueryRow("SELECT COUNT(*) FROM runs WHERE kind='crons' AND status='success' AND substr(created,1,10)=?", day).Scan(&good)
			_ = app.store.db.QueryRow("SELECT COUNT(*) FROM runs WHERE kind='crons' AND status IN ('failed','timeout','interrupted') AND substr(created,1,10)=?", day).Scan(&bad)
			series = append(series, Object{"date": day, "success": good, "failed": bad})
		}
		stats["series"] = series
		stats["recent"], _ = app.runs("crons", 0, 6)
		ok(writer, stats)
		return
	}
	if tail == "runtimes" && request.Method == "GET" {
		result := []Object{}
		for name, program := range map[string]string{"Python": app.python(), "Node.js": "node", "Shell": findShell(), "Git": "git", "npm": "npm"} {
			location, err := exec.LookPath(program)
			entry := Object{"name": name, "available": err == nil, "path": location}
			if name == "Python" {
				// 补充版本与来源，供设置页展示实际生效的解释器。
				entry["version"] = pythonVersion(request.Context(), program)
				entry["source"] = app.pythonSource(request.Context())
				entry["configuredPath"] = strings.TrimSpace(app.store.setting("pythonPath", ""))
				entry["configuredVersion"] = strings.TrimSpace(app.store.setting("pythonVersion", ""))
			}
			result = append(result, entry)
		}
		ok(writer, result)
		return
	}
	// 公告内容：服务端代理远端 tz.json，多源回退，失败返回空文本。
	if tail == "notice" && request.Method == "GET" {
		app.noticeAPI(writer, request)
		return
	}
	if tail == "notifications" {
		app.notificationsAPI(writer, request)
		return
	}
	// 青龙 GET /api/system/notify-log：通知发送日志。
	if tail == "notify-log" && request.Method == "GET" {
		ok(writer, app.notifyLogs())
		return
	}
	// 青龙 GET /api/system/log：系统运行日志文本；DELETE 清空。
	if tail == "log" {
		app.systemLogAPI(writer, request)
		return
	}
	// 青龙 GET /api/system/config 读取系统配置；这里映射到面板设置，保持字段兼容。
	if tail == "config" && request.Method == "GET" {
		settings := app.settings()
		ok(writer, Object{
			"id": 1, "type": "systemConfig",
			"info": Object{
				"timezone":       time.Now().Location().String(),
				"lang":           "zh",
				"concurrency":    settings["concurrency"],
				"timeout":        settings["timeout"],
				"retentionDays":  settings["retentionDays"],
				"githubRepo":     settings["githubRepo"],
				"webhookURL":     settings["webhookURL"],
				"notifyFailures": settings["notifyFailures"],
			},
			"signedUpdates": app.updateKey != "",
		})
		return
	}
	if tail == "settings" {
		if request.Method == "GET" {
			result := app.settings()
			result["signedUpdates"] = app.updateKey != ""
			ok(writer, result)
			return
		}
		if request.Method == "PUT" {
			var body Object
			if err := decode(request, &body); err != nil {
				apiError(writer, err)
				return
			}
			if err := app.saveSettings(body); err != nil {
				apiError(writer, err)
				return
			}
			ok(writer, nil)
			return
		}
	}
	if tail == "notify" && request.Method == "POST" {
		if err := app.notify(request.Context(), "大白菜面板", "通知测试成功"); err != nil {
			apiError(writer, err)
			return
		}
		ok(writer, nil)
		return
	}
	if tail == "cleanup" && request.Method == "PUT" {
		count, err := app.cleanup()
		if err != nil {
			apiError(writer, err)
			return
		}
		ok(writer, Object{"removed": count})
		return
	}
	if strings.HasPrefix(tail, "update") {
		app.updateAPI(writer, request, strings.TrimPrefix(tail, "update"))
		return
	}
	fail(writer, 404, "系统接口不存在")
}

func (app *App) downloadText(ctx context.Context, address string, limit int64) (string, error) {
	parsed, err := url.Parse(address)
	if err != nil || !contains([]string{"http", "https"}, parsed.Scheme) || parsed.Host == "" || parsed.User != nil {
		return "", errors.New("下载地址无效")
	}
	request, err := http.NewRequestWithContext(ctx, "GET", address, nil)
	if err != nil {
		return "", err
	}
	response, err := app.client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return "", fmt.Errorf("下载失败 HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return "", err
	}
	if int64(len(data)) > limit || !utf8.Valid(data) {
		return "", errors.New("下载内容过大或不是 UTF-8 文本")
	}
	return string(data), nil
}

func (app *App) python() string {
	if venv := app.venvPython(); venv != "" {
		return venv
	}
	return app.resolvePythonBase()
}
func (app *App) installDependency(ctx context.Context, item Object, writer io.Writer) error {
	app.depMu.Lock()
	defer app.depMu.Unlock()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	name := text(item["name"])
	remove := boolean(item["remove"])
	npmMirror := strings.TrimSpace(app.store.setting("npmMirror", ""))
	pythonMirror := strings.TrimSpace(app.store.setting("pythonMirror", ""))
	linuxMirror := strings.TrimSpace(app.store.setting("linuxMirror", ""))
	var err error
	switch integer(item["type"]) {
	case 0:
		action := "install"
		if remove {
			action = "uninstall"
		}
		program := "npm"
		if runtime.GOOS == "windows" {
			program = "npm.cmd"
			command := "npm --prefix " + shellQuote(filepath.ToSlash(filepath.Join(app.root, "packages"))) + " " + action + " --no-audit --no-fund --ignore-scripts"
			if npmMirror != "" {
				command += " --registry=" + shellQuote(npmMirror)
			}
			command += " -- " + shellQuote(name)
			err = app.process(ctx, app.root, writer, findShell(), "-c", command)
		} else {
			args := []string{"--prefix", filepath.Join(app.root, "packages"), action, "--no-audit", "--no-fund", "--ignore-scripts"}
			if npmMirror != "" {
				args = append(args, "--registry="+npmMirror)
			}
			args = append(args, "--", name)
			err = app.process(ctx, app.root, writer, program, args...)
		}
	case 1:
		venv := filepath.Join(app.root, "packages", "venv")
		if !app.hasVenv() {
			// 用配置的基础 Python 创建 venv，而不是硬编码的 findPython。
			if err := app.process(ctx, app.root, writer, app.resolvePythonBase(), "-m", "venv", venv); err != nil {
				return err
			}
		}
		if remove {
			err = app.process(ctx, app.root, writer, app.python(), "-m", "pip", "uninstall", "-y", name)
		} else {
			args := []string{"-m", "pip", "install", "--disable-pip-version-check"}
			if pythonMirror != "" {
				args = append(args, "-i", pythonMirror)
			}
			args = append(args, "--", name)
			err = app.process(ctx, app.root, writer, app.python(), args...)
		}
	case 2:
		if runtime.GOOS != "linux" {
			return errors.New("系统依赖安装仅支持 Linux（apt-get / apk），Windows 请在宿主机安装")
		}
		if program, lookupErr := exec.LookPath("apk"); lookupErr == nil {
			action := "add"
			if remove {
				action = "del"
			}
			args := []string{action}
			// apk 镜像：--repository 指向镜像的 community/main 仓库。
			if linuxMirror != "" && action == "add" {
				args = append(args, "--repository", linuxMirror)
			}
			args = append(args, "--", name)
			err = app.process(ctx, app.root, writer, program, args...)
		} else if program, lookupErr := exec.LookPath("apt-get"); lookupErr == nil {
			action := "install"
			if remove {
				action = "remove"
			}
			args := []string{action, "-y"}
			// apt 镜像：通过 -o Dir::Etc::sourcelist 指向临时源文件，避免改动系统配置。
			if linuxMirror != "" && action == "install" {
				sourceList, writeErr := app.temporaryAptSource(linuxMirror)
				if writeErr == nil {
					args = append(args, "-o", "Dir::Etc::sourcelist="+sourceList, "-o", "Dir::Etc::sourceparts=-", "-o", "APT::Get::List-Cleanup=0")
				}
			}
			args = append(args, "--", name)
			err = app.process(ctx, app.root, writer, program, args...)
		} else {
			return errors.New("没有找到 apt-get 或 apk")
		}
	default:
		return errors.New("未知依赖类型")
	}
	if err == nil && remove {
		return app.store.remove("dependencies", integer(item["id"]))
	}
	return err
}

var embeddedCron = regexp.MustCompile("(?mi)(?:cron\\s*[:=]\\s*[\"']?|cron\\s+[\"'])([^\"'\\r\\n]+)")

func (app *App) syncSubscription(ctx context.Context, item Object, writer io.Writer) error {
	alias := text(item["alias"])
	source := filepath.Join(app.root, "repos", alias)
	files := map[string]string{}
	if text(item["type"]) == "file" {
		content, err := app.downloadText(ctx, text(item["url"]), 4<<20)
		if err != nil {
			return err
		}
		parsed, _ := url.Parse(text(item["url"]))
		name := filepath.Base(parsed.Path)
		if !contains([]string{".py", ".js", ".mjs", ".cjs", ".sh"}, strings.ToLower(filepath.Ext(name))) {
			return errors.New("单文件订阅只支持 Python、JS、Shell")
		}
		files[name] = content
	} else {
		if _, err := os.Stat(source); os.IsNotExist(err) {
			args := []string{"-c", "protocol.file.allow=never", "clone", "--depth", "1"}
			if branch := text(item["branch"]); branch != "" {
				args = append(args, "--branch", branch)
			}
			args = append(args, "--", text(item["url"]), source)
			if err := app.process(ctx, app.root, writer, "git", args...); err != nil {
				return err
			}
		} else {
			branch := text(item["branch"])
			if branch == "" {
				branch = "HEAD"
			}
			if err := app.process(ctx, source, writer, "git", "-c", "protocol.file.allow=never", "fetch", "--depth", "1", "--", text(item["url"]), branch); err != nil {
				return err
			}
			if err := app.process(ctx, source, writer, "git", "-c", "core.hooksPath=/dev/null", "reset", "--hard", "FETCH_HEAD"); err != nil {
				return err
			}
		}
		var whitelist, blacklist *regexp.Regexp
		if text(item["whitelist"]) != "" {
			whitelist, _ = regexp.Compile(text(item["whitelist"]))
		}
		if text(item["blacklist"]) != "" {
			blacklist, _ = regexp.Compile(text(item["blacklist"]))
		}
		err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if entry.IsDir() {
				if entry.Name() == ".git" || entry.Name() == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return nil
			}
			relative, _ := filepath.Rel(source, path)
			relative = filepath.ToSlash(relative)
			if !contains([]string{".py", ".js", ".mjs", ".cjs", ".sh", ".json", ".txt"}, strings.ToLower(filepath.Ext(path))) {
				return nil
			}
			if whitelist != nil && !whitelist.MatchString(relative) {
				return nil
			}
			if blacklist != nil && blacklist.MatchString(relative) {
				return nil
			}
			content, err := readText(path, 4<<20)
			if err != nil {
				return err
			}
			files[relative] = content
			if len(files) > 5000 {
				return errors.New("订阅文件超过 5000")
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	current := map[string]bool{}
	for relative, content := range files {
		targetName := alias + "/" + relative
		target, err := safePath(filepath.Join(app.root, "scripts"), targetName)
		if err != nil {
			return err
		}
		if err := atomicWrite(target, []byte(content)); err != nil {
			return err
		}
		current[targetName] = true
		fmt.Fprintln(writer, "同步", targetName)
		if boolean(item["autoAddCron"]) {
			match := embeddedCron.FindStringSubmatch(content)
			if len(match) > 1 {
				spec := strings.TrimSpace(match[1])
				if _, err := scheduleParser.Parse(spec); err == nil {
					command := "task " + shellQuote(targetName)
					tasks, _ := app.store.list("crons")
					exists := false
					for _, task := range tasks {
						if text(task["command"]) == command {
							exists = true
							break
						}
					}
					if !exists {
						if _, err := app.store.save("crons", defaults("crons", Object{"name": relative, "command": command, "schedule": spec, "subscriptionId": item["id"]})); err != nil {
							return err
						}
					}
				}
			}
		}
	}
	if boolean(item["autoDelCron"]) {
		if previous, valid := item["files"].([]any); valid {
			for _, value := range previous {
				relative := text(value)
				if !current[relative] && strings.HasPrefix(relative, alias+"/") {
					if target, err := safePath(filepath.Join(app.root, "scripts"), relative); err == nil {
						_ = os.Remove(target)
					}
				}
			}
		}
		tasks, _ := app.store.list("crons")
		for _, task := range tasks {
			if integer(task["subscriptionId"]) == integer(item["id"]) {
				exists := false
				for relative := range current {
					if text(task["command"]) == "task "+shellQuote(relative) {
						exists = true
						break
					}
				}
				if !exists {
					app.engine.stop("crons", integer(task["id"]), 0)
					_ = app.store.remove("crons", integer(task["id"]))
				}
			}
		}
	}
	names := []string{}
	for name := range current {
		names = append(names, name)
	}
	_, err := app.store.save("subscriptions", Object{"id": item["id"], "files": names, "lastSync": now()})
	app.engine.reschedule()
	return err
}

func (app *App) cleanup() (int, error) {
	days := integer(app.store.setting("retentionDays", "30"))
	cutoff := time.Now().AddDate(0, 0, -int(days)).UTC().Format(time.RFC3339Nano)
	rows, err := app.store.db.Query("SELECT id,log_path FROM runs WHERE created<? AND status NOT IN ('running','queued')", cutoff)
	if err != nil {
		return 0, err
	}
	items := []Object{}
	for rows.Next() {
		var id int64
		var path string
		if err := rows.Scan(&id, &path); err != nil {
			rows.Close()
			return 0, err
		}
		items = append(items, Object{"id": id, "path": path})
	}
	rows.Close()
	for _, item := range items {
		if path, err := safePath(filepath.Join(app.root, "logs"), text(item["path"])); err == nil {
			_ = os.Remove(path)
		}
		if _, err := app.store.db.Exec("DELETE FROM runs WHERE id=?", item["id"]); err != nil {
			return 0, err
		}
	}
	_, _ = app.store.db.Exec("DELETE FROM sessions WHERE expires>0 AND expires<?", time.Now().Unix())
	_, _ = app.store.db.Exec("DELETE FROM authorizations WHERE permanent=0 AND expires>0 AND expires<?", time.Now().Unix())
	return len(items), nil
}

// temporaryAptSource 为 apt-get 生成一份临时 sources.list，指向用户配置的镜像。
// 镜像地址形如 https://mirrors.tuna.tsinghua.edu.cn/ubuntu/，自动补齐 main 组件。
func (app *App) temporaryAptSource(mirror string) (string, error) {
	mirror = strings.TrimSpace(mirror)
	if mirror == "" {
		return "", errors.New("镜像地址为空")
	}
	// 识别发行版代号，用于构造标准源行；无法识别时按 ubuntu 默认处理。
	codename := detectDebianCodename()
	base := strings.TrimRight(mirror, "/")
	lines := []string{}
	if strings.Contains(base, "debian") {
		lines = append(lines,
			"deb "+base+" "+codename+" main contrib non-free non-free-firmware",
			"deb "+base+" "+codename+"-updates main contrib non-free non-free-firmware",
		)
	} else {
		lines = append(lines,
			"deb "+base+" "+codename+" main restricted universe multiverse",
			"deb "+base+" "+codename+"-updates main restricted universe multiverse",
		)
	}
	target := filepath.Join(app.root, "tmp", "sources.list")
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return "", err
	}
	if err := os.WriteFile(target, []byte(strings.Join(lines, "\n")+"\n"), 0600); err != nil {
		return "", err
	}
	return target, nil
}

// detectDebianCodename 读取 /etc/os-release 的 VERSION_CODENAME，失败时回退 bookworm。
func detectDebianCodename() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "bookworm"
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "VERSION_CODENAME=") {
			value := strings.Trim(strings.TrimPrefix(line, "VERSION_CODENAME="), "\"'")
			if value != "" {
				return value
			}
		}
	}
	return "bookworm"
}
