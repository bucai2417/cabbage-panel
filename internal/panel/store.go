package panel

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type Object map[string]any

type Store struct {
	lock *os.File
	db   *sql.DB
	mu   sync.Mutex
}

func openStore(root string) (*Store, error) {
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	lock, err := os.OpenFile(filepath.Join(root, "cabbage.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := lockFile(lock); err != nil {
		lock.Close()
		return nil, errors.New("数据目录已被另一个面板进程占用")
	}
	success := false
	defer func() {
		if !success {
			lock.Close()
		}
	}()
	db, err := sql.Open("sqlite", filepath.Join(root, "cabbage.db"))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec("PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000; PRAGMA foreign_keys=ON;" +
		"CREATE TABLE IF NOT EXISTS records (id INTEGER PRIMARY KEY AUTOINCREMENT, kind TEXT NOT NULL, data TEXT NOT NULL CHECK(json_valid(data)));" +
		"CREATE INDEX IF NOT EXISTS records_kind ON records(kind);" +
		"CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY, value TEXT NOT NULL);" +
		"CREATE TABLE IF NOT EXISTS sessions (token TEXT PRIMARY KEY, principal TEXT NOT NULL, expires INTEGER NOT NULL);" +
		"CREATE TABLE IF NOT EXISTS authorizations (id INTEGER PRIMARY KEY AUTOINCREMENT, app_id INTEGER NOT NULL, token TEXT NOT NULL UNIQUE, name TEXT NOT NULL, created TEXT NOT NULL, expires INTEGER NOT NULL, permanent INTEGER NOT NULL DEFAULT 0);" +
		"CREATE INDEX IF NOT EXISTS authorizations_app ON authorizations(app_id, id DESC);" +
		"CREATE TABLE IF NOT EXISTS runs (id INTEGER PRIMARY KEY AUTOINCREMENT, task_id INTEGER NOT NULL, kind TEXT NOT NULL, name TEXT NOT NULL, command TEXT NOT NULL, status TEXT NOT NULL, started TEXT, finished TEXT, exit_code INTEGER, duration REAL NOT NULL DEFAULT 0, log_path TEXT NOT NULL DEFAULT '', created TEXT NOT NULL);" +
		"CREATE INDEX IF NOT EXISTS runs_task ON runs(kind, task_id, id DESC);" +
		"CREATE TABLE IF NOT EXISTS audit (id INTEGER PRIMARY KEY AUTOINCREMENT, action TEXT NOT NULL, actor TEXT NOT NULL, created TEXT NOT NULL);" +
		"CREATE TABLE IF NOT EXISTS login_logs (id INTEGER PRIMARY KEY AUTOINCREMENT, timestamp INTEGER NOT NULL, address TEXT NOT NULL DEFAULT '', ip TEXT NOT NULL DEFAULT '', platform TEXT NOT NULL DEFAULT '', status INTEGER NOT NULL DEFAULT 0);" +
		"CREATE TABLE IF NOT EXISTS notify_logs (id INTEGER PRIMARY KEY AUTOINCREMENT, timestamp INTEGER NOT NULL, title TEXT NOT NULL DEFAULT '', content TEXT NOT NULL DEFAULT '', status INTEGER NOT NULL DEFAULT 0, notify_type TEXT NOT NULL DEFAULT '', source TEXT NOT NULL DEFAULT '', script_name TEXT NOT NULL DEFAULT '');" +
		"CREATE INDEX IF NOT EXISTS notify_logs_time ON notify_logs(timestamp DESC, id DESC);" +
		"CREATE INDEX IF NOT EXISTS login_logs_time ON login_logs(timestamp DESC, id DESC);" +
		"PRAGMA user_version=1;")
	if err != nil {
		db.Close()
		return nil, err
	}
	if err := ensureAuditColumns(db); err != nil {
		db.Close()
		return nil, err
	}
	if err := hashLegacyAuthorizations(db); err != nil {
		db.Close()
		return nil, err
	}
	if err := purgeLegacyAppSecrets(db); err != nil {
		db.Close()
		return nil, err
	}
	migrateLoginLogs(db)
	success = true
	return &Store{db: db, lock: lock}, nil
}

func ensureAuditColumns(db *sql.DB) error {
	rows, err := db.Query("PRAGMA table_info(audit)")
	if err != nil {
		return err
	}
	defer rows.Close()
	columns := map[string]bool{}
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return err
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, column := range []struct{ name, definition string }{
		{"ip", "ip TEXT NOT NULL DEFAULT ''"},
		{"remote", "remote INTEGER NOT NULL DEFAULT 0"},
	} {
		if columns[column.name] {
			continue
		}
		if _, err := db.Exec("ALTER TABLE audit ADD COLUMN " + column.definition); err != nil {
			return err
		}
	}
	return nil
}

// hashLegacyAuthorizations 把旧版本明文保存的 Authorization 迁移为 SHA-256 散列。
// sessions 表本来就以散列作为键，因此迁移后撤销令牌仍能精确删除对应会话。
func hashLegacyAuthorizations(db *sql.DB) error {
	var marker string
	if db.QueryRow("SELECT value FROM settings WHERE key='authorizations_hashed'").Scan(&marker) == nil && marker == "1" {
		return nil
	}
	rows, err := db.Query("SELECT id, token FROM authorizations")
	if err != nil {
		return err
	}
	type entry struct {
		id    int64
		token string
	}
	items := []entry{}
	for rows.Next() {
		var item entry
		if rows.Scan(&item.id, &item.token) == nil {
			items = append(items, item)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, item := range items {
		if _, err := db.Exec("UPDATE authorizations SET token=? WHERE id=?", hash(item.token), item.id); err != nil {
			return err
		}
	}
	_, err = db.Exec("INSERT INTO settings(key,value) VALUES('authorizations_hashed','1') ON CONFLICT(key) DO UPDATE SET value=excluded.value")
	return err
}

// purgeLegacyAppSecrets 删除旧版本应用记录中的明文 client_secret，只保留 secret_hash。
func purgeLegacyAppSecrets(db *sql.DB) error {
	rows, err := db.Query("SELECT id, data FROM records WHERE kind='apps'")
	if err != nil {
		return err
	}
	type entry struct {
		id   int64
		data string
	}
	items := []entry{}
	for rows.Next() {
		var item entry
		if rows.Scan(&item.id, &item.data) == nil {
			items = append(items, item)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, item := range items {
		var record Object
		if json.Unmarshal([]byte(item.data), &record) != nil {
			continue
		}
		if _, found := record["client_secret"]; !found {
			continue
		}
		delete(record, "client_secret")
		data, err := json.Marshal(record)
		if err != nil {
			return err
		}
		if _, err := db.Exec("UPDATE records SET data=? WHERE kind='apps' AND id=?", string(data), item.id); err != nil {
			return err
		}
	}
	return nil
}

func (store *Store) close() {
	_ = store.db.Close()
	if store.lock != nil {
		_ = store.lock.Close()
	}
}

func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }

// jsTimestamp 与青龙（稳定版）new Date().toString() 输出一致：
// "Sun Oct 04 02:58:24 GMT+08:00 2026"。
func jsTimestamp(value time.Time) string {
	return value.In(time.Local).Format("Mon Jan 02 15:04:05 GMT-07:00 2006")
}
func randomToken() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	return hex.EncodeToString(bytes)
}
func text(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}
func integer(value any) int64 {
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	case json.Number:
		if result, err := typed.Int64(); err == nil {
			return result
		}
		if result, err := typed.Float64(); err == nil {
			return int64(result)
		}
	case string:
		trimmed := strings.TrimSpace(typed)
		if result, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
			return result
		}
		if result, err := strconv.ParseFloat(trimmed, 64); err == nil {
			return int64(result)
		}
	}
	return 0
}
func boolean(value any) bool {
	return value == true || value == float64(1) || value == 1 || value == int64(1)
}
func clone(value Object) Object {
	bytes, _ := json.Marshal(value)
	var result Object
	_ = json.Unmarshal(bytes, &result)
	return result
}

func (store *Store) list(kind string) ([]Object, error) {
	rows, err := store.db.Query("SELECT id,data FROM records WHERE kind=? ORDER BY id", kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Object{}
	for rows.Next() {
		var id int64
		var data string
		if err := rows.Scan(&id, &data); err != nil {
			return nil, err
		}
		var item Object
		if err := json.Unmarshal([]byte(data), &item); err != nil {
			return nil, err
		}
		item["id"] = id
		item["_id"] = strconv.FormatInt(id, 10)
		result = append(result, item)
	}
	return result, rows.Err()
}

func (store *Store) get(kind string, id int64) (Object, error) {
	var data string
	if err := store.db.QueryRow("SELECT data FROM records WHERE kind=? AND id=?", kind, id).Scan(&data); err != nil {
		return nil, err
	}
	var item Object
	err := json.Unmarshal([]byte(data), &item)
	if err != nil {
		return nil, err
	}
	item["id"] = id
	item["_id"] = strconv.FormatInt(id, 10)
	return item, nil
}

func (store *Store) save(kind string, item Object) (Object, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.saveLocked(kind, item)
}

func (store *Store) saveLocked(kind string, item Object) (Object, error) {
	id := integer(item["id"])
	if id != 0 {
		old, err := store.get(kind, id)
		if err != nil {
			return nil, err
		}
		for key, value := range item {
			old[key] = value
		}
		item = old
	} else {
		item = clone(item)
		item["createdAt"] = now()
		item["timestamp"] = jsTimestamp(time.Now())
	}
	item["updatedAt"] = now()
	// 应用密钥只保存散列；任何路径都不允许把明文 secret 写入记录。
	if kind == "apps" {
		delete(item, "client_secret")
	}
	data, err := json.Marshal(item)
	if err != nil {
		return nil, err
	}
	if id == 0 {
		result, err := store.db.Exec("INSERT INTO records(kind,data) VALUES(?,?)", kind, string(data))
		if err != nil {
			return nil, err
		}
		id, err = result.LastInsertId()
		if err != nil {
			return nil, err
		}
	} else {
		if _, err := store.db.Exec("UPDATE records SET data=? WHERE kind=? AND id=?", string(data), kind, id); err != nil {
			return nil, err
		}
	}
	item["id"] = id
	item["_id"] = strconv.FormatInt(id, 10)
	return item, nil
}

func (store *Store) remove(kind string, id int64) error {
	_, err := store.db.Exec("DELETE FROM records WHERE kind=? AND id=?", kind, id)
	return err
}
func (store *Store) setting(key, fallback string) string {
	var value string
	if store.db.QueryRow("SELECT value FROM settings WHERE key=?", key).Scan(&value) != nil {
		return fallback
	}
	return value
}
func (store *Store) setSetting(key, value string) error {
	_, err := store.db.Exec("INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", key, value)
	return err
}

// recordLogin 记录一次成功或失败的登录，字段形状对齐青龙 login-log。
// 公网 IP 的归属地由 enrichLoginLogs 在查询时补全并回写。
func (store *Store) recordLogin(ip, platform, address string, success bool) {
	status := 0
	if !success {
		status = 1
	}
	_, _ = store.db.Exec("INSERT INTO login_logs(timestamp,address,ip,platform,status) VALUES(?,?,?,?,?)", time.Now().UnixMilli(), address, ip, platform, status)
}

// loginLogs 返回最近的登录记录；与青龙一致，只保留 100 条。
func (store *Store) loginLogs() ([]Object, error) {
	_, _ = store.db.Exec("DELETE FROM login_logs WHERE id NOT IN (SELECT id FROM login_logs ORDER BY timestamp DESC, id DESC LIMIT 100)")
	rows, err := store.db.Query("SELECT timestamp,address,ip,platform,status FROM login_logs ORDER BY timestamp DESC, id DESC LIMIT 100")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Object{}
	for rows.Next() {
		var timestamp, status int64
		var address, ip, platform string
		if rows.Scan(&timestamp, &address, &ip, &platform, &status) == nil {
			result = append(result, Object{"timestamp": timestamp, "address": address, "ip": ip, "platform": platform, "status": status})
		}
	}
	return result, rows.Err()
}

// loginAddress 不引入归属地库：本机与内网地址标注为内网IP，其余留空。
func loginAddress(ip string) string {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return ""
	}
	if parsed.IsLoopback() || parsed.IsPrivate() {
		return "内网IP"
	}
	return ""
}

// migrateLoginLogs 把旧 audit 表中的登录记录迁移为青龙形状，只执行一次。
func migrateLoginLogs(db *sql.DB) {
	var marker string
	if db.QueryRow("SELECT value FROM settings WHERE key='login_logs_migrated'").Scan(&marker) == nil && marker == "1" {
		return
	}
	rows, err := db.Query("SELECT created,ip FROM audit WHERE action='login' ORDER BY id")
	if err != nil {
		return
	}
	type legacy struct {
		stamp int64
		ip    string
	}
	items := []legacy{}
	for rows.Next() {
		var created, ip string
		if rows.Scan(&created, &ip) != nil {
			continue
		}
		stamp := time.Now().UnixMilli()
		if parsed, err := time.Parse(time.RFC3339Nano, created); err == nil {
			stamp = parsed.UnixMilli()
		}
		items = append(items, legacy{stamp: stamp, ip: ip})
	}
	_ = rows.Close()
	for _, item := range items {
		_, _ = db.Exec("INSERT INTO login_logs(timestamp,address,ip,platform,status) VALUES(?,?,?,?,?)", item.stamp, loginAddress(item.ip), item.ip, "", 0)
	}
	_, _ = db.Exec("INSERT INTO settings(key,value) VALUES('login_logs_migrated','1') ON CONFLICT(key) DO UPDATE SET value=excluded.value")
}

func safePath(root, relative string) (string, error) {
	if relative == "" || strings.HasPrefix(relative, "/") || strings.ContainsRune(relative, 0) || strings.Contains(relative, "\\") || strings.Contains(relative, ":") || filepath.IsAbs(relative) {
		return "", errors.New("文件路径无效")
	}
	clean := filepath.Clean(filepath.FromSlash(relative))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("禁止访问数据目录之外的文件")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	target := filepath.Join(absolute, clean)
	current := absolute
	for _, part := range strings.Split(clean, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("不允许通过符号链接访问文件")
		}
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
	}
	return target, nil
}
