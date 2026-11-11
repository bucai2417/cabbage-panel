package panel

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

func readText(path string, limit int64) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	bytes, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return "", err
	}
	if int64(len(bytes)) > limit {
		return "", errors.New("文件过大")
	}
	if !utf8.Valid(bytes) {
		return "", errors.New("文件不是 UTF-8 编码，请先转换编码")
	}
	return string(bytes), nil
}
func atomicWrite(path string, bytes []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".cabbage-write-*")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if err := file.Chmod(0600); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(bytes); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
func fileName(body Object) string {
	name := text(body["filename"])
	if name == "" {
		name = text(body["name"])
	}
	directory := text(body["path"])
	if directory != "" {
		return strings.TrimSuffix(directory, "/") + "/" + name
	}
	return name
}

func (app *App) filesAPI(writer http.ResponseWriter, request *http.Request, kind, tail string) {
	dir := "scripts"
	if kind == "configs" {
		dir = "config"
	}
	root := filepath.Join(app.root, dir)
	if kind == "configs" && app.builtinConfigAPI(writer, request, tail) {
		return
	}
	// 青龙 GET /configs/samples：示例配置文件名列表。
	if kind == "configs" && tail == "samples" && request.Method == "GET" {
		ok(writer, []string{"config.sh", "extra.sh", "task_before.sh", "task_after.sh", "task_before.js", "task_before.py"})
		return
	}
	if app.fileOperations(writer, request, kind, tail, root) {
		return
	}
	if request.Method == "POST" && tail == "upload" {
		// 本地文件上传：multipart 的 file 字段，可带 path 目标目录。
		if err := request.ParseMultipartForm(32 << 20); err != nil {
			fail(writer, 400, "请使用 multipart/form-data 上传文件")
			return
		}
		file, header, err := request.FormFile("file")
		if err != nil {
			fail(writer, 400, "缺少 file 字段")
			return
		}
		defer file.Close()
		if header.Size > 32<<20 {
			fail(writer, 400, "仅支持 32MB 以内的文件")
			return
		}
		directory := strings.Trim(strings.TrimSpace(request.FormValue("path")), "/")
		name := filepath.Base(header.Filename)
		if name == "" || name == "." || name == ".." {
			fail(writer, 400, "文件名无效")
			return
		}
		relative := name
		if directory != "" {
			relative = directory + "/" + name
		}
		target, err := safePath(root, relative)
		if err != nil {
			apiError(writer, err)
			return
		}
		if _, statErr := os.Lstat(target); statErr == nil {
			fail(writer, 409, "文件已存在，不会覆盖")
			return
		}
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			apiError(writer, err)
			return
		}
		data, err := io.ReadAll(io.LimitReader(file, 32<<20))
		if err != nil {
			apiError(writer, err)
			return
		}
		if err := atomicWrite(target, data); err != nil {
			apiError(writer, err)
			return
		}
		ok(writer, Object{"path": relative, "size": len(data)})
		return
	}
	if request.Method == "POST" && tail == "archive" {
		// 批量下载：请求体为路径数组，打包成 ZIP 返回。
		var paths []string
		if err := decode(request, &paths); err != nil {
			apiError(writer, err)
			return
		}
		if len(paths) == 0 || len(paths) > 500 {
			fail(writer, 400, "请选择 1–500 个文件")
			return
		}
		writer.Header().Set("Content-Type", "application/zip")
		writer.Header().Set("Content-Disposition", "attachment; filename="+kind+"-"+time.Now().Format("20060102-150405")+".zip")
		archive := zip.NewWriter(writer)
		defer archive.Close()
		for _, relative := range paths {
			source, err := safePath(root, relative)
			if err != nil {
				continue
			}
			info, err := os.Stat(source)
			if err != nil || info.IsDir() {
				continue
			}
			entry, err := archive.Create(filepath.ToSlash(relative))
			if err != nil {
				continue
			}
			file, err := os.Open(source)
			if err != nil {
				continue
			}
			_, _ = io.Copy(entry, io.LimitReader(file, 64<<20))
			file.Close()
		}
		return
	}
	if (tail == "" || tail == "files") && request.Method == "GET" {
		result := []Object{}
		if kind == "configs" {
			result = append(result, app.builtinConfigEntries()...)
		}
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if path == root {
				return nil
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return nil
			}
			if entry.IsDir() {
				if strings.HasPrefix(entry.Name(), ".") || entry.Name() == "node_modules" || entry.Name() == "__pycache__" {
					return filepath.SkipDir
				}
				return nil
			}
			relative, _ := filepath.Rel(root, path)
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if strings.Contains(strings.ToLower(relative), strings.ToLower(request.URL.Query().Get("searchValue"))) {
				slash := filepath.ToSlash(relative)
				parentDir := filepath.ToSlash(filepath.Dir(relative))
				if parentDir == "." {
					parentDir = ""
				}
				// 超集字段：青龙的 title/key/type/parent/size/createTime + 面板的 name/path/isLeaf。
				entry := Object{
					"title":      entry.Name(),
					"key":        slash,
					"type":       "file",
					"parent":     parentDir,
					"size":       info.Size(),
					"createTime": info.ModTime().UnixMilli(),
					"name":       entry.Name(),
					"path":       slash,
					"isLeaf":     true,
					"updatedAt":  info.ModTime().Format(time.RFC3339),
				}
				// 青龙 /configs/files 是下拉选项，需要 value 字段。
				if kind == "configs" {
					entry["value"] = slash
				}
				result = append(result, entry)
			}
			if len(result) > 10000 {
				return errors.New("文件数量超过 10000")
			}
			return nil
		})
		if err != nil {
			apiError(writer, err)
			return
		}
		ok(writer, result)
		return
	}
	if tail == "history" && request.Method == "GET" {
		items, err := app.store.list("history")
		if err != nil {
			apiError(writer, err)
			return
		}
		result := []Object{}
		for _, item := range items {
			if text(item["kind"]) == kind && text(item["path"]) == request.URL.Query().Get("path") {
				delete(item, "content")
				result = append(result, item)
			}
		}
		ok(writer, result)
		return
	}
	if tail == "history/detail" && request.Method == "GET" {
		item, err := app.store.get("history", integer(request.URL.Query().Get("id")))
		if err != nil {
			apiError(writer, err)
			return
		}
		if text(item["kind"]) != kind {
			fail(writer, 403, "无权限")
			return
		}
		ok(writer, item)
		return
	}
	if tail == "history/restore" && request.Method == "PUT" {
		var body Object
		if err := decode(request, &body); err != nil {
			apiError(writer, err)
			return
		}
		item, err := app.store.get("history", integer(body["id"]))
		if err != nil {
			apiError(writer, err)
			return
		}
		if text(item["kind"]) != kind {
			fail(writer, 403, "无权限")
			return
		}
		path, err := safePath(root, text(item["path"]))
		if err != nil {
			apiError(writer, err)
			return
		}
		previous, readErr := readText(path, 4<<20)
		if kind == "configs" && builtinConfig(text(item["path"])) {
			previous, readErr = app.builtinContent(text(item["path"]))
		}
		if readErr == nil {
			if _, err := app.store.save("history", Object{"kind": kind, "path": item["path"], "content": previous}); err != nil {
				apiError(writer, err)
				return
			}
		}
		if kind == "configs" && builtinConfig(text(item["path"])) {
			err = app.writeBuiltin(text(item["path"]), text(item["content"]))
		} else {
			err = atomicWrite(path, []byte(text(item["content"])))
		}
		if err != nil {
			apiError(writer, err)
			return
		}
		ok(writer, nil)
		return
	}
	if request.Method == "GET" {
		relative := tail
		if tail == "detail" {
			relative = request.URL.Query().Get("path")
			if name := request.URL.Query().Get("filename"); name != "" {
				relative = strings.TrimSuffix(relative, "/") + "/" + name
				relative = strings.TrimPrefix(relative, "/")
			}
		} else if directory := request.URL.Query().Get("path"); directory != "" {
			relative = directory + "/" + tail
		}
		path, err := safePath(root, relative)
		if err != nil {
			apiError(writer, err)
			return
		}
		content, err := readText(path, 4<<20)
		if err != nil {
			apiError(writer, err)
			return
		}
		ok(writer, content)
		return
	}
	var body Object
	if err := decode(request, &body); err != nil {
		apiError(writer, err)
		return
	}
	relative := fileName(body)
	if relative == "" {
		relative = text(body["file"])
	}
	path, err := safePath(root, relative)
	if err != nil {
		apiError(writer, err)
		return
	}
	if kind == "configs" && builtinConfig(relative) {
		if request.Method != "PUT" || !contains([]string{"", "save"}, tail) {
			fail(writer, 400, "内置配置仅支持编辑保存")
			return
		}
		old, _ := app.builtinContent(relative)
		if err := app.writeBuiltin(relative, text(body["content"])); err != nil {
			apiError(writer, err)
			return
		}
		_, _ = app.store.save("history", Object{"kind": kind, "path": relative, "content": old})
		ok(writer, Object{"path": relative})
		return
	}
	if tail == "run" && request.Method == "PUT" && kind == "scripts" {
		if _, err := os.Stat(path); err != nil {
			apiError(writer, err)
			return
		}
		result, err := app.engine.enqueueAdhoc("scripts", scriptRunTaskID(relative), "脚本运行 · "+relative, "task "+shellQuote(relative), Object{"timeout": 0}, "")
		if err != nil {
			apiError(writer, err)
			return
		}
		ok(writer, result)
		return
	}
	if tail == "debug" && request.Method == "POST" && kind == "scripts" {
		content := text(body["content"])
		if int64(len(content)) > 4<<20 {
			fail(writer, 400, "调试内容过大")
			return
		}
		base := filepath.Base(relative)
		extension := strings.ToLower(filepath.Ext(base))
		if extension == ".bash" {
			extension = ".sh"
		}
		if !contains([]string{".py", ".js", ".mjs", ".cjs", ".sh"}, extension) {
			fail(writer, 400, "调试仅支持 Python、JavaScript 和 Shell 脚本")
			return
		}
		base = strings.TrimSuffix(base, filepath.Ext(base)) + extension
		directory, err := os.MkdirTemp(filepath.Join(app.root, "debug"), "debug-")
		if err != nil {
			apiError(writer, err)
			return
		}
		target := filepath.Join(directory, base)
		if err := os.WriteFile(target, []byte(content), 0600); err != nil {
			_ = os.RemoveAll(directory)
			apiError(writer, err)
			return
		}
		result, err := app.engine.enqueueInteractive("scripts", scriptRunTaskID(relative), "调试 · "+relative, "task "+shellQuote(filepath.ToSlash(target)), Object{"timeout": 0}, directory)
		if err != nil {
			_ = os.RemoveAll(directory)
			apiError(writer, err)
			return
		}
		ok(writer, result)
		return
	}
	if tail == "stop" && request.Method == "PUT" {
		app.engine.stop("scripts", scriptRunTaskID(relative), 0)
		items, _ := app.store.list("crons")
		for _, item := range items {
			if text(item["command"]) == "task "+shellQuote(relative) {
				app.engine.stop("crons", integer(item["id"]), 0)
			}
		}
		ok(writer, nil)
		return
	}
	if tail == "rename" && request.Method == "PUT" {
		count, err := app.operateFile(root, kind, relative, text(body["newName"]), "rename")
		if err != nil {
			apiError(writer, err)
			return
		}
		ok(writer, Object{"updatedTasks": count})
		return
	}
	if request.Method == "DELETE" && tail == "" {
		info, err := os.Stat(path)
		if err != nil {
			apiError(writer, err)
			return
		}
		if info.IsDir() {
			fail(writer, 400, "只能删除单个文件")
			return
		}
		if err := os.Remove(path); err != nil {
			apiError(writer, err)
			return
		}
		ok(writer, nil)
		return
	}
	if (request.Method == "POST" || request.Method == "PUT") && contains([]string{"", "save", "download"}, tail) {
		if request.Method == "POST" && tail == "" {
			if _, err := os.Lstat(path); err == nil {
				fail(writer, 409, "文件已存在，不会覆盖")
				return
			}
		}
		content := text(body["content"])
		if tail == "download" {
			download, err := app.downloadText(request.Context(), text(body["url"]), 4<<20)
			if err != nil {
				apiError(writer, err)
				return
			}
			content = download
		}
		if !utf8.ValidString(content) || len(content) > 4<<20 {
			fail(writer, 400, "仅支持 4MB 以内的 UTF-8 文本")
			return
		}
		if old, err := readText(path, 4<<20); err == nil {
			_, err = app.store.save("history", Object{"kind": kind, "path": relative, "content": old})
			if err != nil {
				apiError(writer, err)
				return
			}
			_, _ = app.store.db.Exec("DELETE FROM records WHERE kind='history' AND id NOT IN (SELECT id FROM records WHERE kind='history' ORDER BY id DESC LIMIT 100)")
		}
		if err := atomicWrite(path, []byte(content)); err != nil {
			apiError(writer, err)
			return
		}
		ok(writer, Object{"path": relative})
		return
	}
	fail(writer, 405, "方法不支持")
}

