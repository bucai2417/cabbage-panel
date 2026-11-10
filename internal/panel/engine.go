package panel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

var scheduleParser = cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

type Job struct {
	ID, TaskID          int64
	Kind, Name, Command string
	Item                Object
	Cleanup             string
	ctx                 context.Context
	cancel              context.CancelFunc
}

type Engine struct {
	app        *App
	ctx        context.Context
	cancel     context.CancelFunc
	mu         sync.Mutex
	scheduleMu sync.Mutex
	jobs       map[int64]*Job
	queue      chan *Job
	running    int
	wg         sync.WaitGroup
	scheduler  *cron.Cron
	entries    map[string]cron.EntryID
}

func newEngine(app *App) *Engine {
	ctx, cancel := context.WithCancel(context.Background())
	return &Engine{app: app, ctx: ctx, cancel: cancel, jobs: map[int64]*Job{}, queue: make(chan *Job, 256), scheduler: cron.New(cron.WithParser(scheduleParser)), entries: map[string]cron.EntryID{}}
}
func (engine *Engine) start() {
	engine.reschedule()
	engine.scheduler.Start()
	_, _ = engine.scheduler.AddFunc("@daily", func() { engine.app.gate.RLock(); defer engine.app.gate.RUnlock(); _, _ = engine.app.cleanup() })
	engine.wg.Add(1)
	go func() {
		defer engine.wg.Done()
		for {
			select {
			case <-engine.ctx.Done():
				return
			case job := <-engine.queue:
				for {
					engine.mu.Lock()
					limit := int(integer(engine.app.store.setting("concurrency", "4")))
					if limit < 1 {
						limit = 4
					}
					available := engine.running < limit
					if available {
						engine.running++
					}
					engine.mu.Unlock()
					if available {
						break
					}
					select {
					case <-engine.ctx.Done():
						return
					case <-time.After(50 * time.Millisecond):
					}
				}
				engine.wg.Add(1)
				go func() { defer engine.wg.Done(); engine.execute(job) }()
			}
		}
	}()
}
func (engine *Engine) close() {
	<-engine.scheduler.Stop().Done()
	engine.cancel()
	engine.mu.Lock()
	for _, job := range engine.jobs {
		job.cancel()
	}
	engine.mu.Unlock()
	engine.wg.Wait()
}
func (engine *Engine) reschedule() {
	engine.scheduleMu.Lock()
	defer engine.scheduleMu.Unlock()
	for _, entry := range engine.entries {
		engine.scheduler.Remove(entry)
	}
	engine.entries = map[string]cron.EntryID{}
	for _, kind := range []string{"crons", "subscriptions"} {
		items, err := engine.app.store.list(kind)
		if err != nil {
			continue
		}
		for _, item := range items {
			if boolean(item["isDisabled"]) {
				continue
			}
			spec := text(item["schedule"])
			if spec == "" {
				continue
			}
			id := integer(item["id"])
			taskKind := kind
			entry, err := engine.scheduler.AddFunc(spec, func() { engine.app.gate.RLock(); defer engine.app.gate.RUnlock(); _, _ = engine.enqueue(taskKind, id) })
			if err == nil {
				engine.entries[kind+":"+text(id)] = entry
			}
		}
	}
}
func (engine *Engine) decorate(kind string, item Object) Object {
	item = clone(item)
	id := integer(item["id"])
	item["status"] = 1
	item["runningCount"] = 0
	engine.mu.Lock()
	for _, job := range engine.jobs {
		if job.Kind == kind && job.TaskID == id {
			item["status"] = 0
			item["runningCount"] = integer(item["runningCount"]) + 1
		}
	}
	engine.mu.Unlock()
	engine.scheduleMu.Lock()
	entry := engine.scheduler.Entry(engine.entries[kind+":"+text(id)])
	engine.scheduleMu.Unlock()
	if !entry.Next.IsZero() {
		item["nextRun"] = entry.Next.Format(time.RFC3339)
	}
	var status string
	var started, finished *string
	var duration float64
	err := engine.app.store.db.QueryRow("SELECT status,started,finished,duration FROM runs WHERE kind=? AND task_id=? ORDER BY id DESC LIMIT 1", kind, id).Scan(&status, &started, &finished, &duration)
	if err == nil {
		item["lastStatus"] = status
		item["lastRun"] = started
		item["last_execution_time"] = duration
		item["lastFinish"] = finished
	}
	return item
}
func (engine *Engine) enqueue(kind string, id int64) (Object, error) {
	engine.app.scriptMu.RLock()
	defer engine.app.scriptMu.RUnlock()
	item, err := engine.app.store.get(kind, id)
	if err != nil {
		return nil, err
	}
	return engine.enqueueJob(kind, id, text(item["name"]), engine.commandFor(kind, item), clone(item), "")
}

func (engine *Engine) enqueueAdhoc(kind string, taskID int64, name, command string, item Object, cleanup string) (Object, error) {
	engine.app.scriptMu.RLock()
	defer engine.app.scriptMu.RUnlock()
	return engine.enqueueJob(kind, taskID, name, command, item, cleanup)
}

