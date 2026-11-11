package panel

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ipLocationCache 缓存已解析的 IP 归属地，避免重复请求。
var ipLocationCache sync.Map

// lookupIPLocation 查询公网 IP 归属地。
// 优先返回缓存；内网/回环直接标注；其余走在线接口，失败返回空串（不缓存失败结果）。
func (app *App) lookupIPLocation(ip string) string {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return ""
	}
	if parsed.IsLoopback() || parsed.IsPrivate() {
		return "内网IP"
	}
	if cached, found := ipLocationCache.Load(ip); found {
		return cached.(string)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	address := app.queryIPLocation(ctx, ip)
	// 只有成功解析才缓存，避免把网络抖动固化为空结果。
	if address != "" {
		ipLocationCache.Store(ip, address)
	}
	return address
}

// queryIPLocation 调用主接口（ip-api.com，中文 JSON）。
// 备用接口返回 GBK 编码，为免引入额外依赖，这里只保留主接口；失败时返回空串。
func (app *App) queryIPLocation(ctx context.Context, ip string) string {
	return app.queryIPAPI(ctx, ip)
}

func (app *App) queryIPAPI(ctx context.Context, ip string) string {
	request, err := http.NewRequestWithContext(ctx, "GET", "http://ip-api.com/json/"+ip+"?lang=zh-CN&fields=status,country,regionName,city,isp", nil)
	if err != nil {
		return ""
	}
	response, err := app.client.Do(request)
	if err != nil {
		return ""
	}
	defer response.Body.Close()
	var body struct {
		Status     string `json:"status"`
		Country    string `json:"country"`
		RegionName string `json:"regionName"`
		City       string `json:"city"`
		ISP        string `json:"isp"`
	}
	if json.NewDecoder(response.Body).Decode(&body) != nil || body.Status != "success" {
		return ""
	}
	return joinLocation(body.Country, body.RegionName, body.City, body.ISP)
}

// joinLocation 去重拼接归属地，忽略空值与「XX省XX市」里的重复部分。
func joinLocation(parts ...string) string {
	seen := map[string]bool{}
	result := []string{}
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return strings.Join(result, " ")
}

// enrichLoginLogs 为缺少归属地的公网 IP 补全地址，并回写数据库缓存。
// 命中缓存的立即返回；未命中时先返回空，后台异步补全，避免阻塞页面加载。
func (app *App) enrichLoginLogs(logs []Object) []Object {
	pending := []string{}
	for index := range logs {
		address := text(logs[index]["address"])
		ip := text(logs[index]["ip"])
		if address != "" || ip == "" {
			continue
		}
		if cached, found := ipLocationCache.Load(ip); found {
			logs[index]["address"] = cached.(string)
			continue
		}
		parsed := net.ParseIP(ip)
		if parsed == nil {
			continue
		}
		if parsed.IsLoopback() || parsed.IsPrivate() {
			logs[index]["address"] = "内网IP"
			continue
		}
		pending = append(pending, ip)
	}
	if len(pending) > 0 {
		go app.resolveLoginLocations(pending)
	}
	return logs
}

// resolveLoginLocations 后台解析归属地并回写，去重后逐个查询。
func (app *App) resolveLoginLocations(ips []string) {
	seen := map[string]bool{}
	for _, ip := range ips {
		if seen[ip] {
			continue
		}
		seen[ip] = true
		if address := app.lookupIPLocation(ip); address != "" {
			_, _ = app.store.db.Exec("UPDATE login_logs SET address=? WHERE ip=? AND address=''", address, ip)
		}
	}
}
