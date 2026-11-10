package panel

import (
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

func (app *App) updateAPI(writer http.ResponseWriter, request *http.Request, tail string) {
	if tail == "" && request.Method == "GET" {
		release, err := app.latestRelease(request.Context())
		if err != nil {
			apiError(writer, err)
			return
		}
		ok(writer, Object{"current": app.version, "latest": release.Tag, "available": newerVersion(release.Tag, app.version), "body": release.Body, "url": release.URL, "signedUpdates": app.updateKey != "", "staged": app.store.setting("stagedVersion", "")})
		return
	}
	if tail == "/download" && request.Method == "POST" {
		if app.updateKey == "" {
			fail(writer, 409, "当前构建未嵌入发布公钥；请先按发布文档配置签名构建")
			return
		}
		items, _ := app.store.list("updates")
		var id int64
		for _, item := range items {
			id = integer(item["id"])
			break
		}
		if id == 0 {
			item, err := app.store.save("updates", Object{"name": "下载并验证版本更新", "timeout": 600})
			if err != nil {
				apiError(writer, err)
				return
			}
			id = integer(item["id"])
		}
		job, err := app.engine.enqueue("updates", id)
		if err != nil {
			apiError(writer, err)
			return
		}
		ok(writer, job)
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
		next := executable + ".next"
		if err := atomicWrite(next, data); err != nil {
			apiError(writer, err)
			return
		}
		if err := os.Chmod(next, 0755); err != nil {
			apiError(writer, err)
			return
		}
		previous := executable + ".previous"
		if _, err := os.Stat(previous); err == nil {
			if err := os.Remove(previous); err != nil {
				apiError(writer, err)
				return
			}
		}
		if err := os.Rename(executable, previous); err != nil {
			apiError(writer, err)
			return
		}
		if err := os.Rename(next, executable); err != nil {
			_ = os.Rename(previous, executable)
			apiError(writer, err)
			return
		}
		_ = app.store.setSetting("stagedVersion", "")
		ok(writer, Object{"version": version, "restartRequired": true, "message": "已替换程序，重启服务后生效；旧程序保存为 .previous"})
		return
	}
	fail(writer, 404, "更新接口不存在")
}

func (app *App) stageUpdate(ctx context.Context, writer io.Writer) error {
	key, err := base64.StdEncoding.DecodeString(app.updateKey)
	if err != nil || len(key) != ed25519.PublicKeySize {
		return errors.New("发布公钥无效")
	}
	release, err := app.latestRelease(ctx)
	if err != nil {
		return err
	}
	if !newerVersion(release.Tag, app.version) {
		return errors.New("当前已经是最新稳定版本")
	}
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
			data, err := app.fetchBytes(ctx, address, 128<<20)
			if err != nil {
				return err
			}
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
