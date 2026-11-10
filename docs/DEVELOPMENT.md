# 开发与 Agent 接手指南

基线：v1.1.0，记录日期：2026-11-10。此文记录已实现的工程与实际验证边界；接手后先检查源码和工作区现状，不把本文当作尚未完成的任务清单。

## 接手入口

让 Agent 打开包含 go.mod、web/ 和 internal/ 的源码根目录，先读根目录 AGENTS.md，再读本文。发行 ZIP 用于运行程序，不能代替源码工程。

| 文档 | 内容 |
| --- | --- |
| [README.md](../README.md) | 项目功能、启动、同端口 API/MCP、备份 |
| [BUILD.md](BUILD.md) | 工具版本、本地编译、多平台打包、版本号与产物 |
| [WORKSPACE.md](WORKSPACE.md) | 页面功能、编辑器快捷键、系统配置与通知助手 |
| [API.md](API.md) | 接口、权限、青龙兼容边界与扩展接口 |
| [TESTING.md](TESTING.md) | 已完成的行为验证与目标环境限制 |
| [RELEASE.md](RELEASE.md) | 正式签名、GitHub Release、在线更新与回滚 |

用户目标是轻量、自托管的大白菜面板：Go 后端、Vue 3 前端、SQLite，网页、/api、/open、/mcp 在生产环境共用 5700，支持 Python/JS/Shell、异步并发任务、备份导入和 Release 在线更新。保持现有技术栈、接口兼容范围及中文路径支持。

计划公开仓库为 bucai2417/cabbage-panel，目前未授权发布。后续没有明确要求时，不提交、推送、打 tag、发布 Release 或生成正式发布私钥。不要把本机运行数据、账户、脚本或备份带入源码交接包。

## 当前完成状态

- 配置页提供 system.json、notifications.json、config.sh、task_before.sh、task_after.sh；两个 JSON 是 SQLite 设置的虚拟文件视图，保存有校验并立即影响实际配置。
- 内置通知支持八类渠道、多渠道并发、签名和逐渠道测试；Python/JS/Shell 通知助手通过环境注入供任务使用。
- 脚本管理有完整目录树、空文件夹、新建目录、行菜单、多选、移动/复制/重命名/删除/下载和直接运行；不再从脚本页创建定时任务。
- 移动脚本会联动可识别的直接调用任务命令及编辑历史；冲突、路径越界、符号链接、自目录移动会被拒绝。无法安全改写的复杂命令引用会阻止移动。
- CodeMirror 6 提供高亮、补全、代码折叠、缩进辅助线、Tab/Shift+Tab、撤销/重做、面板内查找替换、跳转行、换行和未保存保护。
- 脚本调试为全屏分屏工作区：左侧代码、右侧日志，可直接运行未保存内容；环境变量以弹窗形式管理，调试临时文件在任务结束后清理。
- 脚本直接运行使用独立 scripts 运行记录，不再自动创建“手动运行”定时任务；任务列表中 `task xx` 命令区域可直接打开源码编辑。
- 任务名称打开页内任务详情：首屏为任务概览与执行概览并排，底部嵌入式日志最高 500px 滚动；源码编辑作为详情中的明确操作。
- 运行日志按任务名称两级展示，第一层为任务分组，第二层直接查看运行记录和页内日志；环境变量列表显示 #ID、默认变量值和最后修改时间。
- 脚本编辑器和调试工作区支持停止运行；调试失败时按日志解析错误行并在编辑器对应行显示红色诊断，退出未保存调试内容时先确认且不回写脚本编辑器。
- 任务列表按启用状态分组、置顶优先、创建时间新到旧；环境变量新建在末尾并支持置顶和状态筛选；订阅、依赖、应用和备份按创建时间新到旧。
- 删除、恢复、导入和更新等高风险操作使用统一确认弹窗并列出影响范围；页面、文件、配置页签切换和全屏编辑器关闭时的未保存修改由同一弹窗拦截。
- 后端调度、权限、订阅、依赖、备份恢复、签名更新和 MCP 已有实现及测试；具体能力以 API 和使用文档为准。

## 代码导航

