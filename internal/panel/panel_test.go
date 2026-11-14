package panel

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type testPanel struct {
	app     *App
	handler http.Handler
	token   string
}

func setupPanel(test *testing.T) *testPanel {
	test.Helper()
	app, err := New(test.TempDir(), "1.0.0", "test", "")
	if err != nil {
		test.Fatal(err)
	}
	test.Cleanup(app.Close)
	// 测试使用 127.0.0.1 上的 httptest 服务模拟外部依赖，显式加入出站白名单。
	if err := app.store.setSetting("outboundAllowlist", "127.0.0.1, ::1"); err != nil {
		test.Fatal(err)
	}
	panel := &testPanel{app: app, handler: app.Handler()}
	status, _ := panel.call(test, "POST", "/api/user/init", Object{"username": "admin", "password": "test-password-123", "setupToken": app.setupKey}, "")
	if status != 200 {
		test.Fatalf("init status %d", status)
	}
	status, response := panel.call(test, "POST", "/api/user/login", Object{"username": "admin", "password": "test-password-123"}, "")
	if status != 200 {
		test.Fatalf("login: %v", response)
	}
	panel.token = text(response["data"].(map[string]any)["token"])
	return panel
}

func (panel *testPanel) call(test *testing.T, method, path string, body any, token string) (int, Object) {
	test.Helper()
	return panel.callRemote(test, method, path, "127.0.0.1:10000", body, token)
}
func (panel *testPanel) callRemote(test *testing.T, method, path, remote string, body any, token string) (int, Object) {
	test.Helper()
	content, err := json.Marshal(body)
	if err != nil {
		test.Fatal(err)
	}
	request := httptest.NewRequest(method, path, bytes.NewReader(content))
	request.RemoteAddr = remote
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	panel.handler.ServeHTTP(response, request)
	var result Object
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		test.Fatalf("invalid JSON %s: %s", path, response.Body.String())
	}
	return response.Code, result
}
func (panel *testPanel) data(test *testing.T, method, path string, body any) any {
	test.Helper()
	status, response := panel.call(test, method, path, body, panel.token)
	if status != 200 {
		test.Fatalf("%s %s: %d %v", method, path, status, response)
	}
	return response["data"]
}
func (panel *testPanel) task(test *testing.T, name, command string, extra Object) int64 {
	test.Helper()
	item := Object{"name": name, "command": command, "schedule": ""}
	for key, value := range extra {
		item[key] = value
	}
	result := panel.data(test, "POST", "/api/crons", item).(map[string]any)
	return integer(result["id"])
}
func (panel *testPanel) run(test *testing.T, id int64) int64 {
	test.Helper()
	result := panel.data(test, "PUT", "/api/crons/run", []int64{id}).([]any)
	return integer(result[0].(map[string]any)["runId"])
}
func waitUntil(test *testing.T, timeout time.Duration, condition func() bool) {
	test.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	test.Fatal("condition timed out")
}
func (panel *testPanel) finished(test *testing.T, id int64) Object {
	test.Helper()
	var result Object
	waitUntil(test, 20*time.Second, func() bool {
		var status string
		_ = panel.app.store.db.QueryRow("SELECT status FROM runs WHERE id=?", id).Scan(&status)
		if status != "running" && status != "queued" {
			data := panel.data(test, "GET", "/api/logs/run/"+text(id), nil)
			result = Object(data.(map[string]any))
			return true
		}
		return false
	})
	return result
}

