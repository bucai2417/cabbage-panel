package panel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFolderOperationsAndTaskSource(test *testing.T) {
	panel := setupPanel(test)
	panel.data(test, "POST", "/api/scripts/folder", Object{"path": "中文目录/空文件夹"})
	panel.data(test, "POST", "/api/scripts", Object{"filename": "中文目录/脚本 空格.py", "content": "print('中文')\n"})
	status, _ := panel.call(test, "POST", "/api/scripts", Object{"filename": "中文目录/脚本 空格.py", "content": "覆盖"}, panel.token)
	if status != 409 {
		test.Fatal("creation overwrote an existing file")
	}
	tree := panel.data(test, "GET", "/api/scripts/tree", nil).([]any)
	// 内置 notify.py / sendNotify.js 会随版本投放到 scripts 目录，这里只统计业务文件。
	business := []any{}
	for _, entry := range tree {
		name := text(entry.(map[string]any)["path"])
		if name == "notify.py" || name == "sendNotify.js" {
			continue
		}
		business = append(business, entry)
	}
	if len(business) != 3 {
		test.Fatalf("tree: %v", tree)
	}
	id := panel.task(test, "目录移动验证", "python "+shellQuote("中文目录/脚本 空格.py")+" --name 中文", nil)
	result := panel.data(test, "POST", "/api/scripts/operations", Object{"operation": "rename", "paths": []string{"中文目录"}, "destination": "新目录"}).([]any)
	if !boolean(result[0].(map[string]any)["success"]) || integer(result[0].(map[string]any)["updatedTasks"]) != 1 {
		test.Fatalf("move: %v", result)
	}
	source := panel.data(test, "GET", fmt.Sprintf("/api/crons/%d/source", id), nil).(map[string]any)
	if text(source["path"]) != "新目录/脚本 空格.py" || !strings.Contains(text(source["content"]), "中文") {
		test.Fatalf("source: %v", source)
	}
	task, _ := panel.app.store.get("crons", id)
	if !strings.HasSuffix(text(task["command"]), " --name 中文") {
		test.Fatal("arguments changed")
	}
	panel.data(test, "POST", "/api/scripts/folder", Object{"path": "副本"})
	result = panel.data(test, "POST", "/api/scripts/operations", Object{"operation": "copy", "paths": []string{"新目录", "新目录/脚本 空格.py"}, "destination": "副本"}).([]any)
	if len(result) != 1 || !boolean(result[0].(map[string]any)["success"]) {
		test.Fatalf("copy: %v", result)
	}
	panel.data(test, "GET", "/api/scripts/detail?path="+url.QueryEscape("副本/新目录/脚本 空格.py"), nil)
	for _, body := range []Object{
		{"operation": "move", "paths": []string{"新目录"}, "destination": "新目录/子目录"},
		{"operation": "copy", "paths": []string{"新目录"}, "destination": "副本"},
		{"operation": "delete", "paths": []string{"../config"}},
	} {
		rejected := panel.data(test, "POST", "/api/scripts/operations", body).([]any)
		if boolean(rejected[0].(map[string]any)["success"]) {
			test.Fatalf("unsafe operation accepted: %v", body)
		}
	}
	for _, command := range []string{"python -c 'print(1)'", "echo test.py", "task test.py && rm other.py"} {
		if _, err := scriptWord(command); err == nil {
			test.Fatalf("complex command resolved: %s", command)
		}
	}
	result = panel.data(test, "POST", "/api/scripts/operations", Object{"operation": "delete", "paths": []string{"副本"}}).([]any)
	if !boolean(result[0].(map[string]any)["success"]) {
		test.Fatalf("delete: %v", result)
	}
}

