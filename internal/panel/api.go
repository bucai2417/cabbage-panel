package panel

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
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
		if tail == "login-log" && request.Method == "GET" {
			logs, err := app.store.loginLogs()
			if err != nil {
				apiError(writer, err)
				return
			}
			ok(writer, app.enrichLoginLogs(logs))
			return
		}
		if tail == "ip-blacklist" {
			app.ipBlacklistAPI(writer, request)
			return
		}
		if tail == "" && request.Method == "GET" {
			// 对齐青龙 GET /api/user：{username, avatar, twoFactorActivated}。
			ok(writer, Object{"username": app.store.setting("username", ""), "avatar": "", "twoFactorActivated": false})
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
		// cron 表达式不强制校验：无效或非标准写法允许保存，只是不会自动调度。
		// 用户在任务列表里仍可手动运行。
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
		// 订阅的 cron 同样不强制校验，无效规则只是不会自动同步。
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
		if value := strings.TrimSpace(text(item["expiresAt"])); value != "" {
			if len(value) < 10 {
				return errors.New("应用有效期格式应为 YYYY-MM-DD")
			}
			if _, err := time.Parse("2006-01-02", value[:10]); err != nil {
				return errors.New("应用有效期格式应为 YYYY-MM-DD")
			}
			item["expiresAt"] = value[:10]
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
	if kind == "envs" {
		if _, found := item["remarks"]; !found {
			item["remarks"] = ""
		}
	}
	return item
}

// jsTimestamp 兼容青龙的 new Date().toString() 时间字符串。
func compatTimestamp(item Object) string {
	switch value := item["timestamp"].(type) {
	case string:
		if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
			return jsTimestamp(parsed)
		}
		return value
	case float64:
		if value > 0 {
			return jsTimestamp(time.UnixMilli(int64(value)))
		}
	case int64:
		if value > 0 {
			return jsTimestamp(time.UnixMilli(value))
		}
	}
	if created, valid := item["createdAt"].(string); valid {
		if parsed, err := time.Parse(time.RFC3339Nano, created); err == nil {
			return jsTimestamp(parsed)
		}
	}
	return jsTimestamp(time.Now())
}

// cronCompat 对齐青龙（稳定版）CronItem：
// 含 _id（字符串）、timeoutMs、notifyOnFinish、nextRunTime；extra_schedules 未设置时为 []。
// 面板内部字段（lastStatus、nextRun、duration、runningCount、remarks）保留供网页使用。
func cronCompat(item Object) Object {
	item["_id"] = text(item["id"])
	item["saved"] = true
	item["isSystem"] = integer(item["isSystem"])
	item["isDisabled"] = integer(item["isDisabled"])
	item["isPinned"] = integer(item["isPinned"])
	item["allow_multiple_instances"] = 0
	if boolean(item["allowConcurrent"]) {
		item["allow_multiple_instances"] = 1
	}
	item["notifyOnFinish"] = integer(item["notifyOnFinish"])
	item["timeoutMs"] = integer(item["timeout"]) * 1000
	item["last_running_time"] = integer(item["last_running_time"])
	item["last_execution_time"] = integer(item["last_execution_time"])
	item["nextRunTime"] = integer(item["nextRunTime"])
	for _, key := range []string{"pid", "sub_id", "task_before", "task_after", "work_dir"} {
		if _, found := item[key]; !found {
			item[key] = nil
		}
	}
	if item["extra_schedules"] == nil {
		item["extra_schedules"] = []any{}
	}
	if item["log_path"] == nil {
		item["log_path"] = ""
	}
	// log_name 未指定时，青龙用「{脚本名}_{任务ID}.log」。
	if text(item["log_name"]) == "" {
		item["log_name"] = defaultLogName(text(item["command"]), integer(item["id"]))
	}
	item["timestamp"] = compatTimestamp(item)
	return item
}

// defaultLogName 复刻青龙 getUniqPath：取命令中的脚本名（去扩展名）再拼 _{id}.log。
func defaultLogName(command string, id int64) string {
	name := scriptNameFromCommand(command)
	if name == "" {
		name = "task"
	}
	if id > 0 {
		return fmt.Sprintf("%s_%d.log", name, id)
	}
	return name + ".log"
}