func TestQingLongRecordShapes(test *testing.T) {
	panel := setupPanel(test)
	panel.data(test, "POST", "/api/envs", []Object{{"name": "YPCK", "value": "666", "remarks": ""}})
	panel.task(test, "联通", "task bucai/中国联通-v1.1.2.py", Object{"schedule": "1 0 * * *"})

	envList := panel.data(test, "GET", "/api/envs", nil).([]any)
	env := envList[0].(map[string]any)
	// 青龙 Env 模型没有 _id，只有数字 id。
	if _, ok := env["_id"]; ok {
		test.Fatalf("env should not expose _id: %v", env)
	}
	if integer(env["id"]) == 0 || text(env["name"]) != "YPCK" || text(env["value"]) != "666" {
		test.Fatalf("env identity mismatch: %v", env)
	}
	if _, ok := env["isDisabled"]; ok {
		test.Fatal("env should not expose isDisabled")
	}
	for _, key := range []string{"timestamp", "status", "position", "remarks", "isPinned", "labels"} {
		if _, ok := env[key]; !ok {
			test.Fatalf("env missing %s: %v", key, env)
		}
	}
	if _, err := time.Parse("Mon Jan 02 15:04:05 GMT-07:00 2006", text(env["timestamp"])); err != nil {
		test.Fatalf("env timestamp %q: %v", text(env["timestamp"]), err)
	}

	// data 直接是任务数组，不再嵌套分页对象。
	cronList := panel.data(test, "GET", "/api/crons", nil).([]any)
	if len(cronList) != 1 {
		test.Fatalf("cron list length mismatch: %v", cronList)
	}
	cron := cronList[0].(map[string]any)
	// 青龙（稳定版）CronItem 含 _id（字符串）。
	if text(cron["_id"]) != text(cron["id"]) || text(cron["name"]) != "联通" {
		test.Fatalf("cron identity mismatch: %v", cron)
	}
	for _, key := range []string{"timestamp", "saved", "status", "isSystem", "pid", "isDisabled", "log_path", "isPinned", "labels", "last_running_time", "last_execution_time", "sub_id", "extra_schedules", "task_before", "task_after", "log_name", "allow_multiple_instances", "work_dir", "timeoutMs", "notifyOnFinish", "nextRunTime"} {
		if _, ok := cron[key]; !ok {
			test.Fatalf("cron missing %s: %v", key, cron)
		}
	}
	if cron["saved"] != true || integer(cron["isSystem"]) != 0 || integer(cron["timeoutMs"]) != 0 || integer(cron["allow_multiple_instances"]) != 0 {
		test.Fatalf("cron QingLong fields mismatch: %v", cron)
	}
	if cron["pid"] != nil || cron["sub_id"] != nil || cron["task_before"] != nil || cron["task_after"] != nil || cron["work_dir"] != nil {
		test.Fatalf("cron null fields mismatch: %v", cron)
	}
	if integer(cron["nextRunTime"]) <= 0 {
		test.Fatalf("cron nextRunTime missing: %v", cron)
	}
	if _, ok := cron["extra_schedules"].([]any); !ok {
		test.Fatalf("cron extra_schedules should be an array: %v", cron)
	}
	if !strings.HasSuffix(text(cron["log_name"]), ".log") {
		test.Fatalf("cron log_name should end with .log: %v", cron)
	}
	if _, err := time.Parse("Mon Jan 02 15:04:05 GMT-07:00 2006", text(cron["timestamp"])); err != nil {
		test.Fatalf("cron timestamp %q: %v", text(cron["timestamp"]), err)
	}
}

