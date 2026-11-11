package panel

import (
	"strings"
)

// dependencyCompat 对齐青龙依赖项字段：id、name、type、timestamp、status、log（字符串数组）、
// remark、createdAt、updatedAt。移除面板内部字段。
// type：0 npm、1 Python、2 Linux。status：0 安装成功、1 等待或安装中、2 未就绪。
func dependencyCompat(item Object, log []string) Object {
	for _, key := range []string{"_id", "isDisabled", "isPinned", "labels", "lastStatus", "remove"} {
		delete(item, key)
	}
	item["type"] = integer(item["type"])
	item["status"] = integer(item["status"])
	item["remark"] = text(item["remark"])
	item["timestamp"] = compatTimestamp(item)
	if log == nil {
		log = []string{}
	}
	item["log"] = log
	return item
}

// dependencyLog 取最近一次安装日志，并按青龙习惯切成字符串数组：
// 每段以「开始安装」开头、以「结束时间」结尾，中间保留原文。
func (app *App) dependencyLog(id int64) []string {
	content := app.latestLog("dependencies", id)
	if content == "" {
		return []string{}
	}
	lines := strings.Split(content, "\n")
	segments := []string{}
	current := strings.Builder{}
	flush := func() {
		text := current.String()
		if strings.TrimSpace(text) != "" {
			segments = append(segments, text)
		}
		current.Reset()
	}
	for _, line := range lines {
		// 面板日志头部与尾部是状态行，青龙的 log 只保留安装输出。
		if strings.HasPrefix(line, "大白菜面板 · ") || strings.HasPrefix(line, "开始时间: ") || strings.HasPrefix(line, "结束状态: ") {
			continue
		}
		if strings.HasPrefix(line, "错误: ") {
			current.WriteString(line)
			current.WriteString("\n")
			flush()
			continue
		}
		current.WriteString(line)
		current.WriteString("\n")
	}
	flush()
	if len(segments) == 0 {
		return []string{content}
	}
	return segments
}
