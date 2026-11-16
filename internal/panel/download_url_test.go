package panel

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestNormalizeDownloadURL(test *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    string
		changed bool
	}{
		{
			name:    "blob 转 raw",
			input:   "https://github.com/owner/repo/blob/main/script.js",
			want:    "https://raw.githubusercontent.com/owner/repo/main/script.js",
			changed: true,
		},
		{
			name:    "github.com/raw 短链",
			input:   "https://github.com/owner/repo/raw/main/script.js",
			want:    "https://raw.githubusercontent.com/owner/repo/main/script.js",
			changed: true,
		},
		{
			name:    "www 前缀",
			input:   "https://www.github.com/owner/repo/blob/main/a.py",
			want:    "https://raw.githubusercontent.com/owner/repo/main/a.py",
			changed: true,
		},
		{
			name:    "http 输入统一升级为 https",
			input:   "http://github.com/owner/repo/blob/main/a.py",
			want:    "https://raw.githubusercontent.com/owner/repo/main/a.py",
			changed: true,
		},
		{
			name:    "斜杠分支名整段搬运",
			input:   "https://github.com/vuejs/core/blob/chore/eslint-imports/README.md",
			want:    "https://raw.githubusercontent.com/vuejs/core/chore/eslint-imports/README.md",
			changed: true,
		},
		{
			name:    "丢弃 blob 页面的 plain 参数",
			input:   "https://github.com/owner/repo/blob/main/a.js?plain=1",
			want:    "https://raw.githubusercontent.com/owner/repo/main/a.js",
			changed: true,
		},
		{
			name:    "丢弃行号 fragment",
			input:   "https://github.com/owner/repo/blob/main/a.js#L10",
			want:    "https://raw.githubusercontent.com/owner/repo/main/a.js",
			changed: true,
		},
		{
			name:    "中文路径只编码一次",
			input:   "https://github.com/owner/repo/blob/main/工具/每日.js",
			want:    "https://raw.githubusercontent.com/owner/repo/main/%E5%B7%A5%E5%85%B7/%E6%AF%8F%E6%97%A5.js",
			changed: true,
		},
		{
			name:    "raw.githubusercontent.com 原样返回",
			input:   "https://raw.githubusercontent.com/owner/repo/main/a.js",
			want:    "https://raw.githubusercontent.com/owner/repo/main/a.js",
			changed: false,
		},
		{
			name:    "非 GitHub 域名原样返回",
			input:   "https://gitlab.com/owner/repo/-/blob/main/a.js",
			want:    "https://gitlab.com/owner/repo/-/blob/main/a.js",
			changed: false,
		},
		{
			name:    "普通直链原样返回",
			input:   "https://example.com/script.py",
			want:    "https://example.com/script.py",
			changed: false,
		},
		{
			name:    "blob 后缺文件路径",
			input:   "https://github.com/owner/repo/blob/main",
			want:    "https://github.com/owner/repo/blob/main",
			changed: false,
		},
		{
			name:    "仓库根目录页不匹配",
			input:   "https://github.com/owner/repo",
			want:    "https://github.com/owner/repo",
			changed: false,
		},
		{
			name:    "gist 不处理",
			input:   "https://gist.github.com/user/abc123",
			want:    "https://gist.github.com/user/abc123",
			changed: false,
		},
	}

	for _, sample := range cases {
		test.Run(sample.name, func(test *testing.T) {
			got, changed := normalizeDownloadURL(sample.input)
			if got != sample.want {
				test.Fatalf("地址不符\n输入: %s\n期望: %s\n实际: %s", sample.input, sample.want, got)
			}
			if changed != sample.changed {
				test.Fatalf("转换标记不符：期望 %v，实际 %v", sample.changed, changed)
			}
		})
	}
}

// stubClient 让 downloadText 走本地假响应，不发起真实网络请求。
func stubClient(test *testing.T, panel *testPanel, handler func(*http.Request) (*http.Response, error)) {
	test.Helper()
	panel.app.client.Transport = fixtureTransport(handler)
}

func textResponse(request *http.Request, contentType, body string) *http.Response {
	return &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{contentType}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    request,
	}
}