func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }

func scriptRunTaskID(relative string) int64 {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(filepath.ToSlash(relative)))
	id := int64(hash.Sum64())
	if id > 0 {
		id = -id
	}
	if id == 0 {
		id = -1
	}
	return id
}
func (app *App) latestLog(kind string, id int64) string {
	var name string
	if app.store.db.QueryRow("SELECT log_path FROM runs WHERE kind=? AND task_id=? ORDER BY id DESC LIMIT 1", kind, id).Scan(&name) != nil {
		return ""
	}
	content, _ := app.logTail(name)
	return content
}

// cronLogPayload 对齐青龙 GET /crons/:id/log：顶层返回日志内容与分块元数据。
func (app *App) cronLogPayload(kind string, id, offset, limit int64, tail bool) Object {
	var name, status string
	notFound := Object{
		"code": 200, "data": "日志不存在...", "logStatus": "notFound",
		"offset": 0, "nextOffset": 0, "total": 0, "truncated": false,
	}
	if app.store.db.QueryRow("SELECT log_path,status FROM runs WHERE kind=? AND task_id=? ORDER BY id DESC LIMIT 1", kind, id).Scan(&name, &status) != nil || name == "" {
		return notFound
	}
	content, start, next, total, truncated, err := app.logChunk(name, offset, limit, tail)
	if err != nil {
		return notFound
	}
	logStatus := status
	if status == "running" || status == "queued" {
		logStatus = "running"
	}
	return Object{
		"code": 200, "data": content, "logStatus": logStatus,
		"offset": start, "nextOffset": next, "total": total, "truncated": truncated,
	}
}

