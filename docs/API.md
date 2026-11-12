# API 与青龙兼容范围

JSON 使用 UTF-8。成功为 {code:200,data:...}，错误为对应 HTTP 状态与 {code:400,message:"..."}。Bearer token 鉴权，网页使用同源 HttpOnly Cookie。

/api 是管理接口，/open 是开放应用接口，资源路由相同。应用需具备对应资源权限；apps、user、system、backups、audit 仅管理员可用。MCP 还需额外 mcp 权限。

面板内置「接口文档」页面（侧边栏 → 接口文档），按开放 API / 内置 API 两组列出全部接口，可直接填参数、发请求、查看响应并复制 cURL。它是本文档的交互版本，字段以本文档为准。

## 青龙常用接口

| 资源 | 方法 / 路径（省略前缀） | 形状 |
| --- | --- | --- |
| token | GET /open/auth/token | client_id、client_secret 查询参数；可选 expiresIn（60–2592000 秒，默认 7200）或 permanent=1；token、token_type、expiration（Unix 秒，0 表示永久）、expiresIn、permanent |
| 登录日志 | GET /api/user/login-log | 仅管理员；数组项 timestamp（毫秒）、address、ip、platform（mobile/desktop）、status（0 成功、1 失败），最多保留 100 条 |
| 通知日志 | GET /api/system/notify-log | 仅管理员；数组项 id、timestamp、title、content、status（0 成功、1 失败）、notifyType、source（system/script）、scriptName，最多保留 100 条 |
| 系统日志 | GET / DELETE /api/system/log | GET 返回系统运行日志文本（末尾最多 2MB）；DELETE 清空 |
| 任务列表 | GET /crons | searchValue/page/size；/api 与 /open 的 data 直接是 CronItem 数组，不嵌套分页对象 |
| 任务详情 | GET /crons/:id | 数字 ID |
| 创建任务 | POST /crons | 单个对象 {name,command,schedule,labels?,work_dir?,log_name?,task_before?,task_after?,notifyOnFinish?}，返回创建后的任务；也接受对象数组批量创建 |
| 更新任务 | PUT /crons | 上述字段和 id；支持部分更新 |
| 删除任务 | DELETE /crons | ID 数组 |
| 任务操作 | PUT /crons/run,stop,enable,disable,pin,unpin | ID 数组；run 返回 {id,runId,status} 数组 |
| 任务日志 | GET /crons/:id/log | 顶层返回 data、logStatus、offset、nextOffset、total、truncated；支持 offset/limit/tail |
| 历史实例 | GET /crons/:id/logs、/:id/instances | 最近运行数组 |
| 停止实例 | POST /crons/:id/instances/:runId/stop | 精确停止运行 |
| 标签 | POST / DELETE /crons/labels | {ids:[],labels:[]} |
| 视图 | GET / POST / PUT / DELETE /crons/views | 命名视图记录；不应用青龙私有筛选 DSL |
| 变量列表 | GET /envs | searchValue；data 数组，对象只有数字 id，没有 _id |
| 变量详情 | GET /envs/:id | 单个对象 |
| 新增变量 | POST /envs | [{name,value,remarks?,labels?}] |
| 修改变量 | PUT /envs | {id,name?,value?,remarks?,labels?} |
| 删除变量 | DELETE /envs | ID 数组 |
| 变量状态 | PUT /envs/enable,disable,pin,unpin | ID 数组 |
| 变量排序 | PUT /envs/:id/move | {fromIndex,toIndex} |
| 变量标签 | POST / DELETE /envs/labels | {ids,labels} |
| 批量改名 | PUT /envs/name | {ids,name} |
| 上传导入 | POST /envs/upload | multipart 的 env 字段，JSON 数组 |
| 脚本列表 | GET /scripts | 平铺数组：title/key/type/parent/size/createTime，附带 name/path/isLeaf/updatedAt |
| 脚本内容 | GET /scripts/:file?path=目录 | UTF-8 文本 |
| 精确读取 | GET /scripts/detail?path=目录/文件 | 支持 filename 参数 |
| 新建保存 | POST / PUT /scripts | {filename,path?,content} |
| 删除脚本 | DELETE /scripts | {filename,path?}，仅单个文件 |
| 下载脚本 | POST /scripts/download | {url,filename,path?} |
| 运行停止 | PUT /scripts/run,stop | {filename,path?} |
| 重命名 | PUT /scripts/rename | {filename,path?,newName} |
| 上传文件 | POST /scripts/upload | multipart 的 file 字段 + 可选 path；最大 32MB，同名不覆盖 |
| 打包下载 | POST /scripts/archive | 路径数组，返回 ZIP 附件 |
| 脚本历史 | GET /scripts/history?path=完整路径 | 历史元数据 |
| 历史内容 | GET /scripts/history/detail?id=ID | 历史对象 |
| 历史恢复 | PUT /scripts/history/restore | {id} |
| 配置列表 | GET /configs/files | [{title,value}]，附带面板的 name/path/isLeaf/size/builtin |
| 示例配置 | GET /configs/samples | 示例配置文件名数组 |
| 配置内容 | GET /configs/detail?path=...、/:file | UTF-8 文本 |
| 保存配置 | POST /configs/save | {filename,content}；只保存，不自动执行 |
| 依赖管理 | GET / POST / PUT / DELETE /dependencies | POST 数组，PUT 对象；name/type/remark，type=0 npm、1 Python、2 Linux；列表项 log 为字符串数组，含 timestamp/remark/createdAt/updatedAt |
| 依赖详情 | GET /dependencies/:id | 含最新安装日志 |
| 依赖操作 | PUT /dependencies/reinstall,cancel | ID 数组 |
| 移除记录 | DELETE /dependencies/force | ID 数组，不卸载文件 |
| 订阅管理 | GET / POST / PUT / DELETE /subscriptions | POST/PUT 对象；type=git-repo/file、url、alias、name、schedule、branch、whitelist、blacklist、autoAddCron、autoDelCron |
| 订阅操作 | PUT /subscriptions/run,stop,enable,disable | ID 数组 |
| 订阅日志 | GET /subscriptions/:id/log、/:id/logs | 最新文本 / 运行数组 |
| 应用管理 | GET / POST / PUT / DELETE /api/apps | name/scopes/expiresAt；创建返回 client_id/client_secret；管理员查询列表或详情可直接读取 client_secret |
| 重置密钥 | PUT /api/apps/:id/reset-secret | 撤销 token，返回新 secret，并保存新的可查看密钥 |
| 创建 Authorization | POST /api/apps/:id/token | {expiresIn} 秒或 {permanent:true}；到期返回 403；返回 token、expiration（0 表示永久）、expiresIn、permanent、client_id；Authorization 与 Client ID 用途不同 |
| Authorization 列表 | GET /api/apps/:id/tokens | 管理员读取该应用的令牌登记，含 token、created、expires、permanent、active；刷新页面后可恢复显示 |
| 撤销 Authorization | DELETE /api/apps/:id/tokens/:tokenId | 同时删除令牌登记与 session，立即失效 |

