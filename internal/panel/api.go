package panel

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

var envName = regexp.MustCompile("^[A-Za-z_][A-Za-z0-9_]*$")
var aliasName = regexp.MustCompile("^[a-zA-Z0-9_-]{1,64}$")
var packageName = regexp.MustCompile("^[@A-Za-z0-9][A-Za-z0-9_.@/+<>=!~^-]{0,180}$")

func apiError(writer http.ResponseWriter, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		fail(writer, 404, "记录不存在")
	} else {
		fail(writer, 400, err.Error())
	}
}

func (app *App) api(writer http.ResponseWriter, request *http.Request, path, principal string) {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	kind := parts[0]
	tail := strings.Join(parts[1:], "/")
	switch kind {
	case "crons", "envs", "subscriptions", "dependencies", "apps":
		app.recordsAPI(writer, request, kind, tail)
	case "scripts", "configs":
		app.filesAPI(writer, request, kind, tail)
	case "logs":
		app.logsAPI(writer, request, tail)
	case "system":
		app.systemAPI(writer, request, tail)
	case "audit":
		if request.Method != "GET" {
			fail(writer, 405, "方法不支持")
			return
		}
		rows, err := app.store.db.Query("SELECT id,action,actor,created FROM audit ORDER BY id DESC LIMIT 200")
		if err != nil {
			apiError(writer, err)
			return
		}
		defer rows.Close()
		result := []Object{}
		for rows.Next() {
			var id int64
			var action, actor, created string
			if rows.Scan(&id, &action, &actor, &created) == nil {
				result = append(result, Object{"id": id, "action": action, "actor": actor, "created": created})
			}
		}
		ok(writer, result)
	case "user":
		if tail == "logout" && request.Method == "POST" {
			token := strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer ")
			if token == "" {
				if cookie, err := request.Cookie("cabbage_session"); err == nil {
					token = cookie.Value
				}
			}
			_, _ = app.store.db.Exec("DELETE FROM sessions WHERE token=?", hash(token))
			http.SetCookie(writer, &http.Cookie{Name: "cabbage_session", Value: "", Path: "/", HttpOnly: true, MaxAge: -1, SameSite: http.SameSiteStrictMode})
			ok(writer, nil)
			return
		}
		if tail == "" && request.Method == "GET" {
			ok(writer, Object{"username": app.store.setting("username", ""), "role": "admin"})
			return
		}
		if tail == "" && request.Method == "PUT" {
			var body Object
			if err := decode(request, &body); err != nil {
				apiError(writer, err)
				return
			}
			if bcryptErr := verifyPassword(app.store.setting("password", ""), text(body["oldPassword"])); bcryptErr != nil {
				fail(writer, 403, "当前密码错误")
				return
			}
			if err := app.changeUser(body); err != nil {
				apiError(writer, err)
				return
			}
			ok(writer, nil)
			return
		}
		fail(writer, 404, "接口不存在")
	default:
		fail(writer, 404, "接口不存在")
	}
}

