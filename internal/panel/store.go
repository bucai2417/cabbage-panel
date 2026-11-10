package panel

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
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
		"CREATE TABLE IF NOT EXISTS runs (id INTEGER PRIMARY KEY AUTOINCREMENT, task_id INTEGER NOT NULL, kind TEXT NOT NULL, name TEXT NOT NULL, command TEXT NOT NULL, status TEXT NOT NULL, started TEXT, finished TEXT, exit_code INTEGER, duration REAL NOT NULL DEFAULT 0, log_path TEXT NOT NULL DEFAULT '', created TEXT NOT NULL);" +
		"CREATE INDEX IF NOT EXISTS runs_task ON runs(kind, task_id, id DESC);" +
		"CREATE TABLE IF NOT EXISTS audit (id INTEGER PRIMARY KEY AUTOINCREMENT, action TEXT NOT NULL, actor TEXT NOT NULL, created TEXT NOT NULL);" +
		"PRAGMA user_version=1;")
	if err != nil {
		db.Close()
		return nil, err
	}
	success = true
	return &Store{db: db, lock: lock}, nil
}

func (store *Store) close() {
	_ = store.db.Close()
	if store.lock != nil {
		_ = store.lock.Close()
	}
}

func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }
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
	result, _ := strconv.ParseInt(strings.Split(text(value), ".")[0], 10, 64)
	return result
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
		item["_id"] = id
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
	item["_id"] = id
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
		item["timestamp"] = time.Now().UnixMilli()
	}
	item["updatedAt"] = now()
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
	item["_id"] = id
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
func (store *Store) audit(action, actor string) {
	_, _ = store.db.Exec("INSERT INTO audit(action,actor,created) VALUES(?,?,?)", action, actor, now())
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
