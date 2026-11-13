package panel

import (
	"context"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// runtimeAsset 描述一个可下载的便携运行时包。
// SHA256 与 Size 来自发布方资产实测，内置在源码中，不信任首次下载内容。
type runtimeAsset struct {
	Runtime     string
	Version     string
	GOOS        string
	GOARCH      string
	File        string
	DefaultBase string // 下载基址，设置项为空时使用
	MirrorKey   string // 覆盖基址的设置项键名
	SHA256      string
	Size        int64
	StripRoot   bool // 归档顶层是否只有一个目录需要剥离
}

// runtimeAssets 只收录经过实测指纹的版本，不猜测资产清单。
// 新增版本时先用 curl 取回 digest 再填表。
// 默认源统一走 npmmirror（国内可达，且与 GitHub 上游字节一致，指纹已核对）；
// 需要时可在设置页改回 GitHub 或其他镜像。
var runtimeAssets = []runtimeAsset{
	{Runtime: "Python", Version: "3.12.16", GOOS: "windows", GOARCH: "amd64", File: "cpython-3.12.16+20261110-x86_64-pc-windows-msvc-install_only_stripped.tar.gz", DefaultBase: "https://registry.npmmirror.com/-/binary/python-build-standalone/20261110", MirrorKey: "runtimePythonMirror", SHA256: "355ee61b8207b91574491b1c6dc9b0ebb4408fc56bfafe498f3ef439314b52dc", Size: 22365213, StripRoot: true},
	{Runtime: "Python", Version: "3.12.16", GOOS: "windows", GOARCH: "arm64", File: "cpython-3.12.16+20261110-aarch64-pc-windows-msvc-install_only_stripped.tar.gz", DefaultBase: "https://registry.npmmirror.com/-/binary/python-build-standalone/20261110", MirrorKey: "runtimePythonMirror", SHA256: "c1b334c7b1db8f962b37a333d8362cab0ba7ebda1a7cce917fae56a1e0c8cbe7", Size: 21407490, StripRoot: true},
	{Runtime: "Python", Version: "3.12.16", GOOS: "linux", GOARCH: "amd64", File: "cpython-3.12.16+20261110-x86_64-unknown-linux-gnu-install_only_stripped.tar.gz", DefaultBase: "https://registry.npmmirror.com/-/binary/python-build-standalone/20261110", MirrorKey: "runtimePythonMirror", SHA256: "6a9e62dad033790aef722dbd370e0ce76ce17b73627ba3ae679f537049350b39", Size: 34709145, StripRoot: true},
	{Runtime: "Python", Version: "3.12.16", GOOS: "linux", GOARCH: "arm64", File: "cpython-3.12.16+20261110-aarch64-unknown-linux-gnu-install_only_stripped.tar.gz", DefaultBase: "https://registry.npmmirror.com/-/binary/python-build-standalone/20261110", MirrorKey: "runtimePythonMirror", SHA256: "b6a41defec4975d94b4ff1950236e7383e5c02ba40ea48c9ec33c66087a35aab", Size: 29440910, StripRoot: true},
	{Runtime: "Python", Version: "3.12.16", GOOS: "darwin", GOARCH: "arm64", File: "cpython-3.12.16+20261110-aarch64-apple-darwin-install_only_stripped.tar.gz", DefaultBase: "https://registry.npmmirror.com/-/binary/python-build-standalone/20261110", MirrorKey: "runtimePythonMirror", SHA256: "2f11b8aa3726bdbd87dd9eb76568412faddf4f47e26913fa45d6a2e424972acd", Size: 25237581, StripRoot: true},
	{Runtime: "Python", Version: "3.12.16", GOOS: "darwin", GOARCH: "amd64", File: "cpython-3.12.16+20261110-x86_64-apple-darwin-install_only_stripped.tar.gz", DefaultBase: "https://registry.npmmirror.com/-/binary/python-build-standalone/20261110", MirrorKey: "runtimePythonMirror", SHA256: "b78a4c479f1964ec484880bee567568266f68da102269fb93a05fe8716b45f91", Size: 24957926, StripRoot: true},
	{Runtime: "Node.js", Version: "24.22.3", GOOS: "windows", GOARCH: "amd64", File: "node-v24.22.3-win-x64.zip", DefaultBase: "https://registry.npmmirror.com/-/binary/node/v24.22.3", MirrorKey: "runtimeNodeMirror", SHA256: "e612af99f626ab415ea225af6bce5c971feb448cb5cae250bb1c3fa50c5f7d9f", Size: 37762649, StripRoot: true},
	{Runtime: "Node.js", Version: "24.22.3", GOOS: "windows", GOARCH: "arm64", File: "node-v24.22.3-win-arm64.zip", DefaultBase: "https://registry.npmmirror.com/-/binary/node/v24.22.3", MirrorKey: "runtimeNodeMirror", SHA256: "b3cc74a295b39d4f273271e96f58c06dd2b35757ca12abec64c7d0598b434f14", Size: 33802073, StripRoot: true},
	{Runtime: "Node.js", Version: "24.22.3", GOOS: "linux", GOARCH: "amd64", File: "node-v24.22.3-linux-x64.tar.gz", DefaultBase: "https://registry.npmmirror.com/-/binary/node/v24.22.3", MirrorKey: "runtimeNodeMirror", SHA256: "061ec3e3062115705b56b51fa9cc330ab50d34b6969387d4e4b1a50c6809acc2", Size: 58330843, StripRoot: true},
	{Runtime: "Node.js", Version: "24.22.3", GOOS: "linux", GOARCH: "arm64", File: "node-v24.22.3-linux-arm64.tar.gz", DefaultBase: "https://registry.npmmirror.com/-/binary/node/v24.22.3", MirrorKey: "runtimeNodeMirror", SHA256: "261af2af250dfd7e9f25cf151d1b2713a0adf8f10a9ad116c4947f68627affe8", Size: 58051304, StripRoot: true},
	{Runtime: "Node.js", Version: "24.22.3", GOOS: "darwin", GOARCH: "arm64", File: "node-v24.22.3-darwin-arm64.tar.gz", DefaultBase: "https://registry.npmmirror.com/-/binary/node/v24.22.3", MirrorKey: "runtimeNodeMirror", SHA256: "ff6e17d4874b76d272212b908cc020d6e815fba5a6c7eaaa06add725840307fd", Size: 53084299, StripRoot: true},
	{Runtime: "Node.js", Version: "24.22.3", GOOS: "darwin", GOARCH: "amd64", File: "node-v24.22.3-darwin-x64.tar.gz", DefaultBase: "https://registry.npmmirror.com/-/binary/node/v24.22.3", MirrorKey: "runtimeNodeMirror", SHA256: "977614af0504c81045e7fd85596dbca0e5f951b96681516dadafedcca8c447f7", Size: 54395976, StripRoot: true},
	{Runtime: "Git", Version: "2.56.1", GOOS: "windows", GOARCH: "amd64", File: "MinGit-2.56.1-64-bit.zip", DefaultBase: "https://registry.npmmirror.com/-/binary/git-for-windows/v2.56.1.windows.1", MirrorKey: "runtimeGitMirror", SHA256: "4a92850b6e917da4fd010669b614b55dccf5b0ec333a534cc4be4700dfefdafd", Size: 39849768, StripRoot: false},
	{Runtime: "Git", Version: "2.56.1", GOOS: "windows", GOARCH: "arm64", File: "MinGit-2.56.1-arm64.zip", DefaultBase: "https://registry.npmmirror.com/-/binary/git-for-windows/v2.56.1.windows.1", MirrorKey: "runtimeGitMirror", SHA256: "96571e442e90523a90e5fd8661b4c4e43bc192910f3c6db938816e6955b4cc8c", Size: 37983254, StripRoot: false},
}

// mambaAssets 描述用于在 Linux/macOS 上把 Git 装进隔离前缀的 micromamba 静态二进制。
// .tar.bz2 由标准库 compress/bzip2 解压，无需额外依赖。
var mambaAssets = []runtimeAsset{
	{Runtime: "micromamba", Version: "2.10.0", GOOS: "linux", GOARCH: "amd64", File: "micromamba-linux-64.tar.bz2", DefaultBase: "https://github.com/mamba-org/micromamba-releases/releases/download/2.10.0-0", MirrorKey: "runtimeMambaMirror", SHA256: "9a97976244929bbae28fb1fae504e74a3e7e6d1932870e3a03321f01b7fbd07b", Size: 7317886},
	{Runtime: "micromamba", Version: "2.10.0", GOOS: "linux", GOARCH: "arm64", File: "micromamba-linux-aarch64.tar.bz2", DefaultBase: "https://github.com/mamba-org/micromamba-releases/releases/download/2.10.0-0", MirrorKey: "runtimeMambaMirror", SHA256: "890bc4906e6ab17f1b393f0da21547f75e000078e573a156316ec925b92a4a4c", Size: 8848170},
	{Runtime: "micromamba", Version: "2.10.0", GOOS: "darwin", GOARCH: "arm64", File: "micromamba-osx-arm64.tar.bz2", DefaultBase: "https://github.com/mamba-org/micromamba-releases/releases/download/2.10.0-0", MirrorKey: "runtimeMambaMirror", SHA256: "acef4eaab19ec2174ad5a449e5aad981ca326270db819502ff096a92fc31caa6", Size: 7197729},
	{Runtime: "micromamba", Version: "2.10.0", GOOS: "darwin", GOARCH: "amd64", File: "micromamba-osx-64.tar.bz2", DefaultBase: "https://github.com/mamba-org/micromamba-releases/releases/download/2.10.0-0", MirrorKey: "runtimeMambaMirror", SHA256: "17b20c4271d65c030a5c86788f5ca86eb575187209f626edabdb8c6a875c6838", Size: 7334472},
}

// runtimeNames 是 runtimes 接口的稳定输出顺序。
var runtimeNames = []string{"Python", "Node.js", "npm", "Git", "Shell"}

func runtimeAssetFor(name string) *runtimeAsset {
	for index := range runtimeAssets {
		asset := &runtimeAssets[index]
		if asset.Runtime == name && asset.GOOS == runtime.GOOS && asset.GOARCH == runtime.GOARCH {
			return asset
		}
	}
	return nil
}

// mambaAssetFor 返回当前平台的 micromamba 资产；Windows 不需要（用 MinGit）。
func mambaAssetFor() *runtimeAsset {
	for index := range mambaAssets {
		asset := &mambaAssets[index]
		if asset.GOOS == runtime.GOOS && asset.GOARCH == runtime.GOARCH {
			return asset
		}
	}
	return nil
}

// runtimeDir 是某个便携运行时的解压根目录。
func (app *App) runtimeDir(name string) string {
	return filepath.Join(app.root, "packages", "runtime", name)
}

// firstFile 按顺序返回第一个存在的普通文件路径；全部不存在返回空串。
func firstFile(candidates ...string) string {
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func (app *App) portablePython() string {
	base := app.runtimeDir("Python")
	if runtime.GOOS == "windows" {
		return firstFile(filepath.Join(base, "python.exe"), filepath.Join(base, "python", "python.exe"))
	}
	return firstFile(filepath.Join(base, "bin", "python3"), filepath.Join(base, "python3"), filepath.Join(base, "bin", "python"))
}

func (app *App) portableNode() string {
	base := app.runtimeDir("Node.js")
	if runtime.GOOS == "windows" {
		return firstFile(filepath.Join(base, "node.exe"), filepath.Join(base, "node", "node.exe"))
	}
	return firstFile(filepath.Join(base, "bin", "node"), filepath.Join(base, "node", "bin", "node"))
}

// portableNpm 返回便携 Node 自带的 npm 启动器（Windows 为 npm.cmd）。
func (app *App) portableNpm() string {
	base := app.runtimeDir("Node.js")
	if runtime.GOOS == "windows" {
		return firstFile(filepath.Join(base, "npm.cmd"), filepath.Join(base, "node", "npm.cmd"))
	}
	return firstFile(filepath.Join(base, "bin", "npm"), filepath.Join(base, "node", "bin", "npm"))
}

// portableGit 不猜测 MinGit 布局，逐个探测常见位置。
// Linux/macOS 的 Git 由 micromamba 装到隔离前缀，布局为 bin/git。
func (app *App) portableGit() string {
	base := app.runtimeDir("Git")
	if runtime.GOOS == "windows" {
		return firstFile(
			filepath.Join(base, "cmd", "git.exe"),
			filepath.Join(base, "mingw64", "bin", "git.exe"),
			filepath.Join(base, "mingw32", "bin", "git.exe"),
			filepath.Join(base, "bin", "git.exe"),
			filepath.Join(base, "git", "cmd", "git.exe"),
		)
	}
	return firstFile(
		filepath.Join(base, "bin", "git"),
		filepath.Join(base, "git", "bin", "git"),
	)
}

// portableBash 返回便携 Git 的 shell。
// Windows MinGit 不含 bash.exe，只提供 usr/bin/sh.exe；task 助手已改为 POSIX sh 语法，
// 实测 normal / conc / desi 三种模式在该 sh 下均可运行，因此可作为 Shell 兜底。
// Linux/macOS 的 micromamba 前缀通常同时带 bash 与 sh。
func (app *App) portableBash() string {
	base := app.runtimeDir("Git")
	if runtime.GOOS == "windows" {
		return firstFile(
			filepath.Join(base, "bin", "bash.exe"),
			filepath.Join(base, "usr", "bin", "bash.exe"),
			filepath.Join(base, "usr", "bin", "sh.exe"),
			filepath.Join(base, "git", "bin", "bash.exe"),
			filepath.Join(base, "git", "usr", "bin", "sh.exe"),
		)
	}
	return firstFile(
		filepath.Join(base, "bin", "bash"),
		filepath.Join(base, "bin", "sh"),
		filepath.Join(base, "git", "bin", "bash"),
		filepath.Join(base, "git", "bin", "sh"),
	)
}

// portableBins 返回需要前置到 PATH 的目录，顺序即优先级。
func (app *App) portableBins() []string {
	candidates := []string{}
	if python := app.portablePython(); python != "" {
		candidates = append(candidates, filepath.Dir(python))
	}
	if node := app.portableNode(); node != "" {
		candidates = append(candidates, filepath.Dir(node))
	}
	git := app.runtimeDir("Git")
	for _, sub := range []string{"cmd", "mingw64/bin", "mingw32/bin", "bin", "usr/bin"} {
		dir := filepath.Join(git, filepath.FromSlash(sub))
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			candidates = append(candidates, dir)
		}
	}
	unique := make([]string, 0, len(candidates))
	seen := map[string]bool{}
	for _, dir := range candidates {
		if dir == "" || seen[dir] {
			continue
		}
		seen[dir] = true
		unique = append(unique, dir)
	}
	return unique
}

// RuntimeProgress 是一键安装的实时状态快照，供前端轮询。
type RuntimeProgress struct {
	Running  bool   `json:"running"`
	Done     bool   `json:"done"`
	Runtime  string `json:"runtime"`
	Phase    string `json:"phase"`
	Received int64  `json:"received"`
	Total    int64  `json:"total"`
	Percent  int    `json:"percent"`
	Message  string `json:"message"`
	Error    string `json:"error"`
}

func (app *App) reportRuntimeProgress(mutate func(*RuntimeProgress)) {
	app.runtimeMu.Lock()
	mutate(&app.runtimeState)
	app.runtimeMu.Unlock()
}

func (app *App) currentRuntimeProgress() RuntimeProgress {
	app.runtimeMu.Lock()
	defer app.runtimeMu.Unlock()
	return app.runtimeState
}

// runtimeDownloadURL 组合最终下载地址：设置项覆盖基址，否则用内置默认。
func (app *App) runtimeDownloadURL(asset *runtimeAsset) string {
	base := strings.TrimSpace(app.store.setting(asset.MirrorKey, ""))
	if base == "" {
		base = asset.DefaultBase
	}
	return strings.TrimRight(base, "/") + "/" + asset.File
}

// runtimeEntries 汇总所有运行时的检测结果，顺序稳定，供设置页与仪表盘使用。
func (app *App) runtimeEntries(ctx context.Context) []Object {
	result := make([]Object, 0, len(runtimeNames))
	for _, name := range runtimeNames {
		result = append(result, app.runtimeEntry(ctx, name))
	}
	return result
}

func (app *App) runtimeEntry(ctx context.Context, name string) Object {
	var program, source string
	switch name {
	case "Python":
		program = app.python()
		source = app.pythonSource(ctx)
	case "Node.js":
		program, source = app.nodeProgram(), app.nodeSource()
	case "npm":
		program = app.npmProgram()
		source = "system"
		if app.portableNpm() != "" {
			source = "portable"
		}
	case "Git":
		program, source = app.gitProgram(), app.gitSource()
	case "Shell":
		program = app.shell()
		source = "system"
		if app.portableBash() != "" && program == app.portableBash() {
			source = "portable"
		}
	}
	location, err := exec.LookPath(program)
	available := err == nil || (program != "" && fileExists(program))
	entry := Object{"name": name, "available": available, "path": location}
	if !available && program != "" {
		entry["path"] = program
	}
	if available {
		entry["version"] = app.probeVersion(ctx, program)
	} else {
		entry["version"] = ""
	}
	entry["source"] = source
	asset := runtimeAssetFor(name)
	// npm 随 Node.js 一起提供，不单独安装；Shell 由 Git 提供。
	// Linux/macOS 的 Git 没有直接便携包，但有 micromamba 可用。
	installable := asset != nil
	if !installable && name == "Git" && mambaAssetFor() != nil {
		installable = true
	}
	entry["installable"] = installable
	if asset == nil {
		entry["installHint"] = installHintFor(name)
	}
	if name == "Shell" && !available && (runtimeAssetFor("Git") != nil || mambaAssetFor() != nil) {
		entry["installHint"] = "安装 Git 即可获得 Shell"
	}
	if name == "Python" {
		entry["configuredPath"] = strings.TrimSpace(app.store.setting("pythonPath", ""))
		entry["configuredVersion"] = strings.TrimSpace(app.store.setting("pythonVersion", ""))
	}
	return entry
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func (app *App) nodeProgram() string {
	if node := app.portableNode(); node != "" {
		return node
	}
	return "node"
}

func (app *App) npmProgram() string {
	if npm := app.portableNpm(); npm != "" {
		return npm
	}
	if runtime.GOOS == "windows" {
		return "npm.cmd"
	}
	return "npm"
}

func (app *App) nodeSource() string {
	if app.portableNode() != "" {
		return "portable"
	}
	return "system"
}

func (app *App) gitProgram() string {
	if git := app.portableGit(); git != "" {
		return git
	}
	return "git"
}

func (app *App) gitSource() string {
	if app.portableGit() != "" {
		return "portable"
	}
	return "system"
}

// probeVersion 执行 --version 解析版本号；npm 在 Windows 需要经 shell 调用。
func (app *App) probeVersion(ctx context.Context, program string) string {
	probe, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var command *exec.Cmd
	if runtime.GOOS == "windows" && strings.HasSuffix(strings.ToLower(program), ".cmd") {
		command = exec.CommandContext(probe, app.shell(), "-c", shellQuote(program)+" --version")
	} else {
		command = exec.CommandContext(probe, program, "--version")
	}
	output, err := command.CombinedOutput()
	if err != nil {
		return ""
	}
	match := pythonVersionPattern.FindStringSubmatch(string(output))
	if len(match) > 1 {
		return match[1]
	}
	return strings.TrimSpace(strings.SplitN(string(output), "\n", 2)[0])
}

// installHintFor 为没有内置资产的平台给出可复制的引导命令。
func installHintFor(name string) string {
	switch runtime.GOOS {
	case "linux":
		if packages := systemPackagesFor(name); packages != nil {
			if _, err := exec.LookPath("apk"); err == nil {
				return "apk add " + strings.Join(packages, " ")
			}
			return "apt-get install -y " + strings.Join(packages, " ")
		}
	case "darwin":
		if packages := systemPackagesFor(name); packages != nil {
			return "brew install " + strings.Join(packages, " ")
		}
	}
	return ""
}

// reportMissingRuntimes 在启动时把缺失的运行时写进日志，引导用户到设置页安装。
// 不阻塞启动，也不自动安装。
func (app *App) reportMissingRuntimes() {
	if inContainer() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	missing := []string{}
	for _, entry := range app.runtimeEntries(ctx) {
		if boolean(entry["available"]) {
			continue
		}
		if name := text(entry["name"]); name != "Shell" && name != "npm" {
			missing = append(missing, name)
		}
	}
	if len(missing) == 0 {
		return
	}
	message := "[runtime] 未检测到运行时: " + strings.Join(missing, "、") + "；可在「设置 → 运行环境」一键安装"
	log.Printf("%s", message)
	app.appendSystemLog("warn", message)
}
