package panel

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const attemptWindow = 15 * time.Minute

// parseIP 接受纯 IP、带端口的地址和带方括号的 IPv6。
func parseIP(value string) net.IP {
	value = strings.TrimSpace(value)
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}
	return net.ParseIP(strings.Trim(value, "[]"))
}

// trustedProxy 判断来源地址是否在设置的可信反向代理列表中。
// 只有来自这些地址的请求才允许信任 X-Forwarded-For / X-Real-IP。
func (app *App) trustedProxy(ip net.IP) bool {
	if ip == nil {
		return false
	}
	for _, entry := range strings.Split(app.store.setting("trustedProxies", ""), ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if strings.Contains(entry, "/") {
			if _, network, err := net.ParseCIDR(entry); err == nil && network.Contains(ip) {
				return true
			}
			continue
		}
		if parsed := net.ParseIP(entry); parsed != nil && parsed.Equal(ip) {
			return true
		}
	}
	return false
}

// 校验可信代理配置：逗号分隔的 IP 或 CIDR。
// 拒绝过宽网段，避免 0.0.0.0/0 之类的配置让任意来源伪造 X-Forwarded-For。
func validateTrustedProxies(value string) error {
	for _, entry := range strings.Split(value, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if strings.Contains(entry, "/") {
			_, network, err := net.ParseCIDR(entry)
			if err != nil {
				return fmt.Errorf("可信代理网段无效: %s", entry)
			}
			if tooBroadCIDR(network) {
				return fmt.Errorf("可信代理网段过宽: %s（IPv4 至少 /8，IPv6 至少 /16）", entry)
			}
			continue
		}
		if net.ParseIP(entry) == nil {
			return fmt.Errorf("可信代理地址无效: %s", entry)
		}
	}
	return nil
}

// tooBroadCIDR 判断网段是否宽到不应被信任（IPv4 短于 /8，IPv6 短于 /16）。
func tooBroadCIDR(network *net.IPNet) bool {
	ones, bits := network.Mask.Size()
	if bits == 32 {
		return ones < 8
	}
	return ones < 16
}

func (app *App) clientIP(request *http.Request) string {
	remote := parseIP(request.RemoteAddr)
	if remote == nil {
		return strings.TrimSpace(request.RemoteAddr)
	}
	if !app.trustedProxy(remote) {
		return remote.String()
	}
	// 从右往左跳过可信代理，取第一个不可信地址，避免客户端伪造最左侧条目。
	if forwarded := strings.TrimSpace(request.Header.Get("X-Forwarded-For")); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		for index := len(parts) - 1; index >= 0; index-- {
			ip := parseIP(parts[index])
			if ip == nil {
				return remote.String()
			}
			if !app.trustedProxy(ip) {
				return ip.String()
			}
		}
	}
	if real := parseIP(request.Header.Get("X-Real-IP")); real != nil {
		return real.String()
	}
	return remote.String()
}

// directLoopback 判断请求是否为本机直连，而不是反向代理转发的流量。
// 只有未被列为可信代理、且未携带转发头的回环来源才视为运维通道，
// 避免反代后的外部请求被误放行。
func (app *App) directLoopback(request *http.Request) bool {
	remote := parseIP(request.RemoteAddr)
	if remote == nil || !remote.IsLoopback() || app.trustedProxy(remote) {
		return false
	}
	return strings.TrimSpace(request.Header.Get("X-Forwarded-For")) == "" &&
		strings.TrimSpace(request.Header.Get("X-Real-IP")) == ""
}

// isHTTPS 判断外部连接是否为 HTTPS：直连看 TLS，代理后仅信任可信代理的 X-Forwarded-Proto。
func (app *App) isHTTPS(request *http.Request) bool {
	if request.TLS != nil {
		return true
	}
	remote := parseIP(request.RemoteAddr)
	if remote != nil && app.trustedProxy(remote) {
		// 取最右侧值（最近一跳可信代理写入的断言），避免客户端预置伪造值。
		parts := strings.Split(request.Header.Get("X-Forwarded-Proto"), ",")
		proto := strings.ToLower(strings.TrimSpace(parts[len(parts)-1]))
		return proto == "https"
	}
	return false
}

// 限流：同一时间窗口内每个键最多允许 maximum 次。
func (app *App) limitHit(key string, maximum int) bool {
	app.loginMu.Lock()
	defer app.loginMu.Unlock()
	cutoff := time.Now().Add(-attemptWindow)
	for existing := range app.attempts {
		app.pruneAttemptLocked(existing, cutoff)
	}
	if len(app.attempts[key]) >= maximum {
		return false
	}
	app.attempts[key] = append(app.attempts[key], time.Now())
	return true
}

