package panel

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

type Backup struct {
	Format   int                 `json:"format"`
	Version  string              `json:"version"`
	Created  string              `json:"created"`
	Records  map[string][]Object `json:"records"`
	Settings map[string]string   `json:"settings"`
	Runs     []Object            `json:"runs"`
}

var recordKinds = []string{"crons", "envs", "subscriptions", "dependencies", "apps", "views", "history"}

func (app *App) backupAPI(writer http.ResponseWriter, request *http.Request, path, principal string) {
	if principal != "admin" {
		fail(writer, 403, "需要管理员权限")
		return
	}
	tail := strings.TrimPrefix(path, "/backups")
	if tail == "" && request.Method == "GET" {
		entries, err := os.ReadDir(filepath.Join(app.root, "backups"))
		if err != nil {
			apiError(writer, err)
			return
		}
		items := []Object{}
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".zip") {
				info, err := entry.Info()
				if err != nil {
					continue
				}
				items = append(items, Object{"name": entry.Name(), "size": info.Size(), "created": info.ModTime().Format(time.RFC3339)})
			}
		}
		ok(writer, items)
		return
	}
	if tail == "/export" && request.Method == "POST" {
		if app.engine.active() > 0 || !app.gate.TryLock() {
			fail(writer, 409, "系统正在执行任务或处理请求，请稍后重试")
			return
		}
		defer app.gate.Unlock()
		if app.engine.active() > 0 {
			fail(writer, 409, "请先停止任务并等待队列清空，再创建一致性备份")
			return
		}
		name, err := app.createBackup()
		if err != nil {
			apiError(writer, err)
			return
		}
		app.store.audit("backup.create", "admin", clientIP(request), false)
		ok(writer, Object{"name": name})
		return
	}
	if tail == "/import" && request.Method == "POST" {
		if app.engine.active() > 0 || !app.gate.TryLock() {
			fail(writer, 409, "系统正在执行任务或处理请求，请稍后重试")
			return
		}
		defer app.gate.Unlock()
		if app.engine.active() > 0 {
			fail(writer, 409, "请先停止任务并等待队列清空，再恢复备份")
			return
		}
		contentType := request.Header.Get("Content-Type")
		var reader io.Reader = request.Body
		if strings.HasPrefix(contentType, "multipart/form-data") {
			if err := request.ParseMultipartForm(8 << 20); err != nil {
				apiError(writer, err)
				return
			}
			defer request.MultipartForm.RemoveAll()
			file, _, err := request.FormFile("file")
			if err != nil {
				apiError(writer, err)
				return
			}
			defer file.Close()
			reader = file
		}
		data, err := io.ReadAll(io.LimitReader(reader, 256<<20+1))
		if err != nil || len(data) > 256<<20 {
			fail(writer, 400, "备份最大 256MB")
			return
		}
		safety, err := app.restoreBackup(data)
		if err != nil {
			apiError(writer, err)
			return
		}
		app.engine.reschedule()
		app.store.audit("backup.restore", "admin", clientIP(request), false)
		ok(writer, Object{"safetyBackup": safety, "message": "恢复成功，请重新登录；依赖请在依赖管理中重新安装"})
		return
	}
	name := strings.TrimPrefix(tail, "/")
	if name == "" || filepath.Base(name) != name || !strings.HasSuffix(name, ".zip") {
		fail(writer, 404, "备份不存在")
		return
	}
	target, err := safePath(filepath.Join(app.root, "backups"), name)
	if err != nil {
		apiError(writer, err)
		return
	}
	if request.Method == "GET" {
		writer.Header().Set("Content-Disposition", "attachment; filename="+name)
		writer.Header().Set("Content-Type", "application/zip")
		http.ServeFile(writer, request, target)
		return
	}
	if request.Method == "DELETE" {
		if err := os.Remove(target); err != nil {
			apiError(writer, err)
			return
		}
		ok(writer, nil)
		return
	}
	fail(writer, 405, "方法不支持")
}