func TestBuiltinConfigurationAndHooks(test *testing.T) {
	panel := setupPanel(test)
	entries := panel.data(test, "GET", "/api/configs/tree", nil).([]any)
	if len(entries) != 5 {
		test.Fatalf("builtins: %v", entries)
	}
	settings := panel.data(test, "GET", "/api/configs/detail?path=system.json", nil)
	if !strings.Contains(text(settings), "bucai2417/cabbage-panel") {
		test.Fatal("default update repo missing")
	}
	panel.data(test, "PUT", "/api/configs", Object{"filename": "system.json", "content": "{\"concurrency\":2,\"timeout\":45}"})
	if panel.app.store.setting("concurrency", "") != "2" {
		test.Fatal("virtual file did not apply")
	}
	status, _ := panel.call(test, "PUT", "/api/configs", Object{"filename": "system.json", "content": "{\"concurrency\":0,\"timeout\":99}"}, panel.token)
	if status == 200 || panel.app.store.setting("timeout", "") != "45" {
		test.Fatal("invalid settings partially applied")
	}
	panel.data(test, "PUT", "/api/configs", Object{"filename": "config.sh", "content": "CHINESE_TEST='中文配置'\n"})
	panel.data(test, "PUT", "/api/configs", Object{"filename": "task_before.sh", "content": "echo 前置步骤\n"})
	panel.data(test, "PUT", "/api/configs", Object{"filename": "task_after.sh", "content": "echo 后置步骤:$CABBAGE_EXIT_CODE\n"})
	id := panel.task(test, "钩子验证", "echo \"$CHINESE_TEST\"; exit 3", nil)
	finished := panel.finished(test, panel.run(test, id))
	for _, expected := range []string{"中文配置", "前置步骤", "后置步骤:3", "退出码 3"} {
		if !strings.Contains(text(finished["content"]), expected) {
			test.Fatalf("missing %q in %v", expected, finished)
		}
	}
	history := panel.data(test, "GET", "/api/configs/history?path=system.json", nil).([]any)
	panel.data(test, "PUT", "/api/configs/history/restore", Object{"id": history[0].(map[string]any)["id"]})
	if panel.app.store.setting("concurrency", "") != "4" {
		test.Fatal("restore did not update settings")
	}
	status, _ = panel.call(test, "PUT", "/api/configs", Object{"filename": "notifications.json", "content": "{} {}"}, panel.token)
	if status == 200 {
		test.Fatal("trailing JSON accepted")
	}
}

func TestNotificationDeliveryAndFailures(test *testing.T) {
	panel := setupPanel(test)
	requests := make(chan Object, 10)
	remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload Object
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			writer.WriteHeader(400)
			return
		}
		requests <- payload
		writer.Header().Set("Content-Type", "application/json")
		if request.URL.Path == "/failed" {
			fmt.Fprint(writer, "{\"errcode\":400,\"errmsg\":\"invalid token\"}")
		} else {
			fmt.Fprint(writer, "{\"errcode\":0}")
		}
	}))
	defer remote.Close()
	config := NotificationConfig{NotifyFailures: true, Channels: []NotificationChannel{
		{Name: "中文测试", Type: "wecom", Enabled: true, URL: remote.URL},
		{Name: "失败反馈", Type: "wecom", Enabled: true, URL: remote.URL + "/failed"},
	}}
	panel.data(test, "PUT", "/api/system/notifications", config)
	results := panel.data(test, "POST", "/api/system/notifications", nil).([]any)
	if len(results) != 2 || !boolean(results[0].(map[string]any)["success"]) || boolean(results[1].(map[string]any)["success"]) {
		test.Fatalf("results: %v", results)
	}
	for count := 0; count < 2; count++ {
		payload := <-requests
		if !strings.Contains(text(payload["text"].(map[string]any)["content"]), "中文编码正常") {
			test.Fatal("unicode payload corrupted")
		}
	}
	raw := panel.data(test, "GET", "/api/configs/detail?path=notifications.json", nil)
	if !strings.Contains(text(raw), "中文测试") {
		test.Fatal("notification file not synchronized")
	}
	channel := NotificationChannel{Type: "dingtalk", Token: "token", Secret: "secret"}
	address, _ := channelPayload(channel, "标题", "内容")
	parsed, _ := url.Parse(address)
	if parsed.Query().Get("sign") == "" || parsed.Query().Get("timestamp") == "" {
		test.Fatal("ding signature missing")
	}
	_, payload := channelPayload(NotificationChannel{Type: "feishu", Token: "token", Secret: "secret"}, "标题", "内容")
	if text(payload["sign"]) == "" {
		test.Fatal("feishu signature missing")
	}
	config.NotifyFailures = false
	panel.data(test, "PUT", "/api/system/notifications", config)
	if err := panel.app.notify(context.Background(), "失败任务", "不会推送"); err != nil || len(requests) != 0 {
		test.Fatal("disabled failure notifications still sent")
	}
}

