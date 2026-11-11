package panel

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bucai2417/cabbage-panel/web"
	"golang.org/x/crypto/bcrypt"
)

type App struct {
	store                                      *Store
	engine                                     *Engine
	root, version, commit, updateKey, setupKey string
	webDir                                     string
	helperKey                                  string
	helperAddress                              string
	helperMu                                   sync.RWMutex
	scriptMu                                   sync.RWMutex
	gate                                       sync.RWMutex
	writeMu                                    sync.Mutex
	depMu                                      sync.Mutex
	loginMu                                    sync.Mutex
	attempts                                   map[string][]time.Time
	client                                     *http.Client
	started                                    time.Time
}

func New(root, version, commit, updateKey string) (*App, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	store, err := openStore(absolute)
	if err != nil {
		return nil, err
	}
	app := &App{store: store, root: absolute, version: version, commit: commit, updateKey: updateKey, attempts: map[string][]time.Time{}, client: &http.Client{Timeout: 45 * time.Second}, started: time.Now()}
	app.helperKey = randomToken()
	for _, dir := range []string{"scripts", "logs", "config", "backups", "repos", "packages", "bin", "updates", "debug"} {
		if err := os.MkdirAll(filepath.Join(absolute, dir), 0700); err != nil {
			store.close()
			return nil, err
		}
	}
	if entries, err := os.ReadDir(filepath.Join(absolute, "debug")); err == nil {
		for _, entry := range entries {
			_ = os.RemoveAll(filepath.Join(absolute, "debug", entry.Name()))
		}
	}
	if store.setting("password", "") == "" {
		keyPath := filepath.Join(absolute, "setup.key")
		key, _ := os.ReadFile(keyPath)
		app.setupKey = strings.TrimSpace(string(key))
		if app.setupKey == "" {
			app.setupKey = randomToken()
			if err := os.WriteFile(keyPath, []byte(app.setupKey), 0600); err != nil {
				store.close()
				return nil, err
			}
		}
		log.Printf("首次设置密钥: %s (保存在 %s)", app.setupKey, keyPath)
	}
	_, _ = store.db.Exec("UPDATE runs SET status='interrupted',finished=? WHERE status IN ('running','queued')", now())
	_, _ = store.db.Exec("DELETE FROM sessions WHERE expires>0 AND expires<?", time.Now().Unix())
	_, _ = store.db.Exec("DELETE FROM authorizations WHERE permanent=0 AND expires>0 AND expires<?", time.Now().Unix())
	if err := app.initializeConfigs(); err != nil {
		store.close()
		return nil, err
	}
	app.engine = newEngine(app)
	if err := app.installHelpers(); err != nil {
		store.close()
		return nil, err
	}
	app.engine.start()
	return app, nil
}

func (app *App) Close() { app.engine.close(); app.store.close() }

func (app *App) Serve(ctx context.Context, addr string) error {
	server := &http.Server{Addr: addr, Handler: app.Handler(), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 90 * time.Second, IdleTimeout: 90 * time.Second, MaxHeaderBytes: 1 << 20}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		app.Close()
		return err
	}
	host, port, _ := net.SplitHostPort(listener.Addr().String())
	if host == "" || host == "::" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	app.helperMu.Lock()
	app.helperAddress = "http://" + net.JoinHostPort(host, port) + "/api/notifications/send"
	app.helperMu.Unlock()
	log.Printf("大白菜面板 v%s 已启动 http://localhost%s，MCP: /mcp，数据目录: %s", app.version, addr, app.root)
	app.pruneSystemLog()
	app.appendSystemLog("info", "[boot] 大白菜面板 v"+app.version+" 已启动 http://localhost"+addr)
	finished := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			_ = server.Shutdown(shutdown)
		case <-finished:
		}
	}()
	err = server.Serve(listener)
	close(finished)
	app.Close()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func respond(writer http.ResponseWriter, status int, data any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(data)
}
func ok(writer http.ResponseWriter, data any) {
	respond(writer, 200, Object{"code": 200, "data": data})
}
func fail(writer http.ResponseWriter, status int, message string) {
	respond(writer, status, Object{"code": status, "message": message})
}
func decode(request *http.Request, value any) error {
	decoder := json.NewDecoder(io.LimitReader(request.Body, 8<<20))
	if err := decoder.Decode(value); err != nil {
		return errors.New("JSON 请求格式错误")
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return errors.New("请求包含多余或过大的内容")
	}
	return nil
}
func hash(value string) string {
	result := sha256.Sum256([]byte(value))
	return hex.EncodeToString(result[:])
}