func TestDownloadTextConvertsGitHubBlob(test *testing.T) {
	panel := setupPanel(test)
	var requested string
	stubClient(test, panel, func(request *http.Request) (*http.Response, error) {
		requested = request.URL.String()
		return textResponse(request, "text/plain; charset=utf-8", "print('ok')\n"), nil
	})

	content, err := panel.app.downloadText(test.Context(), "https://github.com/owner/repo/blob/main/a.py", 4<<20)
	if err != nil {
		test.Fatal(err)
	}
	if content != "print('ok')\n" {
		test.Fatalf("内容不符: %q", content)
	}
	if requested != "https://raw.githubusercontent.com/owner/repo/main/a.py" {
		test.Fatalf("请求地址未转换: %s", requested)
	}
}

func TestDownloadTextRejectsHTMLForConvertedBlob(test *testing.T) {
	panel := setupPanel(test)
	stubClient(test, panel, func(request *http.Request) (*http.Response, error) {
		return textResponse(request, "text/html; charset=utf-8", "<html></html>"), nil
	})

	_, err := panel.app.downloadText(test.Context(), "https://github.com/owner/repo/blob/main/missing.py", 4<<20)
	if err == nil {
		test.Fatal("blob 链接返回网页时应当报错")
	}
	if !strings.Contains(err.Error(), "网页") {
		test.Fatalf("错误信息不符: %v", err)
	}
}

func TestDownloadTextKeepsHTMLForNonGitHub(test *testing.T) {
	panel := setupPanel(test)
	stubClient(test, panel, func(request *http.Request) (*http.Response, error) {
		return textResponse(request, "text/html; charset=utf-8", "<html>hello</html>"), nil
	})

	// 非 GitHub 站点返回 text/html 时保持原有行为，不额外拦截。
	content, err := panel.app.downloadText(test.Context(), "https://example.com/a.js", 4<<20)
	if err != nil {
		test.Fatal(err)
	}
	if content != "<html>hello</html>" {
		test.Fatalf("内容不符: %q", content)
	}
}

func TestRemoteDownloadFromBlobURL(test *testing.T) {
	panel := setupPanel(test)
	stubClient(test, panel, func(request *http.Request) (*http.Response, error) {
		return textResponse(request, "text/plain; charset=utf-8", "#!/usr/bin/env node\nconsole.log(1)\n"), nil
	})

	status, response := panel.call(test, "POST", "/api/scripts/download",
		Object{"url": "https://github.com/owner/repo/blob/main/remote.js", "filename": "remote.js"}, panel.token)
	if status != 200 {
		test.Fatalf("创建失败 %d: %v", status, response)
	}
	detail := panel.data(test, "GET", "/api/scripts/detail?path=remote.js", nil)
	if !strings.Contains(text(detail), "console.log(1)") {
		test.Fatalf("落盘内容不符: %q", text(detail))
	}
}

func TestFileSubscriptionAcceptsBlobURL(test *testing.T) {
	panel := setupPanel(test)
	stubClient(test, panel, func(request *http.Request) (*http.Response, error) {
		return textResponse(request, "text/plain; charset=utf-8", "# cron: 0 8 * * *\nprint('同步成功')\n"), nil
	})

	item := panel.data(test, "POST", "/api/subscriptions", Object{
		"name": "blob 订阅", "type": "file",
		"url":   "https://github.com/owner/repo/blob/main/daily.py",
		"alias": "blobdaily", "schedule": "", "autoAddCron": true,
	}).(map[string]any)
	result := panel.data(test, "PUT", "/api/subscriptions/run", []int64{integer(item["id"])}).([]any)
	runID := integer(result[0].(map[string]any)["runId"])
	if status := panel.finished(test, runID)["status"]; status != "success" {
		test.Fatalf("同步失败: %v\n%s", status, panel.app.latestLog("subscriptions", integer(item["id"])))
	}
	items, _ := panel.app.store.list("crons")
	if len(items) != 1 || text(items[0]["schedule"]) != "0 8 * * *" {
		test.Fatal(items)
	}
}