func TestScriptNotificationHelpers(test *testing.T) {
	panel := setupPanel(test)
	delivered := make(chan string, 3)
	remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload Object
		if json.NewDecoder(request.Body).Decode(&payload) != nil {
			writer.WriteHeader(400)
			return
		}
		delivered <- text(payload["title"])
		fmt.Fprint(writer, "{}")
	}))
	defer remote.Close()
	panel.data(test, "PUT", "/api/system/notifications", NotificationConfig{NotifyFailures: false, Channels: []NotificationChannel{{Type: "webhook", Name: "脚本通知", Enabled: true, URL: remote.URL}}})
	server := httptest.NewServer(panel.handler)
	defer server.Close()
	panel.app.helperMu.Lock()
	panel.app.helperAddress = server.URL + "/api/notifications/send"
	panel.app.helperMu.Unlock()
	commands := []string{
		shellQuote(findPython()) + " -c " + shellQuote("from notify import send; send('Python中文', '内容')"),
		"node -e " + shellQuote("require('sendNotify').sendNotify('JS中文', '内容').catch(error => { console.error(error); process.exitCode=1 })"),
		"notify 'Shell中文' '内容'",
	}
	for index, command := range commands {
		id := panel.task(test, fmt.Sprint("通知助手", index), command, nil)
		result := panel.finished(test, panel.run(test, id))
		if text(result["status"]) != "success" {
			test.Fatalf("helper failed: %v", result)
		}
	}
	if len(delivered) != 3 {
		test.Fatalf("notification count %d", len(delivered))
	}
	status, _ := panel.call(test, "POST", "/api/notifications/send", Object{"title": "bad", "content": "bad"}, "invalid")
	if status != 401 {
		test.Fatal("helper accepted invalid token")
	}
	status, _ = panel.call(test, "GET", "/api/configs", nil, panel.app.helperKey)
	if status != 401 {
		test.Fatal("helper token could read configuration")
	}
}

func TestRecursiveFileOperationsRejectSymlinks(test *testing.T) {
	panel := setupPanel(test)
	root := filepath.Join(panel.app.root, "scripts")
	if err := os.Mkdir(filepath.Join(root, "linked"), 0700); err != nil {
		test.Fatal(err)
	}
	if err := os.Symlink(test.TempDir(), filepath.Join(root, "linked", "outside")); err != nil {
		test.Skip("symlink unavailable on this host")
	}
	if _, err := panel.app.operateFile(root, "scripts", "linked", "copy", "copy"); err == nil {
		test.Fatal("copied symlink tree")
	}
}