func (app *App) limitExceeded(key string, maximum int) bool {
	app.loginMu.Lock()
	defer app.loginMu.Unlock()
	app.pruneAttemptLocked(key, time.Now().Add(-attemptWindow))
	return len(app.attempts[key]) >= maximum
}

// limitCount 返回当前窗口内某个键的尝试次数。
func (app *App) limitCount(key string) int {
	app.loginMu.Lock()
	defer app.loginMu.Unlock()
	app.pruneAttemptLocked(key, time.Now().Add(-attemptWindow))
	return len(app.attempts[key])
}

func (app *App) limitRecord(key string) {
	app.loginMu.Lock()
	defer app.loginMu.Unlock()
	app.pruneAttemptLocked(key, time.Now().Add(-attemptWindow))
	app.attempts[key] = append(app.attempts[key], time.Now())
}

func (app *App) limitReset(key string) {
	app.loginMu.Lock()
	defer app.loginMu.Unlock()
	delete(app.attempts, key)
}

func (app *App) pruneAttemptLocked(key string, cutoff time.Time) {
	times := app.attempts[key]
	valid := make([]time.Time, 0, len(times))
	for _, stamp := range times {
		if stamp.After(cutoff) {
			valid = append(valid, stamp)
		}
	}
	if len(valid) == 0 {
		delete(app.attempts, key)
		return
	}
	app.attempts[key] = valid
}

// blockedOutboundIP 判断出站目标是否属于禁止访问的地址段。
// 覆盖回环、私网、链路本地（含云元数据 169.254.169.254）、CGNAT 与保留段。
func blockedOutboundIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	for _, network := range blockedOutboundNets {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

var blockedOutboundNets = func() []*net.IPNet {
	result := []*net.IPNet{}
	for _, value := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "198.18.0.0/15", "240.0.0.0/4"} {
		if _, network, err := net.ParseCIDR(value); err == nil {
			result = append(result, network)
		}
	}
	return result
}()

// proxyAddress 判断地址是否为环境变量配置的 HTTP 代理；
// 代理由部署者显式设置，视为可信中转，允许连接到本机代理端口。
func proxyAddress(address string) bool {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		host = address
	}
	for _, key := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy"} {
		value := strings.TrimSpace(os.Getenv(key))
		if value == "" {
			continue
		}
		if parsed, err := url.Parse(value); err == nil && parsed.Host != "" && strings.EqualFold(parsed.Hostname(), host) {
			return true
		}
	}
	return false
}

// outboundAllowed 判断目标是否在显式出站白名单中（host 或 host:port）。
// 白名单只用于管理员明确信任的局域网地址（例如自建 webhook 或镜像），默认留空。
func (app *App) outboundAllowed(host, port string) bool {
	target := strings.ToLower(strings.TrimSpace(host))
	endpoint := strings.ToLower(net.JoinHostPort(host, port))
	for _, entry := range strings.Split(app.store.setting("outboundAllowlist", ""), ",") {
		entry = strings.ToLower(strings.TrimSpace(entry))
		if entry == "" {
			continue
		}
		if entry == target || entry == endpoint {
			return true
		}
	}
	return false
}

func validOutboundAllowlist(value string) bool {
	for _, entry := range strings.Split(value, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if strings.ContainsAny(entry, "/@?#") {
			return false
		}
		host := entry
		if parsedHost, _, err := net.SplitHostPort(entry); err == nil {
			host = parsedHost
		}
		if strings.TrimSpace(host) == "" {
			return false
		}
	}
	return true
}

func safeOutboundTransport(allowed func(host, port string) bool) *http.Transport {
	dialer := &net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			if proxyAddress(address) {
				return dialer.DialContext(ctx, network, address)
			}
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, errors.New("出站地址无效")
			}
			if allowed != nil && allowed(host, port) {
				return dialer.DialContext(ctx, network, address)
			}
			addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			for _, candidate := range addresses {
				if blockedOutboundIP(candidate.IP) {
					return nil, errors.New("已拒绝访问内网或保留地址")
				}
			}
			var lastErr error
			for _, candidate := range addresses {
				conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(candidate.IP.String(), port))
				if err == nil {
					return conn, nil
				}
				lastErr = err
			}
			if lastErr == nil {
				lastErr = errors.New("无法解析出站地址")
			}
			return nil, lastErr
		},
		ResponseHeaderTimeout: 60 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   20 * time.Second,
		ForceAttemptHTTP2:     true,
	}
}

func newOutboundClient(allowed func(host, port string) bool, timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: safeOutboundTransport(allowed),
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("重定向次数过多")
			}
			if request.URL.Scheme != "http" && request.URL.Scheme != "https" {
				return errors.New("重定向目标协议不受支持")
			}
			return nil
		},
	}
}
