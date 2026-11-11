package panel

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// notifyLogs 返回最近的通知发送记录，形状对齐青龙 /api/system/notify-log。
func (app *App) notifyLogs() []Object {
	rows, err := app.store.db.Query(`SELECT id, timestamp, title, content, status, notify_type, source, script_name FROM notify_logs ORDER BY timestamp DESC, id DESC LIMIT 100`)
	if err != nil {
		return []Object{}
	}
	defer rows.Close()
	result := []Object{}
	for rows.Next() {
		var id, timestamp, status int64
		var title, content, notifyType, source, scriptName string
		if rows.Scan(&id, &timestamp, &title, &content, &status, &notifyType, &source, &scriptName) == nil {
			result = append(result, Object{
				"id":         id,
				"timestamp":  timestamp,
				"title":      title,
				"content":    content,
				"status":     status,
				"notifyType": notifyType,
				"source":     source,
				"scriptName": scriptName,
			})
		}
	}
	return result
}

// systemLogPath 是系统运行日志落盘位置。
func (app *App) systemLogPath() string {
	return filepath.Join(app.root, "logs", "system.log")
}

// appendSystemLog 追加一行系统日志；调用方已格式化好正文。
func (app *App) appendSystemLog(level, message string) {
	file, err := os.OpenFile(app.systemLogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return
	}
	defer file.Close()
	stamp := time.Now().Format("2006-01-02 15:04:05")
	_, _ = file.WriteString("[" + level + " " + stamp + "]:\t" + message + "\n")
}

// systemLogAPI 处理 GET（读取日志文本）与 DELETE（清空）。
func (app *App) systemLogAPI(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case "GET":
		content, err := os.ReadFile(app.systemLogPath())
		if err != nil {
			// 没有日志文件时返回空文本，不视为错误。
			ok(writer, "")
			return
		}
		text := string(content)
		// 与青龙一致：默认只返回末尾 2MB，避免一次拉取过大。
		if len(text) > 2<<20 {
			text = text[len(text)-(2<<20):]
		}
		ok(writer, strings.ToValidUTF8(text, "�"))
	case "DELETE":
		if err := os.Remove(app.systemLogPath()); err != nil && !os.IsNotExist(err) {
			apiError(writer, err)
			return
		}
		ok(writer, nil)
	default:
		fail(writer, 405, "方法不支持")
	}
}

// logStartup 在启动时记录一条系统日志。
func (app *App) logStartup(version string) {
	app.appendSystemLog("info", "[boot] 大白菜面板 v"+version+" 启动")
	app.appendSystemLog("info", "[boot] 数据目录 "+app.root)
	app.appendSystemLog("info", "[boot] HTTP 服务就绪")
}

// pruneSystemLog 限制系统日志大小，超过 4MB 时截断保留末尾 2MB。
func (app *App) pruneSystemLog() {
	info, err := os.Stat(app.systemLogPath())
	if err != nil || info.Size() <= 4<<20 {
		return
	}
	file, err := os.Open(app.systemLogPath())
	if err != nil {
		return
	}
	defer file.Close()
	if _, err := file.Seek(-(2 << 20), io.SeekEnd); err != nil {
		return
	}
	tail, err := io.ReadAll(io.LimitReader(file, 2<<20))
	if err != nil {
		return
	}
	_ = os.WriteFile(app.systemLogPath(), tail, 0600)
}

// sortedStrings 便于稳定输出（保留给后续扩展使用）。
func sortedStrings(values []string) []string {
	result := append([]string{}, values...)
	sort.Strings(result)
	return result
}