func TestAuthScopesAndMCP(test *testing.T) {
	panel := setupPanel(test)
	status, _ := panel.call(test, "GET", "/api/crons", nil, "")
	if status != 401 {
		test.Fatal("anonymous access allowed")
	}
	created := panel.data(test, "POST", "/api/apps", Object{"name": "测试客户端", "scopes": []string{"crons", "mcp"}}).(map[string]any)
	listed := panel.data(test, "GET", "/api/apps", nil).([]any)
	if _, found := listed[0].(map[string]any)["secret_hash"]; found {
		test.Fatal("secret hash leaked")
	}
	if text(listed[0].(map[string]any)["client_secret"]) != "" {
		test.Fatal("client secret must not be stored or listed")
	}
	status, response := panel.call(test, "POST", "/open/auth/token", Object{"client_id": text(created["client_id"]), "client_secret": text(created["client_secret"])}, "")
	if status != 200 {
		test.Fatal(response)
	}
	token := text(response["data"].(map[string]any)["token"])
	status, _ = panel.call(test, "GET", "/open/envs", nil, token)
	if status != 403 {
		test.Fatal("scope bypass")
	}
	status, response = panel.call(test, "POST", "/mcp", Object{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": Object{"protocolVersion": "2025-11-25"}}, token)
	if status != 200 || response["result"].(map[string]any)["protocolVersion"] != "2025-11-25" {
		test.Fatal(response)
	}
	_, response = panel.call(test, "POST", "/mcp", Object{"jsonrpc": "2.0", "id": 2, "method": "tools/list"}, token)
	listedTools := response["result"].(map[string]any)["tools"].([]any)
	if len(listedTools) != 8 {
		test.Fatalf("unexpected tool access %d", len(listedTools))
	}
	_, response = panel.call(test, "POST", "/mcp", Object{"jsonrpc": "2.0", "id": 3, "method": "tools/call", "params": Object{"name": "create_task", "arguments": Object{"name": "MCP 中文任务", "command": "echo MCP", "schedule": ""}}}, token)
	if response["result"].(map[string]any)["isError"] != false {
		test.Fatal(response)
	}
	status, response = panel.call(test, "GET", "/open/crons", nil, token)
	if status != 200 || len(response["data"].([]any)) != 1 {
		test.Fatal(response)
	}
	status, response = panel.call(test, "GET", "/api/crons", nil, panel.token)
	if status != 200 || len(response["data"].([]any)) != 1 {
		test.Fatal(response)
	}
	panel.data(test, "DELETE", "/api/apps", []int64{integer(created["id"])})
	status, _ = panel.call(test, "GET", "/open/crons", nil, token)
	if status != 401 {
		test.Fatal("deleted app token still valid")
	}
	request := httptest.NewRequest("PUT", "/api/crons/run", strings.NewReader("[1]"))
	request.Header.Set("Origin", "https://evil.invalid")
	request.Header.Set("Authorization", "Bearer "+panel.token)
	recorder := httptest.NewRecorder()
	panel.handler.ServeHTTP(recorder, request)
	if recorder.Code != 403 {
		test.Fatal("cross origin request allowed")
	}
}

func TestUTF8ScriptsAndEnvironment(test *testing.T) {
	panel := setupPanel(test)
	panel.data(test, "POST", "/api/envs", []Object{{"name": "CABBAGE_TEST", "value": "白菜测试✓", "remarks": "中文环境变量"}})
	scripts := []struct{ name, content string }{{"中文.py", "import os\nprint('你好 Python ' + os.environ['CABBAGE_TEST'])\n"}, {"中文.js", "console.log('你好 Node ' + process.env.CABBAGE_TEST)\n"}, {"中文.sh", "printf '你好 Shell %s\\n' \"$CABBAGE_TEST\"\n"}}
	for _, script := range scripts {
		test.Run(script.name, func(test *testing.T) {
			panel.data(test, "POST", "/api/scripts", Object{"filename": script.name, "content": script.content})
			content := panel.data(test, "GET", "/api/scripts/detail?path="+script.name, nil)
			if content != script.content {
				test.Fatal("UTF-8 roundtrip failed")
			}
			id := panel.task(test, script.name, "task "+shellQuote(script.name), nil)
			runID := panel.run(test, id)
			result := panel.finished(test, runID)
			if result["status"] != "success" || !strings.Contains(text(result["content"]), "白菜测试✓") {
				test.Fatal(result)
			}
		})
	}
	panel.data(test, "PUT", "/api/scripts", Object{"filename": "中文.py", "content": "print('修改')\n"})
	history := panel.data(test, "GET", "/api/scripts/history?path=中文.py", nil).([]any)
	if len(history) != 1 {
		test.Fatalf("history count %d", len(history))
	}
	panel.data(test, "PUT", "/api/scripts/history/restore", Object{"id": history[0].(map[string]any)["id"]})
	if panel.data(test, "GET", "/api/scripts/detail?path=中文.py", nil) != scripts[0].content {
		test.Fatal("history restore failed")
	}
}

func TestConcurrencyCancellationAndTimeout(test *testing.T) {
	panel := setupPanel(test)
	panel.data(test, "PUT", "/api/system/settings", Object{"concurrency": "2"})
	ids := []int64{}
	for _, name := range []string{"甲", "乙", "丙"} {
		ids = append(ids, panel.task(test, name, "sleep 3; echo finished", nil))
	}
	first := panel.run(test, ids[0])
	second := panel.run(test, ids[1])
	third := panel.run(test, ids[2])
	_ = first
	_ = second
	status, _ := panel.call(test, "PUT", "/api/crons/run", []int64{ids[0]}, panel.token)
	if status != 400 {
		test.Fatal("duplicate task not rejected")
	}
	waitUntil(test, 5*time.Second, func() bool {
		var count int
		_ = panel.app.store.db.QueryRow("SELECT COUNT(*) FROM runs WHERE status='running'").Scan(&count)
		return count == 2
	})
	var queued string
	_ = panel.app.store.db.QueryRow("SELECT status FROM runs WHERE id=?", third).Scan(&queued)
	if queued != "queued" {
		test.Fatal("concurrency limit exceeded")
	}
	panel.data(test, "PUT", "/api/crons/stop", ids)
	for _, runID := range []int64{first, second, third} {
		result := panel.finished(test, runID)
		if result["status"] != "cancelled" {
			test.Fatal(result)
		}
	}
	id := panel.task(test, "超时", "sleep 10", Object{"timeout": 1})
	result := panel.finished(test, panel.run(test, id))
	if result["status"] != "timeout" {
		test.Fatal(result)
	}
	id = panel.task(test, "失败", "exit 7", nil)
	runID := panel.run(test, id)
	result = panel.finished(test, runID)
	var exitCode int
	_ = panel.app.store.db.QueryRow("SELECT exit_code FROM runs WHERE id=?", runID).Scan(&exitCode)
	if result["status"] != "failed" || exitCode != 7 {
		test.Fatal(result, exitCode)
	}
}

func TestCronSchedulingAndDisable(test *testing.T) {
	panel := setupPanel(test)
	id := panel.task(test, "秒级任务", "echo scheduled", Object{"schedule": "*/1 * * * * *"})
	waitUntil(test, 5*time.Second, func() bool {
		var count int
		_ = panel.app.store.db.QueryRow("SELECT COUNT(*) FROM runs WHERE task_id=?", id).Scan(&count)
		return count > 0
	})
	panel.data(test, "PUT", "/api/crons/disable", []int64{id})
	waitUntil(test, 5*time.Second, func() bool { return panel.app.engine.active() == 0 })
	var before int
	_ = panel.app.store.db.QueryRow("SELECT COUNT(*) FROM runs WHERE task_id=?", id).Scan(&before)
	time.Sleep(1200 * time.Millisecond)
	var after int
	_ = panel.app.store.db.QueryRow("SELECT COUNT(*) FROM runs WHERE task_id=?", id).Scan(&after)
	if before != after {
		test.Fatal("disabled task still scheduled")
	}
	// 无效 cron 允许保存（不自动调度），不再返回 400。
	status, response := panel.call(test, "POST", "/api/crons", Object{"name": "bad", "command": "echo x", "schedule": "not a cron"}, panel.token)
	if status != 200 {
		test.Fatalf("invalid cron should be accepted for manual run: %v", response)
	}
	if boolean(response["data"].(map[string]any)["scheduleValid"]) {
		test.Fatal("invalid cron should be flagged scheduleValid=false")
	}
}

func TestBackupRestoreAndTraversal(test *testing.T) {
	panel := setupPanel(test)
	panel.task(test, "备份任务", "echo backup", nil)
	panel.data(test, "POST", "/api/scripts", Object{"filename": "目录/中文.py", "content": "print('白菜')\n"})
	panel.data(test, "POST", "/api/envs", []Object{{"name": "RESTORE_ME", "value": "secret"}})
	for _, path := range []string{"../escape.py", "/tmp/escape", "C:/escape", "folder/../../escape", "folder\\escape"} {
		status, _ := panel.call(test, "POST", "/api/scripts", Object{"filename": path, "content": "x"}, panel.token)
		if status != 400 {
			test.Fatal("unsafe path allowed", path)
		}
	}
	name := text(panel.data(test, "POST", "/api/backups/export", nil).(map[string]any)["name"])
	data, err := os.ReadFile(filepath.Join(panel.app.root, "backups", name))
	if err != nil {
		test.Fatal(err)
	}
	panel.data(test, "PUT", "/api/scripts", Object{"filename": "目录/中文.py", "content": "changed"})
	panel.task(test, "额外任务", "echo extra", nil)
	safety, err := panel.app.restoreBackup(data)
	if err != nil {
		test.Fatal(err)
	}
	if safety == "" {
		test.Fatal("no safety backup")
	}
	items, _ := panel.app.store.list("crons")
	if len(items) != 1 || text(items[0]["name"]) != "备份任务" {
		test.Fatal(items)
	}
	script, err := os.ReadFile(filepath.Join(panel.app.root, "scripts", "目录", "中文.py"))
	if err != nil || string(script) != "print('白菜')\n" {
		test.Fatal(string(script), err)
	}
	status, _ := panel.call(test, "GET", "/api/crons", nil, panel.token)
	if status != 401 {
		test.Fatal("restore did not revoke sessions")
	}
	var malicious bytes.Buffer
	archive := zip.NewWriter(&malicious)
	entry, _ := archive.Create("scripts/../../outside")
	_, _ = entry.Write([]byte("escape"))
	_ = archive.Close()
	if _, err := panel.app.restoreBackup(malicious.Bytes()); err == nil {
		test.Fatal("zip traversal accepted")
	}
}

func TestAutoBackupDirectoryAndSchedule(test *testing.T) {
	panel := setupPanel(test)
	if err := panel.app.store.setSetting("autoBackupEnabled", "true"); err != nil {
		test.Fatal(err)
	}
	if err := panel.app.store.setSetting("autoBackupTime", "03:00"); err != nil {
		test.Fatal(err)
	}
	first := time.Date(2026, 11, 14, 3, 0, 20, 0, time.Local)
	panel.app.maybeAutoBackup(first)
	items := panel.data(test, "GET", "/api/backups", nil).([]any)
	if len(items) != 1 {
		test.Fatalf("auto backup count %d", len(items))
	}
	item := items[0].(map[string]any)
	name := text(item["name"])
	if item["auto"] != true || !strings.HasPrefix(name, "auto-") {
		test.Fatalf("auto backup not stored in auto directory: %v", item)
	}
	if _, err := os.Stat(filepath.Join(panel.app.root, "backups", "auto", name)); err != nil {
		test.Fatal(err)
	}
	// 同一分钟重复检查不应产生第二份。
	panel.app.maybeAutoBackup(first.Add(20 * time.Second))
	if count := len(panel.data(test, "GET", "/api/backups", nil).([]any)); count != 1 {
		test.Fatalf("same-day auto backup duplicated: %d", count)
	}
	// 次日同一时间再备份一次。
	panel.app.maybeAutoBackup(first.Add(24 * time.Hour))
	if count := len(panel.data(test, "GET", "/api/backups", nil).([]any)); count != 2 {
		test.Fatalf("second-day auto backup missing: %d", count)
	}
	status, _ := panel.call(test, "DELETE", "/api/backups/"+name+"?auto=1", nil, panel.token)
	if status != 200 {
		test.Fatalf("auto backup delete status %d", status)
	}
}

func TestAutoBackupPrunesOldFiles(test *testing.T) {
	panel := setupPanel(test)
	directory := filepath.Join(panel.app.root, "backups", "auto")
	if err := os.MkdirAll(directory, 0700); err != nil {
		test.Fatal(err)
	}
	base := time.Now().Add(-20 * time.Hour)
	for index := 0; index < 16; index++ {
		target := filepath.Join(directory, fmt.Sprintf("auto-%02d.zip", index))
		if err := os.WriteFile(target, []byte("x"), 0600); err != nil {
			test.Fatal(err)
		}
		stamp := base.Add(time.Duration(index) * time.Minute)
		if err := os.Chtimes(target, stamp, stamp); err != nil {
			test.Fatal(err)
		}
	}
	panel.app.pruneAutoBackups(directory, 14)
	entries, err := os.ReadDir(directory)
	if err != nil {
		test.Fatal(err)
	}
	if len(entries) != 14 {
		test.Fatalf("auto backup retention mismatch: %d", len(entries))
	}
	if _, err := os.Stat(filepath.Join(directory, "auto-00.zip")); !os.IsNotExist(err) {
		test.Fatal("oldest auto backup not pruned")
	}
}

func TestAutoBackupSettingsValidation(test *testing.T) {
	panel := setupPanel(test)
	for _, value := range []string{"25:00", "3:00", "12:60", "abc"} {
		status, response := panel.call(test, "PUT", "/api/system/settings", Object{"autoBackupTime": value}, panel.token)
		if status != 400 {
			test.Fatalf("invalid backup time accepted: %s %v", value, response)
		}
	}
	status, response := panel.call(test, "PUT", "/api/system/settings", Object{"autoBackupEnabled": "true", "autoBackupTime": "03:30"}, panel.token)
	if status != 200 {
		test.Fatalf("valid auto backup settings rejected: %v", response)
	}
}

func TestFileSubscriptionAndAutoCron(test *testing.T) {
	panel := setupPanel(test)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte("# cron: 0 8 * * *\nprint('同步成功')\n"))
	}))
	defer server.Close()
	item := panel.data(test, "POST", "/api/subscriptions", Object{"name": "文件订阅", "type": "file", "url": server.URL + "/daily.py", "alias": "daily", "schedule": "", "autoAddCron": true}).(map[string]any)
	result := panel.data(test, "PUT", "/api/subscriptions/run", []int64{integer(item["id"])}).([]any)
	runID := integer(result[0].(map[string]any)["runId"])
	if panel.finished(test, runID)["status"] != "success" {
		test.Fatal(panel.app.latestLog("subscriptions", integer(item["id"])))
	}
	items, _ := panel.app.store.list("crons")
	if len(items) != 1 || text(items[0]["schedule"]) != "0 8 * * *" {
		test.Fatal(items)
	}
}

