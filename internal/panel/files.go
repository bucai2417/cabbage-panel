package panel

import (
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
	if app.fileOperations(writer, request, kind, tail, root) {
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
				result = append(result, Object{"name": entry.Name(), "title": filepath.ToSlash(relative), "key": filepath.ToSlash(relative), "path": filepath.ToSlash(relative), "isLeaf": true, "size": info.Size(), "updatedAt": info.ModTime().Format(time.RFC3339)})
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
		result, err := app.engine.enqueueAdhoc("scripts", scriptRunTaskID(relative), "调试 · "+relative, "task "+shellQuote(filepath.ToSlash(target)), Object{"timeout": 0}, directory)
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
func (app *App) logTail(name string) (string, error) {
	path, err := safePath(filepath.Join(app.root, "logs"), name)
	if err != nil {
		return "", err
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	prefix := ""
	if info.Size() > 2<<20 {
		_, _ = file.Seek(-(2 << 20), io.SeekEnd)
		prefix = "[仅显示末尾 2MB 日志]\n"
	}
	bytes, err := io.ReadAll(io.LimitReader(file, 2<<20))
	return prefix + strings.ToValidUTF8(string(bytes), "�"), err
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
	if tail == "" && request.Method == "GET" {
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
		ids, err := parseIDs(request)
		if err != nil {
			apiError(writer, err)
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
	content, err := app.logTail(relative)
	if err != nil {
		apiError(writer, err)
		return
	}
	ok(writer, content)
}