func (app *App) validate(kind string, item Object, update bool) error {
	if update && integer(item["id"]) == 0 {
		return errors.New("缺少记录 ID")
	}
	switch kind {
	case "crons":
		if strings.TrimSpace(text(item["name"])) == "" || strings.TrimSpace(text(item["command"])) == "" {
			return errors.New("任务名称和命令不能为空")
		}
		if len(text(item["command"])) > 32768 {
			return errors.New("命令过长")
		}
		if spec := text(item["schedule"]); spec != "" {
			if _, err := scheduleParser.Parse(spec); err != nil {
				return fmt.Errorf("Cron 表达式无效: %w", err)
			}
		}
		if integer(item["timeout"]) < 0 || integer(item["timeout"]) > 604800 {
			return errors.New("超时范围为 0–604800 秒")
		}
	case "envs":
		if !envName.MatchString(text(item["name"])) || strings.ContainsRune(text(item["value"]), 0) {
			return errors.New("变量名或变量值无效")
		}
	case "subscriptions":
		parsed, err := url.Parse(text(item["url"]))
		if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" || parsed.User != nil {
			return errors.New("订阅地址必须是无内嵌凭证的 HTTP(S) URL")
		}
		if !aliasName.MatchString(text(item["alias"])) {
			return errors.New("订阅别名只能包含字母、数字、下划线和连字符")
		}
		if text(item["type"]) != "git-repo" && text(item["type"]) != "file" {
			return errors.New("订阅类型为 git-repo 或 file")
		}
		if strings.HasPrefix(text(item["branch"]), "-") {
			return errors.New("分支名称无效")
		}
		if spec := text(item["schedule"]); spec != "" {
			if _, err := scheduleParser.Parse(spec); err != nil {
				return fmt.Errorf("Cron 表达式无效: %w", err)
			}
		}
		for _, field := range []string{"whitelist", "blacklist"} {
			if text(item[field]) != "" {
				if _, err := regexp.Compile(text(item[field])); err != nil {
					return fmt.Errorf("%s 正则表达式无效", field)
				}
			}
		}
		items, err := app.store.list(kind)
		if err != nil {
			return err
		}
		for _, other := range items {
			if text(other["alias"]) == text(item["alias"]) && integer(other["id"]) != integer(item["id"]) {
				return errors.New("订阅别名已存在")
			}
		}
	case "dependencies":
		if !packageName.MatchString(text(item["name"])) || strings.Contains(text(item["name"]), "..") {
			return errors.New("依赖名称无效；每条记录只能安装一个包")
		}
		if integer(item["type"]) < 0 || integer(item["type"]) > 2 {
			return errors.New("依赖类型: 0 Node.js / 1 Python / 2 系统包")
		}
	case "apps":
		if strings.TrimSpace(text(item["name"])) == "" {
			return errors.New("应用名称不能为空")
		}
		values, _ := item["scopes"].([]any)
		allowed := map[string]bool{"crons": true, "envs": true, "scripts": true, "logs": true, "configs": true, "dependencies": true, "subscriptions": true, "mcp": true}
		for _, value := range values {
			if !allowed[text(value)] {
				return errors.New("应用权限无效")
			}
		}
	}
	return nil
}

func defaults(kind string, item Object) Object {
	if _, found := item["status"]; !found {
		item["status"] = 0
	}
	if _, found := item["isDisabled"]; !found {
		item["isDisabled"] = 0
	}
	if _, found := item["isPinned"]; !found {
		item["isPinned"] = 0
	}
	if item["labels"] == nil {
		item["labels"] = []string{}
	}
	if kind == "crons" {
		item["status"] = 1
	}
	return item
}

