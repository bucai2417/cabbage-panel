package panel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
)

type Tool struct {
	Name, Description, Scope, Method, Path string
	Properties                             Object
	Required                               []string
	Array                                  bool
}

func tools() []Tool {
	stringField := func(description string) Object { return Object{"type": "string", "description": description} }
	idField := Object{"type": "integer", "minimum": 1}
	idsField := Object{"type": "array", "items": idField, "minItems": 1, "maxItems": 500}
	return []Tool{
		{"list_tasks", "列出定时任务及最近执行状态", "crons", "GET", "/crons", Object{"searchValue": stringField("按名称或命令搜索")}, []string{}, false},
		{"create_task", "创建支持五段或六段 Cron 的任务；schedule 为空时仅手动执行。命令统一用 task 前缀，如 task example.py / task example.js / task example.ts", "crons", "POST", "/crons", Object{"name": stringField("名称"), "command": stringField("命令，统一用 task example.py 形式"), "schedule": stringField("Cron 表达式"), "timeout": Object{"type": "integer", "minimum": 0}, "allowConcurrent": Object{"type": "boolean"}}, []string{"name", "command", "schedule"}, false},
		{"update_task", "修改已有定时任务", "crons", "PUT", "/crons", Object{"id": idField, "name": stringField("名称"), "command": stringField("命令"), "schedule": stringField("Cron 表达式"), "timeout": Object{"type": "integer", "minimum": 0}, "allowConcurrent": Object{"type": "boolean"}}, []string{"id"}, false},
		{"run_tasks", "异步运行任务，返回 runId，可用 read_log 查询", "crons", "PUT", "/crons/run", Object{"ids": idsField}, []string{"ids"}, true},
		{"stop_tasks", "停止任务及其子进程", "crons", "PUT", "/crons/stop", Object{"ids": idsField}, []string{"ids"}, true},
		{"enable_tasks", "启用任务定时调度", "crons", "PUT", "/crons/enable", Object{"ids": idsField}, []string{"ids"}, true},
		{"disable_tasks", "停用任务定时调度", "crons", "PUT", "/crons/disable", Object{"ids": idsField}, []string{"ids"}, true},
		{"delete_tasks", "永久删除任务定义", "crons", "DELETE", "/crons", Object{"ids": idsField}, []string{"ids"}, true},
		{"list_envs", "列出环境变量，值默认隐藏；includeValues=true 显示敏感值", "envs", "GET", "/envs", Object{"searchValue": stringField("搜索"), "includeValues": Object{"type": "boolean"}}, []string{}, false},
		{"create_envs", "批量新增环境变量", "envs", "POST", "/envs", Object{"items": Object{"type": "array", "items": Object{"type": "object", "properties": Object{"name": stringField("变量名"), "value": stringField("变量值"), "remarks": stringField("备注")}, "required": []string{"name", "value"}}, "minItems": 1}}, []string{"items"}, true},
		{"list_scripts", "列出脚本文件", "scripts", "GET", "/scripts", Object{}, []string{}, false},
		{"read_script", "读取 UTF-8 脚本文件", "scripts", "GET", "/scripts/detail", Object{"path": stringField("相对于脚本目录的完整文件名")}, []string{"path"}, false},
		{"write_script", "保存 UTF-8 脚本文件，已有内容会保存历史版本", "scripts", "PUT", "/scripts", Object{"filename": stringField("文件名或相对路径"), "content": stringField("UTF-8 内容")}, []string{"filename", "content"}, false},
		{"list_logs", "列出最近运行记录", "logs", "GET", "/logs", Object{"taskId": idField, "limit": Object{"type": "integer", "minimum": 1, "maximum": 1000}}, []string{}, false},
		{"read_log", "按 runId 获取日志与运行状态", "logs", "GET", "/logs/run/{id}", Object{"id": idField}, []string{"id"}, false},
		{"list_subscriptions", "列出订阅", "subscriptions", "GET", "/subscriptions", Object{}, []string{}, false},
		{"sync_subscriptions", "异步同步订阅", "subscriptions", "PUT", "/subscriptions/run", Object{"ids": idsField}, []string{"ids"}, true},
		{"list_dependencies", "列出已管理的依赖与安装状态", "dependencies", "GET", "/dependencies", Object{}, []string{}, false},
	}
}