func TestScriptRunAndDebugDoNotCreateCron(test *testing.T) {
	panel := setupPanel(test)
	panel.data(test, "POST", "/api/scripts", Object{"filename": "调试测试.py", "content": "print('已保存内容')\n"})
	before, err := panel.app.store.list("crons")
	if err != nil {
		test.Fatal(err)
	}
	run := panel.data(test, "PUT", "/api/scripts/run", Object{"filename": "调试测试.py"}).(map[string]any)
	log := panel.finished(test, integer(run["runId"]))
	if !strings.Contains(text(log["content"]), "已保存内容") {
		test.Fatalf("script run log: %v", log)
	}
	after, err := panel.app.store.list("crons")
	if err != nil {
		test.Fatal(err)
	}
	if len(after) != len(before) {
		test.Fatalf("script run created cron records: %v", after)
	}
	debug := panel.data(test, "POST", "/api/scripts/debug", Object{"filename": "调试测试.py", "content": "print('调试未保存内容')\n"}).(map[string]any)
	log = panel.finished(test, integer(debug["runId"]))
	content := text(log["content"])
	if !strings.Contains(content, "调试未保存内容") || strings.Contains(content, "已保存内容") {
		test.Fatalf("debug did not use edited content: %s", content)
	}
	waitUntil(test, 5*time.Second, func() bool {
		entries, readErr := os.ReadDir(filepath.Join(panel.app.root, "debug"))
		return readErr == nil && len(entries) == 0
	})
	status, _ := panel.call(test, "POST", "/api/scripts/debug", Object{"filename": "调试测试.exe", "content": "x"}, panel.token)
	if status != 400 {
		test.Fatalf("unsupported debug extension status %d", status)
	}
}

func TestResourceDefaultOrderingAndPinning(test *testing.T) {
	panel := setupPanel(test)
	panel.data(test, "POST", "/api/envs", []Object{{"name": "FIRST_ENV", "value": "1"}})
	secondEnv := panel.data(test, "POST", "/api/envs", []Object{{"name": "SECOND_ENV", "value": "2"}}).([]any)[0].(map[string]any)
	envs := panel.data(test, "GET", "/api/envs", nil).([]any)
	if text(envs[0].(map[string]any)["name"]) != "FIRST_ENV" || text(envs[1].(map[string]any)["name"]) != "SECOND_ENV" {
		test.Fatalf("env order: %v", envs)
	}
	panel.data(test, "PUT", "/api/envs/pin", []int64{integer(secondEnv["id"])})
	envs = panel.data(test, "GET", "/api/envs", nil).([]any)
	if text(envs[0].(map[string]any)["name"]) != "SECOND_ENV" {
		test.Fatalf("env pin order: %v", envs)
	}
	firstTask := panel.task(test, "旧任务", "echo first", nil)
	secondTask := panel.task(test, "新任务", "echo second", nil)
	list := panel.data(test, "GET", "/api/crons", nil).([]any)
	if integer(list[0].(map[string]any)["id"]) != secondTask || integer(list[1].(map[string]any)["id"]) != firstTask {
		test.Fatalf("task creation order: %v", list)
	}
	panel.data(test, "PUT", "/api/crons/disable", []int64{secondTask})
	list = panel.data(test, "GET", "/api/crons", nil).([]any)
	if integer(list[0].(map[string]any)["id"]) != firstTask || integer(list[1].(map[string]any)["id"]) != secondTask {
		test.Fatalf("disabled task order: %v", list)
	}
	panel.data(test, "PUT", "/api/crons/pin", []int64{secondTask})
	list = panel.data(test, "GET", "/api/crons", nil).([]any)
	if integer(list[0].(map[string]any)["id"]) != firstTask {
		test.Fatalf("disabled pinned task floated above enabled task: %v", list)
	}
}