func TestVersionComparison(test *testing.T) {
	for _, sample := range []struct {
		next, current string
		newer         bool
	}{{"v1.10.0", "1.9.9", true}, {"1.0.0", "1.0.0", false}, {"0.9.9", "1.0.0", false}, {"1.2.0-beta", "1.0.0", false}, {"garbage", "1.0.0", false}} {
		if newerVersion(sample.next, sample.current) != sample.newer {
			test.Fatal(sample)
		}
	}
}

func TestClientIPOnlyTrustsConfiguredProxies(test *testing.T) {
	app, err := New(test.TempDir(), "1.0.0", "test", "")
	if err != nil {
		test.Fatal(err)
	}
	test.Cleanup(app.Close)
	request := httptest.NewRequest("GET", "/", nil)
	request.RemoteAddr = "127.0.0.1:1234"
	request.Header.Set("X-Forwarded-For", "203.0.113.7")
	if got := app.clientIP(request); got != "127.0.0.1" {
		test.Fatalf("untrusted proxy header must be ignored: %s", got)
	}
	if app.directLoopback(request) {
		test.Fatal("request carrying forwarding headers must not count as direct loopback")
	}
	local := httptest.NewRequest("GET", "/", nil)
	local.RemoteAddr = "127.0.0.1:1234"
	if !app.directLoopback(local) {
		test.Fatal("loopback should be treated as a direct operator channel")
	}
	if err := app.store.setSetting("trustedProxies", "127.0.0.1"); err != nil {
		test.Fatal(err)
	}
	if app.directLoopback(local) {
		test.Fatal("loopback listed as trusted proxy must not bypass login delays")
	}
	if got := app.clientIP(request); got != "203.0.113.7" {
		test.Fatalf("trusted proxy header not honored: %s", got)
	}
	// 最左侧条目可被客户端伪造，应从右向左跳过可信代理取第一个不可信地址。
	request.Header.Set("X-Forwarded-For", "1.2.3.4, 198.51.100.9")
	if got := app.clientIP(request); got != "198.51.100.9" {
		test.Fatalf("spoofed leftmost entry accepted: %s", got)
	}
	// 非可信来源即使带 X-Forwarded-For 也应使用直连地址。
	request.RemoteAddr = "203.0.113.9:5555"
	if got := app.clientIP(request); got != "203.0.113.9" {
		test.Fatalf("public client spoofed forwarded header: %s", got)
	}
	if app.directLoopback(request) {
		test.Fatal("public client treated as loopback")
	}
}