func (app *App) createBackup() (string, error) {
	manifest := Backup{Format: 1, Version: app.version, Created: now(), Records: map[string][]Object{}, Settings: map[string]string{}}
	for _, kind := range recordKinds {
		items, err := app.store.list(kind)
		if err != nil {
			return "", err
		}
		manifest.Records[kind] = items
	}
	rows, err := app.store.db.Query("SELECT key,value FROM settings")
	if err != nil {
		return "", err
	}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			rows.Close()
			return "", err
		}
		manifest.Settings[key] = value
	}
	if err := rows.Close(); err != nil {
		return "", err
	}
	manifest.Runs, err = app.runs("", 0, 10000)
	if err != nil {
		return "", err
	}
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		return "", err
	}
	if len(manifestData) > 32<<20 {
		return "", errors.New("备份元数据超过 32MB，请先清理历史记录")
	}
	totalBytes := int64(len(manifestData))
	fileCount := 1
	name := time.Now().UTC().Format("20060102-150405") + "-" + randomToken()[:8] + ".zip"
	target := filepath.Join(app.root, "backups", name)
	temporary := target + ".tmp"
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", err
	}
	defer os.Remove(temporary)
	archive := zip.NewWriter(file)
	writeErr := func() error {
		entry, err := archive.Create("manifest.json")
		if err != nil {
			return err
		}
		if _, err := entry.Write(manifestData); err != nil {
			return err
		}
		for _, directory := range []string{"scripts", "config", "logs"} {
			root := filepath.Join(app.root, directory)
			if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if entry.IsDir() {
					if entry.Name() == "node_modules" || entry.Name() == "__pycache__" || entry.Name() == ".git" {
						return filepath.SkipDir
					}
					return nil
				}
				if entry.Type()&os.ModeSymlink != 0 {
					return errors.New("备份目录中包含符号链接，请先移除")
				}
				relative, _ := filepath.Rel(app.root, path)
				info, err := entry.Info()
				if err != nil {
					return err
				}
				if info.Size() > 32<<20 {
					return fmt.Errorf("文件过大，不能备份: %s", relative)
				}
				totalBytes += info.Size()
				fileCount++
				if totalBytes > 512<<20 || fileCount > 15000 {
					return errors.New("备份大小或文件数量超过可恢复上限，请先清理历史日志")
				}
				header, err := zip.FileInfoHeader(info)
				if err != nil {
					return err
				}
				header.Name = filepath.ToSlash(relative)
				header.Method = zip.Deflate
				output, err := archive.CreateHeader(header)
				if err != nil {
					return err
				}
				input, err := os.Open(path)
				if err != nil {
					return err
				}
				defer input.Close()
				_, err = io.Copy(output, input)
				return err
			}); err != nil {
				return err
			}
		}
		return nil
	}()
	closeErr := archive.Close()
	syncErr := file.Sync()
	fileErr := file.Close()
	for _, err := range []error{writeErr, closeErr, syncErr, fileErr} {
		if err != nil {
			return "", err
		}
	}
	if info, err := os.Stat(temporary); err != nil {
		return "", err
	} else if info.Size() > 256<<20 {
		return "", errors.New("压缩备份超过 256MB，请先清理历史日志")
	}
	if err := os.Rename(temporary, target); err != nil {
		return "", err
	}
	return name, nil
}