func TestAuditIPRemoteAndAppAuthorization(test *testing.T) {
	panel := setupPanel(test)
	app := panel.data(test, "POST", "/api/apps", Object{"name": "授权测试", "scopes": []string{"crons"}}).(map[string]any)
	if text(app["client_secret"]) == "" {
		test.Fatal("app secret is not readable after creation")
	}
	listed := panel.data(test, "GET", "/api/apps", nil).([]any)
	for _, entry := range listed {
		item := entry.(map[string]any)
		if integer(item["id"]) == integer(app["id"]) && (text(item["client_secret"]) != "" || text(item["secret_hash"]) != "") {
			test.Fatal("app secret must not be stored or listed in plaintext")
		}
	}
	created := panel.data(test, "POST", fmt.Sprintf("/api/apps/%d/token", integer(app["id"])), Object{"expiresIn": 3600}).(map[string]any)
	token := text(created["token"])
	if token == "" || integer(created["expiresIn"]) != 3600 {
		test.Fatalf("app authorization: %v", created)
	}
	status, _ := panel.call(test, "GET", "/open/crons", nil, token)
	if status != 200 {
		test.Fatalf("app authorization could not access crons: %d", status)
	}
	query := "/open/auth/token"
	status, response := panel.call(test, "POST", query, Object{"client_id": text(app["client_id"]), "client_secret": text(app["client_secret"]), "expiresIn": 1800}, "")
	if status != 200 || integer(response["data"].(map[string]any)["expiresIn"]) != 1800 {
		test.Fatalf("custom token ttl: %d %v", status, response)
	}
	permanent := panel.data(test, "POST", fmt.Sprintf("/api/apps/%d/token", integer(app["id"])), Object{"permanent": true}).(map[string]any)
	if !boolean(permanent["permanent"]) || integer(permanent["expiration"]) != 0 || integer(permanent["expiresIn"]) != 0 {
		test.Fatalf("permanent authorization: %v", permanent)
	}
	permanentToken := text(permanent["token"])
	status, _ = panel.call(test, "GET", "/open/crons", nil, permanentToken)
	if status != 200 {
		test.Fatalf("permanent authorization could not access crons: %d", status)
	}
	var storedExpiry int64
	if err := panel.app.store.db.QueryRow("SELECT expires FROM sessions WHERE token=?", hash(permanentToken)).Scan(&storedExpiry); err != nil || storedExpiry != 0 {
		test.Fatalf("permanent session expiry: %d %v", storedExpiry, err)
	}
	tokens := panel.data(test, "GET", fmt.Sprintf("/api/apps/%d/tokens", integer(app["id"])), nil).([]any)
	if len(tokens) < 2 {
		test.Fatalf("authorization registry: %v", tokens)
	}
	var revokeID int64
	for _, entry := range tokens {
		item := entry.(map[string]any)
		// 令牌以加密形式保存，列表接口返回解密后的明文，便于管理员随时查看。
		if text(item["token"]) == "" {
			test.Fatalf("authorization listing must expose the decrypted token: %v", item)
		}
		if text(item["secret_hash"]) != "" {
			test.Fatal("authorization listing must not expose secret_hash")
		}
		// 限时令牌 expires 非 0；永久令牌 expires 为 0。
		if integer(item["expires"]) != 0 && integer(item["expires"]) > time.Now().Unix() {
			revokeID = integer(item["id"])
		}
	}
	if revokeID == 0 {
		test.Fatal("stored authorization token not listed")
	}
	panel.data(test, "DELETE", fmt.Sprintf("/api/apps/%d/tokens/%d", integer(app["id"]), revokeID), nil)
	status, _ = panel.call(test, "GET", "/open/crons", nil, token)
	if status != 401 {
		test.Fatalf("revoked authorization remained valid: %d", status)
	}
	permanentQuery := Object{"client_id": text(app["client_id"]), "client_secret": text(app["client_secret"]), "permanent": true}
	status, response = panel.call(test, "POST", "/open/auth/token", permanentQuery, "")
	if status != 200 || !boolean(response["data"].(map[string]any)["permanent"]) {
		test.Fatalf("credential permanent token: %d %v", status, response)
	}
	expiredApp := panel.data(test, "POST", "/api/apps", Object{"name": "过期应用", "scopes": []string{"crons"}, "expiresAt": "2020-01-01"}).(map[string]any)
	status, _ = panel.call(test, "POST", fmt.Sprintf("/api/apps/%d/token", integer(expiredApp["id"])), Object{"expiresIn": 3600}, panel.token)
	if status != 403 {
		test.Fatalf("expired app issued authorization: %d", status)
	}
	expiredQuery := Object{"client_id": text(expiredApp["client_id"]), "client_secret": text(expiredApp["client_secret"])}
	status, _ = panel.call(test, "POST", "/open/auth/token", expiredQuery, "")
	if status != 403 {
		test.Fatalf("expired app exchanged credentials: %d", status)
	}
	panel.data(test, "PUT", "/api/apps", Object{"id": integer(app["id"]), "expiresAt": "2020-01-01"})
	status, _ = panel.call(test, "GET", "/open/crons", nil, token)
	if status != 401 {
		test.Fatalf("token of expired app remained valid: %d", status)
	}
	status, _ = panel.call(test, "GET", "/open/crons", nil, permanentToken)
	if status != 401 {
		test.Fatalf("permanent token outlived expired app: %d", status)
	}
	encoded, _ := json.Marshal(Object{"username": "admin", "password": "test-password-123"})
	request := httptest.NewRequest("POST", "/api/user/login", bytes.NewReader(encoded))
	request.RemoteAddr = "203.0.113.9:4444"
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	panel.handler.ServeHTTP(recorder, request)
	if recorder.Code != 200 {
		test.Fatalf("remote login status %d: %s", recorder.Code, recorder.Body.String())
	}
	logs := panel.data(test, "GET", "/api/user/login-log", nil).([]any)
	found := false
	for _, entry := range logs {
		item := entry.(map[string]any)
		if text(item["ip"]) == "203.0.113.9" {
			found = true
			if integer(item["status"]) != 0 {
				test.Fatalf("successful login recorded as failed: %v", item)
			}
			if integer(item["timestamp"]) == 0 || item["platform"] == nil {
				test.Fatalf("login log missing compatible fields: %v", item)
			}
		}
	}
	if !found {
		test.Fatal("login IP not recorded")
	}
}