| 文件或目录 | 主要职责 |
| --- | --- |
| cmd/cabbage/main.go | 启动参数、版本、公钥注入 |
| internal/panel/app.go | HTTP 路由、鉴权、权限、同端口服务及应用锁 |
| internal/panel/store.go | SQLite、记录/设置/会话/运行记录、路径安全 |
| internal/panel/api.go | 资源 CRUD、任务批量操作和运行历史 |
| internal/panel/engine.go | Cron、队列、并发、取消/超时、运行环境和钩子 |
| internal/panel/files.go、file_ops.go | 文本/历史/日志、文件树、目录操作和任务路径联动 |
| internal/panel/configuration.go | 系统设置校验、虚拟配置文件和 Shell 配置 |
| internal/panel/notifications.go、notification_helpers.go | 通知渠道、签名、助手模块和通知专用接口 |
| internal/panel/services.go | 系统监控、订阅和依赖管理 |
| internal/panel/backup.go、update.go、mcp.go | 备份恢复、签名更新、MCP 工具 |
| internal/panel/process_*.go、lock_*.go | 平台进程管理和数据目录排他锁 |
| web/embed.go | 编译时嵌入 web/dist |
| web/src/App.vue、api.js、guard.js、dialogs.js | 登录、导航、异步离开守卫、统一确认弹窗、请求/轮询/提示 |
| web/src/pages/ | 仪表盘、任务、脚本/配置、资源、日志、设置 |
| web/src/components/ | CodeEditor、FileEditor、TaskDetail、TaskEditorDialog、ScriptDebugDialog、EnvironmentDialog、ConfirmDialog、通知设置、日志等 |
| web/tests/panel.spec.js | 独立实例的桌面、工作区、手机端、删除确认、未保存拦截和日志分组测试 |

CSS 加载顺序为 style.css → panels.css → responsive.css → workspace.css → ui.css。workspace.css 包含紧凑布局和编辑器样式，ui.css 包含全局确认弹窗、按钮变体和滚动条等收尾样式。调整样式前先核对覆盖关系，避免新旧规则相互抵消。

## 独立开发程序

已存在的 5700 服务与 data/ 属于用户工作环境。先核对正在运行的程序、端口和数据目录，不假设上一次的进程号仍有效。根目录 cabbage.exe 是早期产物，不作为当前版本入口。

Windows 开发可编译到 .tools/cabbage-dev.exe，并用 .tools/dev-data 与 15701 端口隔离。以下在源码根目录的新 PowerShell 终端执行，需 Go/npm 已在 PATH；本机构建脚本另有 D:\Go\bin 的自动查找逻辑。

~~~powershell
New-Item -ItemType Directory -Force -Path '.tools/gopath', '.tools/gocache', '.tools/tmp', '.tools/npm-cache' | Out-Null
$env:GOPATH = Join-Path $PWD.Path '.tools/gopath'
$env:GOCACHE = Join-Path $PWD.Path '.tools/gocache'
$env:GOTMPDIR = Join-Path $PWD.Path '.tools/tmp'
$env:TEMP = $env:GOTMPDIR
$env:TMP = $env:GOTMPDIR
$env:npm_config_cache = Join-Path $PWD.Path '.tools/npm-cache'
npm.cmd --prefix web ci
if ($LASTEXITCODE -ne 0) { throw 'npm ci failed' }
npm.cmd --prefix web run build
if ($LASTEXITCODE -ne 0) { throw 'frontend build failed' }
$env:CGO_ENABLED = '0'
$env:GOOS = 'windows'
$env:GOARCH = 'amd64'
$devVersion = (Get-Content -LiteralPath 'VERSION' -Raw -Encoding UTF8).Trim()
go build -trimpath -buildvcs=false -ldflags "-s -w -X main.version=$devVersion" -o .tools/cabbage-dev.exe ./cmd/cabbage
if ($LASTEXITCODE -ne 0) { throw 'Go build failed' }
.\.tools\cabbage-dev.exe --data .\.tools\dev-data --addr 127.0.0.1:15701
~~~

这会启动包含生产前端的独立实例。初始化自己的开发账号，不复制真实账户或密钥。再次编译同名开发程序前，只停止自己的开发实例。

需要前端热更新时运行 npm --prefix web run dev。web/vite.config.js 通过 CABBAGE_DEV_API 指定 /api、/open、/mcp 的代理目标，默认 5700；若开发后端使用 15701，先设置 `$env:CABBAGE_DEV_API = 'http://127.0.0.1:15701'`，避免通过开发页面修改真实服务数据。开发端口与生产环境的同端口部署要求无冲突。

生产程序通过 go:embed 把 web/dist 嵌入可执行文件，Playwright 测试启动的是编译后的程序，所以涉及前端资源的自动化测试需要先构建 web 再编译 Go。只改前端时可以用两种更快的本地方式：

- 热更新：设置 CABBAGE_DEV_API 后运行 npm --prefix web run dev，Vite 直接服务 .vue 源码，不需要构建 web，也不需要重编译 Go。
- 单端口预览：执行 npm --prefix web run build 后，用 `--web web/dist` 启动已有开发程序，例如 `.tools\cabbage-dev.exe --data .tools\dev-data --addr 127.0.0.1:15701 --web web/dist`；只重新构建前端，不重编译 Go。`--web` 仅用于开发，正式发布仍使用内嵌资源。

