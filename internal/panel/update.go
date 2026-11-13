package panel

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type ReleaseAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}
type Release struct {
	Tag    string         `json:"tag_name"`
	Body   string         `json:"body"`
	URL    string         `json:"html_url"`
	Assets []ReleaseAsset `json:"assets"`
}
type UpdateAsset struct {
	OS     string `json:"os"`
	Arch   string `json:"arch"`
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
}
type UpdateManifest struct {
	Version string        `json:"version"`
	Assets  []UpdateAsset `json:"assets"`
}

// UpdateProgress 是后台下载更新的实时状态快照，供前端轮询显示真实进度。
type UpdateProgress struct {
	Running  bool   `json:"running"`
	Done     bool   `json:"done"`
	Phase    string `json:"phase"`
	Version  string `json:"version"`
	Received int64  `json:"received"`
	Total    int64  `json:"total"`
	Percent  int    `json:"percent"`
	Message  string `json:"message"`
	Error    string `json:"error"`
}

func (app *App) reportUpdateProgress(mutate func(*UpdateProgress)) {
	app.updateMu.Lock()
	mutate(&app.updateState)
	app.updateMu.Unlock()
}

func (app *App) currentUpdateProgress() UpdateProgress {
	app.updateMu.Lock()
	defer app.updateMu.Unlock()
	return app.updateState
}

func newerVersion(candidate, current string) bool {
	parse := func(value string) ([3]int, bool) {
		parts := strings.Split(strings.TrimPrefix(value, "v"), ".")
		result := [3]int{}
		if len(parts) != 3 {
			return result, false
		}
		for index, part := range parts {
			number, err := strconv.Atoi(part)
			if err != nil || number < 0 {
				return result, false
			}
			result[index] = number
		}
		return result, true
	}
	next, valid := parse(candidate)
	if !valid {
		return false
	}
	previous, valid := parse(current)
	if !valid {
		return false
	}
	for index := range next {
		if next[index] > previous[index] {
			return true
		}
		if next[index] < previous[index] {
			return false
		}
	}
	return false
}

func (app *App) latestRelease(ctx context.Context) (Release, error) {
	var release Release
	repo := app.store.setting("githubRepo", "")
	if repo == "" {
		return release, errors.New("请先设置 GitHub 仓库 owner/repo")
	}
	request, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/repos/"+repo+"/releases/latest", nil)
	if err != nil {
		return release, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "cabbage-panel/"+app.version)
	response, err := app.client.Do(request)
	if err != nil {
		return release, err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return release, fmt.Errorf("GitHub 返回 HTTP %d", response.StatusCode)
	}
	err = json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&release)
	return release, err
}
func releaseURL(release Release, name string) (string, error) {
	for _, asset := range release.Assets {
		if asset.Name == name {
			parsed, err := url.Parse(asset.URL)
			if err != nil || parsed.Scheme != "https" || parsed.Host != "github.com" {
				return "", errors.New("Release 下载来源无效")
			}
			return asset.URL, nil
		}
	}
	return "", fmt.Errorf("Release 缺少 %s", name)
}
func (app *App) fetchBytes(ctx context.Context, address string, limit int64) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, "GET", address, nil)
	if err != nil {
		return nil, err
	}
	response, err := app.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("下载失败 HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("下载文件超过大小上限")
	}
	return data, nil
}

// fetchBytesProgress 在下载过程中按块回调已接收字节与总长度，用于展示真实进度。
func (app *App) fetchBytesProgress(ctx context.Context, address string, limit int64, onProgress func(received, total int64)) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, "GET", address, nil)
	if err != nil {
		return nil, err
	}
	response, err := app.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("下载失败 HTTP %d", response.StatusCode)
	}
	total := response.ContentLength
	if onProgress != nil {
		onProgress(0, total)
	}
	var buffer bytes.Buffer
	chunk := make([]byte, 64*1024)
	var received int64
	for {
		read, readErr := response.Body.Read(chunk)
		if read > 0 {
			received += int64(read)
			if received > limit {
				return nil, errors.New("下载文件超过大小上限")
			}
			buffer.Write(chunk[:read])
			if onProgress != nil {
				onProgress(received, total)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, readErr
		}
	}
	return buffer.Bytes(), nil
}

