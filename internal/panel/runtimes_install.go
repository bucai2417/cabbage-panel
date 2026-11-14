package panel

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// runtimeProgressAPI 返回一键安装的实时状态。
func (app *App) runtimeProgressAPI(writer http.ResponseWriter, request *http.Request) {
	if request.Method != "GET" {
		fail(writer, 405, "方法不支持")
		return
	}
	ok(writer, app.currentRuntimeProgress())
}

// runtimeInstallAPI 启动一次运行时安装。
// 由 Handler 在 gate 读锁之前分发，因此这里不能假设已持锁。
func (app *App) runtimeInstallAPI(writer http.ResponseWriter, request *http.Request) {
	if request.Method != "POST" {
		fail(writer, 405, "方法不支持")
		return
	}
	var body Object
	if err := decode(request, &body); err != nil {
		apiError(writer, err)
		return
	}
	name := strings.TrimSpace(text(body["name"]))
	switch name {
	case "Python", "Node.js", "Git":
	default:
		fail(writer, 400, "不支持的运行时: "+name)
		return
	}
	if inContainer() {
		fail(writer, 409, "容器环境请在镜像内预装运行时，或使用内置运行时的官方镜像")
		return
	}
	if app.engine.active() > 0 {
		fail(writer, 409, "请先停止运行中的任务并等待队列清空")
		return
	}
	if app.currentRuntimeProgress().Running {
		fail(writer, 409, "已有运行时正在安装，请稍候")
		return
	}
	asset := runtimeAssetFor(name)
	if asset == nil && !(name == "Git" && mambaInstallSupported()) && !hasSystemPackageManager() {
		fail(writer, 400, "当前平台没有可用的自动安装方式，请参考提示手动安装")
		return
	}
	app.reportRuntimeProgress(func(state *RuntimeProgress) {
		*state = RuntimeProgress{Running: true, Runtime: name, Phase: "准备安装"}
	})
	go func() {
		err := app.installRuntime(context.Background(), name, asset)
		app.reportRuntimeProgress(func(state *RuntimeProgress) {
			state.Running = false
			state.Done = true
			if err != nil {
				state.Phase = "失败"
				state.Error = err.Error()
				app.appendSystemLog("error", fmt.Sprintf("[runtime] %s 安装失败: %v", name, err))
				return
			}
			state.Phase = "完成"
			state.Percent = 100
			state.Message = name + " 已就绪"
			app.appendSystemLog("info", fmt.Sprintf("[runtime] %s 安装完成", name))
		})
	}()
	ok(writer, Object{"started": true})
}

// hasSystemPackageManager 判断当前平台是否可能通过系统包管理器安装运行时。
func hasSystemPackageManager() bool {
	switch runtime.GOOS {
	case "linux":
		for _, program := range []string{"apt-get", "apk", "dnf", "yum"} {
			if _, err := exec.LookPath(program); err == nil {
				return true
			}
		}
	case "darwin":
		_, err := exec.LookPath("brew")
		return err == nil
	}
	return false
}

// installRuntime 执行完整安装：下载 → 校验 → 解压 → 原子替换 → 收尾。
// Windows 用便携包；Linux/macOS 的 Git 用 micromamba；其余走系统包管理器。
func (app *App) installRuntime(ctx context.Context, name string, asset *runtimeAsset) error {
	// 全局 client 有 45 秒总超时，几十 MB 的包在国内网络会中断，这里单独放宽。
	deadline, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	if asset != nil {
		return app.installPortableRuntime(deadline, asset)
	}
	if name == "Git" {
		if mamba := mambaAssetFor(); mamba != nil {
			return app.installGitWithMicromamba(deadline, mamba)
		}
	}
	return app.installSystemRuntime(deadline, name)
}