## 必须保留的行为

- 所有文本文档、脚本和配置用 UTF-8；PowerShell 读写中文明确指定编码。不能用替换字符掩盖解码失败。
- 文件路径始终通过现有安全检查，不绕过根目录限制和符号链接拒绝逻辑。
- gate 协调备份/恢复与普通操作及任务，writeMu 串行化 API 写操作，scriptMu 协调脚本移动/删除与入队，depMu 保护依赖安装，helperMu 保护通知监听地址；改动前检查各调用点和锁顺序。
- 不把任务进程、依赖安装、订阅同步或更新下载改成阻塞 HTTP 请求的长操作；保留队列上限、取消、超时和进程清理。
- 脚本移动/删除必须继续阻止与排队或运行任务的竞态，并维护任务参数和编辑历史。
- 系统配置与 SQLite 设置共用同一数据源，不另外创建脱节的 system.json/notifications.json 实体文件。
- 编辑器缩进使用独立历史分组；更改快捷键时验证 Tab、Shift+Tab、连续撤销、重做、保存和关闭前提示。
- 资源 API、开放应用与 MCP 必须继续执行相应权限校验，通知助手专用凭证不能获得普通管理权限。
- 数据恢复后要撤销会话；签名更新不得接受未签名或校验失败的程序。数据库变更需考虑旧数据迁移及恢复路径。

## 验证方式

变更相关逻辑后运行适当的行为测试。前端构建成功后，后端基础检查为：

~~~sh
go test ./... -count=1 -timeout 180s
go vet ./...
~~~

并发或锁逻辑改动应运行 go test -race ./... -count=1 -timeout 180s。Windows 需 C 编译器及 CGO_ENABLED=1；部分 MinGW 不支持中文临时目录，GOTMPDIR、TEMP、TMP 应指向已创建的英文目录，如 D:/cabbage-panel-build-cache/tmp。正式跨平台构建仍使用 CGO_ENABLED=0；不要沿用交叉编译的 GOOS/GOARCH 来运行本机测试。

浏览器测试默认寻找 dist/cabbage-windows-amd64.exe。用上面的独立开发程序时，在另一个终端执行：

~~~powershell
$env:CABBAGE_TEST_BINARY = Join-Path $PWD.Path '.tools/cabbage-dev.exe'
npm.cmd --prefix web run test:e2e
~~~

测试自行启动 127.0.0.1:15700 和独立临时数据目录，不要预先在该端口启动测试服务。Windows 使用本机 Edge；其他平台需安装 Playwright Chromium 并设置对应平台 CABBAGE_TEST_BINARY。测试截图保存在 test-results/。

只调整前端时，可额外设置 `$env:CABBAGE_TEST_WEB_DIR = Join-Path $PWD.Path 'web/dist'`，Playwright 会给测试程序加 `--web` 并从磁盘读取最新前端；此时只需执行 npm 前端构建，不必重新编译 Go。测试程序需要包含 `--web` 支持；不设置该变量时继续使用程序内嵌前端。

后端现有测试包括 panel_test.go、subscription_test.go、dependency_test.go、update_test.go 和 workspace_test.go，后者覆盖中文文件操作、配置、通知协议与三种脚本助手。详细已验证项见 [TESTING.md](TESTING.md)。不要把历史通过记录当作新修改后的验证结果。

## 已知边界

- 编辑器补全为关键词、片段和当前文档符号，尚未接入 Python/JS 语言服务器，不提供完整跨文件语义分析。
- 青龙兼容范围以 API.md 为准，未承诺全部私有接口或直接导入青龙完整备份。
- 通知通过本地 HTTP 替身与实际脚本助手测试，未使用用户真实厂商凭据发送通知。
- Docker 尚未本机运行验收；Linux/macOS 已交叉编译，但尚未在目标操作系统执行。
- 当前本地程序无正式更新公钥，尚未对真实 GitHub Release 完成在线升级演练。发布前需由所有者配置签名身份，再验证更新与回滚。
- 仓库尚未指定开源许可证，由所有者在公开发布前选择；不要自行添加许可证。

## 可复制的接手提示

> 请在当前大白菜面板源码目录继续开发。先阅读 AGENTS.md、docs/DEVELOPMENT.md、docs/BUILD.md 和 docs/API.md，核对现有实现与未提交改动，再完成我接下来给出的需求。保持 Go + Vue 3 + SQLite、生产环境同端口 5700、现有接口和 UTF-8 中文支持。用独立数据目录进行测试，保护现有账户与数据；未经明确要求不要提交、推送、打 tag 或发布 Release。完成后说明改动文件、验证结果和剩余限制。
