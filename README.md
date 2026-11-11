# 大白菜面板

轻量、自托管的自动化任务面板。Go 后端、Vue 3 前端、SQLite 数据库，前端静态资源嵌入单个程序。默认端口 **5700**，网页、管理 API、开放 API 和 MCP 共用同一端口。

当前版本：**1.1.0**。

文档入口：[使用指南](docs/WORKSPACE.md) · [构建与打包](docs/BUILD.md) · [开发 / Agent 接手](docs/DEVELOPMENT.md) · [发布与在线更新](docs/RELEASE.md)。其他 Agent 请打开源码根目录，从 [AGENTS.md](AGENTS.md) 开始；发行 ZIP 用于运行，不包含开发源码。

## 功能

- 定时任务：五段 / 六段 Cron、@every、手动执行、启停、批量操作、标签、置顶、搜索和分页；启用任务在上、禁用任务单独排下方，新建任务优先展示。
- 异步执行：最多 32 个并发任务、256 个在途任务上限；默认避免同任务重叠，可独立开启；支持超时、停止和子进程清理。
- 脚本：Python、JavaScript、Shell；完整文件夹树、新建文件夹、行菜单、多选、移动/复制/重命名、直接运行和全屏调试工作区。直接运行使用独立运行记录，不会创建定时任务；运行中可停止。
- 编辑器：CodeMirror 6，语法高亮、代码折叠、括号匹配、缩进辅助线、自动补全、撤销/重做、面板内查找替换（大小写、整词、正则、全部替换）、跳转行、自动换行和保存历史。调试工作区左侧编辑、右侧日志，修改无需保存即可运行，错误会在对应行显示红色诊断，并可在弹窗中管理环境变量。
- 内置配置：system.json、notifications.json、config.sh、任务前后置钩子；设置页面与脚本管理的配置文件页共用 SQLite 中的真实设置。
- 日志：按任务名称两级展示，运行记录与日志在同页查看；支持历史运行切换、内容筛选、完整日志下载、批量删除、退出码、耗时、实时读取、SSE 流、保留周期和自动清理；单次输出上限 20MB。
- 环境变量：增删改查、排序、启停、备注、标签、置顶和状态筛选；默认显示变量值，列表展示 ID 标识和最后修改时间，新建变量排在末尾，运行时注入进程。
- 订阅：HTTP(S) Git 仓库 / 单文件、分支、Cron 同步、正则白黑名单、自动识别 cron: 注释并创建任务。
- 依赖：Node.js npm、Python 独立 venv、Linux apt/apk；后台安装、卸载、重装和日志。
- 监控：真实 CPU、内存、磁盘、面板堆内存、执行统计和最近运行任务。
- 账户与应用：首次设置密钥、bcrypt、HttpOnly Cookie、应用权限与应用有效期、可在列表中直接查看的 Client Secret、可设置有效期或永久并持久登记的 Authorization（与 Client ID 用途分离）、与青龙 `login-log` 形状一致的登录日志（成功/失败、IP、设备，最多 100 条）；MCP 配置只使用 Authorization。
- 通知：企业微信、钉钉、飞书、Bark、Telegram、PushPlus、Server酱 SCT、通用 Webhook；支持多渠道并发、签名、逐渠道测试、失败/超时通知，以及 Python/JS/Shell 通知助手。
- 备份：ZIP 导出 / 导入，恢复前自动备份，数据库事务和文件目录回滚；拒绝路径穿越与符号链接。
- MCP：POST /mcp，无状态 Streamable HTTP，18 个工具，复用应用和资源权限。
- 更新：GitHub Release 下载、Ed25519 签名和 SHA-256 校验、程序替换、旧版本保留。

页面与配置操作说明见 [使用指南](docs/WORKSPACE.md)。默认更新仓库为 `bucai2417/cabbage-panel`，本地构建不自动发布任何内容。

项目独立实现，不包含青龙源码。核心开放 API 对齐青龙的路径、方法、数字 ID 和响应结构；具体范围见 [接口文档](docs/API.md)，不宣称全部历史版本内部接口逐项兼容。

## 直接运行

Windows 需要 Python 3、Node.js 和 Git for Windows。程序自动寻找 C:/Program Files/Git/bin/bash.exe，也可设置 CABBAGE_BASH 环境变量指定 Bash。

    .\dist\cabbage-windows-amd64.exe --data .\data --addr :5700

Linux / macOS 安装 Python 3（含 venv）、Node.js、Bash 和 Git，将对应 Release 程序保存为 cabbage：

    chmod +x cabbage
    ./cabbage --data ./data --addr :5700

访问 http://localhost:5700。首次启动生成 data/setup.key 并在控制台显示设置密钥，输入密钥创建管理员。密码至少 12 字节，无默认账号密码；初始化后删除设置密钥文件。

--version 输出版本。--addr 127.0.0.1:5700 限制本机访问。公网部署使用 HTTPS 反向代理，保留原始 Host，API 和 /mcp 不缓存。

## Docker

    docker compose up -d --build
    docker compose logs cabbage