func (app *App) recordsAPI(writer http.ResponseWriter, request *http.Request, kind, tail string) {
	method := request.Method
	if kind == "crons" && method == "GET" && strings.HasSuffix(tail, "/source") {
		app.taskSource(writer, request, integer(strings.TrimSuffix(tail, "/source")))
		return
	}
	if kind == "crons" && strings.HasPrefix(tail, "views") {
		app.viewsAPI(writer, request, strings.TrimPrefix(tail, "views"))
		return
	}
	if tail == "" && method == "GET" {
		items, err := app.store.list(kind)
		if err != nil {
			apiError(writer, err)
			return
		}
		filtered := []Object{}
		search := strings.ToLower(request.URL.Query().Get("searchValue"))
		for _, item := range items {
			bytes, _ := json.Marshal(item)
			if search != "" && !strings.Contains(strings.ToLower(string(bytes)), search) {
				continue
			}
			if kind == "dependencies" && request.URL.Query().Get("type") != "" && text(item["type"]) != request.URL.Query().Get("type") {
				continue
			}
			if kind == "crons" || kind == "subscriptions" {
				item = app.engine.decorate(kind, item)
			}
			delete(item, "secret_hash")
			filtered = append(filtered, item)
		}
		sort.SliceStable(filtered, func(left, right int) bool {
			l, r := filtered[left], filtered[right]
			if kind == "crons" {
				if integer(l["isDisabled"]) != integer(r["isDisabled"]) {
					return integer(l["isDisabled"]) < integer(r["isDisabled"])
				}
				if integer(l["isPinned"]) != integer(r["isPinned"]) {
					return integer(l["isPinned"]) > integer(r["isPinned"])
				}
				if text(l["createdAt"]) != text(r["createdAt"]) {
					return text(l["createdAt"]) > text(r["createdAt"])
				}
				return integer(l["id"]) > integer(r["id"])
			}
			if integer(l["isPinned"]) != integer(r["isPinned"]) {
				return integer(l["isPinned"]) > integer(r["isPinned"])
			}
			if kind == "envs" {
				if integer(l["position"]) != integer(r["position"]) {
					return integer(l["position"]) < integer(r["position"])
				}
				if text(l["createdAt"]) != text(r["createdAt"]) {
					return text(l["createdAt"]) < text(r["createdAt"])
				}
				return integer(l["id"]) < integer(r["id"])
			}
			if text(l["createdAt"]) != text(r["createdAt"]) {
				return text(l["createdAt"]) > text(r["createdAt"])
			}
			return integer(l["id"]) > integer(r["id"])
		})
		if kind == "crons" {
			total := len(filtered)
			page, size := numberQuery(request, "page", 1), numberQuery(request, "size", 10000)
			if size > 10000 {
				size = 10000
			}
			start := (page - 1) * size
			if start > total {
				start = total
			}
			end := start + size
			if end > total {
				end = total
			}
			ok(writer, Object{"data": filtered[start:end], "total": total})
			return
		}
		ok(writer, filtered)
		return
	}
	if tail == "" && (method == "POST" || method == "PUT") {
		var raw json.RawMessage
		if err := decode(request, &raw); err != nil {
			apiError(writer, err)
			return
		}
		batch := []Object{}
		isArray := len(raw) > 0 && raw[0] == '['
		if isArray {
			if err := json.Unmarshal(raw, &batch); err != nil {
				apiError(writer, err)
				return
			}
		} else {
			var item Object
			if err := json.Unmarshal(raw, &item); err != nil {
				apiError(writer, err)
				return
			}
			batch = append(batch, item)
		}
		if len(batch) == 0 || len(batch) > 500 || (method == "PUT" && len(batch) != 1) {
			fail(writer, 400, "记录数量无效")
			return
		}
		for index, item := range batch {
			if item == nil {
				fail(writer, 400, "记录不能为空")
				return
			}
			if method == "PUT" {
				old, err := app.store.get(kind, integer(item["id"]))
				if err != nil {
					apiError(writer, err)
					return
				}
				for key, value := range item {
					old[key] = value
				}
				batch[index] = old
				item = old
			} else {
				delete(item, "id")
				delete(item, "_id")
				defaults(kind, item)
			}
			if err := app.validate(kind, item, method == "PUT"); err != nil {
				apiError(writer, err)
				return
			}
		}
		result := []Object{}
		envPosition := int64(0)
		if kind == "envs" && method == "POST" {
			existing, _ := app.store.list("envs")
			maxPosition := int64(-1)
			for _, entry := range existing {
				if position := integer(entry["position"]); position > maxPosition {
					maxPosition = position
				}
			}
			envPosition = maxPosition + 1
		}
		for _, item := range batch {
			secret := ""
			if kind == "envs" && method == "POST" {
				item["position"] = envPosition
				envPosition++
			}
			if kind == "apps" {
				if method == "POST" {
					secret = randomToken()
					item["client_id"] = randomToken()[:24]
					item["secret_hash"] = hash(secret)
				} else {
					old, _ := app.store.get(kind, integer(item["id"]))
					item["client_id"] = old["client_id"]
					item["secret_hash"] = old["secret_hash"]
				}
				delete(item, "client_secret")
			}
			if kind == "dependencies" {
				item["status"] = 1
				item["remove"] = false
			}
			saved, err := app.store.save(kind, item)
			if err != nil {
				apiError(writer, err)
				return
			}
			if secret != "" {
				saved["client_secret"] = secret
			}
			delete(saved, "secret_hash")
			result = append(result, saved)
			if kind == "dependencies" {
				if _, err := app.engine.enqueue(kind, integer(saved["id"])); err != nil {
					_, _ = app.store.save(kind, Object{"id": saved["id"], "status": 2})
					saved["message"] = err.Error()
				}
			}
		}
		if kind == "crons" || kind == "subscriptions" {
			app.engine.reschedule()
		}
		if isArray {
			ok(writer, result)
		} else {
			ok(writer, result[0])
		}
		return
	}
	if tail == "labels" && (method == "POST" || method == "DELETE") {
		var body struct {
			IDs    []int64
			Labels []string
		}
		if err := decode(request, &body); err != nil {
			apiError(writer, err)
			return
		}
		for _, id := range body.IDs {
			item, err := app.store.get(kind, id)
			if err != nil {
				apiError(writer, err)
				return
			}
			labels := map[string]bool{}
			if existing, valid := item["labels"].([]any); valid {
				for _, label := range existing {
					labels[text(label)] = true
				}
			}
			for _, label := range body.Labels {
				if method == "POST" {
					labels[label] = true
				} else {
					delete(labels, label)
				}
			}
			output := []string{}
			for label := range labels {
				output = append(output, label)
			}
			sort.Strings(output)
			if _, err := app.store.save(kind, Object{"id": id, "labels": output}); err != nil {
				apiError(writer, err)
				return
			}
		}
		ok(writer, nil)
		return
	}
	if (tail == "" && method == "DELETE") || ((method == "PUT" || method == "DELETE") && contains([]string{"run", "stop", "enable", "disable", "pin", "unpin", "reinstall", "cancel", "force"}, tail)) {
		ids, err := parseIDs(request)
		if err != nil {
			apiError(writer, err)
			return
		}
		result := []Object{}
		for _, id := range ids {
			if _, err := app.store.get(kind, id); err != nil {
				apiError(writer, err)
				return
			}
		}
		for _, id := range ids {
			switch tail {
			case "run", "reinstall":
				if !contains([]string{"crons", "subscriptions", "dependencies"}, kind) {
					fail(writer, 400, "此资源不能执行")
					return
				}
				if kind == "dependencies" {
					if _, err := app.store.save(kind, Object{"id": id, "remove": false, "status": 1}); err != nil {
						apiError(writer, err)
						return
					}
				}
				run, err := app.engine.enqueue(kind, id)
				if err != nil {
					apiError(writer, err)
					return
				}
				result = append(result, run)
			case "stop", "cancel":
				app.engine.stop(kind, id, 0)
			case "enable", "disable":
				value := 0
				if tail == "disable" {
					value = 1
				}
				field := "isDisabled"
				if kind == "envs" {
					field = "status"
				}
				_, err = app.store.save(kind, Object{"id": id, field: value})
			case "pin", "unpin":
				value := 0
				if tail == "pin" {
					value = 1
				}
				_, err = app.store.save(kind, Object{"id": id, "isPinned": value})
			default:
				app.engine.stop(kind, id, 0)
				if kind == "dependencies" && tail != "force" {
					item, _ := app.store.get(kind, id)
					item["remove"] = true
					_, err = app.store.save(kind, item)
					if err == nil {
						_, err = app.engine.enqueue(kind, id)
					}
				} else {
					err = app.store.remove(kind, id)
				}
				if kind == "apps" {
					_, _ = app.store.db.Exec("DELETE FROM sessions WHERE principal=?", "app:"+text(id))
				}
			}
			if err != nil {
				apiError(writer, err)
				return
			}
		}
		app.engine.reschedule()
		ok(writer, result)
		return
	}
	segments := strings.Split(tail, "/")
	id := integer(segments[0])
	if id > 0 {
		item, err := app.store.get(kind, id)
		if err != nil {
			apiError(writer, err)
			return
		}
		if len(segments) == 1 && method == "GET" {
			if kind == "crons" || kind == "subscriptions" {
				item = app.engine.decorate(kind, item)
			}
			if kind == "dependencies" {
				item["log"] = app.latestLog(kind, id)
			}
			delete(item, "secret_hash")
			ok(writer, item)
			return
		}
		if len(segments) == 2 {
			switch segments[1] {
			case "log":
				if method == "GET" {
					ok(writer, app.latestLog(kind, id))
					return
				}
			case "logs", "instances":
				if method == "GET" {
					runs, err := app.runs(kind, id, 100)
					if err != nil {
						apiError(writer, err)
						return
					}
					ok(writer, runs)
					return
				}
			case "move":
				if method == "PUT" {
					var body Object
					if err := decode(request, &body); err != nil {
						apiError(writer, err)
						return
					}
					items, err := app.store.list(kind)
					if err != nil {
						apiError(writer, err)
						return
					}
					sort.SliceStable(items, func(left, right int) bool {
						return integer(items[left]["position"]) < integer(items[right]["position"])
					})
					ordered := []Object{}
					for _, other := range items {
						if integer(other["id"]) != id {
							ordered = append(ordered, other)
						}
					}
					position := int(integer(body["toIndex"]))
					if position < 0 {
						position = 0
					}
					if position > len(ordered) {
						position = len(ordered)
					}
					ordered = append(ordered, nil)
					copy(ordered[position+1:], ordered[position:])
					ordered[position] = item
					for index, other := range ordered {
						_, err = app.store.save(kind, Object{"id": other["id"], "position": index})
						if err != nil {
							apiError(writer, err)
							return
						}
					}
					ok(writer, nil)
					return
				}
			case "reset-secret":
				if kind == "apps" && method == "PUT" {
					secret := randomToken()
					item["secret_hash"] = hash(secret)
					saved, err := app.store.save(kind, item)
					if err != nil {
						apiError(writer, err)
						return
					}
					_, _ = app.store.db.Exec("DELETE FROM sessions WHERE principal=?", "app:"+text(id))
					delete(saved, "secret_hash")
					saved["client_secret"] = secret
					ok(writer, saved)
					return
				}
			}
		}
		if len(segments) == 4 && segments[1] == "instances" && segments[3] == "stop" && method == "POST" {
			app.engine.stop(kind, id, integer(segments[2]))
			ok(writer, nil)
			return
		}
	}
	fail(writer, 404, "此兼容接口尚未实现，请查看 docs/API.md")
}
func contains(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func (app *App) viewsAPI(writer http.ResponseWriter, request *http.Request, tail string) {
	if tail == "" && request.Method == "GET" {
		items, err := app.store.list("views")
		if err != nil {
			apiError(writer, err)
			return
		}
		ok(writer, items)
		return
	}
	if tail == "" && (request.Method == "POST" || request.Method == "PUT") {
		var body Object
		if err := decode(request, &body); err != nil {
			apiError(writer, err)
			return
		}
		if text(body["name"]) == "" {
			fail(writer, 400, "视图名称不能为空")
			return
		}
		if request.Method == "POST" {
			delete(body, "id")
		}
		item, err := app.store.save("views", body)
		if err != nil {
			apiError(writer, err)
			return
		}
		ok(writer, item)
		return
	}
	if tail == "" && request.Method == "DELETE" {
		ids, err := parseIDs(request)
		if err != nil {
			apiError(writer, err)
			return
		}
		for _, id := range ids {
			if err := app.store.remove("views", id); err != nil {
				apiError(writer, err)
				return
			}
		}
		ok(writer, nil)
		return
	}
	fail(writer, 404, "视图接口不存在")
}

func (app *App) runs(kind string, id int64, limit int) ([]Object, error) {
	query := "SELECT id,task_id,kind,name,command,status,started,finished,exit_code,duration,log_path,created FROM runs WHERE 1=1"
	args := []any{}
	if kind != "" {
		query += " AND kind=?"
		args = append(args, kind)
	}
	if id > 0 {
		query += " AND task_id=?"
		args = append(args, id)
	}
	query += " ORDER BY id DESC LIMIT ?"
	args = append(args, limit)
	rows, err := app.store.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Object{}
	for rows.Next() {
		var runID, taskID int64
		var runKind, name, command, status, logPath, created string
		var started, finished *string
		var exitCode *int
		var duration float64
		if err := rows.Scan(&runID, &taskID, &runKind, &name, &command, &status, &started, &finished, &exitCode, &duration, &logPath, &created); err != nil {
			return nil, err
		}
		result = append(result, Object{"id": runID, "taskId": taskID, "kind": runKind, "name": name, "command": command, "status": status, "started": started, "finished": finished, "exitCode": exitCode, "duration": duration, "logPath": logPath, "created": created})
	}
	return result, rows.Err()
}

func timestamp(value any) time.Time {
	parsed, _ := time.Parse(time.RFC3339Nano, text(value))
	return parsed
}