func (engine *Engine) commandFor(kind string, item Object) string {
	switch kind {
	case "subscriptions":
		return text(item["url"])
	case "dependencies":
		return text(item["name"])
	default:
		return text(item["command"])
	}
}

func (engine *Engine) enqueueJob(kind string, id int64, name, command string, item Object, cleanup string) (Object, error) {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	if engine.ctx.Err() != nil {
		return nil, errors.New("服务正在停止")
	}
	if len(engine.jobs) >= 256 {
		return nil, errors.New("队列已满，请稍后再试")
	}
	for _, job := range engine.jobs {
		if job.Kind == kind && job.TaskID == id && (kind != "crons" || !boolean(item["allowConcurrent"])) {
			return nil, errors.New("任务已在运行或排队")
		}
	}
	result, err := engine.app.store.db.Exec("INSERT INTO runs(task_id,kind,name,command,status,created) VALUES(?,?,?,?,?,?)", id, kind, name, command, "queued", now())
	if err != nil {
		return nil, err
	}
	runID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(engine.ctx)
	job := &Job{ID: runID, TaskID: id, Kind: kind, Name: name, Command: command, Item: clone(item), Cleanup: cleanup, ctx: ctx, cancel: cancel}
	engine.jobs[runID] = job
	engine.queue <- job
	return Object{"id": id, "runId": runID, "status": "queued"}, nil
}
func (engine *Engine) stop(kind string, id, runID int64) {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	for _, job := range engine.jobs {
		if job.Kind == kind && job.TaskID == id && (runID == 0 || job.ID == runID) {
			job.cancel()
		}
	}
}
func (engine *Engine) active() int {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	return len(engine.jobs)
}

type boundedLog struct {
	mu   sync.Mutex
	file *os.File
	size int64
}

func (writer *boundedLog) Write(bytes []byte) (int, error) {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	length := len(bytes)
	if writer.size >= 20<<20 {
		return length, nil
	}
	remaining := (20 << 20) - writer.size
	if int64(len(bytes)) > remaining {
		bytes = bytes[:remaining]
	}
	written, err := writer.file.Write(bytes)
	writer.size += int64(written)
	if err != nil {
		return written, err
	}
	return length, nil
}

func (engine *Engine) execute(job *Job) {
	engine.app.gate.RLock()
	defer engine.app.gate.RUnlock()
	defer func() {
		job.cancel()
		engine.mu.Lock()
		delete(engine.jobs, job.ID)
		engine.running--
		engine.mu.Unlock()
		if job.Cleanup != "" {
			_ = os.RemoveAll(job.Cleanup)
		}
	}()
	started := time.Now()
	logName := fmt.Sprintf("%d.log", job.ID)
	file, err := os.OpenFile(filepath.Join(engine.app.root, "logs", logName), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		_, _ = engine.app.store.db.Exec("UPDATE runs SET status='failed',finished=? WHERE id=?", now(), job.ID)
		return
	}
	defer file.Close()
	writer := &boundedLog{file: file}
	_, _ = engine.app.store.db.Exec("UPDATE runs SET status='running',started=?,log_path=? WHERE id=?", now(), logName, job.ID)
	fmt.Fprintf(writer, "大白菜面板 · %s\n开始时间: %s\n\n", job.Name, time.Now().Format(time.RFC3339))
	timeout := integer(job.Item["timeout"])
	if timeout <= 0 {
		timeout = integer(engine.app.store.setting("timeout", "3600"))
	}
	if timeout < 1 {
		timeout = 3600
	}
	ctx, cancel := context.WithTimeout(job.ctx, time.Duration(timeout)*time.Second)
	defer cancel()
	exitCode := 0
	if ctx.Err() != nil {
		err = ctx.Err()
	} else {
		switch job.Kind {
		case "dependencies":
			err = engine.app.installDependency(ctx, job.Item, writer)
		case "subscriptions":
			err = engine.app.syncSubscription(ctx, job.Item, writer)
		case "updates":
			err = engine.app.stageUpdate(ctx, writer)
		default:
			err = engine.app.runCommand(ctx, job.Command, writer)
		}
	}
	status := "success"
	if err != nil {
		status = "failed"
		exitCode = 1
		var processError *exec.ExitError
		if errors.As(err, &processError) {
			exitCode = processError.ExitCode()
		}
		fmt.Fprintf(writer, "\n错误: %v\n", err)
	}
	if ctx.Err() == context.DeadlineExceeded {
		status = "timeout"
	} else if job.ctx.Err() != nil {
		status = "cancelled"
	}
	duration := time.Since(started).Seconds()
	fmt.Fprintf(writer, "\n结束状态: %s · 耗时 %.2fs · 退出码 %d\n", status, duration, exitCode)
	_ = file.Sync()
	_, _ = engine.app.store.db.Exec("UPDATE runs SET status=?,finished=?,exit_code=?,duration=? WHERE id=?", status, now(), exitCode, duration, job.ID)
	if job.Kind == "dependencies" {
		depStatus := 0
		if status != "success" {
			depStatus = 2
		}
		_, _ = engine.app.store.save("dependencies", Object{"id": job.TaskID, "status": depStatus, "lastStatus": status})
	}
	if job.Kind == "crons" && (status == "failed" || status == "timeout") {
		engine.app.notify(job.ctx, job.Name+" 执行失败", fmt.Sprintf("任务 #%d，运行 #%d，退出码 %d", job.TaskID, job.ID, exitCode))
	}
}

