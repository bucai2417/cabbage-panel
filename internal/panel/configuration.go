package panel

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var settingDefaults = map[string]string{"concurrency": "4", "timeout": "3600", "retentionDays": "30", "githubRepo": "bucai2417/cabbage-panel", "webhookURL": "", "notifyFailures": "true", "npmMirror": "", "pythonMirror": "", "linuxMirror": "", "pythonPath": "", "pythonVersion": "", "runtimePythonMirror": "", "runtimeNodeMirror": "", "runtimeGitMirror": "", "runtimeMambaMirror": "", "runtimeMambaChannel": "", "trustedProxies": "", "outboundAllowlist": "", "autoInstallDeps": "false", "allowUnsignedUpdates": "false", "ipLocationEnabled": "true", "ipLocationURL": "", "autoBackupEnabled": "false", "autoBackupTime": "03:00"}

func (app *App) settings() Object {
	result := Object{}
	for key, fallback := range settingDefaults {
		result[key] = app.store.setting(key, fallback)
	}
	return result
}

func (app *App) saveSettings(body Object) error {
	for key, value := range body {
		if _, found := settingDefaults[key]; !found {
			return fmt.Errorf("不支持的设置项: %s", key)
		}
		switch key {
		case "concurrency", "timeout", "retentionDays":
			maximum := int64(32)
			if key == "timeout" {
				maximum = 604800
			}
			if key == "retentionDays" {
				maximum = 3650
			}
			number := integer(value)
			if number < 1 || number > maximum || text(value) != fmt.Sprint(number) {
				return fmt.Errorf("%s 需为 1–%d 的整数", key, maximum)
			}
		case "githubRepo":
			if text(value) != "" && !regexp.MustCompile("^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$").MatchString(text(value)) {
				return errors.New("仓库格式为 owner/repo")
			}
		case "webhookURL":
			if text(value) != "" && !validHTTP(text(value)) {
				return errors.New("Webhook 地址无效")
			}
		case "notifyFailures", "autoInstallDeps", "allowUnsignedUpdates", "ipLocationEnabled", "autoBackupEnabled":
			if text(value) != "true" && text(value) != "false" {
				return fmt.Errorf("%s 只能为 true 或 false", key)
			}
		case "autoBackupTime":
			// 每日自动备份时间，形如 03:00。
			parsed, err := time.Parse("15:04", strings.TrimSpace(text(value)))
			if err != nil || parsed.Format("15:04") != strings.TrimSpace(text(value)) {
				return errors.New("自动备份时间格式应为 HH:MM，例如 03:00")
			}
		case "npmMirror", "pythonMirror", "linuxMirror", "runtimePythonMirror", "runtimeNodeMirror", "runtimeGitMirror", "runtimeMambaMirror":
			if address := strings.TrimSpace(text(value)); address != "" && !validHTTP(address) {
				return errors.New("镜像地址必须是无内嵌凭证的 HTTP(S) URL")
			}
		case "runtimeMambaChannel":
			// conda 频道：空表示 conda-forge；非空必须是合法 URL，便于指向国内镜像。
			if address := strings.TrimSpace(text(value)); address != "" && !validHTTP(address) {
				return errors.New("conda 频道必须是无内嵌凭证的 HTTP(S) URL")
			}
		case "pythonPath":
			// 留空表示自动检测；非空必须是存在的文件。
			if path := strings.TrimSpace(text(value)); path != "" {
				info, err := os.Stat(path)
				if err != nil || info.IsDir() {
					return errors.New("Python 路径不存在或不是可执行文件")
				}
			}
		case "pythonVersion":
			// 形如 3.10、3.12；留空表示不指定。
			if version := strings.TrimSpace(text(value)); version != "" && !regexp.MustCompile(`^\d+\.\d{1,2}$`).MatchString(version) {
				return errors.New("Python 版本格式应为 3.10 这样的「主.次」")
			}
		case "ipLocationURL":
			// 归属地接口模板，{ip} 会被替换成目标地址；留空使用内置 HTTPS 接口。
			address := strings.ReplaceAll(strings.TrimSpace(text(value)), "{ip}", "1.1.1.1")
			if address != "" && !validHTTP(address) {
				return errors.New("归属地接口必须是无内嵌凭证的 HTTP(S) URL，可用 {ip} 占位")
			}
		case "trustedProxies":
			// 逗号分隔的 IP 或 CIDR；只有这些地址转发的 X-Forwarded-* 会被信任。
			if err := validateTrustedProxies(text(value)); err != nil {
				return err
			}
		case "outboundAllowlist":
			// 逗号分隔的 host 或 host:port；仅用于显式放行的局域网地址。
			if !validOutboundAllowlist(text(value)) {
				return errors.New("出站白名单需为逗号分隔的 host 或 host:port")
			}
		}
	}
	transaction, err := app.store.db.Begin()
	if err != nil {
		return err
	}
	defer transaction.Rollback()
	for key, value := range body {
		if _, err := transaction.Exec("INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", key, text(value)); err != nil {
			return err
		}
	}
	return transaction.Commit()
}

