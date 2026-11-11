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
	// interactive 为 true 时保留 stdin 管道，供调试时逐行输入。
	interactive bool
	stdin       io.WriteCloser
	stdinMu     sync.Mutex
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
				// 每个任务独立等待并发槽位，避免一个任务阻塞整个队列。
				engine.wg.Add(1)
				go func(job *Job) {
					defer engine.wg.Done()
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
							engine.finishCancelled(job)
							return
						case <-job.ctx.Done():
							// 排队期间被停止：标记为已取消并从任务表移除。
							engine.finishCancelled(job)
							return
						case <-time.After(50 * time.Millisecond):
						}
					}
					engine.execute(job)
				}(job)
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
		item["nextRunTime"] = entry.Next.UnixMilli()
	}
	// 标记 cron 表达式是否有效：无效时不自动调度，但允许手动运行。
	if spec := strings.TrimSpace(text(item["schedule"])); spec != "" {
		if _, parseErr := scheduleParser.Parse(spec); parseErr != nil {
			item["scheduleValid"] = false
		}
	}
	var status, logPath string
	var started, finished *string
	var duration float64
	err := engine.app.store.db.QueryRow("SELECT status,started,finished,duration,log_path FROM runs WHERE kind=? AND task_id=? ORDER BY id DESC LIMIT 1", kind, id).Scan(&status, &started, &finished, &duration, &logPath)
	if err == nil {
		item["lastStatus"] = status
		item["lastRun"] = started
		item["lastFinish"] = finished
		item["duration"] = duration
		if started != nil {
			if value, parseErr := time.Parse(time.RFC3339Nano, *started); parseErr == nil {
				item["last_running_time"] = value.UnixMilli()
			}
		}
		if finished != nil {
			if value, parseErr := time.Parse(time.RFC3339Nano, *finished); parseErr == nil {
				item["last_execution_time"] = value.UnixMilli()
			}
		}
		if logPath != "" {
			item["log_path"] = filepath.Join(engine.app.root, "logs", logPath)
			item["log_name"] = logPath
		}
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

// enqueueInteractive 创建保留 stdin 的任务，供调试时逐行输入。
func (engine *Engine) enqueueInteractive(kind string, taskID int64, name, command string, item Object, cleanup string) (Object, error) {
	engine.app.scriptMu.RLock()
	defer engine.app.scriptMu.RUnlock()
	result, err := engine.enqueueJob(kind, taskID, name, command, item, cleanup)
	if err != nil {
		return nil, err
	}
	engine.mu.Lock()
	if job, found := engine.jobs[integer(result["runId"])]; found {
		job.interactive = true
	}
	engine.mu.Unlock()
	return result, nil
}

// writeStdin 向调试任务的 stdin 写入一行，模拟终端输入。
func (engine *Engine) writeStdin(runID int64, line string) error {
	engine.mu.Lock()
	job := engine.jobs[runID]
	engine.mu.Unlock()
	if job == nil {
		return errors.New("任务不存在或已结束")
	}
	if !job.interactive {
		return errors.New("该任务不支持输入")
	}
	job.stdinMu.Lock()
	defer job.stdinMu.Unlock()
	if job.stdin == nil {
		return errors.New("任务尚未开始执行或已结束")
	}
	_, err := io.WriteString(job.stdin, line+"\n")
	return err
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
// finishCancelled 把排队中就被停止的任务标记为 cancelled 并清理。
func (engine *Engine) finishCancelled(job *Job) {
	_, _ = engine.app.store.db.Exec("UPDATE runs SET status='cancelled',finished=? WHERE id=?", now(), job.ID)
	engine.mu.Lock()
	delete(engine.jobs, job.ID)
	engine.mu.Unlock()
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
	// 日志文件名必须唯一：默认用运行 ID，避免同一任务多次运行互相覆盖。
	logName := fmt.Sprintf("%d.log", job.ID)
	if custom := strings.TrimSpace(text(job.Item["log_name"])); custom != "" {
		safe := strings.ReplaceAll(custom, "\\", "/")
		if index := strings.LastIndex(safe, "/"); index >= 0 {
			safe = safe[index+1:]
		}
		if safe != "" && safe != "." && safe != ".." {
			// 去掉 .log 后缀后拼接运行 ID，保证每次运行独立。
			safe = strings.TrimSuffix(safe, ".log")
			logName = fmt.Sprintf("%s_%d.log", safe, job.ID)
		}
	}
	file, err := os.OpenFile(filepath.Join(engine.app.root, "logs", logName), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		_, _ = engine.app.store.db.Exec("UPDATE runs SET status='failed',finished=? WHERE id=?", now(), job.ID)
		return
	}
	defer file.Close()
	writer := &boundedLog{file: file}
	_, _ = engine.app.store.db.Exec("UPDATE runs SET status='running',started=?,log_path=? WHERE id=?", now(), logName, job.ID)
	fmt.Fprintf(writer, "大白菜面板 · %s\n开始时间: %s\n\n", job.Name, time.Now().Format("2006-01-02 15:04:05"))
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
			err = engine.app.runTask(ctx, job.Item, job.Command, writer, job)
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
	fmt.Fprintf(writer, "\n结束状态: %s · 耗时 %.2fs · 退出码 %d · 结束时间 %s\n", status, duration, exitCode, time.Now().Format("2006-01-02 15:04:05"))
	_ = file.Sync()
	_, _ = engine.app.store.db.Exec("UPDATE runs SET status=?,finished=?,exit_code=?,duration=? WHERE id=?", status, now(), exitCode, duration, job.ID)
	level := "info"
	if status != "success" {
		level = "warn"
	}
	engine.app.appendSystemLog(level, fmt.Sprintf("[panel][Task finished] %s (#%d) 状态 %s，退出码 %d，耗时 %.2fs", job.Name, job.ID, status, exitCode, duration))
	if job.Kind == "dependencies" {
		depStatus := 0
		if status != "success" {
			depStatus = 2
		}
		_, _ = engine.app.store.save("dependencies", Object{"id": job.TaskID, "status": depStatus, "lastStatus": status})
	}
	// 任务失败时自动识别缺失依赖并安装，下次运行即可直接使用。
	if job.Kind == "crons" && status == "failed" {
		if missing := engine.app.detectMissingDependencies(logName); len(missing) > 0 {
			if created := engine.app.autoInstallMissing(missing); len(created) > 0 {
				fmt.Fprintf(writer, "\n已自动添加依赖: %s（安装完成后可重新运行）\n", strings.Join(created, ", "))
				_ = file.Sync()
				engine.app.appendSystemLog("info", fmt.Sprintf("[panel][AutoDeps] 任务 %s 缺少依赖，已自动安装: %s", job.Name, strings.Join(created, ", ")))
			}
		}
	}
	if job.Kind == "crons" {
		if integer(job.Item["notifyOnFinish"]) == 1 {
			engine.app.notify(job.ctx, job.Name+" 执行结束", fmt.Sprintf("任务 #%d，运行 #%d，状态 %s，退出码 %d", job.TaskID, job.ID, status, exitCode))
		} else if status == "failed" || status == "timeout" {
			engine.app.notify(job.ctx, job.Name+" 执行失败", fmt.Sprintf("任务 #%d，运行 #%d，退出码 %d", job.TaskID, job.ID, exitCode))
		}
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
	return app.processWithStdin(ctx, dir, writer, nil, program, args...)
}

// processWithStdin 与 process 相同，但可由 job 提供 stdin 管道（调试交互用）。
func (app *App) processWithStdin(ctx context.Context, dir string, writer io.Writer, job *Job, program string, args ...string) error {
	command := exec.CommandContext(ctx, program, args...)
	command.Dir = dir
	command.Env = app.environment()
	command.Stdout = writer
	command.Stderr = writer
	if job != nil && job.interactive {
		stdin, err := command.StdinPipe()
		if err == nil {
			job.stdinMu.Lock()
			job.stdin = stdin
			job.stdinMu.Unlock()
		}
	}
	configureProcess(command)
	// 主进程已被同步 Kill，1 秒足够等待管道关闭；过长会拖慢停止响应。
	command.WaitDelay = time.Second
	command.Cancel = func() error {
		if command.Process != nil {
			return terminateProcess(command)
		}
		return nil
	}
	return command.Run()
}
func (app *App) runCommand(ctx context.Context, command string, writer io.Writer) error {
	return app.runTask(ctx, Object{}, command, writer, nil)
}

// runTask 执行任务命令，支持青龙的 task_before / task_after / work_dir。
func (app *App) runTask(ctx context.Context, item Object, command string, writer io.Writer, job *Job) error {
	workDir := filepath.Join(app.root, "scripts")
	if custom := strings.TrimSpace(text(item["work_dir"])); custom != "" {
		if filepath.IsAbs(custom) {
			workDir = custom
		} else {
			workDir = filepath.Join(app.root, custom)
		}
	}
	var builder strings.Builder
	builder.WriteString("set -a\n")
	builder.WriteString("if [ -f \"$CABBAGE_DATA/config/config.sh\" ]; then . \"$CABBAGE_DATA/config/config.sh\" || exit $?; fi\n")
	builder.WriteString("set +a\n")
	// 任务级前置脚本优先，否则回落到全局 task_before.sh。
	if before := strings.TrimSpace(text(item["task_before"])); before != "" {
		builder.WriteString("(\n" + before + "\n) || exit $?\n")
	} else {
		builder.WriteString("if [ -f \"$CABBAGE_DATA/config/task_before.sh\" ]; then . \"$CABBAGE_DATA/config/task_before.sh\" || exit $?; fi\n")
	}
	builder.WriteString("(\n" + command + "\n)\ncabbage_exit=$?\nexport CABBAGE_EXIT_CODE=$cabbage_exit\n")
	if after := strings.TrimSpace(text(item["task_after"])); after != "" {
		builder.WriteString("(\n" + after + "\n)\n")
	} else {
		builder.WriteString("if [ -f \"$CABBAGE_DATA/config/task_after.sh\" ]; then . \"$CABBAGE_DATA/config/task_after.sh\"; fi\n")
	}
	builder.WriteString("exit $cabbage_exit")
	return app.processWithStdin(ctx, workDir, writer, job, findShell(), "-c", builder.String())
}
func (app *App) installHelpers() error {
	// task 助手：统一 task 前缀，按后缀选解释器，支持 now / conc / desi 子命令。
	if err := os.WriteFile(filepath.Join(app.root, "bin", "task"), []byte(taskHelperSource), 0700); err != nil {
		return err
	}
	return app.installNotificationHelpers()
}

// taskHelperSource 是随版本内置的 task 命令脚本。
const taskHelperSource = `#!/usr/bin/env bash
# 大白菜面板 task 助手。
#   task <脚本>                  依次执行
#   task <脚本> now              立即执行（忽略随机延迟），前台输出日志
#   task <脚本> conc <变量> [n...] 并发执行：把变量按 & 拆分，逐个并发运行
#   task <脚本> desi <变量> n...  指定账号执行：只跑指定序号（支持 1-3 / 1~3 / 1_3）
set -u

script="${1:-}"
if [ -z "$script" ]; then
  cat >&2 <<'USAGE'
用法:
  task <脚本路径>                         依次执行
  task <脚本路径> now                     立即执行，前台输出日志
  task <脚本路径> conc <变量名> [序号...]   并发执行，可指定账号
  task <脚本路径> desi <变量名> <序号...>   指定账号执行

支持 .py / .js / .mjs / .cjs / .ts / .mts / .cts / .sh
USAGE
  exit 2
fi
shift

mode="normal"
if [ $# -gt 0 ]; then
  case "$1" in
    now|conc|desi) mode="$1"; shift ;;
  esac
fi

# 按后缀选择解释器；输出「程序名」或空（空表示 shell 脚本，需要 source 语义）。
pick_program() {
  case "$1" in
    *.py) printf '%s' "${CABBAGE_PYTHON:-python3}" ;;
    *.js|*.mjs|*.cjs|*.ts|*.mts|*.cts) printf 'node' ;;
    *.sh) printf 'bash' ;;
    *) printf '' ;;
  esac
}

run_one() {
  # $1 脚本；其余为参数
  local file="$1"; shift
  local program
  program="$(pick_program "$file")"
  if [ -z "$program" ]; then
    echo "task 不支持该文件类型: $file" >&2
    echo '支持 .py / .js / .mjs / .cjs / .ts / .mts / .cts / .sh' >&2
    return 2
  fi
  if [ "$program" = "node" ]; then
    exec node "$file" "$@"
  elif [ "$program" = "bash" ]; then
    exec bash "$file" "$@"
  else
    exec "$program" -u "$file" "$@"
  fi
}

# 把「1-3 5」这类序号展开成逐个序号，去重保序。
expand_indexes() {
  local raw="$1"
  local expanded=""
  local token
  for token in $raw; do
    case "$token" in
      *[-~_]*[0-9])
        local from="${token%%[-~_]*}"
        local to="${token##*[-~_]}"
        if [ "$from" -le "$to" ] 2>/dev/null; then
          local i="$from"
          while [ "$i" -le "$to" ]; do expanded="$expanded $i"; i=$((i + 1)); done
        fi
        ;;
      *) expanded="$expanded $token" ;;
    esac
  done
  printf '%s' "$expanded"
}

# 读取变量并按 & 拆分成分行列表。
split_env() {
  local name="$1"
  local value
  value="$(eval "printf '%s' \"\${$name:-}\"")"
  printf '%s' "$value" | sed 's/&/\n/g'
}

case "$mode" in
  normal|now)
    run_one "$script" "$@"
    ;;
  conc)
    if [ $# -lt 1 ]; then
      echo "缺少并发运行的环境变量参数，用法: task $script conc <变量名> [序号...]" >&2
      exit 1
    fi
    env_name="$1"; shift
    mapfile -t accounts < <(split_env "$env_name")
    if [ ${#accounts[@]} -eq 0 ]; then
      echo "环境变量 $env_name 为空，无法并发执行" >&2
      exit 1
    fi
    # 未指定序号时跑全部。
    indexes=()
    if [ $# -gt 0 ]; then
      mapfile -t indexes < <(echo "$(expand_indexes "$*")" | tr ' ' '\n' | awk 'NF && !seen[$0]++')
    else
      for i in $(seq 1 ${#accounts[@]}); do indexes+=("$i"); done
    fi
    pids=()
    for idx in "${indexes[@]}"; do
      if ! [ "$idx" -ge 1 ] 2>/dev/null || [ "$idx" -gt ${#accounts[@]} ]; then
        continue
      fi
      account="${accounts[$((idx - 1))]}"
      echo "## 并发启动第 $idx 个账号"
      ( export "${env_name}=${account}"; run_one "$script" ) &
      pids+=($!)
    done
    if [ ${#pids[@]} -eq 0 ]; then
      echo "没有匹配到可执行的账号序号" >&2
      exit 1
    fi
    exit_code=0
    for pid in "${pids[@]}"; do
      wait "$pid" || exit_code=1
    done
    exit $exit_code
    ;;
  desi)
    if [ $# -lt 2 ]; then
      echo "缺少指定账号参数，用法: task $script desi <变量名> <序号...>" >&2
      exit 1
    fi
    env_name="$1"; shift
    mapfile -t accounts < <(split_env "$env_name")
    mapfile -t indexes < <(echo "$(expand_indexes "$*")" | tr ' ' '\n' | awk 'NF && !seen[$0]++')
    exit_code=0
    for idx in "${indexes[@]}"; do
      if ! [ "$idx" -ge 1 ] 2>/dev/null || [ "$idx" -gt ${#accounts[@]} ]; then
        echo "跳过无效序号 $idx（共 ${#accounts[@]} 个账号）" >&2
        continue
      fi
      account="${accounts[$((idx - 1))]}"
      echo "## 执行第 $idx 个账号"
      ( export "${env_name}=${account}"; run_one "$script" ) || exit_code=1
    done
    exit $exit_code
    ;;
esac`