func clientIP(request *http.Request) string {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil {
		host = request.RemoteAddr
	}
	host = strings.TrimSpace(strings.Trim(host, "[]"))
	if host == "" {
		return ""
	}
	if ip := net.ParseIP(host); ip != nil && (ip.IsLoopback() || ip.IsPrivate()) {
		if forwarded := request.Header.Get("X-Forwarded-For"); forwarded != "" {
			first := strings.TrimSpace(strings.Split(forwarded, ",")[0])
			if net.ParseIP(first) != nil {
				return first
			}
		}
		if realIP := strings.TrimSpace(request.Header.Get("X-Real-IP")); net.ParseIP(realIP) != nil {
			return realIP
		}
	}
	return host
}

// loginPlatform 按青龙约定把登录设备归为 mobile 或 desktop。
func loginPlatform(request *http.Request) string {
	agent := strings.ToLower(request.Header.Get("User-Agent"))
	for _, marker := range []string{"android", "iphone", "ipad", "ipod", "mobile"} {
		if strings.Contains(agent, marker) {
			return "mobile"
		}
	}
	return "desktop"
}

func sessionTTL(seconds int64, fallback time.Duration) time.Duration {
	if seconds < 60 {
		return fallback
	}
	if seconds > 30*24*60*60 {
		seconds = 30 * 24 * 60 * 60
	}
	return time.Duration(seconds) * time.Second
}

func appExpired(item Object) bool {
	value := strings.TrimSpace(text(item["expiresAt"]))
	if value == "" {
		return false
	}
	if len(value) > 10 {
		value = value[:10]
	}
	deadline, err := time.ParseInLocation("2006-01-02", value, time.Local)
	if err != nil {
		return false
	}
	return !time.Now().Before(deadline.Add(24 * time.Hour))
}

func (app *App) SetWebDir(directory string) error {
	if directory == "" {
		app.webDir = ""
		return nil
	}
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("前端目录无效")
	}
	app.webDir = absolute
	return nil
}

func (app *App) Handler() http.Handler {
	var assets fs.FS
	if app.webDir != "" {
		assets = os.DirFS(app.webDir)
	} else {
		assets, _ = fs.Sub(web.Assets, "dist")
	}
	static := http.FileServer(http.FS(assets))
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("request panic: %v", recovered)
				fail(writer, 500, "内部错误，请查看服务日志")
			}
		}()
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("X-Frame-Options", "DENY")
		writer.Header().Set("Referrer-Policy", "no-referrer")
		writer.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		path := strings.TrimRight(request.URL.Path, "/")
		if path == "" {
			path = "/"
		}
		if path == "/healthz" {
			ok(writer, Object{"status": "ok"})
			return
		}
		if path == "/mcp" || strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/open/") {
			writer.Header().Set("Cache-Control", "no-store")
			if origin := request.Header.Get("Origin"); origin != "" {
				parsed, err := url.Parse(origin)
				if err != nil || !strings.EqualFold(parsed.Host, request.Host) {
					fail(writer, 403, "来源校验失败")
					return
				}
			}
			request.Body = http.MaxBytesReader(writer, request.Body, 256<<20)
			if path == "/api/notifications/send" {
				app.helperNotification(writer, request)
				return
			}
			if app.public(writer, request, path) {
				return
			}
			principal, err := app.authenticate(request)
			if err != nil {
				fail(writer, 401, "请先登录或提供有效的 Bearer token")
				return
			}
			if path == "/mcp" {
				if !app.allowed(principal, "mcp") {
					fail(writer, 403, "应用没有 MCP 权限")
					return
				}
				app.mcp(writer, request, principal)
				return
			}
			apiPath := strings.TrimPrefix(strings.TrimPrefix(path, "/api"), "/open")
			resource := strings.Split(strings.TrimPrefix(apiPath, "/"), "/")[0]
			if !app.allowed(principal, resource) {
				fail(writer, 403, "应用没有此接口的权限")
				return
			}
			if resource == "crons" && strings.HasSuffix(apiPath, "/source") && !app.allowed(principal, "scripts") {
				fail(writer, 403, "读取任务源码需要 scripts 权限")
				return
			}
			if strings.HasPrefix(apiPath, "/backups") {
				app.backupAPI(writer, request, apiPath, principal)
				return
			}
			app.gate.RLock()
			defer app.gate.RUnlock()
			if request.Method != "GET" {
				app.writeMu.Lock()
				defer app.writeMu.Unlock()
			}
			app.api(writer, request, apiPath, principal)
			return
		}
		if request.Method != "GET" && request.Method != "HEAD" {
			fail(writer, 405, "方法不支持")
			return
		}
		assetPath := strings.TrimPrefix(path, "/")
		if assetPath != "" {
			if info, err := fs.Stat(assets, assetPath); err == nil && !info.IsDir() {
				if strings.HasPrefix(assetPath, "assets/") {
					writer.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				static.ServeHTTP(writer, request)
				return
			}
		}
		index, err := fs.ReadFile(assets, "index.html")
		if err != nil {
			http.Error(writer, "前端尚未构建，请执行 npm --prefix web ci && npm --prefix web run build 后重新编译", 503)
			return
		}
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		writer.Header().Set("Cache-Control", "no-cache")
		_, _ = writer.Write(index)
	})
}