func TestTokenCipherRoundTripAndExpiryPurge(test *testing.T) {
	panel := setupPanel(test)
	cipherSet := panel.app.tokens
	if cipherSet == nil {
		test.Fatal("token cipher not initialized")
	}
	secret, err := cipherSet.encrypt("plain-token-value")
	if err != nil {
		test.Fatal(err)
	}
	if secret == "plain-token-value" || secret == "" {
		test.Fatal("token was not encrypted")
	}
	plain, err := cipherSet.decrypt(secret)
	if err != nil || plain != "plain-token-value" {
		test.Fatalf("cipher round trip failed: %q %v", plain, err)
	}
	if _, err := cipherSet.decrypt("not-base64!!"); err == nil {
		test.Fatal("corrupted ciphertext accepted")
	}

	app := panel.data(test, "POST", "/api/apps", Object{"name": "多令牌应用", "scopes": []string{"crons", "mcp"}}).(map[string]any)
	appID := integer(app["id"])
	// 生成三个令牌：两个限时、一个永久。
	created := []string{}
	for _, payload := range []Object{{"expiresIn": 3600}, {"expiresIn": 7200}, {"permanent": true}} {
		result := panel.data(test, "POST", fmt.Sprintf("/api/apps/%d/token", appID), payload).(map[string]any)
		created = append(created, text(result["token"]))
	}
	listed := panel.data(test, "GET", fmt.Sprintf("/api/apps/%d/tokens", appID), nil).([]any)
	if len(listed) != 3 {
		test.Fatalf("expected three authorizations: %v", listed)
	}
	seen := map[string]bool{}
	for _, entry := range listed {
		token := text(entry.(map[string]any)["token"])
		if token == "" {
			test.Fatal("token listing returned empty token")
		}
		seen[token] = true
	}
	for _, token := range created {
		if !seen[token] {
			test.Fatal("listed token does not match the issued token")
		}
	}
	// 明文不应以原文形式落库。
	var raw string
	if err := panel.app.store.db.QueryRow("SELECT token_secret FROM authorizations WHERE app_id=? LIMIT 1", appID).Scan(&raw); err != nil {
		test.Fatal(err)
	}
	for _, token := range created {
		if raw == token {
			test.Fatal("token stored in plaintext")
		}
	}

	// 把一条令牌改成已过期，清理后应被删除，且会话同步失效。
	var expiredID int64
	var expiredHash string
	if err := panel.app.store.db.QueryRow("SELECT id, token FROM authorizations WHERE app_id=? AND permanent=0 ORDER BY id LIMIT 1", appID).Scan(&expiredID, &expiredHash); err != nil {
		test.Fatal(err)
	}
	if _, err := panel.app.store.db.Exec("UPDATE authorizations SET expires=? WHERE id=?", time.Now().Add(-time.Hour).Unix(), expiredID); err != nil {
		test.Fatal(err)
	}
	panel.app.purgeExpiredAuthorizations()
	var remaining int
	if err := panel.app.store.db.QueryRow("SELECT COUNT(*) FROM authorizations WHERE id=?", expiredID).Scan(&remaining); err != nil {
		test.Fatal(err)
	}
	if remaining != 0 {
		test.Fatal("expired authorization not purged")
	}
	if err := panel.app.store.db.QueryRow("SELECT COUNT(*) FROM sessions WHERE token=?", expiredHash).Scan(&remaining); err != nil {
		test.Fatal(err)
	}
	if remaining != 0 {
		test.Fatal("expired authorization session not removed")
	}
	if len(panel.data(test, "GET", fmt.Sprintf("/api/apps/%d/tokens", appID), nil).([]any)) != 2 {
		test.Fatal("purge removed the wrong authorizations")
	}
}