// logChunk 按青龙 /logs/detail 语义分块读取：offset 起始字节、limit 字节数、tail 表示从末尾读。
// 返回内容、实际起始偏移、下一次偏移、总字节数和是否被截断。
func (app *App) logChunk(name string, offset, limit int64, tail bool) (string, int64, int64, int64, bool, error) {
	path, err := safePath(filepath.Join(app.root, "logs"), name)
	if err != nil {
		return "", 0, 0, 0, false, err
	}
	file, err := os.Open(path)
	if err != nil {
		return "", 0, 0, 0, false, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", 0, 0, 0, false, err
	}
	total := info.Size()
	const defaultLimit = int64(2 << 20)
	if limit <= 0 || limit > defaultLimit {
		limit = defaultLimit
	}
	start := offset
	if start < 0 {
		start = 0
	}
	if tail {
		start = total - limit
		if start < 0 {
			start = 0
		}
	}
	if start > total {
		start = total
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return "", 0, 0, 0, false, err
	}
	bytes, err := io.ReadAll(io.LimitReader(file, limit))
	next := start + int64(len(bytes))
	return strings.ToValidUTF8(string(bytes), "�"), start, next, total, next < total, err
}

func (app *App) logTail(name string) (string, error) {
	content, _, _, _, _, err := app.logChunk(name, 0, 0, true)
	return content, err
}