func TestBlacklistRejectsAuthenticatedRequests(test *testing.T) {
	panel := setupPanel(test)
	// 直接写库模拟其他来源已加入黑名单；API 会拒绝封禁当前访问来源的条目。
	if err := panel.app.store.setSetting(ipBlacklistKey, "127.0.0.1"); err != nil {
		test.Fatal(err)
	}
	status, _ := panel.call(test, "GET", "/api/user", nil, panel.token)
	if status != 401 {
		test.Fatalf("blacklisted IP kept an authenticated session: %d", status)
	}
}

func TestBlacklistRejectsSelfLockAndBroadRanges(test *testing.T) {
	panel := setupPanel(test)
	for _, entry := range []string{"127.0.0.1", "127.0.0.0/8", "0.0.0.0/0", "10.0.0.0/4"} {
		status, response := panel.call(test, "PUT", "/api/user/ip-blacklist", Object{"ip": entry}, panel.token)
		if status != 400 {
			test.Fatalf("unsafe blacklist entry accepted: %s %v", entry, response)
		}
	}
	if entries := panel.app.ipBlacklist(); len(entries) != 0 {
		test.Fatalf("unsafe entries persisted: %v", entries)
	}
	status, _ := panel.call(test, "PUT", "/api/user/ip-blacklist", Object{"ip": "203.0.113.7"}, panel.token)
	if status != 200 {
		test.Fatalf("valid public blacklist rejected: %d", status)
	}
}

