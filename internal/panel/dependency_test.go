package panel

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDependencyFailureRecoveryAndRemoval(test *testing.T) {
	panel := setupPanel(test)
	bin := filepath.Join(panel.app.root, "bin")
	npm := "#!/usr/bin/env bash\nprintf 'npm-operation:%s\\n' \"$*\"\nif [ \"$CABBAGE_FAKE_FAIL\" = \"1\" ]; then exit 7; fi\n"
	if err := os.WriteFile(filepath.Join(bin, "npm"), []byte(npm), 0700); err != nil {
		test.Fatal(err)
	}
	test.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	waitJob := func(id int64) Object {
		test.Helper()
		var runID int64
		waitUntil(test, 5*time.Second, func() bool {
			_ = panel.app.store.db.QueryRow("SELECT id FROM runs WHERE kind='dependencies' AND task_id=? ORDER BY id DESC LIMIT 1", id).Scan(&runID)
			return runID > 0
		})
		result := panel.finished(test, runID)
		waitUntil(test, 5*time.Second, func() bool { return panel.app.engine.active() == 0 })
		return result
	}
	created := panel.data(test, "POST", "/api/dependencies", []Object{{"name": "fixture-package", "type": 0}}).([]any)[0].(map[string]any)
	id := integer(created["id"])
	if result := waitJob(id); result["status"] != "success" || !strings.Contains(text(result["content"]), " install ") {
		test.Fatal(result)
	}
	test.Setenv("CABBAGE_FAKE_FAIL", "1")
	panel.data(test, "DELETE", "/api/dependencies", []int64{id})
	if result := waitJob(id); result["status"] != "failed" {
		test.Fatal(result)
	}
	test.Setenv("CABBAGE_FAKE_FAIL", "0")
	panel.data(test, "PUT", "/api/dependencies/reinstall", []int64{id})
	if result := waitJob(id); result["status"] != "success" || !strings.Contains(text(result["content"]), " install ") {
		test.Fatal("reinstall did not recover from failed uninstall", result)
	}
	panel.data(test, "DELETE", "/api/dependencies", []int64{id})
	if result := waitJob(id); result["status"] != "success" {
		test.Fatal(result)
	}
	if _, err := panel.app.store.get("dependencies", id); !errors.Is(err, sql.ErrNoRows) {
		test.Fatal("uninstalled record remains", err)
	}
}
