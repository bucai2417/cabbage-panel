package panel

import (
	"net/url"
	"strings"
)

// githubHosts 是 GitHub 页面域名。gist.github.com 不在此列——gist 的原始文件
// 地址规则不同，保持原样交给用户自己填。
var githubHosts = map[string]bool{
	"github.com":     true,
	"www.github.com": true,
}

// normalizeDownloadURL 把 GitHub blob / raw 页面地址转换为 raw 原始文件地址。
//
// 用户从浏览器地址栏复制的一般是 blob 页面：
//
//	https://github.com/owner/repo/blob/main/path/file.js
//
// 这个地址返回的是 HTML 页面（实测约 610KB），不是脚本内容。对应的原始文件是：
//
//	https://raw.githubusercontent.com/owner/repo/main/path/file.js
//
// 返回转换后的地址，第二个返回值表示是否发生了转换。非 GitHub 地址、路径不匹配
// 时原样返回，调用方据此决定要不要做额外的 HTML 检查。
func normalizeDownloadURL(address string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(address))
	if err != nil || !githubHosts[strings.ToLower(parsed.Hostname())] {
		return address, false
	}

	// 路径形如 /owner/repo/blob/<ref 与文件路径>。ref 可能自带斜杠（feature/x），
	// 因此不拆分 ref 与文件路径，整段原样搬到 raw 地址，由 GitHub 自行最长匹配。
	// 用 EscapedPath 取原始转义形式，再 PathUnescape 得到解码值交给 url.URL，
	// 由 String() 统一重新编码，避免中文文件名被二次编码。
	segments := strings.SplitN(strings.TrimPrefix(parsed.EscapedPath(), "/"), "/", 4)
	if len(segments) < 4 || (segments[2] != "blob" && segments[2] != "raw") {
		return address, false
	}
	tail, err := url.PathUnescape(segments[3])
	// 只剩分支名（如 /blob/main）说明没有文件路径；分支名可含斜杠，无法可靠
	// 区分「斜杠分支」与「分支+文件」，因此只有单段时才判定为无效。
	if err != nil || tail == "" || !strings.Contains(tail, "/") {
		return address, false
	}

	converted := &url.URL{
		Scheme: "https",
		Host:   "raw.githubusercontent.com",
		Path:   "/" + segments[0] + "/" + segments[1] + "/" + tail,
	}
	// blob 页面的 ?plain=1 之类参数对 raw 地址无意义，直接丢弃；fragment 同理。
	return converted.String(), true
}