func TestTrustedProxyRejectsBroadRanges(test *testing.T) {
	panel := setupPanel(test)
	for _, value := range []string{"0.0.0.0/0", "::/0", "10.0.0.0/4", "::/8"} {
		status, response := panel.call(test, "PUT", "/api/system/settings", Object{"trustedProxies": value}, panel.token)
		if status != 400 {
			test.Fatalf("broad trusted proxy accepted: %s %v", value, response)
		}
	}
	status, response := panel.call(test, "PUT", "/api/system/settings", Object{"trustedProxies": "127.0.0.1, 10.0.0.0/8"}, panel.token)
	if status != 200 {
		test.Fatalf("valid trusted proxy rejected: %v", response)
	}
}

func TestDependencyNameValidation(test *testing.T) {
	for _, name := range []string{"requests", "pkg.module", "Flask"} {
		if !validDependencyName(name, 1) {
			test.Fatalf("valid Python package rejected: %s", name)
		}
	}
	for _, name := range []string{"axios", "@scope/pkg", "lodash"} {
		if !validDependencyName(name, 0) {
			test.Fatalf("valid npm package rejected: %s", name)
		}
	}
	for _, name := range []string{"--index-url", "-r", "../evil", "http://evil", "a b", ""} {
		if validDependencyName(name, 1) || validDependencyName(name, 0) {
			test.Fatalf("unsafe dependency name accepted: %q", name)
		}
	}
}

