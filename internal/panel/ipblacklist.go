package panel

import (
	"net"
	"net/http"
	"strings"
)

const ipBlacklistKey = "ipBlacklist"

// ipBlacklist 读取 IP 黑名单（逗号分隔的 IP 或 CIDR）。
func (app *App) ipBlacklist() []string {
	raw := strings.TrimSpace(app.store.setting(ipBlacklistKey, ""))
	if raw == "" {
		return []string{}
	}
	result := []string{}
	for _, entry := range strings.Split(raw, ",") {
		if value := strings.TrimSpace(entry); value != "" {
			result = append(result, value)
		}
	}
	return result
}

// ipBlocked 判断来源 IP 是否命中黑名单（支持单个 IP 与 CIDR）。
func (app *App) ipBlocked(ip string) bool {
	target := net.ParseIP(ip)
	if target == nil {
		return false
	}
	for _, entry := range app.ipBlacklist() {
		if strings.Contains(entry, "/") {
			if _, network, err := net.ParseCIDR(entry); err == nil && network.Contains(target) {
				return true
			}
			continue
		}
		if parsed := net.ParseIP(entry); parsed != nil && parsed.Equal(target) {
			return true
		}
	}
	return false
}

// saveIPBlacklist 写回黑名单，去重并保持顺序。
func (app *App) saveIPBlacklist(entries []string) error {
	seen := map[string]bool{}
	cleaned := []string{}
	for _, entry := range entries {
		value := strings.TrimSpace(entry)
		if value == "" || seen[value] {
			continue
		}
		if strings.Contains(value, "/") {
			if _, _, err := net.ParseCIDR(value); err != nil {
				return errInvalidIP(value)
			}
		} else if net.ParseIP(value) == nil {
			return errInvalidIP(value)
		}
		seen[value] = true
		cleaned = append(cleaned, value)
	}
	_, err := app.store.db.Exec("INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", ipBlacklistKey, strings.Join(cleaned, ","))
	return err
}

type invalidIPError string

func (value invalidIPError) Error() string { return "IP 地址无效: " + string(value) }

func errInvalidIP(value string) error { return invalidIPError(value) }

// ipBlacklistAPI 处理 GET / PUT / DELETE /api/user/ip-blacklist。
func (app *App) ipBlacklistAPI(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case "GET":
		ok(writer, app.ipBlacklist())
	case "PUT":
		var body struct {
			IP string `json:"ip"`
		}
		if err := decode(request, &body); err != nil {
			apiError(writer, err)
			return
		}
		entries := append(app.ipBlacklist(), body.IP)
		if err := app.saveIPBlacklist(entries); err != nil {
			apiError(writer, err)
			return
		}
		ok(writer, app.ipBlacklist())
	case "DELETE":
		var body struct {
			IP string `json:"ip"`
		}
		if err := decode(request, &body); err != nil {
			apiError(writer, err)
			return
		}
		target := strings.TrimSpace(body.IP)
		entries := []string{}
		for _, entry := range app.ipBlacklist() {
			if entry != target {
				entries = append(entries, entry)
			}
		}
		if err := app.saveIPBlacklist(entries); err != nil {
			apiError(writer, err)
			return
		}
		ok(writer, app.ipBlacklist())
	default:
		fail(writer, 405, "方法不支持")
	}
}
