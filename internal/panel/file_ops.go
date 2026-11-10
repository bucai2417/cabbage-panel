package panel

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

type commandWord struct {
	value      string
	start, end int
}

func commandWords(command string) ([]commandWord, error) {
	words := []commandWord{}
	for offset := 0; offset < len(command); {
		for offset < len(command) && unicode.IsSpace(rune(command[offset])) {
			offset++
		}
		if offset == len(command) {
			break
		}
		start := offset
		var value strings.Builder
		var quote byte
		for offset < len(command) {
			character := command[offset]
			if quote == 0 && unicode.IsSpace(rune(character)) {
				break
			}
			if quote == 0 && strings.ContainsRune(";&|<>`$()", rune(character)) {
				return nil, errors.New("复杂命令请手动选择脚本文件")
			}
			if character == 92 && quote != 39 {
				offset++
				if offset >= len(command) {
					return nil, errors.New("命令转义不完整")
				}
				value.WriteByte(command[offset])
				offset++
				continue
			}
			if character == 39 || character == 34 {
				if quote == 0 {
					quote = character
					offset++
					continue
				}
				if quote == character {
					quote = 0
					offset++
					continue
				}
			}
			value.WriteByte(character)
			offset++
		}
		if quote != 0 {
			return nil, errors.New("命令引号不完整")
		}
		words = append(words, commandWord{value.String(), start, offset})
	}
	return words, nil
}

func scriptWord(command string) (commandWord, error) {
	words, err := commandWords(command)
	if err != nil || len(words) < 2 {
		return commandWord{}, errors.New("无法定位脚本，请从文件树手动选择；原命令不会修改")
	}
	program := strings.ToLower(filepath.Base(words[0].value))
	if !contains([]string{"task", "python", "python3", "python.exe", "python3.exe", "node", "node.exe", "bash", "sh"}, program) {
		return commandWord{}, errors.New("此命令不是直接运行脚本，请手动选择文件")
	}
	for _, word := range words[1:] {
		if contains([]string{"-c", "-e", "--eval", "-m"}, word.value) {
			break
		}
		if strings.HasPrefix(word.value, "-") {
			continue
		}
		extension := strings.ToLower(filepath.Ext(word.value))
		if contains([]string{".py", ".js", ".cjs", ".mjs", ".sh"}, extension) {
			word.value = strings.TrimPrefix(word.value, "./")
			return word, nil
		}
		break
	}
	return commandWord{}, errors.New("没有找到脚本路径，请手动选择文件")
}

func (app *App) taskSource(writer http.ResponseWriter, request *http.Request, id int64) {
	task, err := app.store.get("crons", id)
	if err != nil {
		apiError(writer, err)
		return
	}
	word, err := scriptWord(text(task["command"]))
	if err != nil {
		apiError(writer, err)
		return
	}
	target, err := safePath(filepath.Join(app.root, "scripts"), word.value)
	if err != nil {
		apiError(writer, err)
		return
	}
	content, err := readText(target, 4<<20)
	if err != nil {
		apiError(writer, err)
		return
	}
	ok(writer, Object{"path": word.value, "content": content})
}

func (app *App) fileOperations(writer http.ResponseWriter, request *http.Request, kind, tail, root string) bool {
	if !contains([]string{"tree", "folder", "operations"}, tail) {
		return false
	}
	if tail == "tree" && request.Method == "GET" {
		entries := []Object{}
		err := filepath.WalkDir(root, func(target string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if target == root {
				return nil
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return nil
			}
			if entry.IsDir() && contains([]string{".git", "node_modules", "__pycache__"}, entry.Name()) {
				return filepath.SkipDir
			}
			if strings.HasPrefix(entry.Name(), ".cabbage-") {
				return nil
			}
			relative, _ := filepath.Rel(root, target)
			info, err := entry.Info()
			if err != nil {
				return err
			}
			entries = append(entries, Object{"name": entry.Name(), "path": filepath.ToSlash(relative), "isLeaf": !entry.IsDir(), "size": info.Size(), "updatedAt": info.ModTime()})
			if len(entries) > 10000 {
				return errors.New("目录条目超过 10000")
			}
			return nil
		})
		if err != nil {
			apiError(writer, err)
			return true
		}
		if kind == "configs" {
			entries = append(entries, app.builtinConfigEntries()...)
		}
		ok(writer, entries)
		return true
	}
	if request.Method != "POST" {
		fail(writer, 405, "方法不支持")
		return true
	}
	var body struct {
		Path        string
		Paths       []string
		Operation   string
		Destination string
	}
	if err := decode(request, &body); err != nil {
		apiError(writer, err)
		return true
	}
	if tail == "folder" {
		target, err := safePath(root, body.Path)
		if err == nil && kind == "configs" && builtinConfig(body.Path) {
			err = errors.New("系统配置名称保留")
		}
		if err == nil {
			err = os.MkdirAll(filepath.Dir(target), 0700)
		}
		if err == nil {
			err = os.Mkdir(target, 0700)
		}
		if err != nil {
			apiError(writer, err)
			return true
		}
		ok(writer, Object{"path": body.Path})
		return true
	}
	if len(body.Paths) == 0 || len(body.Paths) > 500 || !contains([]string{"move", "copy", "delete", "rename"}, body.Operation) || (body.Operation == "rename" && len(body.Paths) != 1) {
		fail(writer, 400, "文件操作参数无效")
		return true
	}
	sort.Slice(body.Paths, func(left, right int) bool { return len(body.Paths[left]) < len(body.Paths[right]) })
	roots := []string{}
	results := []Object{}
	for _, relative := range body.Paths {
		normalized := filepath.ToSlash(filepath.Clean(relative))
		covered := false
		for _, parent := range roots {
			if normalized == parent || strings.HasPrefix(normalized, parent+"/") {
				covered = true
				break
			}
		}
		if covered {
			continue
		}
		roots = append(roots, normalized)
		destination := body.Destination
		if body.Operation != "rename" {
			destination = strings.TrimSuffix(destination, "/")
			if destination != "" {
				destination += "/"
			}
			destination += filepath.Base(normalized)
		}
		affected, err := app.operateFile(root, kind, relative, destination, body.Operation)
		result := Object{"path": relative, "destination": destination, "success": err == nil, "updatedTasks": affected}
		if err != nil {
			result["error"] = err.Error()
		}
		results = append(results, result)
	}
	ok(writer, results)
	return true
}

