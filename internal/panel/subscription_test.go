package panel

import (
	"net/http"
	"net/http/cgi"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
)

func TestGitSubscriptionSynchronizesAndRemoves(test *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		test.Skip("Git is unavailable")
	}
	panel := setupPanel(test)
	repository := test.TempDir()
	git := func(args ...string) {
		test.Helper()
		command := exec.Command("git", append([]string{"-c", "user.name=Cabbage Test", "-c", "user.email=test@example.invalid"}, args...)...)
		command.Dir = repository
		configureProcess(command)
		if output, err := command.CombinedOutput(); err != nil {
			test.Fatalf("git %v: %s %v", args, output, err)
		}
	}
	git("init", "--initial-branch=main")
	if err := os.WriteFile(filepath.Join(repository, "daily.py"), []byte("# cron: 0 8 * * *\nprint('Git 订阅')\n"), 0600); err != nil {
		test.Fatal(err)
	}
	git("add", "daily.py")
	git("commit", "-m", "fixture")
	exportRoot := test.TempDir()
	bare := filepath.Join(exportRoot, "fixture.git")
	git("clone", "--bare", ".", bare)
	execPath, err := exec.Command("git", "--exec-path").Output()
	if err != nil {
		test.Fatal(err)
	}
	backend := filepath.Join(strings.TrimSpace(string(execPath)), "git-http-backend")
	if runtime.GOOS == "windows" {
		backend += ".exe"
	}
	server := httptest.NewServer(&cgi.Handler{Path: backend, Env: []string{"GIT_PROJECT_ROOT=" + exportRoot, "GIT_HTTP_EXPORT_ALL=1"}})
	defer server.Close()
	item := panel.data(test, "POST", "/api/subscriptions", Object{"name": "Git 测试", "type": "git-repo", "url": server.URL + "/fixture.git", "alias": "git-test", "branch": "main", "autoAddCron": true, "autoDelCron": true}).(map[string]any)
	sync := func() {
		test.Helper()
		runs := panel.data(test, "PUT", "/api/subscriptions/run", []int64{integer(item["id"])}).([]any)
		runID := integer(runs[0].(map[string]any)["runId"])
		result := panel.finished(test, runID)
		if result["status"] != "success" {
			test.Fatal(result)
		}
	}
	sync()
	if _, err := os.Stat(filepath.Join(panel.app.root, "scripts", "git-test", "daily.py")); err != nil {
		test.Fatal(err)
	}
	git("rm", "daily.py")
	git("commit", "-m", "remove fixture")
	git("push", bare, "main")
	sync()
	if _, err := os.Stat(filepath.Join(panel.app.root, "scripts", "git-test", "daily.py")); !os.IsNotExist(err) {
		test.Fatal("removed subscription file remains", err)
	}
	tasks, err := panel.app.store.list("crons")
	if err != nil || len(tasks) != 0 {
		test.Fatal("orphan cron remains", tasks, err)
	}
}

func TestScriptHeaderCron(test *testing.T) {
	for _, sample := range []struct {
		name    string
		content string
		want    string
	}{
		{"python colon", "# cron: 0 8 * * *\nprint('x')\n", "0 8 * * *"},
		{"python quoted", "# cron \"30 9 * * 1-5\"\nprint('x')\n", "30 9 * * 1-5"},
		{"python at", "# @cron 0 0 * * *\n", "0 0 * * *"},
		{"javascript line", "// cron: 0 12 * * *\nconsole.log('x')\n", "0 12 * * *"},
		{"block comment", "/*\n * cron \"0 3 * * *\"\n * new Env(\"JD_COOKIE\")\n */\nconsole.log('x')\n", "0 3 * * *"},
		{"single line block", "/* cron: 0 0 * * * */\n", "0 0 * * *"},
		{"six fields", "# cron: 0 0 8 * * *\n", "0 0 8 * * *"},
		{"trailing script", "# cron 0 8 * * * daily.py\n", "0 8 * * *"},
		{"quoted trailing script", "# cron \"0 8 * * *\" daily.py\n", "0 8 * * *"},
		{"new env only", "# new Env(\"JD_COOKIE\")\nprint('x')\n", ""},
		{"non comment", "print('cron: 0 0 * * *')\n", ""},
		{"invalid cron", "# cron: not a cron\n", ""},
		{"second valid", "# cron: 说明\n# cron: 0 8 * * *\n", "0 8 * * *"},
		{"after header", strings.Repeat("pass\n", 50) + "# cron: 0 8 * * *\n", ""},
	} {
		test.Run(sample.name, func(test *testing.T) {
			if got := scriptHeaderCron(sample.content); got != sample.want {
				test.Fatalf("scriptHeaderCron(%q) = %q, want %q", sample.content, got, sample.want)
			}
		})
	}
}

func TestFileSubscriptionUpdatesCronFromHeader(test *testing.T) {
	panel := setupPanel(test)
	var body atomic.Value
	body.Store("# cron: 0 8 * * *\n# new Env(\"SHOULD_NOT_CREATE\")\nprint('同步成功')\n")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(body.Load().(string)))
	}))
	defer server.Close()
	item := panel.data(test, "POST", "/api/subscriptions", Object{"name": "头部注释订阅", "type": "file", "url": server.URL + "/daily.py", "alias": "daily", "schedule": "", "autoAddCron": true}).(map[string]any)
	sync := func() {
		test.Helper()
		runs := panel.data(test, "PUT", "/api/subscriptions/run", []int64{integer(item["id"])}).([]any)
		result := panel.finished(test, integer(runs[0].(map[string]any)["runId"]))
		if result["status"] != "success" {
			test.Fatal(result)
		}
	}
	sync()
	tasks, _ := panel.app.store.list("crons")
	if len(tasks) != 1 || text(tasks[0]["name"]) != "daily.py" || text(tasks[0]["schedule"]) != "0 8 * * *" {
		test.Fatalf("auto task mismatch: %v", tasks)
	}
	taskID := integer(tasks[0]["id"])
	if envs := panel.data(test, "GET", "/api/envs", nil).([]any); len(envs) != 0 {
		test.Fatalf("new Env should not create variables: %v", envs)
	}

	body.Store("# cron: 0 9 * * *\nprint('同步成功')\n")
	sync()
	tasks, _ = panel.app.store.list("crons")
	if len(tasks) != 1 || integer(tasks[0]["id"]) != taskID || text(tasks[0]["schedule"]) != "0 9 * * *" {
		test.Fatalf("auto task was not updated: %v", tasks)
	}

	body.Store("print('没有 cron 注释')\n")
	sync()
	tasks, _ = panel.app.store.list("crons")
	if len(tasks) != 1 || integer(tasks[0]["id"]) != taskID || text(tasks[0]["schedule"]) != "0 9 * * *" {
		test.Fatalf("removing annotation should keep existing task: %v", tasks)
	}
}