func (app *App) rateAllowed(request *http.Request) bool {
	host, _, _ := net.SplitHostPort(request.RemoteAddr)
	app.loginMu.Lock()
	defer app.loginMu.Unlock()
	cutoff := time.Now().Add(-15 * time.Minute)
	for key, times := range app.attempts {
		if len(times) == 0 || times[len(times)-1].Before(cutoff) {
			delete(app.attempts, key)
		}
	}
	times := app.attempts[host]
	valid := []time.Time{}
	for _, stamp := range times {
		if stamp.After(cutoff) {
			valid = append(valid, stamp)
		}
	}
	if len(valid) >= 30 || len(app.attempts) > 10000 {
		return false
	}
	app.attempts[host] = append(valid, time.Now())
	return true
}

func (app *App) public(writer http.ResponseWriter, request *http.Request, path string) bool {
	if path == "/api/system/init" && request.Method == "GET" {
		ok(writer, Object{"isInitialized": app.store.setting("password", "") != "", "version": app.version})
		return true
	}
	if path == "/api/user/init" && request.Method == "POST" {
		app.writeMu.Lock()
		defer app.writeMu.Unlock()
		if !app.rateAllowed(request) {
			fail(writer, 429, "尝试过于频繁，请稍后再试")
			return true
		}
		if app.store.setting("password", "") != "" {
			fail(writer, 409, "已经完成初始化")
			return true
		}
		var body Object
		if err := decode(request, &body); err != nil {
			fail(writer, 400, err.Error())
			return true
		}
		if subtle.ConstantTimeCompare([]byte(text(body["setupToken"])), []byte(app.setupKey)) != 1 {
			fail(writer, 403, "首次设置密钥错误，请查看服务启动日志或 data/setup.key")
			return true
		}
		if err := app.changeUser(body); err != nil {
			fail(writer, 400, err.Error())
			return true
		}
		_ = os.Remove(filepath.Join(app.root, "setup.key"))
		ok(writer, Object{"initialized": true})
		return true
	}
	if path == "/api/user/login" && request.Method == "POST" {
		if !app.rateAllowed(request) {
			fail(writer, 429, "尝试过于频繁，请稍后再试")
			return true
		}
		// 命中 IP 黑名单直接拒绝，并记录一次失败登录。
		if ip := clientIP(request); app.ipBlocked(ip) {
			app.store.recordLogin(ip, loginPlatform(request), loginAddress(ip), false)
			fail(writer, 403, "该 IP 已被加入黑名单")
			return true
		}
		var body Object
		if err := decode(request, &body); err != nil {
			fail(writer, 400, err.Error())
			return true
		}
		passwordHash := app.store.setting("password", "")
		ip := clientIP(request)
		platform := loginPlatform(request)
		if passwordHash == "" || bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(text(body["password"]))) != nil || text(body["username"]) != app.store.setting("username", "") {
			if passwordHash != "" {
				app.store.recordLogin(ip, platform, loginAddress(ip), false)
			}
			fail(writer, 401, "用户名或密码错误")
			return true
		}
		token, err := app.newSession("admin", 24*time.Hour)
		if err != nil {
			fail(writer, 500, err.Error())
			return true
		}
		http.SetCookie(writer, &http.Cookie{Name: "cabbage_session", Value: token, HttpOnly: true, Secure: request.TLS != nil, SameSite: http.SameSiteStrictMode, Path: "/", MaxAge: 86400})
		app.store.recordLogin(ip, platform, loginAddress(ip), true)
		ok(writer, Object{"token": token, "token_type": "Bearer", "expiration": time.Now().Add(24 * time.Hour).Unix(), "username": text(body["username"])})
		return true
	}
	if path == "/open/auth/token" && request.Method == "GET" {
		if !app.rateAllowed(request) {
			fail(writer, 429, "尝试过于频繁")
			return true
		}
		apps, err := app.store.list("apps")
		if err != nil {
			fail(writer, 500, err.Error())
			return true
		}
		for _, item := range apps {
			if text(item["client_id"]) == request.URL.Query().Get("client_id") && subtle.ConstantTimeCompare([]byte(text(item["secret_hash"])), []byte(hash(request.URL.Query().Get("client_secret")))) == 1 {
				if appExpired(item) {
					fail(writer, 403, "应用已过期")
					return true
				}
				ttl := sessionTTL(integer(request.URL.Query().Get("expiresIn")), 2*time.Hour)
				permanent := request.URL.Query().Get("permanent") == "1" || strings.EqualFold(request.URL.Query().Get("permanent"), "true")
				if permanent {
					ttl = 0
				}
				token, record, err := app.createAppToken(integer(item["id"]), text(item["name"]), ttl, permanent)
				if err != nil {
					fail(writer, 500, err.Error())
					return true
				}
				ok(writer, Object{"token": token, "token_type": "Bearer", "expiration": record["expires"], "expiresIn": record["expiresIn"], "permanent": record["permanent"]})
				return true
			}
		}
		fail(writer, 401, "应用凭证错误")
		return true
	}
	return false
}