// scriptNameFromCommand 从命令里提取脚本文件名并去掉扩展名，用于默认 log_name。
func scriptNameFromCommand(command string) string {
	fields := strings.Fields(command)
	for _, field := range fields {
		if strings.Contains(field, ".") && !strings.HasPrefix(field, "-") {
			base := filepath.Base(field)
			if index := strings.LastIndex(base, "."); index > 0 {
				return base[:index]
			}
		}
	}
	return ""
}

// envCompat 对齐青龙 Env 模型：没有 _id，只有 id；timestamp 为 new Date().toString() 形式。
func envCompat(item Object) Object {
	delete(item, "isDisabled")
	delete(item, "_id")
	item["status"] = integer(item["status"])
	item["isPinned"] = integer(item["isPinned"])
	item["position"] = integer(item["position"])
	if item["remarks"] == nil {
		item["remarks"] = ""
	}
	if item["labels"] == nil {
		item["labels"] = []any{}
	}
	item["timestamp"] = compatTimestamp(item)
	return item
}

func (app *App) cronView(item Object) Object {
	return cronCompat(app.engine.decorate("crons", item))
}

// normalizeCronInput 接受青龙的 timeoutMs / allow_multiple_instances 写入字段，并忽略客户端回传的只读字段。
// log_name / task_before / task_after / work_dir / notifyOnFinish 是用户可写字段，保留。
func normalizeCronInput(item Object) {
	if value, found := item["timeoutMs"]; found {
		if millis := integer(value); millis > 0 {
			item["timeout"] = millis / 1000
		}
	}
	if value, found := item["allow_multiple_instances"]; found {
		item["allowConcurrent"] = integer(value) == 1 || boolean(value)
	}
	for _, key := range []string{"_id", "saved", "pid", "log_path", "last_running_time", "last_execution_time", "nextRunTime", "nextRun", "lastRun", "lastFinish", "lastStatus", "duration", "runningCount", "status", "createdAt", "updatedAt", "queued_token", "timeoutMs", "allow_multiple_instances"} {
		delete(item, key)
	}
}

func normalizeEnvInput(item Object) {
	for _, key := range []string{"_id", "isDisabled", "createdAt", "updatedAt"} {
		delete(item, key)
	}
}