func (app *App) mcp(writer http.ResponseWriter, request *http.Request, principal string) {
	if request.Method != "POST" {
		writer.Header().Set("Allow", "POST")
		fail(writer, 405, "MCP 使用无状态 Streamable HTTP POST")
		return
	}
	if !strings.HasPrefix(request.Header.Get("Content-Type"), "application/json") {
		fail(writer, 415, "需要 application/json")
		return
	}
	protocol := request.Header.Get("MCP-Protocol-Version")
	if protocol != "" && !contains([]string{"2024-11-05", "2025-03-26", "2025-06-18", "2025-11-25"}, protocol) {
		fail(writer, 400, "不支持的 MCP 协议版本")
		return
	}
	var message struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Method  string          `json:"method"`
		Params  Object          `json:"params"`
	}
	rpcError := func(id any, code int, message string) {
		respond(writer, 200, Object{"jsonrpc": "2.0", "id": id, "error": Object{"code": code, "message": message}})
	}
	if err := decode(request, &message); err != nil {
		rpcError(nil, -32700, "Parse error")
		return
	}
	if message.JSONRPC != "2.0" || message.Method == "" {
		rpcError(nil, -32600, "Invalid Request")
		return
	}
	if len(message.ID) == 0 {
		if strings.HasPrefix(message.Method, "notifications/") {
			writer.WriteHeader(202)
		} else {
			rpcError(nil, -32600, "Request id is required")
		}
		return
	}
	var id any
	if err := json.Unmarshal(message.ID, &id); err != nil {
		rpcError(nil, -32600, "Invalid id")
		return
	}
	switch id.(type) {
	case string, float64:
	default:
		rpcError(nil, -32600, "Invalid id")
		return
	}
	var result any
	switch message.Method {
	case "initialize":
		version := text(message.Params["protocolVersion"])
		if !contains([]string{"2024-11-05", "2025-03-26", "2025-06-18", "2025-11-25"}, version) {
			version = "2025-11-25"
		}
		result = Object{"protocolVersion": version, "capabilities": Object{"tools": Object{"listChanged": false}}, "serverInfo": Object{"name": "cabbage-panel", "version": app.version}, "instructions": "大白菜面板：执行命令会访问主机资源。仅在用户授权后调用写入、执行和删除工具。"}
	case "ping":
		result = Object{}
	case "tools/list":
		items := []Object{}
		for _, tool := range tools() {
			if !app.allowed(principal, tool.Scope) {
				continue
			}
			items = append(items, Object{"name": tool.Name, "description": tool.Description, "inputSchema": Object{"type": "object", "properties": tool.Properties, "required": tool.Required, "additionalProperties": false}, "annotations": Object{"readOnlyHint": tool.Method == "GET", "destructiveHint": tool.Method == "DELETE", "openWorldHint": tool.Method != "GET"}})
		}
		result = Object{"tools": items}
	case "tools/call":
		name := text(message.Params["name"])
		arguments, valid := message.Params["arguments"].(map[string]any)
		if message.Params["arguments"] == nil {
			arguments = map[string]any{}
			valid = true
		}
		if !valid {
			rpcError(id, -32602, "arguments 必须是对象")
			return
		}
		var selected *Tool
		for _, tool := range tools() {
			if tool.Name == name {
				copy := tool
				selected = &copy
				break
			}
		}
		if selected == nil {
			rpcError(id, -32602, "未知工具")
			return
		}
		if !app.allowed(principal, selected.Scope) {
			rpcError(id, -32602, "应用没有此工具的权限")
			return
		}
		for _, key := range selected.Required {
			if _, found := arguments[key]; !found {
				rpcError(id, -32602, "缺少参数 "+key)
				return
			}
		}
		for key := range arguments {
			if _, found := selected.Properties[key]; !found {
				rpcError(id, -32602, "未知参数 "+key)
				return
			}
		}
		apiPath := strings.ReplaceAll(selected.Path, "{id}", text(arguments["id"]))
		values := url.Values{}
		var body any = arguments
		if selected.Array {
			body = arguments["ids"]
			if name == "create_envs" {
				body = arguments["items"]
			}
		}
		if selected.Method == "GET" {
			for key, value := range arguments {
				values.Set(key, text(value))
			}
			apiPath += "?" + values.Encode()
		}
		encoded, _ := json.Marshal(body)
		internal := httptest.NewRequest(selected.Method, apiPath, bytes.NewReader(encoded))
		internal = internal.WithContext(request.Context())
		recorder := httptest.NewRecorder()
		app.gate.RLock()
		if selected.Method != "GET" {
			app.writeMu.Lock()
		}
		app.api(recorder, internal, internal.URL.Path, principal)
		if selected.Method != "GET" {
			app.writeMu.Unlock()
		}
		app.gate.RUnlock()
		var response Object
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			rpcError(id, -32603, "内部返回格式错误")
			return
		}
		if name == "list_envs" && !boolean(arguments["includeValues"]) {
			if items, valid := response["data"].([]any); valid {
				for _, value := range items {
					if item, valid := value.(map[string]any); valid {
						item["value"] = "••••••••"
					}
				}
			}
		}
		data, _ := json.Marshal(response)
		result = Object{"content": []Object{{"type": "text", "text": string(data)}}, "structuredContent": response, "isError": recorder.Code >= 400}
	case "resources/list":
		result = Object{"resources": []any{}}
	case "prompts/list":
		result = Object{"prompts": []any{}}
	default:
		rpcError(id, -32601, fmt.Sprintf("Method not found: %s", message.Method))
		return
	}
	respond(writer, 200, Object{"jsonrpc": "2.0", "id": id, "result": result})
}