任务 status=0 表示有在途实例，1 表示空闲；isDisabled/isPinned=0/1。变量 status=0 启用，1 停用。依赖 status=0 安装成功，1 等待或安装中，2 未就绪。lastStatus 和运行记录另用字符串状态，不能与青龙所有内部枚举直接等值比较。

任务与环境变量的 JSON 形状按青龙（稳定版）输出。`timestamp` 为 `Sun Oct 04 02:58:24 GMT+08:00 2026` 形式，旧记录中的毫秒时间戳会在读取时转换。

**任务（CronItem）** 字段：`id`、`_id`（字符串）、`name`、`command`、`schedule`、`timestamp`、`saved`、`status`、`isSystem`、`pid`、`isDisabled`、`log_path`、`isPinned`、`labels`、`last_running_time`、`last_execution_time`、`sub_id`、`extra_schedules`、`task_before`、`task_after`、`log_name`、`allow_multiple_instances`、`work_dir`、`timeoutMs`、`notifyOnFinish`、`nextRunTime`。`extra_schedules` 未设置时为 `[]`；`log_name` 未指定时为「脚本名_任务ID.log」；`timeoutMs` 由秒级 `timeout` 换算；`nextRunTime` 为下次执行的毫秒时间戳。面板额外输出 `lastStatus`、`nextRun`、`duration`、`runningCount`、`remarks`、`createdAt`、`updatedAt` 供网页使用，青龙客户端会忽略。