// task 助手依赖 bash 数组处理并发账号。Debian 的 /bin/sh 是 dash，
// 直接以 sh 解释会报 `Syntax error: "(" unexpected`，因此脚本必须能自举到 bash。
func TestTaskHelperBootstrapsBash(test *testing.T) {
	dir := test.TempDir()
	target := filepath.Join(dir, "task")
	if err := os.WriteFile(target, []byte(taskHelperSource), 0700); err != nil {
		test.Fatal(err)
	}
	script := filepath.Join(dir, "echo.py")
	if err := os.WriteFile(script, []byte("print('task helper ok')\n"), 0600); err != nil {
		test.Fatal(err)
	}
	python := findPython()
	// 分别用 bash 与 sh 执行，sh 场景验证自举逻辑不会因 dash 不支持数组而失败。
	for _, shell := range []string{"bash", "sh"} {
		if _, err := exec.LookPath(shell); err != nil {
			continue
		}
		command := exec.Command(shell, target, script)
		command.Dir = dir
		command.Env = append(os.Environ(), "CABBAGE_PYTHON="+python, "PYTHONIOENCODING=utf-8", "PYTHONUTF8=1")
		output, err := command.CombinedOutput()
		if err != nil {
			test.Fatalf("%s task helper failed: %v\n%s", shell, err, output)
		}
		if !strings.Contains(string(output), "task helper ok") {
			test.Fatalf("%s task helper output mismatch: %s", shell, output)
		}
	}
}

func TestSetupKeyLoggedOnFirstBootOnly(test *testing.T) {
	root := test.TempDir()

	captureLog := func(run func()) string {
		var buffer bytes.Buffer
		previous := log.Writer()
		log.SetOutput(&buffer)
		defer log.SetOutput(previous)
		run()
		return buffer.String()
	}

	var firstKey string
	firstLog := captureLog(func() {
		app, err := New(root, "1.0.0", "test", "")
		if err != nil {
			test.Fatal(err)
		}
		firstKey = app.setupKey
		app.Close()
	})
	if firstKey == "" {
		test.Fatal("首次启动未生成设置密钥")
	}
	// 首次启动应把密钥本身打进日志，方便容器环境直接从 docker logs 读取。
	if !strings.Contains(firstLog, firstKey) {
		test.Fatalf("首次启动日志未包含密钥，实际输出: %s", firstLog)
	}

	// 仍无管理员时重启，不应重复打印密钥，只提示路径。
	secondLog := captureLog(func() {
		app, err := New(root, "1.0.0", "test", "")
		if err != nil {
			test.Fatal(err)
		}
		app.Close()
	})
	if strings.Contains(secondLog, firstKey) {
		test.Fatal("重启时重复打印了设置密钥")
	}
	if !strings.Contains(secondLog, "setup.key") {
		test.Fatalf("重启时未提示 setup.key 路径，实际输出: %s", secondLog)
	}
}