func (app *App) restoreBackup(data []byte) (string, error) {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", errors.New("备份不是有效的 ZIP 文件")
	}
	if len(archive.File) > 15000 {
		return "", errors.New("备份文件数量超限")
	}
	stage, err := os.MkdirTemp(app.root, ".restore-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(stage)
	var manifest Backup
	seen := map[string]bool{}
	var total uint64
	for _, entry := range archive.File {
		name := entry.Name
		if path.Clean(strings.TrimSuffix(name, "/")) != strings.TrimSuffix(name, "/") {
			return "", errors.New("备份包含非规范路径")
		}
		canonical := strings.ToLower(name)
		if seen[canonical] {
			return "", errors.New("备份含重复文件")
		}
		seen[canonical] = true
		total += entry.UncompressedSize64
		if total > 512<<20 || entry.UncompressedSize64 > 32<<20 || entry.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("备份解压大小超限或包含符号链接")
		}
		if name != "manifest.json" && !strings.HasPrefix(name, "scripts/") && !strings.HasPrefix(name, "config/") && !strings.HasPrefix(name, "logs/") {
			return "", fmt.Errorf("备份包含不允许的路径: %s", name)
		}
		if entry.FileInfo().IsDir() {
			continue
		}
		target, err := safePath(stage, name)
		if err != nil {
			return "", err
		}
		input, err := entry.Open()
		if err != nil {
			return "", err
		}
		content, err := io.ReadAll(io.LimitReader(input, 32<<20+1))
		closeErr := input.Close()
		if err != nil {
			return "", err
		}
		if closeErr != nil {
			return "", closeErr
		}
		if len(content) > 32<<20 {
			return "", errors.New("备份条目过大")
		}
		if name == "manifest.json" {
			if err := json.Unmarshal(content, &manifest); err != nil {
				return "", err
			}
		} else {
			if err := atomicWrite(target, content); err != nil {
				return "", err
			}
		}
	}
	if manifest.Format != 1 || manifest.Settings["password"] == "" || manifest.Settings["username"] == "" {
		return "", errors.New("备份格式版本或账号数据无效")
	}
	if _, err := bcrypt.Cost([]byte(manifest.Settings["password"])); err != nil {
		return "", errors.New("备份密码散列无效")
	}
	ids := map[int64]bool{}
	for kind, items := range manifest.Records {
		if !contains(recordKinds, kind) {
			return "", errors.New("未知记录类型")
		}
		for _, item := range items {
			id := integer(item["id"])
			if id <= 0 || ids[id] {
				return "", errors.New("备份记录 ID 无效或重复")
			}
			ids[id] = true
		}
	}
	safety, err := app.createBackup()
	if err != nil {
		return "", fmt.Errorf("无法创建恢复前备份: %w", err)
	}
	tx, err := app.store.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	for _, table := range []string{"records", "settings", "runs", "sessions"} {
		if _, err := tx.Exec("DELETE FROM " + table); err != nil {
			return "", err
		}
	}
	for kind, items := range manifest.Records {
		for _, item := range items {
			if kind == "dependencies" {
				item["status"] = 2
				item["remove"] = false
			}
			content, err := json.Marshal(item)
			if err != nil {
				return "", err
			}
			if _, err := tx.Exec("INSERT INTO records(id,kind,data) VALUES(?,?,?)", integer(item["id"]), kind, string(content)); err != nil {
				return "", err
			}
		}
	}
	for key, value := range manifest.Settings {
		if _, err := tx.Exec("INSERT INTO settings(key,value) VALUES(?,?)", key, value); err != nil {
			return "", err
		}
	}
	for _, run := range manifest.Runs {
		status := text(run["status"])
		if status == "running" || status == "queued" {
			status = "interrupted"
		}
		if _, err := tx.Exec("INSERT INTO runs(id,task_id,kind,name,command,status,started,finished,exit_code,duration,log_path,created) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)", run["id"], run["taskId"], run["kind"], run["name"], run["command"], status, run["started"], run["finished"], run["exitCode"], run["duration"], run["logPath"], run["created"]); err != nil {
			return "", err
		}
	}
	moved := []string{}
	rollback := func() {
		for index := len(moved) - 1; index >= 0; index-- {
			directory := moved[index]
			_ = os.Rename(filepath.Join(app.root, directory), filepath.Join(stage, directory+"-failed"))
			_ = os.Rename(filepath.Join(stage, directory+"-old"), filepath.Join(app.root, directory))
		}
	}
	for _, directory := range []string{"scripts", "config", "logs"} {
		if err := os.MkdirAll(filepath.Join(stage, directory), 0700); err != nil {
			rollback()
			return "", err
		}
		if err := os.Rename(filepath.Join(app.root, directory), filepath.Join(stage, directory+"-old")); err != nil {
			rollback()
			return "", err
		}
		if err := os.Rename(filepath.Join(stage, directory), filepath.Join(app.root, directory)); err != nil {
			_ = os.Rename(filepath.Join(stage, directory+"-old"), filepath.Join(app.root, directory))
			rollback()
			return "", err
		}
		moved = append(moved, directory)
	}
	if err := tx.Commit(); err != nil {
		rollback()
		return "", err
	}
	return safety, nil
}