func validHTTP(address string) bool {
	parsed, err := url.Parse(address)
	return err == nil && contains([]string{"http", "https"}, parsed.Scheme) && parsed.Host != "" && parsed.User == nil && parsed.Fragment == ""
}

func builtinConfig(name string) bool {
	name = filepath.ToSlash(filepath.Clean(name))
	return name == "system.json" || name == "notifications.json"
}

func (app *App) builtinContent(name string) (string, error) {
	name = filepath.ToSlash(filepath.Clean(name))
	var value any
	switch name {
	case "system.json":
		settings := app.settings()
		delete(settings, "webhookURL")
		delete(settings, "notifyFailures")
		value = settings
	case "notifications.json":
		value = app.notificationConfig()
	default:
		return "", errors.New("内置配置不存在")
	}
	data, err := json.MarshalIndent(value, "", "  ")
	return string(data) + "\n", err
}

func (app *App) builtinConfigEntries() []Object {
	result := []Object{}
	for _, name := range []string{"system.json", "notifications.json"} {
		content, _ := app.builtinContent(name)
		result = append(result, Object{
			"title": name, "value": name, "key": name, "type": "file", "parent": "", "size": len(content),
			"name": name, "path": name, "isLeaf": true, "builtin": true,
		})
	}
	return result
}

func (app *App) writeBuiltin(name, content string) error {
	name = filepath.ToSlash(filepath.Clean(name))
	if name == "system.json" {
		var body Object
		if err := json.Unmarshal([]byte(content), &body); err != nil || body == nil {
			return errors.New("系统配置必须为有效 JSON 对象")
		}
		for key := range body {
			if key == "webhookURL" || key == "notifyFailures" {
				return errors.New("请在 notifications.json 修改通知配置")
			}
		}
		return app.saveSettings(body)
	}
	if !json.Valid([]byte(content)) {
		return errors.New("通知 JSON 格式无效")
	}
	var config NotificationConfig
	decoder := json.NewDecoder(strings.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return fmt.Errorf("通知 JSON 配置无效: %w", err)
	}
	return app.saveNotifications(config)
}

func (app *App) builtinConfigAPI(writer http.ResponseWriter, request *http.Request, tail string) bool {
	if request.Method == "GET" {
		name := tail
		if tail == "detail" {
			name = request.URL.Query().Get("path")
			if filename := request.URL.Query().Get("filename"); filename != "" {
				name = strings.Trim(request.URL.Query().Get("path")+"/"+filename, "/")
			}
		}
		if !builtinConfig(name) {
			return false
		}
		content, err := app.builtinContent(name)
		if err != nil {
			apiError(writer, err)
		} else {
			ok(writer, content)
		}
		return true
	}
	return false
}

func (app *App) initializeConfigs() error {
	for _, name := range []string{"config.sh", "task_before.sh", "task_after.sh"} {
		target, err := safePath(filepath.Join(app.root, "config"), name)
		if err != nil {
			return err
		}
		if _, err := os.Stat(target); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := atomicWrite(target, []byte("\n")); err != nil {
			return err
		}
	}
	if app.store.setting("githubRepo", "") == "" {
		return app.store.setSetting("githubRepo", "bucai2417/cabbage-panel")
	}
	return nil
}
