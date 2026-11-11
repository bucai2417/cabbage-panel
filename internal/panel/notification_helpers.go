package panel

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func (app *App) helperNotification(writer http.ResponseWriter, request *http.Request) {
	if request.Method != "POST" {
		fail(writer, 405, "仅支持 POST")
		return
	}
	token := strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer ")
	if app.helperKey == "" || subtle.ConstantTimeCompare([]byte(token), []byte(app.helperKey)) != 1 {
		fail(writer, 401, "脚本通知凭证无效")
		return
	}
	var body struct {
		Title   string
		Content string
	}
	if err := decode(request, &body); err != nil {
		apiError(writer, err)
		return
	}
	if len(body.Title) == 0 || len(body.Title) > 512 || len(body.Content) > 65536 {
		fail(writer, 400, "通知标题或内容长度无效")
		return
	}
	app.gate.RLock()
	defer app.gate.RUnlock()
	results := app.sendNotifications(request.Context(), body.Title, body.Content, app.notificationConfig())
	if len(results) == 0 {
		fail(writer, 400, "尚未启用通知渠道，请先配置通知")
		return
	}
	// 脚本来源记为 script，便于在通知日志里区分系统通知与脚本通知。
	app.recordNotifyLog(body.Title, body.Content, results, "script", "")
	ok(writer, results)
}

// notifyPySource 是随版本打包的通知脚本，签名与青龙一致：sendNotify(title, content)。
const notifyPySource = `#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""大白菜面板通知助手，随版本内置。

用法（在任务脚本中）：
    from notify import sendNotify
    sendNotify("标题", "内容")

通知渠道在「配置文件 → 通知配置」中设置；本脚本通过面板注入的
CABBAGE_NOTIFY_URL / CABBAGE_NOTIFY_TOKEN 转发，不读取青龙 config.sh。
"""
import json
import os
import urllib.request


def sendNotify(title, content, *args, **kwargs):
    address = os.environ.get("CABBAGE_NOTIFY_URL", "")
    if not address:
        raise RuntimeError("请在大白菜面板任务中调用通知助手")
    payload = json.dumps({"title": str(title), "content": str(content)}, ensure_ascii=False).encode("utf-8")
    request = urllib.request.Request(
        address,
        data=payload,
        headers={
            "Content-Type": "application/json; charset=utf-8",
            "Authorization": "Bearer " + os.environ.get("CABBAGE_NOTIFY_TOKEN", ""),
        },
    )
    with urllib.request.urlopen(request, timeout=15) as response:
        result = json.loads(response.read().decode("utf-8"))
    failed = [item for item in result["data"] if not item.get("success")]
    if failed:
        raise RuntimeError("; ".join(item.get("name", "") + ": " + item.get("error", "") for item in failed))
    return result["data"]


send = sendNotify

if __name__ == "__main__":
    import sys

    sendNotify(sys.argv[1] if len(sys.argv) > 1 else "通知", sys.argv[2] if len(sys.argv) > 2 else "")
`

// sendNotifyJSSource 是随版本打包的 Node 通知脚本，签名与青龙一致。
const sendNotifyJSSource = `#!/usr/bin/env node
/**
 * 大白菜面板通知助手，随版本内置。
 *
 * 用法（在任务脚本中）：
 *   const { sendNotify } = require('./sendNotify');
 *   await sendNotify('标题', '内容');
 *
 * 通知渠道在「配置文件 → 通知配置」中设置；本脚本通过面板注入的
 * CABBAGE_NOTIFY_URL / CABBAGE_NOTIFY_TOKEN 转发。
 */
async function sendNotify(title, content) {
  const address = process.env.CABBAGE_NOTIFY_URL;
  if (!address) throw new Error('请在大白菜面板任务中调用通知助手');
  const response = await fetch(address, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json; charset=utf-8',
      Authorization: 'Bearer ' + (process.env.CABBAGE_NOTIFY_TOKEN || ''),
    },
    body: JSON.stringify({ title: String(title), content: String(content) }),
    signal: AbortSignal.timeout(15000),
  });
  const result = await response.json();
  if (!response.ok) throw new Error(result.message || '通知失败');
  const failed = (result.data || []).filter(item => !item.success);
  if (failed.length) throw new Error(failed.map(item => item.name + ': ' + item.error).join('; '));
  return result.data;
}

module.exports = { sendNotify, send: sendNotify };

if (require.main === module) {
  sendNotify(process.argv[2] || '通知', process.argv[3] || '').catch(error => {
    console.error(error.message);
    process.exit(1);
  });
}
`

func (app *App) installNotificationHelpers() error {
	files := map[string]string{
		"notify.py":     notifyPySource,
		"sendNotify.js": sendNotifyJSSource,
		"notify": `#!/usr/bin/env bash
exec "$CABBAGE_PYTHON" -c 'import sys; from notify import sendNotify; sendNotify(sys.argv[1], sys.argv[2])' "$@"
`,
	}
	for name, content := range files {
		target, err := safePath(filepath.Join(app.root, "bin"), name)
		if err != nil {
			return err
		}
		if err := os.WriteFile(target, []byte(content), 0700); err != nil {
			return err
		}
	}
	// 同时投放一份到 scripts 目录，方便任务直接 import / require。
	// 已存在时不覆盖，避免覆盖用户修改过的版本。
	scriptDir := filepath.Join(app.root, "scripts")
	if err := os.MkdirAll(scriptDir, 0700); err != nil {
		return err
	}
	for name, content := range map[string]string{"notify.py": notifyPySource, "sendNotify.js": sendNotifyJSSource} {
		target := filepath.Join(scriptDir, name)
		if _, statErr := os.Lstat(target); statErr == nil {
			continue
		}
		if err := os.WriteFile(target, []byte(content), 0644); err != nil {
			return err
		}
	}
	return nil
}

func validateNotificationDestination(channel NotificationChannel) error {
	if channel.Type == "serverchan" && strings.HasPrefix(channel.Token, "sctp") && channel.URL == "" {
		return errors.New("Server酱 Turbo（sctp）请提供完整发送 URL")
	}
	return nil
}