func (app *App) installPortableRuntime(ctx context.Context, asset *runtimeAsset) error {
	target := app.runtimeDir(asset.Runtime)
	workspace := filepath.Join(app.root, "packages", "runtime", fmt.Sprintf(".tmp-%s-%d", strings.ReplaceAll(asset.Runtime, ".", ""), time.Now().UnixNano()))
	if err := os.MkdirAll(workspace, 0700); err != nil {
		return err
	}
	defer os.RemoveAll(workspace)
	archive := filepath.Join(workspace, asset.File)

	address := app.runtimeDownloadURL(asset)
	limit := asset.Size
	if limit <= 0 {
		limit = 512 << 20
	} else {
		// 允许 1% 或 1MB 的偏差，防止 Content-Length 与实际略有差异时误判。
		limit = limit + limit/100 + 1<<20
	}
	app.reportRuntimeProgress(func(state *RuntimeProgress) { state.Phase = "下载中" })
	if err := app.downloadWithRetry(ctx, address, archive, limit, func(received, total int64) {
		app.reportRuntimeProgress(func(state *RuntimeProgress) {
			state.Received = received
			state.Total = total
			if total > 0 {
				state.Percent = int(received * 100 / total)
				if state.Percent > 100 {
					state.Percent = 100
				}
			}
		})
	}); err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}

	app.reportRuntimeProgress(func(state *RuntimeProgress) { state.Phase = "校验完整性" })
	if err := verifyFileSHA256(archive, asset.SHA256); err != nil {
		return err
	}

	app.reportRuntimeProgress(func(state *RuntimeProgress) { state.Phase = "解压中" })
	stage := filepath.Join(workspace, "extract")
	if err := os.MkdirAll(stage, 0755); err != nil {
		return err
	}
	if err := extractRuntimeArchive(archive, stage, asset.StripRoot); err != nil {
		return fmt.Errorf("解压失败: %w", err)
	}
	if err := app.verifyPortableLayout(asset.Runtime, stage); err != nil {
		return err
	}

	app.reportRuntimeProgress(func(state *RuntimeProgress) { state.Phase = "替换文件" })
	if err := app.swapRuntimeDir(stage, target); err != nil {
		return err
	}

	if asset.Runtime == "Python" && !app.hasVenv() {
		app.reportRuntimeProgress(func(state *RuntimeProgress) { state.Phase = "创建 Python 环境" })
		// venv 创建失败不影响运行时本身可用，只记录警告，避免整个安装被误判失败。
		if err := app.process(ctx, app.root, io.Discard, app.resolvePythonBase(), "-m", "venv", filepath.Join(app.root, "packages", "venv")); err != nil {
			app.appendSystemLog("warn", "[runtime] 便携 Python 已安装，但自动创建 venv 失败: "+err.Error()+"；可在设置页点「重建 Python 环境」重试")
			app.reportRuntimeProgress(func(state *RuntimeProgress) { state.Message = "Python 已就绪，venv 待重建" })
		}
	}
	return nil
}

// verifyPortableLayout 确认解压结果里确实有可执行文件，避免空包被当成成功。
func (app *App) verifyPortableLayout(name, dir string) error {
	var candidates []string
	switch name {
	case "Python":
		if runtime.GOOS == "windows" {
			candidates = []string{"python.exe", "python/python.exe"}
		} else {
			candidates = []string{"bin/python3", "python3", "bin/python"}
		}
	case "Node.js":
		if runtime.GOOS == "windows" {
			candidates = []string{"node.exe", "node/node.exe"}
		} else {
			candidates = []string{"bin/node", "node/bin/node"}
		}
	case "Git":
		if runtime.GOOS == "windows" {
			candidates = []string{"cmd/git.exe", "mingw64/bin/git.exe", "bin/git.exe", "bin/bash.exe", "usr/bin/bash.exe"}
		} else {
			// micromamba 前缀与 PortableGit 都是 bin/git。
			candidates = []string{"bin/git", "git/bin/git"}
		}
	}
	for _, relative := range candidates {
		if info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(relative))); err == nil && !info.IsDir() {
			return nil
		}
	}
	return fmt.Errorf("%s 安装包内容不完整，未找到可执行文件", name)
}

// swapRuntimeDir 用 gate 写锁保护目录替换，失败时回滚到旧目录。
func (app *App) swapRuntimeDir(stage, target string) error {
	app.gate.Lock()
	defer app.gate.Unlock()
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return err
	}
	backup := target + ".old-" + fmt.Sprint(time.Now().UnixNano())
	hasOld := false
	if _, err := os.Stat(target); err == nil {
		if err := os.Rename(target, backup); err != nil {
			return fmt.Errorf("旧运行时目录被占用，请先停止运行中的脚本: %w", err)
		}
		hasOld = true
	}
	if err := os.Rename(stage, target); err != nil {
		if hasOld {
			_ = os.Rename(backup, target)
		}
		return err
	}
	_ = os.RemoveAll(backup)
	return nil
}