func (app *App) recordsAPI(writer http.ResponseWriter, request *http.Request, kind, tail string) {
	method := request.Method
	if kind == "crons" && method == "GET" && strings.HasSuffix(tail, "/source") {
		app.taskSource(writer, request, integer(strings.TrimSuffix(tail, "/source")))
		return
	}
	if kind == "crons" && strings.HasPrefix(tail, "views") {
		app.viewsAPI(writer, request, strings.Trim(strings.TrimPrefix(tail, "views"), "/"))
		return
	}
	// 青龙 GET /crons/import：从 crontab 导入任务，面板没有系统 crontab，返回空结果。
	if kind == "crons" && tail == "import" && method == "GET" {
		ok(writer, []Object{})
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
			switch kind {
			case "crons":
				item = app.cronView(item)
			case "envs":
				item = envCompat(item)
			case "subscriptions":
				item = app.engine.decorate(kind, item)
			case "dependencies":
				item = dependencyCompat(item, app.dependencyLog(integer(item["id"])))
			}
			delete(item, "secret_hash")
			delete(item, "client_secret")
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
			// 环境变量按启用状态分组：启用在先，停用的单独排在下方；组内再按置顶、position 排序。
			// position 在启停切换时保持不变，因此重新启用后会回到原位置。
			if kind == "envs" && (integer(l["status"]) == 0) != (integer(r["status"]) == 0) {
				return integer(l["status"]) == 0
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
			// data 直接是任务数组，不再嵌套分页对象。
			ok(writer, filtered[start:end])
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
		// 青龙 POST /crons 接收单个对象；数组仍接受以兼容批量写入。
		if len(batch) == 0 || len(batch) > 500 || (method == "PUT" && len(batch) != 1) {
			fail(writer, 400, "记录数量无效")
			return
		}
		for index, item := range batch {
			if item == nil {
				fail(writer, 400, "记录不能为空")
				return
			}
			if kind == "crons" {
				normalizeCronInput(item)
			}
			if kind == "envs" {
				normalizeEnvInput(item)
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
				// 明文 secret 只在创建/重置的响应里返回一次，不进入数据库。
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
		for index := range result {
			switch kind {
			case "crons":
				result[index] = app.cronView(result[index])
			case "envs":
				result[index] = envCompat(result[index])
			}
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
	// 青龙 /envs/name：批量改名。
	if kind == "envs" && tail == "name" && method == "PUT" {
		var body struct {
			IDs  []int64 `json:"ids"`
			Name string  `json:"name"`
		}
		if err := decode(request, &body); err != nil {
			apiError(writer, err)
			return
		}
		if len(body.IDs) == 0 || !envName.MatchString(body.Name) {
			fail(writer, 400, "变量名无效")
			return
		}
		for _, id := range body.IDs {
			if _, err := app.store.get(kind, id); err != nil {
				apiError(writer, err)
				return
			}
			if _, err := app.store.save(kind, Object{"id": id, "name": body.Name}); err != nil {
				apiError(writer, err)
				return
			}
		}
		ok(writer, nil)
		return
	}
	// 青龙 /envs/upload：上传 JSON 文件批量导入变量。
	if kind == "envs" && tail == "upload" && method == "POST" {
		if err := request.ParseMultipartForm(5 << 20); err != nil {
			fail(writer, 400, "请使用 multipart/form-data 上传 env 文件")
			return
		}
		file, _, err := request.FormFile("env")
		if err != nil {
			fail(writer, 400, "缺少 env 文件字段")
			return
		}
		defer file.Close()
		var payload []Object
		if err := json.NewDecoder(io.LimitReader(file, 5<<20)).Decode(&payload); err != nil {
			fail(writer, 400, "文件内容必须是变量对象数组")
			return
		}
		created := []Object{}
		position := int64(0)
		if existing, _ := app.store.list("envs"); len(existing) > 0 {
			for _, entry := range existing {
				if value := integer(entry["position"]); value > position {
					position = value
				}
			}
			position++
		}
		for _, item := range payload {
			if item == nil {
				continue
			}
			name := text(item["name"])
			value := text(item["value"])
			if !envName.MatchString(name) || value == "" {
				fail(writer, 400, "每条数据的 name 或 value 不能为空")
				return
			}
			record := Object{"name": name, "value": value, "remarks": text(item["remarks"]), "labels": item["labels"], "position": position}
			defaults(kind, record)
			position++
			saved, err := app.store.save(kind, record)
			if err != nil {
				apiError(writer, err)
				return
			}
			created = append(created, envCompat(saved))
		}
		ok(writer, created)
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
					_, _ = app.store.db.Exec("DELETE FROM authorizations WHERE app_id=?", id)
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
			switch kind {
			case "crons":
				item = app.cronView(item)
			case "envs":
				item = envCompat(item)
			case "subscriptions":
				item = app.engine.decorate(kind, item)
			}
			if kind == "dependencies" {
				item = dependencyCompat(item, app.dependencyLog(id))
			}
			delete(item, "secret_hash")
			delete(item, "client_secret")
			ok(writer, item)
			return
		}
		if len(segments) == 2 {
			switch segments[1] {
			case "log":
				if method == "GET" {
					query := request.URL.Query()
					respond(writer, 200, app.cronLogPayload(
						kind, id,
						integer(query.Get("offset")),
						integer(query.Get("limit")),
						query.Get("tail") == "true" || query.Get("tail") == "1",
					))
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
			case "tokens":
				if kind == "apps" && method == "GET" {
					// 数据库只保存令牌散列，列表仅返回可管理元数据，不返回令牌本身。
					rows, err := app.store.db.Query("SELECT id,name,created,expires,permanent FROM authorizations WHERE app_id=? ORDER BY id DESC", id)
					if err != nil {
						apiError(writer, err)
						return
					}
					defer rows.Close()
					result := []Object{}
					nowUnix := time.Now().Unix()
					for rows.Next() {
						var tokenID, expires, permanent int64
						var name, created string
						if rows.Scan(&tokenID, &name, &created, &expires, &permanent) == nil {
							result = append(result, Object{"id": tokenID, "appId": id, "name": name, "created": created, "expires": expires, "permanent": permanent == 1, "active": permanent == 1 || expires > nowUnix})
						}
					}
					ok(writer, result)
					return
				}
				fail(writer, 405, "方法不支持")
				return
			case "token":
				if kind == "apps" && method == "POST" {
					if appExpired(item) {
						fail(writer, 403, "应用已过期")
						return
					}
					var body Object
					if err := decode(request, &body); err != nil {
						apiError(writer, err)
						return
					}
					ttl := sessionTTL(integer(body["expiresIn"]), 24*time.Hour)
					permanent := boolean(body["permanent"])
					if permanent {
						ttl = 0
					}
					token, record, err := app.createAppToken(id, text(item["name"]), ttl, permanent)
					if err != nil {
						apiError(writer, err)
						return
					}
					ok(writer, Object{
						"token": token, "token_type": "Bearer",
						"expiration": record["expires"], "expiresIn": record["expiresIn"], "permanent": record["permanent"], "id": record["id"],
						"name": text(item["name"]), "client_id": text(item["client_id"]),
					})
					return
				}
				fail(writer, 405, "方法不支持")
				return
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
					delete(item, "client_secret")
					saved, err := app.store.save(kind, item)
					if err != nil {
						apiError(writer, err)
						return
					}
					_, _ = app.store.db.Exec("DELETE FROM sessions WHERE principal=?", "app:"+text(id))
					_, _ = app.store.db.Exec("DELETE FROM authorizations WHERE app_id=?", id)
					delete(saved, "secret_hash")
					saved["client_secret"] = secret
					ok(writer, saved)
					return
				}
			}
		}
		if len(segments) == 3 && kind == "apps" && segments[1] == "tokens" && method == "DELETE" {
			tokenID := integer(segments[2])
			// 登记表中保存的已经是散列，可直接删除对应会话。
			var token string
			if err := app.store.db.QueryRow("SELECT token FROM authorizations WHERE id=? AND app_id=?", tokenID, id).Scan(&token); err != nil {
				apiError(writer, err)
				return
			}
			if _, err := app.store.db.Exec("DELETE FROM sessions WHERE token=?", token); err != nil {
				apiError(writer, err)
				return
			}
			if _, err := app.store.db.Exec("DELETE FROM authorizations WHERE id=?", tokenID); err != nil {
				apiError(writer, err)
				return
			}
			ok(writer, nil)
			return
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
	// 补齐青龙 CronView 字段：position/isDisabled/filters/sorts/filterRelation/type。
	decorate := func(item Object) Object {
		if _, found := item["position"]; !found {
			item["position"] = integer(item["id"]) * 1000000
		}
		if _, found := item["isDisabled"]; !found {
			item["isDisabled"] = 0
		}
		for _, key := range []string{"filters", "sorts", "filterRelation"} {
			if _, found := item[key]; !found {
				item[key] = nil
			}
		}
		if _, found := item["type"]; !found {
			item["type"] = 0
		}
		return item
	}
	if tail == "" && request.Method == "GET" {
		items, err := app.store.list("views")
		if err != nil {
			apiError(writer, err)
			return
		}
		for index := range items {
			items[index] = decorate(items[index])
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
	if tail == "move" && request.Method == "PUT" {
		var body struct {
			FromIndex int64 `json:"fromIndex"`
			ToIndex   int64 `json:"toIndex"`
			ID        int64 `json:"id"`
		}
		if err := decode(request, &body); err != nil {
			apiError(writer, err)
			return
		}
		items, err := app.store.list("views")
		if err != nil {
			apiError(writer, err)
			return
		}
		sort.SliceStable(items, func(left, right int) bool {
			return integer(items[left]["position"]) < integer(items[right]["position"])
		})
		from, to := int(body.FromIndex), int(body.ToIndex)
		if from < 0 || from >= len(items) || to < 0 || to >= len(items) {
			fail(writer, 400, "排序位置无效")
			return
		}
		moved := items[from]
		ordered := append(append(append([]Object{}, items[:from]...), items[from+1:]...), Object{})
		copy(ordered[to+1:], ordered[to:])
		ordered[to] = moved
		for index, item := range ordered {
			if _, err := app.store.save("views", Object{"id": item["id"], "position": (index + 1) * 1000000}); err != nil {
				apiError(writer, err)
				return
			}
		}
		ok(writer, nil)
		return
	}
	// 青龙 /crons/views/enable|disable：批量启用或停用视图。
	if (tail == "enable" || tail == "disable") && request.Method == "PUT" {
		ids, err := parseIDs(request)
		if err != nil {
			apiError(writer, err)
			return
		}
		disabled := 0
		if tail == "disable" {
			disabled = 1
		}
		for _, id := range ids {
			if _, err := app.store.save("views", Object{"id": id, "isDisabled": disabled}); err != nil {
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