func TestRememberLoginIssuesLongSession(test *testing.T) {
	panel := setupPanel(test)
	encoded, _ := json.Marshal(Object{"username": "admin", "password": "test-password-123", "remember": true})
	request := httptest.NewRequest("POST", "/api/user/login", bytes.NewReader(encoded))
	request.RemoteAddr = "127.0.0.1:10000"
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	panel.handler.ServeHTTP(recorder, request)
	if recorder.Code != 200 {
		test.Fatalf("remember login: %d %s", recorder.Code, recorder.Body.String())
	}
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == "cabbage_session" && cookie.MaxAge == 30*24*60*60 {
			return
		}
	}
	test.Fatal("remember login did not issue a 30-day cookie")
}

func TestAccountFailuresDelayInsteadOfLock(test *testing.T) {
	panel := setupPanel(test)
	const remote = "198.51.100.9:40000"
	start := time.Now()
	for attempt := 0; attempt < 10; attempt++ {
		status, _ := panel.callRemote(test, "POST", "/api/user/login", remote, Object{"username": "admin", "password": "wrong-password"}, "")
		if status != 401 {
			test.Fatalf("failed login %d: %d", attempt, status)
		}
	}
	// 超过阈值后不再返回 429 硬锁，而是延迟后继续校验。
	status, _ := panel.callRemote(test, "POST", "/api/user/login", remote, Object{"username": "admin", "password": "wrong-password"}, "")
	if status != 401 {
		test.Fatalf("account hard-locked instead of delayed: %d", status)
	}
	// 管理员使用正确密码仍必须能登录，第三方无法把账号锁死。
	status, response := panel.callRemote(test, "POST", "/api/user/login", remote, Object{"username": "admin", "password": "test-password-123"}, "")
	if status != 200 {
		test.Fatalf("correct password blocked after failures: %d %v", status, response)
	}
	if elapsed := time.Since(start); elapsed < 500*time.Millisecond {
		test.Fatalf("progressive login delay missing: %v", elapsed)
	}
}