**环境变量（Env）** 没有 `_id`，只有数字 `id`；也不输出 `isDisabled`，停用状态由 `status` 表示。

`GET /crons/:id/log` 顶层返回 `data`（日志内容）、`logStatus`、`offset`、`nextOffset`、`total`、`truncated`，支持 `offset` / `limit` / `tail` 分块。

任务支持 `work_dir`（工作目录，相对数据目录）、`log_name`（日志文件名）、`task_before` / `task_after`（任务级前后置脚本，优先于全局 config 脚本）、`notifyOnFinish`（执行结束通知）。创建和更新任务时也接受 `timeoutMs` 与 `allow_multiple_instances`。

## 任务命令

命令统一使用 `task` 前缀，按文件后缀自动选择解释器：

| 命令 | 解释器 |
| --- | --- |
| `task example.py` | Python（`CABBAGE_PYTHON`，默认 python3） |
| `task example.js` / `.mjs` / `.cjs` | Node.js |
| `task example.ts` / `.mts` / `.cts` | Node.js（20.6+ 原生运行 TypeScript） |
| `task example.sh` | Bash |

也支持任意 Shell 命令。工作目录默认是 `data/scripts`，可用任务的 `work_dir` 覆盖。

### 运行模式

| 用法 | 说明 |
| --- | --- |
| `task example.py` | 依次执行 |
| `task example.py now` | 立即执行（忽略随机延迟），前台输出日志 |
| `task example.py conc 变量名 [序号...]` | **并发执行**：把变量值按 `&` 拆分，逐个并发运行；省略序号则全部运行 |
| `task example.py desi 变量名 序号...` | **指定账号执行**：只跑指定序号 |

序号支持单个（`1 3`）与范围（`1-3`、`1~3`、`1_3`），会去重保序。序号是 task 自身的参数，不会传给脚本。

例如环境变量 `COOKIE` 的值为 `a&b&c`：

- `task demo.js conc COOKIE` — 三个账号并发跑
- `task demo.js conc COOKIE 1 3` — 只跑第 1、3 个账号，并发
- `task demo.js desi COOKIE 1-2` — 依次跑第 1、2 个账号


## 登录日志 IP 归属地

`GET /api/user/login-log` 返回的 `address` 字段：内网与回环地址标注为「内网IP」；公网地址在首次查询时异步解析（ip-api.com 中文接口），解析结果回写 SQLite 并缓存，后续查询直接命中。格式为「国家 省 市 运营商」，例如「中国 广东 佛山市 Chinanet」。

## 大白菜扩展

| 方法 / 路径 | 用途 |
| --- | --- |
| GET /healthz | 无鉴权健康检查 |
| GET /api/system/init | 初始化状态与版本 |
| POST /api/user/init | {setupToken,username,password} |
| POST /api/user/login | {username,password} |
| POST /api/user/logout | 撤销当前会话 |
| GET / PUT /api/user | 账号查询 / 修改，修改需要 oldPassword |
| GET /api/system/stats | 真实资源、统计和最近任务 |
| GET /api/system/runtimes | Python/Node/Bash/Git/npm 检测 |
| GET / PUT /api/system/settings | concurrency/timeout/retentionDays/githubRepo/webhookURL/notifyFailures/npmMirror/pythonMirror/linuxMirror/pythonPath/pythonVersion |
| POST /api/system/python/rebuild | 用当前配置的 Python 重建 venv 并重装 Python 依赖；有任务运行时返回 409 |
| GET / PUT / DELETE /api/user/ip-blacklist | IP 黑名单：GET 列表、PUT {ip} 添加、DELETE {ip} 移除；命中后无法登录 |
| PUT /api/system/cleanup | 清理过期日志 |
| POST /api/system/notify | 测试已保存 Webhook |
| GET /api/logs | 对齐青龙：按任务分组的日志目录树 [{title,key,type,parent,createTime,size,children}] |
| GET /api/logs/detail?path=任务&file=1.log&offset=&limit=&tail= | 分块读取；顶层返回 offset、nextOffset、total、truncated |
| POST /api/logs/download | {filename,path}，按路径下载日志 |
| GET /api/logs/runs?kind=crons&taskId=1&limit=200 | 面板扩展：平铺运行记录，最大 1000 条 |
| GET /api/logs/run/:runId | {content,status}，尾部最多 2MB |
| GET /api/logs/stream?id=runId | SSE log/status 事件 |
| DELETE /api/logs | 青龙语义 {filename,path} 删除单个日志；也兼容 ID 数组删除运行记录 |
| GET /api/backups | 本地备份列表 |
| POST /api/backups/export | 创建 ZIP，返回 name |
| GET / DELETE /api/backups/:name.zip | 下载 / 删除 |
| POST /api/backups/import | multipart file 或 application/zip |
| GET /api/system/update | GitHub 最新稳定 Release |
| POST /api/system/update/download | 后台下载验证，返回 runId |
| POST /api/system/update/apply | 替换程序，返回 restartRequired=true |
| POST /mcp | JSON-RPC initialize/ping/tools/list/tools/call |

