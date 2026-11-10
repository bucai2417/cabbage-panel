package panel

import (
	"net/http/cgi"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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