func findPython() string {
	for _, name := range []string{"python3", "python"} {
		if found, err := exec.LookPath(name); err == nil {
			return found
		}
	}
	return "python3"
}
func findShell() string {
	if runtime.GOOS == "windows" {
		for _, location := range []string{os.Getenv("CABBAGE_BASH"), "C:/Program Files/Git/bin/bash.exe", "D:/Git/bin/bash.exe"} {
			if location != "" {
				if _, err := os.Stat(location); err == nil {
					return location
				}
			}
		}
	}
	for _, name := range []string{"bash", "sh"} {
		if found, err := exec.LookPath(name); err == nil {
			return found
		}
	}
	return "bash"
}

func (app *App) environment() []string {
	values := map[string]string{}
	for _, entry := range os.Environ() {
		pair := strings.SplitN(entry, "=", 2)
		if len(pair) == 2 {
			key := pair[0]
			if runtime.GOOS == "windows" && strings.EqualFold(key, "PATH") {
				key = "PATH"
			}
			values[key] = pair[1]
		}
	}
	items, _ := app.store.list("envs")
	sort.SliceStable(items, func(left, right int) bool {
		return integer(items[left]["position"]) < integer(items[right]["position"])
	})
	for _, item := range items {
		if integer(item["status"]) == 0 {
			values[text(item["name"])] = text(item["value"])
		}
	}
	values["PYTHONIOENCODING"] = "utf-8"
	values["PYTHONUTF8"] = "1"
	values["PYTHONUNBUFFERED"] = "1"
	values["CABBAGE_DATA"] = app.root
	values["PYTHONPATH"] = filepath.Join(app.root, "bin") + string(os.PathListSeparator) + filepath.Join(app.root, "packages", "python") + string(os.PathListSeparator) + filepath.Join(app.root, "scripts")
	values["NODE_PATH"] = filepath.Join(app.root, "bin") + string(os.PathListSeparator) + filepath.Join(app.root, "packages", "node_modules")
	app.helperMu.RLock()
	values["CABBAGE_NOTIFY_URL"] = app.helperAddress
	app.helperMu.RUnlock()
	values["CABBAGE_NOTIFY_TOKEN"] = app.helperKey
	values["PATH"] = filepath.Join(app.root, "bin") + string(os.PathListSeparator) + values["PATH"]
	values["CABBAGE_PYTHON"] = filepath.ToSlash(app.python())
	if app.python() != findPython() {
		values["PATH"] = filepath.Dir(app.python()) + string(os.PathListSeparator) + values["PATH"]
	}
	values["GIT_TERMINAL_PROMPT"] = "0"
	result := []string{}
	for key, value := range values {
		result = append(result, key+"="+value)
	}
	sort.Strings(result)
	return result
}
func (app *App) process(ctx context.Context, dir string, writer io.Writer, program string, args ...string) error {
	command := exec.CommandContext(ctx, program, args...)
	command.Dir = dir
	command.Env = app.environment()
	command.Stdout = writer
	command.Stderr = writer
	configureProcess(command)
	command.WaitDelay = 3 * time.Second
	command.Cancel = func() error {
		if command.Process != nil {
			return terminateProcess(command)
		}
		return nil
	}
	return command.Run()
}
func (app *App) runCommand(ctx context.Context, command string, writer io.Writer) error {
	wrapped := "set -a\nif [ -f \"$CABBAGE_DATA/config/config.sh\" ]; then . \"$CABBAGE_DATA/config/config.sh\" || exit $?; fi\nset +a\nif [ -f \"$CABBAGE_DATA/config/task_before.sh\" ]; then . \"$CABBAGE_DATA/config/task_before.sh\" || exit $?; fi\n(\n" + command + "\n)\ncabbage_exit=$?\nexport CABBAGE_EXIT_CODE=$cabbage_exit\nif [ -f \"$CABBAGE_DATA/config/task_after.sh\" ]; then . \"$CABBAGE_DATA/config/task_after.sh\"; fi\nexit $cabbage_exit"
	return app.process(ctx, filepath.Join(app.root, "scripts"), writer, findShell(), "-c", wrapped)
}
func (app *App) installHelpers() error {
	helper := "#!/usr/bin/env bash\nset -e\nscript=\"$1\"\nshift\ncase \"$script\" in\n  *.py) exec \"$CABBAGE_PYTHON\" -u \"$script\" \"$@\" ;;\n  *.js|*.mjs|*.cjs) exec node \"$script\" \"$@\" ;;\n  *.sh) exec bash \"$script\" \"$@\" ;;\n  *) echo 'task 仅支持 python/js/shell 脚本' >&2; exit 2 ;;\nesac\n"
	if err := os.WriteFile(filepath.Join(app.root, "bin", "task"), []byte(helper), 0700); err != nil {
		return err
	}
	return app.installNotificationHelpers()
}
