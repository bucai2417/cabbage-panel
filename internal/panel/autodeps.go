package panel

import (
	"path/filepath"
	"regexp"
	"strings"
)

// 依赖缺失的典型错误模式。
var (
	// Python: ModuleNotFoundError: No module named 'requests' / ImportError
	pythonMissing = regexp.MustCompile(`(?m)(?:ModuleNotFoundError|ImportError):\s+No module named ['"]?([A-Za-z0-9_.\-]+)['"]?`)
	// Node: Error: Cannot find module 'axios' / require('axios')
	nodeMissing = regexp.MustCompile(`(?m)Cannot find module ['"]([^'"]+)['"]`)
	// Shell: command not found（仅记录，不自动安装，避免误装）
	shellMissing = regexp.MustCompile(`(?m)^([A-Za-z0-9_.\-]+): command not found`)
)

// 包名白名单：只允许普通包名字符，拒绝命令行参数、路径和 URL 形态。
var (
	pythonPackageName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$`)
	nodePackageName   = regexp.MustCompile(`^(@[a-z0-9][a-z0-9._-]{0,63}/)?[a-z0-9][a-z0-9._-]{0,63}$`)
)

// validDependencyName 判断从日志提取出的包名是否可安全交给包管理器。
func validDependencyName(name string, kind int) bool {
	switch kind {
	case 0:
		return nodePackageName.MatchString(name) && !strings.Contains(name, "..")
	case 1:
		return pythonPackageName.MatchString(name) && !strings.Contains(name, "..")
	default:
		return false
	}
}

// missingDependency 描述一个从日志中识别出的缺失依赖。
type missingDependency struct {
	Name string
	Type int // 0 npm、1 Python、2 Linux
}

// detectMissingDependencies 扫描任务日志末尾，识别缺失依赖。
func (app *App) detectMissingDependencies(logName string) []missingDependency {
	target, err := safePath(filepath.Join(app.root, "logs"), logName)
	if err != nil {
		return nil
	}
	content, err := readText(target, 512<<10)
	if err != nil {
		return nil
	}
	result := []missingDependency{}
	seen := map[string]bool{}
	add := func(name string, kind int) {
		name = strings.TrimSpace(name)
		// Node 的包名可能带子路径（如 lodash/fp），取顶层包名。
		if kind == 0 {
			if index := strings.Index(name, "/"); index > 0 && !strings.HasPrefix(name, "@") {
				name = name[:index]
			} else if strings.HasPrefix(name, "@") {
				if parts := strings.Split(name, "/"); len(parts) >= 2 {
					name = parts[0] + "/" + parts[1]
				}
			}
		}
		// Python 包名可能带点（如 pkg.module），取顶层。
		if kind == 1 {
			if index := strings.Index(name, "."); index > 0 {
				name = name[:index]
			}
		}
		if name == "" || seen[name] || !validDependencyName(name, kind) {
			return
		}
		seen[name] = true
		result = append(result, missingDependency{Name: name, Type: kind})
	}
	for _, match := range pythonMissing.FindAllStringSubmatch(content, 5) {
		add(match[1], 1)
	}
	for _, match := range nodeMissing.FindAllStringSubmatch(content, 5) {
		// 过滤相对路径与内置模块，这些不是可安装的包。
		name := match[1]
		if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "/") || strings.HasPrefix(name, "node:") {
			continue
		}
		add(name, 0)
	}
	return result
}

// autoInstallMissing 为识别出的缺失依赖创建安装任务（若尚未存在）。
// 返回实际创建的依赖名称列表。
func (app *App) autoInstallMissing(missing []missingDependency) []string {
	if len(missing) == 0 {
		return nil
	}
	existing, _ := app.store.list("dependencies")
	installed := map[string]bool{}
	for _, item := range existing {
		installed[text(item["name"])] = true
	}
	created := []string{}
	for _, dep := range missing {
		if installed[dep.Name] || !validDependencyName(dep.Name, dep.Type) {
			continue
		}
		item := Object{"name": dep.Name, "type": dep.Type, "remark": "任务运行失败后自动添加", "status": 1, "remove": false}
		saved, err := app.store.save("dependencies", defaults("dependencies", item))
		if err != nil {
			continue
		}
		if _, err := app.engine.enqueue("dependencies", integer(saved["id"])); err != nil {
			_, _ = app.store.save("dependencies", Object{"id": saved["id"], "status": 2})
			continue
		}
		installed[dep.Name] = true
		created = append(created, dep.Name)
	}
	return created
}