## v1.1 扩展接口

下列接口同时支持 /api 与 /open 前缀，沿用资源权限。新增树接口不改变原有平铺文件列表。

| 接口 | 说明 |
| --- | --- |
| GET /api/scripts/tree、/api/configs/tree | 平铺树节点含目录与空目录；path、name、isLeaf、size，内置配置额外 builtin=true |
| POST /api/scripts/folder、/api/configs/folder | {"path":"目录/子目录"}，拒绝覆盖已有同名项 |
| POST /api/scripts/operations、/api/configs/operations | {"operation":"copy/move/rename/delete","paths":["a.py"],"destination":"目录"}；rename 的 destination 是完整新路径；返回逐项 success/error/updatedTasks |
| PUT /api/scripts/run、/api/scripts/stop | 直接运行/停止脚本；运行生成独立 scripts 运行记录，不创建定时任务 |
| POST /api/scripts/debug | {"filename":"目录/脚本.py","content":"未保存源码"}；写入临时文件并运行当前编辑内容，返回 {id,runId,status}；支持 py/js/mjs/cjs/sh/bash，任务结束后清理临时文件 |
| POST /api/logs/input | {"id":runId,"input":"一行文本"}；向调试任务的标准输入写入一行，用于 `input()` / `readline()` / `read` 等交互脚本 |
| GET /api/crons/:id/source | 返回 {path,content}；应用同时需要 crons 与 scripts 权限；复杂命令返回明确错误 |
| GET /api/logs/download/:runId | 以附件下载完整 UTF-8 日志 |
| GET /api/system/notifications | 读取通知配置，管理员权限 |
| PUT /api/system/notifications | 保存 {notifyFailures:boolean,channels:[{type,name,enabled,url,token,secret,chatId}]} |
| POST /api/system/notifications | 测试已保存且启用的渠道，返回逐渠道 success/error |

configs/detail 与 configs PUT 支持虚拟 system.json、notifications.json，保存会验证并立即更新 SQLite。history/restore 也会应用配置，恢复前保存当前内容。GET /api/system/settings 与系统配置文件共用数据。

GET /crons 默认把启用任务排在禁用任务前，组内置顶优先，其余按创建时间新到旧；POST /envs 不指定 position 时自动追加到环境变量末尾，GET /envs 按置顶、position 和创建时间升序返回。应用 expiresAt 为空表示永久有效；到期后不能再换取 Authorization，已有应用 token 也会失效。

应用 Client Secret 以明文保存在本地 SQLite 以支持管理员随时查看；备份文件同样包含该字段，请保护 data/ 与备份。永久 Authorization 的 sessions.expires 为 0，不参与过期清理；应用自身到期后该令牌仍会失效。

脚本助手内部使用 POST /api/notifications/send，JSON 为 {title,content}。此端点只接受面板注入的临时通知专用 Bearer 凭证；不能使用该凭证访问其他 API，也不能使用管理员会话绕过此端点的专用鉴权。

`data/scripts/notify.py` 与 `data/scripts/sendNotify.js` 随版本内置（首次启动写入，已存在不覆盖），签名与青龙一致：Python 用 `from notify import sendNotify`，Node 用 `require('./sendNotify')`。两者都通过面板注入的 `CABBAGE_NOTIFY_URL` / `CABBAGE_NOTIFY_TOKEN` 转发，通知渠道在「配置文件 → 通知配置」中设置，不读取青龙 config.sh。