镜像包含 Python、Node.js、Bash、Git、pip、npm；数据使用命名卷，默认使用非 root 用户。Python / Node 依赖可在面板安装。系统包在自定义 Dockerfile 预装；默认容器和 systemd 服务不提权安装系统包。

容器通过更新镜像并重新创建容器升级。面板内程序替换用于原生发行包。不要挂载 Docker socket。

## 源码构建

需要 Go **1.26+**、Node.js **22.12+**、npm。依赖由 go.mod/go.sum 和 web/package-lock.json 锁定。

Windows：

    powershell -ExecutionPolicy Bypass -File .\scripts\build.ps1

脚本把 Go/npm 缓存放入项目 .tools 目录，避免占用系统盘。网络需要时，在当前终端设置 GOPROXY=https://goproxy.cn,direct，不修改全局配置。

Linux / macOS：

    bash scripts/build.sh

输出在 dist/。使用 CGO_ENABLED=0、-trimpath、-s -w，前端不生成 source map。运行面板本身不需要 Node，但 JS 脚本需要 Node。

前端开发执行 npm --prefix web run dev，Vite 代理 API 到 5700。开发端口仅用于开发，发行包全部服务同端口。

## 脚本示例

在脚本管理保存 hello.py：

    import os
    print("你好，大白菜！")
    print(os.environ.get("MY_SETTING", "未配置"))

任务命令填 task hello.py，每日 08:00 执行填 0 8 * * *。也可使用 node hello.js、bash hello.sh、python hello.py。工作目录是数据目录中的 scripts。

HTTP 请求立即返回 runId，后台队列执行，结果从日志查询。非零退出码标记 failed，超时标记 timeout，停止标记 cancelled，重启前未完成的执行标记 interrupted，不自动重放。

同名环境变量按顺序靠后的启用值覆盖，不自动拼接 Cookie。脚本可以创建文件、访问网络和启动进程，仅运行可信脚本。并发限制不是操作系统沙箱，多租户隔离需要独立容器。

## MCP 与开放接口

设置 → 开放应用 / MCP，创建应用并选择 mcp 和实际资源权限。密钥仅显示一次。调用以下接口获取有效期 2 小时的 token：

    GET /open/auth/token?client_id=CLIENT_ID&client_secret=CLIENT_SECRET

随后带 Authorization: Bearer ACCESS_TOKEN 访问 /open 下的资源。

MCP 配置：

    {
      "mcpServers": {
        "cabbage": {
          "type": "http",
          "url": "http://localhost:5700/mcp",
          "headers": { "Authorization": "Bearer ACCESS_TOKEN" }
        }
      }
    }

客户端需定期重新调用凭证接口刷新 token。没有实现 OAuth 自动发现或交互授权。MCP 可访问任务、变量、文件、订阅、依赖和日志；无权限工具不出现在列表中。变量默认遮蔽值。

## 备份与恢复

设置 → 备份恢复，创建 ZIP 并妥善保存。导入覆盖账号、任务、变量、应用、脚本和配置，撤销所有会话，随后使用备份内账号登录。导入前自动创建安全备份。

备份与恢复时须清空运行和排队任务。导入最大 256MB，解压总计最大 512MB，单文件最大 32MB；脚本编辑和日志另有较小限制。备份未加密且包含敏感配置。

Python venv、Node 包和 Git 工作目录不打包，避免跨平台二进制不兼容。恢复后重新安装依赖、同步订阅。只支持大白菜备份格式，不直接导入青龙完整备份；任务和变量可通过兼容 API 迁移。

## 发布与更新

参阅 [发布文档](docs/RELEASE.md)。开发构建无发布公钥，禁止未签名下载更新。配置 GitHub 仓库及签名密钥后，Actions 生成各平台程序、签名清单和 SHA-256 文件。应用更新后重启服务生效。

公开源码无法隐藏源码；二进制去符号仅降低逆向便利度，浏览器始终可读取前端。重点是避免公开密钥、数据、备份和本机调试路径。

## 验证

    npm --prefix web ci
    npm --prefix web run build
    go test ./... -count=1 -timeout 120s
    go vet ./...

浏览器测试使用独立数据目录和 127.0.0.1:15700，不操作正式数据。构建程序后执行 npm --prefix web run test:e2e；Windows 使用本机 Edge，Linux CI 安装 Playwright Chromium。Linux 可运行 go test -race ./...。

systemd 示例在 deploy/cabbage.service。创建独立 cabbage 用户，将程序和数据放入 /opt/cabbage；原生在线更新需要该用户拥有程序目录写权限。

## 目录与许可证

    cmd/cabbage/       程序入口与版本
    internal/panel/    SQLite、API、调度、MCP、备份、更新、测试
    web/src/          Vue 管理界面
    web/tests/        端到端测试
    scripts/          构建与签名
    deploy/           systemd
    docs/             API 与发布
    data/             运行数据，不提交

仓库尚未指定开源许可证，由项目所有者在公开发布前选择。依赖许可证请参阅各上游包，构建保留依赖锁定信息。