func TestOutboundBlocksPrivateAddresses(test *testing.T) {
	for _, address := range []string{"127.0.0.1", "10.1.2.3", "192.168.1.1", "169.254.169.254", "100.64.0.1", "::1", "fc00::1", "fe80::1"} {
		if !blockedOutboundIP(net.ParseIP(address)) {
			test.Fatalf("private address not blocked: %s", address)
		}
	}
	for _, address := range []string{"8.8.8.8", "1.1.1.1", "2001:4860:4860::8888"} {
		if blockedOutboundIP(net.ParseIP(address)) {
			test.Fatalf("public address wrongly blocked: %s", address)
		}
	}
}

func TestIPLocationUsesHTTPSAndParses(t *testing.T) {
	panel := setupPanel(t)
	if err := panel.app.store.setSetting("ipLocationEnabled", "false"); err != nil {
		t.Fatal(err)
	}
	panel.app.client.Transport = fixtureTransport(func(request *http.Request) (*http.Response, error) {
		t.Fatal("disabled lookup must not send requests")
		return nil, nil
	})
	if address := panel.app.lookupIPLocation("8.8.8.8"); address != "" {
		t.Fatalf("disabled location lookup returned %q", address)
	}
	if err := panel.app.store.setSetting("ipLocationEnabled", "true"); err != nil {
		t.Fatal(err)
	}
	var requested string
	panel.app.client.Transport = fixtureTransport(func(request *http.Request) (*http.Response, error) {
		requested = request.URL.String()
		body := `{"success":true,"country":"美国","region":"加利福尼亚州","city":"圣何塞","connection":{"isp":"Google LLC"}}`
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
	})
	address := panel.app.lookupIPLocation("8.8.4.4")
	if !strings.HasPrefix(requested, "https://") {
		t.Fatalf("location lookup is not HTTPS: %s", requested)
	}
	for _, part := range []string{"美国", "加利福尼亚州", "Google LLC"} {
		if !strings.Contains(address, part) {
			t.Fatalf("location missing %s: %q", part, address)
		}
	}
}

func TestMain(main *testing.M) {
	if _, err := exec.LookPath(findShell()); err != nil {
		panic("test suite requires bash or sh")
	}
	os.Exit(main.Run())
}