依赖安装镜像在「系统设置 → 常规设置 → 依赖安装镜像」配置：npm 通过 `--registry`，pip 通过 `-i` 生效；留空使用官方源。Linux 软件包镜像（`linuxMirror`）在 Linux 上生效：apt 会生成临时 sources.list 指向镜像（自动识别发行版代号），apk 通过 `--repository` 指定，不改动系统原有源配置。

## Python 运行时

**依赖不随程序打包**：二进制只嵌入前端，Python 依赖装在 `data/packages/venv`（独立 venv），Node 依赖装在 `data/packages/node_modules`，运行脚本需要宿主机提供 Python 与 Node。

解释器解析优先级（`resolvePythonBase`）：

1. `pythonPath` — 绝对路径，必须存在且是文件
2. `pythonVersion` — 形如 `3.10`，按 `python3.10` → `python3` → `python` 顺序查找
3. 自动检测 — `python3` → `python`

已有 venv 时优先使用 venv 内的解释器。修改配置后必须调用 `POST /api/system/python/rebuild` 重建 venv 才生效；重建会先备份旧环境，失败自动回滚，成功后自动重装所有 `type=1` 的依赖。

`GET /api/system/runtimes` 的 Python 项额外返回：`version`（如 `3.10.9`）、`source`（`system` / `configured-path` / `configured-version` / `configured-fallback`）、`configuredPath`、`configuredVersion`。配置项无效时 `source` 为 `configured-fallback`，表示已回退到系统默认。

## 依赖缺失自动安装

任务运行失败后，面板会扫描日志末尾识别缺失依赖：

| 错误特征 | 识别为 | 处理 |
| --- | --- | --- |
| `ModuleNotFoundError: No module named 'x'` | Python（type=1） | 自动创建依赖记录并安装 |
| `Cannot find module 'x'` | Node.js（type=0） | 自动创建依赖记录并安装 |

识别到的依赖会加入「依赖管理」并立即开始安装，任务日志末尾追加「已自动添加依赖: xxx（安装完成后可重新运行）」。相对路径与 Node 内置模块（`node:` 前缀、以 `.` 或 `/` 开头）会被忽略。已存在的同名依赖不会重复添加。

## 调试交互输入

调试运行（`POST /api/scripts/debug`）会保留标准输入管道。当脚本执行到 `input()` / `readline()` / `read` 等需要输入的语句时，日志末尾会停在提示符上，调试面板底部自动出现输入框；提交后通过 `POST /api/logs/input` 写入进程 stdin，脚本继续执行，输入框随之消失。

只有调试运行支持输入；普通任务运行不保留 stdin，避免任务在无人值守时挂起。输入超时由任务超时设置控制。

## 差异

不提供青龙全部私有功能：OTP/二维码登录、SSH 配置、终端 WebSocket、依赖扫描刷新作业、全部通知厂商、仓库钩子 DSL、视图筛选 DSL、容器管理。未实现路径返回 404，不伪装成功。`GET /crons/import` 与 `/configs/samples` 作为兼容占位保留。

审计只保留登录日志，不记录日常写操作；旧的 `GET /api/audit` 已移除。登录日志沿用青龙 `login-log` 形状，成功与失败登录都会记录，旧版本 audit 表中的登录记录会在升级时迁移一次。本机与内网地址的 `address` 标注为「内网IP」，不引入 IP 归属地库。

日志 API 已对齐青龙：`GET /api/logs` 返回目录树，`/logs/detail` 支持 offset/limit/tail 分块；面板自身的平铺运行记录移到 `/api/logs/runs`。脚本列表同时输出青龙字段与面板字段。订阅不执行 sub_before/sub_after、代理设置或自定义安装钩子。pip 和系统包安装本身可能执行第三方构建。Git 私有仓库认证使用宿主 Git 凭据，URL 禁止嵌入密码。

常见 token、环境变量、任务 CRUD / 运行客户端可沿用；不承诺所有青龙内部接口或未公开字段兼容。完整迁移前按此表运行契约测试。
