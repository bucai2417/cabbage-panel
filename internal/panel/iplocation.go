package panel

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ipLocationCache 缓存已解析的 IP 归属地，避免重复请求。
var ipLocationCache sync.Map

// defaultIPLocationURL 使用 HTTPS 接口，避免把管理员登录 IP 明文发往第三方。
const defaultIPLocationURL = "https://ipwho.is/{ip}?lang=zh-CN"

// lookupIPLocation 查询公网 IP 归属地。
// 优先返回缓存；内网/回环直接标注；其余走在线接口，失败返回空串（不缓存失败结果）。
func (app *App) lookupIPLocation(ip string) string {
	if app.store.setting("ipLocationEnabled", "true") != "true" {
		return ""
	}
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

// queryIPLocation 调用可配置的归属地接口，默认使用 HTTPS 的 ipwho.is。
// 自定义接口可用 {ip} 占位；兼容 ipwho.is 与 ip-api 的字段命名。
func (app *App) queryIPLocation(ctx context.Context, ip string) string {
	template := strings.TrimSpace(app.store.setting("ipLocationURL", ""))
	if template == "" {
		template = defaultIPLocationURL
	}
	address := strings.ReplaceAll(template, "{ip}", ip)
	request, err := http.NewRequestWithContext(ctx, "GET", address, nil)
	if err != nil {
		return ""
	}
	request.Header.Set("User-Agent", "cabbage-panel/"+app.version)
	response, err := app.client.Do(request)
	if err != nil {
		return ""
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return ""
	}
	var body struct {
		Status     string `json:"status"`
		Success    *bool  `json:"success"`
		Country    string `json:"country"`
		RegionName string `json:"regionName"`
		Region     string `json:"region"`
		City       string `json:"city"`
		ISP        string `json:"isp"`
		Connection struct {
			ISP string `json:"isp"`
		} `json:"connection"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&body) != nil {
		return ""
	}
	success := body.Status == "success" || (body.Success != nil && *body.Success)
	if !success && body.Country == "" && body.City == "" {
		return ""
	}
	region := body.RegionName
	if region == "" {
		region = body.Region
	}
	isp := body.ISP
	if isp == "" {
		isp = body.Connection.ISP
	}
	return joinLocation(body.Country, region, body.City, isp)
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
	if app.store.setting("ipLocationEnabled", "true") != "true" {
		return logs
	}
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