func (app *App) changeUser(body Object) error {
	username := strings.TrimSpace(text(body["username"]))
	password := text(body["password"])
	if len(username) < 2 || len(username) > 64 || len(password) < 6 || len(password) > 72 {
		return errors.New("用户名需为 2–64 字节，密码需为 6–72 字节")
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	tx, err := app.store.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for key, value := range map[string]string{"username": username, "password": string(passwordHash)} {
		if _, err := tx.Exec("INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", key, value); err != nil {
			return err
		}
	}
	if _, err := tx.Exec("DELETE FROM sessions"); err != nil {
		return err
	}
	return tx.Commit()
}

func (app *App) newSession(principal string, duration time.Duration) (string, error) {
	token := randomToken()
	expires := int64(0)
	if duration > 0 {
		expires = time.Now().Add(duration).Unix()
	}
	_, err := app.store.db.Exec("INSERT INTO sessions(token,principal,expires) VALUES(?,?,?)", hash(token), principal, expires)
	return token, err
}

func (app *App) createAppToken(appID int64, appName string, ttl time.Duration, permanent bool) (string, Object, error) {
	token, err := app.newSession("app:"+text(appID), ttl)
	if err != nil {
		return "", nil, err
	}
	expires := int64(0)
	if ttl > 0 {
		expires = time.Now().Add(ttl).Unix()
	}
	flag := 0
	if permanent {
		flag = 1
	}
	created := now()
	result, err := app.store.db.Exec("INSERT INTO authorizations(app_id,token,name,created,expires,permanent) VALUES(?,?,?,?,?,?)", appID, token, appName, created, expires, flag)
	if err != nil {
		_, _ = app.store.db.Exec("DELETE FROM sessions WHERE token=?", hash(token))
		return "", nil, err
	}
	id, _ := result.LastInsertId()
	return token, Object{"id": id, "appId": appID, "name": appName, "token": token, "created": created, "expires": expires, "permanent": permanent, "expiresIn": int64(ttl / time.Second)}, nil
}
func (app *App) authenticate(request *http.Request) (string, error) {
	token := strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer ")
	if token == "" {
		if cookie, err := request.Cookie("cabbage_session"); err == nil {
			token = cookie.Value
		}
	}
	var principal string
	err := app.store.db.QueryRow("SELECT principal FROM sessions WHERE token=? AND (expires=0 OR expires>?)", hash(token), time.Now().Unix()).Scan(&principal)
	if err == nil && strings.HasPrefix(principal, "app:") {
		item, getErr := app.store.get("apps", integer(strings.TrimPrefix(principal, "app:")))
		if getErr != nil || appExpired(item) {
			err = errors.New("应用已过期或不存在")
		}
	}
	return principal, err
}
func (app *App) allowed(principal, resource string) bool {
	if principal == "admin" {
		return true
	}
	if resource == "apps" || resource == "user" || resource == "system" || resource == "backups" {
		return false
	}
	item, err := app.store.get("apps", integer(strings.TrimPrefix(principal, "app:")))
	if err != nil || appExpired(item) {
		return false
	}
	values, _ := item["scopes"].([]any)
	for _, value := range values {
		if text(value) == resource {
			return true
		}
	}
	return false
}

func parseIDs(request *http.Request) ([]int64, error) {
	var values []any
	if err := decode(request, &values); err != nil {
		return nil, err
	}
	if len(values) == 0 || len(values) > 500 {
		return nil, errors.New("请选择 1–500 条记录")
	}
	ids := []int64{}
	for _, value := range values {
		id := integer(value)
		if id <= 0 {
			return nil, errors.New("ID 无效")
		}
		ids = append(ids, id)
	}
	return ids, nil
}
func numberQuery(request *http.Request, key string, fallback int) int {
	value, err := strconv.Atoi(request.URL.Query().Get(key))
	if err != nil || value < 1 {
		return fallback
	}
	return value
}
