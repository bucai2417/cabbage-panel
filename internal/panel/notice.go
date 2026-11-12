package panel

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// noticeSources 是公告内容的多个来源，按顺序尝试；任一成功即返回。
// GitHub 直链在国内常不可达，因此优先用 raw 与 jsDelivr 镜像。
var noticeSources = []string{
	"https://raw.githubusercontent.com/bucai2417/cabbage-panel/main/tz.json",
	"https://cdn.jsdelivr.net/gh/bucai2417/cabbage-panel@main/tz.json",
	"https://github.com/bucai2417/cabbage-panel/raw/refs/heads/main/tz.json",
}

// noticeCache 缓存公告，避免每次打开都请求外网。
var noticeCache struct {
	text    string
	fetched time.Time
	ok      bool
}

const noticeCacheTTL = 10 * time.Minute

// noticeAPI 返回远端公告内容；拉取失败时返回空文本而不是错误。
// 前端异步调用，不阻塞主界面。
func (app *App) noticeAPI(writer http.ResponseWriter, request *http.Request) {
	if request.Method != "GET" {
		fail(writer, 405, "方法不支持")
		return
	}
	if noticeCache.ok && time.Since(noticeCache.fetched) < noticeCacheTTL {
		ok(writer, Object{"text": noticeCache.text, "cached": true})
		return
	}
	text := app.fetchNotice(request.Context())
	if text != "" {
		noticeCache.text, noticeCache.fetched, noticeCache.ok = text, time.Now(), true
	}
	ok(writer, Object{"text": text, "cached": false})
}

// fetchNotice 并发请求所有来源，取最先成功的结果；全部失败返回空串。
// 并发而非串行，避免某个源超时拖慢整体响应。
func (app *App) fetchNotice(parent context.Context) string {
	type result struct{ text string }
	results := make(chan result, len(noticeSources))
	ctx, cancel := context.WithTimeout(parent, 6*time.Second)
	defer cancel()
	for _, source := range noticeSources {
		go func(source string) {
			results <- result{app.fetchNoticeFrom(ctx, source)}
		}(source)
	}
	for range noticeSources {
		select {
		case <-ctx.Done():
			return ""
		case value := <-results:
			if value.text != "" {
				return value.text
			}
		}
	}
	return ""
}

// fetchNoticeFrom 从单个来源拉取公告文本。
func (app *App) fetchNoticeFrom(ctx context.Context, source string) string {
	request, err := http.NewRequestWithContext(ctx, "GET", source, nil)
	if err != nil {
		return ""
	}
	response, err := app.client.Do(request)
	if err != nil {
		return ""
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return ""
	}
	var body struct {
		TZ string `json:"tz"`
	}
	if json.NewDecoder(response.Body).Decode(&body) != nil {
		return ""
	}
	return strings.TrimSpace(body.TZ)
}