func (app *App) updateAPI(writer http.ResponseWriter, request *http.Request, tail string) {
	if tail == "" && request.Method == "GET" {
		release, err := app.latestRelease(request.Context())
		if err != nil {
			apiError(writer, err)
			return
		}
		ok(writer, Object{"current": app.version, "latest": release.Tag, "available": newerVersion(release.Tag, app.version), "body": release.Body, "url": release.URL, "signedUpdates": app.updateKey != "", "staged": app.store.setting("stagedVersion", ""), "restartMode": restartMode()})
		return
	}
	if tail == "/progress" && request.Method == "GET" {
		ok(writer, app.currentUpdateProgress())
		return
	}
	if tail == "/download" && request.Method == "POST" {
		if app.updateKey == "" {
			fail(writer, 409, "当前构建未嵌入发布公钥；请先按发布文档配置签名构建")
			return
		}
		if app.currentUpdateProgress().Running {
			fail(writer, 409, "已有更新正在下载，请稍候")
			return
		}
		app.reportUpdateProgress(func(state *UpdateProgress) {
			*state = UpdateProgress{Running: true, Phase: "准备下载"}
		})
		go func() {
			err := app.stageUpdate(context.Background(), io.Discard)
			app.reportUpdateProgress(func(state *UpdateProgress) {
				state.Running = false
				state.Done = true
				if err != nil {
					state.Phase = "失败"
					state.Error = err.Error()
					return
				}
				state.Phase = "完成"
				state.Percent = 100
				if state.Version == "" {
					state.Version = strings.TrimPrefix(app.store.setting("stagedVersion", ""), "v")
				}
			})
		}()
		ok(writer, Object{"started": true})
		return
	}
	if tail == "/apply" && request.Method == "POST" {
		if app.engine.active() > 0 {
			fail(writer, 409, "请等待所有任务完成后应用更新")
			return
		}
		version := app.store.setting("stagedVersion", "")
		if version == "" {
			fail(writer, 409, "没有已验证的更新")
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
		staged := filepath.Join(app.root, "updates", "cabbage.next")
		data, err := os.ReadFile(staged)
		if err != nil {
			apiError(writer, err)
			return
		}
		digest := sha256.Sum256(data)
		if hex.EncodeToString(digest[:]) != app.store.setting("stagedSHA256", "") {
			fail(writer, 400, "暂存文件校验失败，请重新下载")
			return
		}
		restartLabel := "服务将自动重启并加载新版本"
		if inContainer() || underSystemd() {
			if err := inPlaceReplace(executable, data); err != nil {
				if inContainer() {
					fail(writer, 500, "容器内替换程序失败："+err.Error()+"；请在宿主机拉取新镜像后重建容器")
				} else {
					fail(writer, 500, "替换程序失败："+err.Error()+"；请确认服务用户对程序目录有写权限")
				}
				return
			}
			if inContainer() {
				restartLabel = "已在容器内替换程序，服务将退出并依赖容器 restart 策略重启"
			} else {
				restartLabel = "已替换程序，服务将退出并由 systemd 重新拉起（需 Restart=always）"
			}
		} else {
			message, err := nativeRestart(executable, data, os.Args[1:])
			if err != nil {
				apiError(writer, err)
				return
			}
			restartLabel = message
		}
		_ = os.Remove(staged)
		_ = app.store.setSetting("stagedVersion", "")
		ok(writer, Object{"version": version, "restartRequired": false, "restarting": true, "mode": restartMode(), "message": restartLabel})
		app.scheduleShutdown(600 * time.Millisecond)
		return
	}
	fail(writer, 404, "更新接口不存在")
}

func (app *App) stageUpdate(ctx context.Context, writer io.Writer) error {
	key, err := base64.StdEncoding.DecodeString(app.updateKey)
	if err != nil || len(key) != ed25519.PublicKeySize {
		return errors.New("发布公钥无效")
	}
	app.reportUpdateProgress(func(state *UpdateProgress) { state.Phase = "检查最新版本" })
	release, err := app.latestRelease(ctx)
	if err != nil {
		return err
	}
	if !newerVersion(release.Tag, app.version) {
		return errors.New("当前已经是最新稳定版本")
	}
	app.reportUpdateProgress(func(state *UpdateProgress) { state.Phase = "下载签名清单" })
	manifestURL, err := releaseURL(release, "manifest.json")
	if err != nil {
		return err
	}
	signatureURL, err := releaseURL(release, "manifest.sig")
	if err != nil {
		return err
	}
	manifestData, err := app.fetchBytes(ctx, manifestURL, 1<<20)
	if err != nil {
		return err
	}
	signatureData, err := app.fetchBytes(ctx, signatureURL, 4096)
	if err != nil {
		return err
	}
	app.reportUpdateProgress(func(state *UpdateProgress) { state.Phase = "校验发布签名" })
	signature, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(signatureData)))
	if err != nil || !ed25519.Verify(key, manifestData, signature) {
		return errors.New("Release 签名校验失败，更新已拒绝")
	}
	var manifest UpdateManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return err
	}
	if strings.TrimPrefix(release.Tag, "v") != manifest.Version {
		return errors.New("Release 版本与签名清单不一致")
	}
	for _, asset := range manifest.Assets {
		if asset.OS == runtime.GOOS && asset.Arch == runtime.GOARCH {
			address, err := releaseURL(release, asset.Name)
			if err != nil {
				return err
			}
			fmt.Fprintln(writer, "下载", asset.Name)
			app.reportUpdateProgress(func(state *UpdateProgress) {
				state.Phase = "下载程序"
				state.Version = manifest.Version
				state.Received = 0
				state.Total = 0
				state.Percent = 0
			})
			data, err := app.fetchBytesProgress(ctx, address, 128<<20, func(received, total int64) {
				app.reportUpdateProgress(func(state *UpdateProgress) {
					state.Received = received
					state.Total = total
					if total > 0 {
						state.Percent = int(received * 100 / total)
					}
				})
			})
			if err != nil {
				return err
			}
			app.reportUpdateProgress(func(state *UpdateProgress) { state.Phase = "校验程序文件"; state.Percent = 100 })
			digest := sha256.Sum256(data)
			if !strings.EqualFold(hex.EncodeToString(digest[:]), asset.SHA256) {
				return errors.New("程序 SHA-256 校验失败")
			}
			if err := atomicWrite(filepath.Join(app.root, "updates", "cabbage.next"), data); err != nil {
				return err
			}
			if err := app.store.setSetting("stagedSHA256", asset.SHA256); err != nil {
				return err
			}
			if err := app.store.setSetting("stagedVersion", manifest.Version); err != nil {
				return err
			}
			fmt.Fprintln(writer, "签名和文件校验通过，版本", manifest.Version, "已就绪")
			return nil
		}
	}
	return errors.New("Release 不包含当前平台")
}
