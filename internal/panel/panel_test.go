package panel

import (
	"archive/zip"
	"bytes"
	"encoding/json"
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
	content, err := json.Marshal(body)
	if err != nil {
		test.Fatal(err)
	}
	request := httptest.NewRequest(method, path, bytes.NewReader(content))
	request.RemoteAddr = "127.0.0.1:10000"
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
	if text(listed[0].(map[string]any)["client_secret"]) != text(created["client_secret"]) {
		test.Fatal("client secret is not available for administrators")
	}
	status, response := panel.call(test, "GET", "/open/auth/token?client_id="+text(created["client_id"])+"&client_secret="+text(created["client_secret"]), nil, "")
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
	status, _ := panel.call(test, "POST", "/api/crons", Object{"name": "bad", "command": "echo x", "schedule": "not a cron"}, panel.token)
	if status != 400 {
		test.Fatal("invalid cron accepted")
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

func TestMain(main *testing.M) {
	if _, err := exec.LookPath(findShell()); err != nil {
		panic("test suite requires bash or sh")
	}
	os.Exit(main.Run())
}
