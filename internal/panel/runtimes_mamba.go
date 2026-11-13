package panel

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// installGitWithMicromamba 在 Linux/macOS 上把 Git 装进 data/packages/runtime/Git。
// 下载 micromamba 静态二进制（SHA-256 校验）→ 解压 → 用 conda-forge 创建隔离前缀。
// 全程不写系统目录、不需要 root。
func (app *App) installGitWithMicromamba(ctx context.Context, mamba *runtimeAsset) error {
	target := app.runtimeDir("Git")
	workspace := filepath.Join(app.root, "packages", "runtime", fmt.Sprintf(".tmp-mamba-%d", time.Now().UnixNano()))
	if err := os.MkdirAll(workspace, 0700); err != nil {
		return err
	}
	defer os.RemoveAll(workspace)

	archive := filepath.Join(workspace, mamba.File)
	limit := mamba.Size
	if limit <= 0 {
		limit = 64 << 20
	} else {
		limit = limit + limit/100 + 1<<20
	}
	app.reportRuntimeProgress(func(state *RuntimeProgress) { state.Phase = "下载 micromamba" })
	if err := app.downloadWithRetry(ctx, app.runtimeDownloadURL(mamba), archive, limit, func(received, total int64) {
		app.reportRuntimeProgress(func(state *RuntimeProgress) {
			state.Received = received
			state.Total = total
			if total > 0 {
				state.Percent = int(received * 100 / total)
			}
		})
	}); err != nil {
		return fmt.Errorf("下载 micromamba 失败: %w", err)
	}

	app.reportRuntimeProgress(func(state *RuntimeProgress) { state.Phase = "校验 micromamba" })
	if err := verifyFileSHA256(archive, mamba.SHA256); err != nil {
		return err
	}

	app.reportRuntimeProgress(func(state *RuntimeProgress) { state.Phase = "解压 micromamba" })
	stage := filepath.Join(workspace, "mamba")
	if err := os.MkdirAll(stage, 0755); err != nil {
		return err
	}
	if err := extractTarBz2(archive, stage, false); err != nil {
		return fmt.Errorf("解压 micromamba 失败: %w", err)
	}
	binary := firstFile(filepath.Join(stage, "bin", "micromamba"), filepath.Join(stage, "micromamba"))
	if binary == "" {
		return fmt.Errorf("micromamba 安装包内容不完整")
	}
	if err := os.Chmod(binary, 0755); err != nil {
		return err
	}

	app.reportRuntimeProgress(func(state *RuntimeProgress) { state.Phase = "创建 Git 环境" })
	prefix := filepath.Join(workspace, "prefix")
	rootPrefix := filepath.Join(app.root, "packages", "runtime", ".mamba")
	if err := os.MkdirAll(rootPrefix, 0700); err != nil {
		return err
	}
	var log bytes.Buffer
	args := []string{"create", "-y", "-p", prefix}
	if channel := strings.TrimSpace(app.store.setting("runtimeMambaChannel", "")); channel != "" {
		args = append(args, "-c", channel)
	} else {
		args = append(args, "-c", "conda-forge")
	}
	args = append(args, "git=2.56.1")
	command := exec.CommandContext(ctx, binary, args...)
	command.Dir = workspace
	command.Env = append(os.Environ(), "MAMBA_ROOT_PREFIX="+rootPrefix, "CONDARC="+filepath.Join(workspace, "condarc"))
	command.Stdout = &log
	command.Stderr = &log
	configureProcess(command)
	if err := command.Run(); err != nil {
		return fmt.Errorf("创建 Git 环境失败: %v: %s", err, lastLines(log.String(), 8))
	}

	if info, err := os.Stat(filepath.Join(prefix, "bin", "git")); err != nil || info.IsDir() {
		return fmt.Errorf("conda 环境创建后未找到 bin/git: %s", lastLines(log.String(), 6))
	}

	app.reportRuntimeProgress(func(state *RuntimeProgress) { state.Phase = "替换文件" })
	if err := app.swapRuntimeDir(prefix, target); err != nil {
		return err
	}
	app.appendSystemLog("info", "[runtime] 已通过 micromamba 安装 Git 2.56.1 到 "+filepath.ToSlash(target))
	return nil
}

// mambaInstallSupported 判断当前平台是否可用 micromamba 安装 Git。
func mambaInstallSupported() bool {
	if runtime.GOOS == "windows" {
		return false
	}
	return mambaAssetFor() != nil
}
