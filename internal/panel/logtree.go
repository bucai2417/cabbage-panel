package panel

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// logTree 输出青龙 /api/logs 的目录树形状。
// 面板日志是扁平的 <runId>.log，这里按任务名分组映射成一层目录 + 文件，
// 让青龙客户端（含官方前端）可以直接浏览和下载。
func (app *App) logTree() []Object {
	rows, err := app.store.db.Query(`SELECT id, name, kind, log_path FROM runs WHERE log_path != '' ORDER BY id DESC LIMIT 5000`)
	if err != nil {
		return []Object{}
	}
	defer rows.Close()

	type entry struct {
		title      string
		key        string
		parent     string
		size       int64
		createTime int64
	}
	groups := map[string][]entry{}
	order := []string{}
	for rows.Next() {
		var id int64
		var taskName, kind, logPath string
		if rows.Scan(&id, &taskName, &kind, &logPath) != nil {
			continue
		}
		target, pathErr := safePath(filepath.Join(app.root, "logs"), logPath)
		if pathErr != nil {
			continue
		}
		info, statErr := os.Stat(target)
		if statErr != nil {
			continue
		}
		group := strings.TrimSpace(taskName)
		if group == "" {
			group = kind
		}
		if group == "" {
			group = "未命名"
		}
		if _, seen := groups[group]; !seen {
			order = append(order, group)
		}
		groups[group] = append(groups[group], entry{
			title:      filepath.Base(logPath),
			key:        group + "/" + filepath.Base(logPath),
			parent:     group,
			size:       info.Size(),
			createTime: info.ModTime().UnixMilli(),
		})
	}

	sort.SliceStable(order, func(left, right int) bool { return order[left] < order[right] })
	tree := []Object{}
	for _, group := range order {
		files := groups[group]
		sort.SliceStable(files, func(left, right int) bool { return files[left].createTime > files[right].createTime })
		children := []Object{}
		var newest int64
		for _, file := range files {
			if file.createTime > newest {
				newest = file.createTime
			}
			children = append(children, Object{
				"title":      file.title,
				"key":        file.key,
				"type":       "file",
				"parent":     file.parent,
				"size":       file.size,
				"createTime": file.createTime,
			})
		}
		tree = append(tree, Object{
			"title":      group,
			"key":        group,
			"type":       "directory",
			"parent":     "",
			"createTime": newest,
			"children":   children,
		})
	}
	return tree
}