func inspectFileTree(target string) error {
	count := 0
	var size int64
	return filepath.WalkDir(target, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("目录包含符号链接，拒绝操作")
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return errors.New("不支持特殊文件")
		}
		count++
		size += info.Size()
		if count > 10000 || size > 512<<20 {
			return errors.New("单次目录操作上限为 10000 项 / 512 MB")
		}
		return nil
	})
}

func copyFileTree(source, destination string) error {
	return filepath.WalkDir(source, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, name)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0700)
		}
		input, err := os.Open(name)
		if err != nil {
			return err
		}
		defer input.Close()
		info, err := entry.Info()
		if err != nil {
			return err
		}
		output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm()&0700)
		if err != nil {
			return err
		}
		_, err = io.Copy(output, input)
		closeErr := output.Close()
		if err != nil {
			return err
		}
		return closeErr
	})
}

func (app *App) operateFile(root, kind, relative, destination, operation string) (int, error) {
	if kind == "scripts" && operation != "copy" {
		app.scriptMu.Lock()
		defer app.scriptMu.Unlock()
	}
	if kind == "scripts" && operation != "copy" && app.engine.active() > 0 {
		return 0, errors.New("存在执行中或排队的作业，请待作业结束后移动、重命名或删除脚本")
	}
	source, err := safePath(root, relative)
	if err != nil {
		return 0, err
	}
	if kind == "configs" && (builtinConfig(relative) || builtinConfig(destination)) {
		return 0, errors.New("内置配置不能移动、复制或删除")
	}
	if err := inspectFileTree(source); err != nil {
		return 0, err
	}
	if operation == "delete" {
		return 0, os.RemoveAll(source)
	}
	target, err := safePath(root, destination)
	if err != nil {
		return 0, err
	}
	sourceKey, targetKey := strings.ToLower(source), strings.ToLower(target)
	if sourceKey == targetKey || strings.HasPrefix(targetKey, sourceKey+string(filepath.Separator)) {
		return 0, errors.New("目标不能是自身或自身的子目录")
	}
	if _, err := os.Lstat(target); err == nil {
		return 0, errors.New("目标已存在，不会覆盖")
	} else if !os.IsNotExist(err) {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return 0, err
	}
	if operation == "copy" {
		if err := copyFileTree(source, target); err != nil {
			_ = os.RemoveAll(target)
			return 0, err
		}
		return 0, nil
	}
	if err := os.Rename(source, target); err != nil {
		return 0, err
	}
	if kind != "scripts" {
		return 0, nil
	}
	count, err := app.relocateTaskScripts(filepath.ToSlash(filepath.Clean(relative)), filepath.ToSlash(filepath.Clean(destination)))
	if err != nil {
		rollback := os.Rename(target, source)
		return 0, fmt.Errorf("任务路径更新失败，文件回滚结果 %v: %w", rollback, err)
	}
	return count, nil
}

func (app *App) relocateTaskScripts(source, destination string) (int, error) {
	tasks, err := app.store.list("crons")
	if err != nil {
		return 0, err
	}
	histories, err := app.store.list("history")
	if err != nil {
		return 0, err
	}
	transaction, err := app.store.db.Begin()
	if err != nil {
		return 0, err
	}
	defer transaction.Rollback()
	count := 0
	for _, task := range tasks {
		command := text(task["command"])
		word, err := scriptWord(command)
		if err != nil && strings.Contains(command, source) {
			return 0, fmt.Errorf("任务 %s 使用复合命令，请先手动更新其中的路径", text(task["name"]))
		}
		if err != nil || (word.value != source && !strings.HasPrefix(word.value, source+"/")) {
			continue
		}
		newPath := destination + strings.TrimPrefix(word.value, source)
		command = command[:word.start] + shellQuote(newPath) + command[word.end:]
		if _, err := transaction.Exec("UPDATE records SET data=json_set(data,'$.command',?,'$.updatedAt',?) WHERE kind='crons' AND id=?", command, now(), integer(task["id"])); err != nil {
			return 0, err
		}
		count++
	}
	for _, history := range histories {
		oldPath := text(history["path"])
		if text(history["kind"]) != "scripts" || (oldPath != source && !strings.HasPrefix(oldPath, source+"/")) {
			continue
		}
		if _, err := transaction.Exec("UPDATE records SET data=json_set(data,'$.path',?) WHERE kind='history' AND id=?", destination+strings.TrimPrefix(oldPath, source), integer(history["id"])); err != nil {
			return 0, err
		}
	}
	return count, transaction.Commit()
}