// installSystemRuntime 通过系统包管理器安装（Linux / macOS）。
func (app *App) installSystemRuntime(ctx context.Context, name string) error {
	app.reportRuntimeProgress(func(state *RuntimeProgress) { state.Phase = "系统包安装中" })
	var log bytes.Buffer
	var err error
	switch runtime.GOOS {
	case "linux":
		packages := systemPackagesFor(name)
		if packages == nil {
			return fmt.Errorf("没有找到可安装 %s 的系统包", name)
		}
		if program, lookupErr := exec.LookPath("apt-get"); lookupErr == nil {
			args := append([]string{"install", "-y", "--"}, packages...)
			err = app.process(ctx, app.root, &log, program, args...)
		} else if program, lookupErr := exec.LookPath("apk"); lookupErr == nil {
			args := append([]string{"add", "--"}, packages...)
			err = app.process(ctx, app.root, &log, program, args...)
		} else if program, lookupErr := exec.LookPath("dnf"); lookupErr == nil {
			args := append([]string{"install", "-y", "--"}, packages...)
			err = app.process(ctx, app.root, &log, program, args...)
		} else if program, lookupErr := exec.LookPath("yum"); lookupErr == nil {
			args := append([]string{"install", "-y", "--"}, packages...)
			err = app.process(ctx, app.root, &log, program, args...)
		} else {
			return errors.New("没有找到可用的系统包管理器")
		}
	case "darwin":
		if _, lookupErr := exec.LookPath("brew"); lookupErr != nil {
			return errors.New("没有找到 Homebrew")
		}
		packages := systemPackagesFor(name)
		if packages == nil {
			return fmt.Errorf("没有找到可安装 %s 的 Homebrew 包", name)
		}
		err = app.process(ctx, app.root, &log, "brew", append([]string{"install"}, packages...)...)
	default:
		return errors.New("当前平台不支持自动安装")
	}
	if err != nil {
		return fmt.Errorf("%w: %s", err, lastLines(log.String(), 6))
	}
	app.reportRuntimeProgress(func(state *RuntimeProgress) { state.Phase = "完成" })
	return nil
}

func systemPackagesFor(name string) []string {
	switch runtime.GOOS {
	case "linux":
		switch name {
		case "Python":
			return []string{"python3", "python3-venv", "python3-pip"}
		case "Node.js":
			return []string{"nodejs", "npm"}
		case "Git":
			return []string{"git"}
		}
	case "darwin":
		switch name {
		case "Python":
			return []string{"python@3.12"}
		case "Node.js":
			return []string{"node"}
		case "Git":
			return []string{"git"}
		}
	}
	return nil
}

// downloadWithRetry 在连接中断时用 HTTP Range 续传，最多重试 5 次。
// 国内访问 GitHub 容易中途断开，一次性下载失败率很高。
func (app *App) downloadWithRetry(ctx context.Context, address, path string, limit int64, onProgress func(received, total int64)) error {
	const attempts = 5
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		resume := int64(0)
		if info, err := os.Stat(path); err == nil && info.Size() > 0 {
			resume = info.Size()
		}
		err := app.downloadToFile(ctx, address, path, limit, resume, onProgress)
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		lastErr = err
		if attempt < attempts {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt) * 2 * time.Second):
			}
		}
	}
	return lastErr
}

// downloadToFile 边下边写盘，避免几十 MB 的包整块驻留内存。
// resume > 0 时用 Range 请求续传；使用独立的无总超时客户端，由调用方 ctx 控制整体时限。
func (app *App) downloadToFile(ctx context.Context, address, path string, limit, resume int64, onProgress func(received, total int64)) error {
	request, err := http.NewRequestWithContext(ctx, "GET", address, nil)
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", "cabbage-panel/"+app.version)
	if resume > 0 {
		request.Header.Set("Range", "bytes="+strconv.FormatInt(resume, 10)+"-")
	}
	client := &http.Client{Transport: safeOutboundTransport(app.outboundAllowed)}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	// 续传成功返回 206；服务器不支持 Range 时回退到整体重下（200）。
	appendMode := resume > 0 && response.StatusCode == http.StatusPartialContent
	if response.StatusCode != 200 && response.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("下载失败 HTTP %d", response.StatusCode)
	}
	if response.StatusCode == http.StatusPartialContent && resume == 0 {
		return errors.New("服务器返回了意外的部分内容")
	}
	if !appendMode {
		resume = 0
	}
	total := response.ContentLength
	if total > 0 {
		total += resume
	}
	if onProgress != nil {
		onProgress(resume, total)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	flags := os.O_CREATE | os.O_WRONLY
	if appendMode {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	file, err := os.OpenFile(path, flags, 0600)
	if err != nil {
		return err
	}
	success := false
	defer func() {
		file.Close()
		if !success && !appendMode {
			_ = os.Remove(path)
		}
	}()
	buffer := make([]byte, 64*1024)
	received := resume
	for {
		read, readErr := response.Body.Read(buffer)
		if read > 0 {
			received += int64(read)
			if received > limit {
				return errors.New("下载文件超过大小上限")
			}
			if _, writeErr := file.Write(buffer[:read]); writeErr != nil {
				return writeErr
			}
			if onProgress != nil {
				onProgress(received, total)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	success = true
	return nil
}

func verifyFileSHA256(path, expected string) error {
	if expected == "" {
		return errors.New("缺少内置校验值，拒绝安装")
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return err
	}
	actual := hex.EncodeToString(digest.Sum(nil))
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("校验失败：期望 %s，实际 %s", expected, actual)
	}
	return nil
}

// lastLines 截取日志末尾若干行，用于错误摘要。
func lastLines(value string, count int) string {
	lines := strings.Split(strings.TrimRight(value, "\n"), "\n")
	if len(lines) > count {
		lines = lines[len(lines)-count:]
	}
	return strings.Join(lines, " | ")
}
