package panel

import (
	"fmt"
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
// requester 非空时拒绝会封禁当前访问来源的条目，避免管理员把自己锁在门外。
func (app *App) saveIPBlacklist(entries []string, requester string) error {
	seen := map[string]bool{}
	cleaned := []string{}
	requestIP := net.ParseIP(strings.TrimSpace(requester))
	for _, entry := range entries {
		value := strings.TrimSpace(entry)
		if value == "" || seen[value] {
			continue
		}
		if strings.Contains(value, "/") {
			_, network, err := net.ParseCIDR(value)
			if err != nil {
				return errInvalidIP(value)
			}
			if tooBroadCIDR(network) {
				return fmt.Errorf("黑名单网段过宽: %s（IPv4 至少 /8，IPv6 至少 /16）", value)
			}
		} else if net.ParseIP(value) == nil {
			return errInvalidIP(value)
		}
		if requestIP != nil && blacklistMatches(value, requestIP) {
			return fmt.Errorf("该条目会封禁当前访问来源 %s，已拒绝保存", requestIP.String())
		}
		seen[value] = true
		cleaned = append(cleaned, value)
	}
	_, err := app.store.db.Exec("INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", ipBlacklistKey, strings.Join(cleaned, ","))
	return err
}

// blacklistMatches 判断单条黑名单条目是否命中指定 IP。
func blacklistMatches(entry string, ip net.IP) bool {
	if strings.Contains(entry, "/") {
		_, network, err := net.ParseCIDR(entry)
		return err == nil && network.Contains(ip)
	}
	parsed := net.ParseIP(entry)
	return parsed != nil && parsed.Equal(ip)
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
		if err := app.saveIPBlacklist(entries, app.clientIP(request)); err != nil {
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
		if err := app.saveIPBlacklist(entries, ""); err != nil {
			apiError(writer, err)
			return
		}
		ok(writer, app.ipBlacklist())
	default:
		fail(writer, 405, "方法不支持")
	}
}