func (app *App) logsAPI(writer http.ResponseWriter, request *http.Request, tail string) {
	if strings.HasPrefix(tail, "download/") && request.Method == "GET" {
		id := integer(strings.TrimPrefix(tail, "download/"))
		var name string
		if err := app.store.db.QueryRow("SELECT log_path FROM runs WHERE id=?", id).Scan(&name); err != nil {
			apiError(writer, err)
			return
		}
		target, err := safePath(filepath.Join(app.root, "logs"), name)
		if err != nil {
			apiError(writer, err)
			return
		}
		writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
		writer.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=run-%d.log", id))
		http.ServeFile(writer, request, target)
		return
	}
	// GET /api/logs 对齐青龙：返回日志目录树。面板自身的运行记录改由 /api/logs/runs 提供。
	if tail == "" && request.Method == "GET" {
		ok(writer, app.logTree())
		return
	}
	if tail == "runs" && request.Method == "GET" {
		limit := numberQuery(request, "limit", 200)
		if limit > 1000 {
			limit = 1000
		}
		items, err := app.runs(request.URL.Query().Get("kind"), integer(request.URL.Query().Get("taskId")), limit)
		if err != nil {
			apiError(writer, err)
			return
		}
		ok(writer, items)
		return
	}
	if tail == "" && request.Method == "DELETE" {
		var body struct {
			Filename string `json:"filename"`
			Path     string `json:"path"`
			Type     string `json:"type"`
		}
		// 青龙语义：按 filename + path 删除单个日志文件；兼容面板旧用法（ID 数组）。
		raw, err := io.ReadAll(io.LimitReader(request.Body, 1<<20))
		if err != nil {
			apiError(writer, err)
			return
		}
		if len(raw) > 0 && raw[0] == '{' && json.Unmarshal(raw, &body) == nil && body.Filename != "" {
			relative := strings.Trim(strings.TrimPrefix(body.Path, "/")+"/"+body.Filename, "/")
			if target, err := safePath(filepath.Join(app.root, "logs"), relative); err == nil {
				_ = os.Remove(target)
				ok(writer, nil)
				return
			}
			fail(writer, 403, "暂无权限")
			return
		}
		var ids []int64
		if err := json.Unmarshal(raw, &ids); err != nil {
			fail(writer, 400, "JSON 请求格式错误")
			return
		}
		if len(ids) == 0 || len(ids) > 500 {
			fail(writer, 400, "请选择 1–500 条记录")
			return
		}
		for _, id := range ids {
			var name, status string
			if err := app.store.db.QueryRow("SELECT log_path,status FROM runs WHERE id=?", id).Scan(&name, &status); err != nil {
				apiError(writer, err)
				return
			}
			if status == "running" || status == "queued" {
				fail(writer, 409, "运行中的日志不能删除")
				return
			}
			if path, err := safePath(filepath.Join(app.root, "logs"), name); err == nil {
				_ = os.Remove(path)
			}
			_, _ = app.store.db.Exec("DELETE FROM runs WHERE id=?", id)
		}
		ok(writer, nil)
		return
	}
	// POST /api/logs/download 对齐青龙：按 filename + path 下载单个日志。
	if tail == "download" && request.Method == "POST" {
		var body struct {
			Filename string `json:"filename"`
			Path     string `json:"path"`
		}
		if err := decode(request, &body); err != nil {
			apiError(writer, err)
			return
		}
		relative := strings.Trim(strings.TrimPrefix(body.Path, "/")+"/"+body.Filename, "/")
		target, err := safePath(filepath.Join(app.root, "logs"), relative)
		if err != nil {
			fail(writer, 403, "暂无权限")
			return
		}
		writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
		writer.Header().Set("Content-Disposition", "attachment; filename="+body.Filename)
		http.ServeFile(writer, request, target)
		return
	}
	// POST /api/logs/input：向调试任务的 stdin 写入一行。
	if tail == "input" && request.Method == "POST" {
		var body struct {
			ID    int64  `json:"id"`
			Input string `json:"input"`
		}
		if err := decode(request, &body); err != nil {
			apiError(writer, err)
			return
		}
		if body.ID <= 0 {
			fail(writer, 400, "缺少运行 ID")
			return
		}
		if err := app.engine.writeStdin(body.ID, body.Input); err != nil {
			apiError(writer, err)
			return
		}
		ok(writer, nil)
		return
	}
	if request.Method != "GET" {
		fail(writer, 405, "方法不支持")
		return
	}
	if strings.HasPrefix(tail, "run/") {
		var name, status string
		if err := app.store.db.QueryRow("SELECT log_path,status FROM runs WHERE id=?", integer(strings.TrimPrefix(tail, "run/"))).Scan(&name, &status); err != nil {
			apiError(writer, err)
			return
		}
		content := ""
		if name != "" {
			var err error
			content, err = app.logTail(name)
			if err != nil {
				apiError(writer, err)
				return
			}
		}
		ok(writer, Object{"content": content, "status": status})
		return
	}
	if tail == "stream" {
		id := integer(request.URL.Query().Get("id"))
		var name string
		if err := app.store.db.QueryRow("SELECT log_path FROM runs WHERE id=?", id).Scan(&name); err != nil {
			apiError(writer, err)
			return
		}
		flusher, valid := writer.(http.Flusher)
		if !valid {
			fail(writer, 500, "当前连接不支持流式响应")
			return
		}
		writer.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		writer.Header().Set("X-Accel-Buffering", "no")
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		previous := ""
		for {
			var status string
			_ = app.store.db.QueryRow("SELECT log_path,status FROM runs WHERE id=?", id).Scan(&name, &status)
			content, _ := app.logTail(name)
			if content != previous {
				fmt.Fprintf(writer, "event: log\ndata: %s\n\n", strconv.Quote(content))
				previous = content
			}
			fmt.Fprintf(writer, "event: status\ndata: %s\n\n", strconv.Quote(status))
			flusher.Flush()
			if status != "running" && status != "queued" {
				return
			}
			select {
			case <-request.Context().Done():
				return
			case <-ticker.C:
			}
		}
	}
	relative := tail
	if tail == "detail" {
		relative = request.URL.Query().Get("path")
		if file := request.URL.Query().Get("file"); file != "" {
			relative = strings.Trim(relative, "/") + "/" + file
			relative = strings.TrimPrefix(relative, "/")
		}
	}
	// 目录树里的 key 是「任务名/文件名」，落盘路径只取最后一段。
	if strings.Contains(relative, "/") {
		relative = relative[strings.LastIndex(relative, "/")+1:]
	}
	query := request.URL.Query()
	content, start, next, total, truncated, err := app.logChunk(
		relative,
		integer(query.Get("offset")),
		integer(query.Get("limit")),
		query.Get("tail") == "true" || query.Get("tail") == "1",
	)
	if err != nil {
		apiError(writer, err)
		return
	}
	// 青龙 /logs/detail 在顶层返回分块元数据，content 放在 data。
	respond(writer, 200, Object{
		"code":       200,
		"data":       content,
		"offset":     start,
		"nextOffset": next,
		"total":      total,
		"truncated":  truncated,
	})
}
