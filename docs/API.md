# API 与青龙兼容范围

JSON 使用 UTF-8。成功为 {code:200,data:...}，错误为对应 HTTP 状态与 {code:400,message:"..."}。Bearer token 鉴权，网页使用同源 HttpOnly Cookie。

/api 是管理接口，/open 是开放应用接口，资源路由相同。应用需具备对应资源权限；apps、user、system、backups、audit 仅管理员可用。MCP 还需额外 mcp 权限。

## 青龙常用接口

| 资源 | 方法 / 路径（省略前缀） | 形状 |
| --- | --- | --- |
| token | GET /open/auth/token | client_id、client_secret 查询参数；可选 expiresIn（60–2592000 秒，默认 7200）或 permanent=1；token、token_type、expiration（Unix 秒，0 表示永久）、expiresIn、permanent |
| 任务列表 | GET /crons | searchValue/page/size；/api 与 /open 均返回 data 为 CronItem[]，不再嵌套分页对象 |
| 任务详情 | GET /crons/:id | 数字 ID |
| 创建任务 | POST /crons | {name,command,schedule,labels?,remarks?,timeout?,allowConcurrent?} |
| 更新任务 | PUT /crons | 上述字段和 id；支持部分更新 |
| 删除任务 | DELETE /crons | ID 数组 |
| 任务操作 | PUT /crons/run,stop,enable,disable,pin,unpin | ID 数组；run 返回 {id,runId,status} 数组 |
| 任务日志 | GET /crons/:id/log | 最新日志字符串 |
| 历史实例 | GET /crons/:id/logs、/:id/instances | 最近运行数组 |
| 停止实例 | POST /crons/:id/instances/:runId/stop | 精确停止运行 |
| 标签 | POST / DELETE /crons/labels | {ids:[],labels:[]} |
| 视图 | GET / POST / PUT / DELETE /crons/views | 命名视图记录；不应用青龙私有筛选 DSL |
| 变量列表 | GET /envs | searchValue；data 数组 |
| 变量详情 | GET /envs/:id | 单个对象 |
| 新增变量 | POST /envs | [{name,value,remarks?,labels?}] |
| 修改变量 | PUT /envs | {id,name?,value?,remarks?,labels?} |
| 删除变量 | DELETE /envs | ID 数组 |
| 变量状态 | PUT /envs/enable,disable,pin,unpin | ID 数组 |
| 变量排序 | PUT /envs/:id/move | {fromIndex,toIndex} |
| 变量标签 | POST / DELETE /envs/labels | {ids,labels} |
| 脚本列表 | GET /scripts | 平铺数组：name/path/key/title/isLeaf/size |
| 脚本内容 | GET /scripts/:file?path=目录 | UTF-8 文本 |
| 精确读取 | GET /scripts/detail?path=目录/文件 | 支持 filename 参数 |
| 新建保存 | POST / PUT /scripts | {filename,path?,content} |
| 删除脚本 | DELETE /scripts | {filename,path?}，仅单个文件 |
| 下载脚本 | POST /scripts/download | {url,filename,path?} |
| 运行停止 | PUT /scripts/run,stop | {filename,path?} |
| 重命名 | PUT /scripts/rename | {filename,path?,newName} |
| 脚本历史 | GET /scripts/history?path=完整路径 | 历史元数据 |
| 历史内容 | GET /scripts/history/detail?id=ID | 历史对象 |
| 历史恢复 | PUT /scripts/history/restore | {id} |
| 配置列表 | GET /configs/files | 平铺文件数组 |
| 配置内容 | GET /configs/detail?path=...、/:file | UTF-8 文本 |
| 保存配置 | POST /configs/save | {filename,content}；只保存，不自动执行 |
| 依赖管理 | GET / POST / PUT / DELETE /dependencies | POST 数组，PUT 对象；name/type/remark，type=0 npm、1 Python、2 Linux |
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
| GET / PUT /api/system/settings | concurrency/timeout/retentionDays/githubRepo/webhookURL/notifyFailures |
| PUT /api/system/cleanup | 清理过期日志 |
| POST /api/system/notify | 测试已保存 Webhook |
| GET /api/logs?kind=crons&taskId=1&limit=200 | 平铺运行记录，最大 1000 条 |
| GET /api/logs/run/:runId | {content,status}，尾部最多 2MB |
| GET /api/logs/stream?id=runId | SSE log/status 事件 |
| DELETE /api/logs | ID 数组，删除已结束日志 |
| GET /api/backups | 本地备份列表 |
| POST /api/backups/export | 创建 ZIP，返回 name |
| GET / DELETE /api/backups/:name.zip | 下载 / 删除 |
| POST /api/backups/import | multipart file 或 application/zip |
| GET /api/system/update | GitHub 最新稳定 Release |
| POST /api/system/update/download | 后台下载验证，返回 runId |
| POST /api/system/update/apply | 替换程序，返回 restartRequired=true |
| GET /api/audit | 最近 200 条操作；含 ip、remote（异地登录表示与上一次管理员登录 IP 不同） |
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
| GET /api/crons/:id/source | 返回 {path,content}；应用同时需要 crons 与 scripts 权限；复杂命令返回明确错误 |
| GET /api/logs/download/:runId | 以附件下载完整 UTF-8 日志 |
| GET /api/system/notifications | 读取通知配置，管理员权限 |
| PUT /api/system/notifications | 保存 {notifyFailures:boolean,channels:[{type,name,enabled,url,token,secret,chatId}]} |
| POST /api/system/notifications | 测试已保存且启用的渠道，返回逐渠道 success/error |

configs/detail 与 configs PUT 支持虚拟 system.json、notifications.json，保存会验证并立即更新 SQLite。history/restore 也会应用配置，恢复前保存当前内容。GET /api/system/settings 与系统配置文件共用数据。

GET /crons 默认把启用任务排在禁用任务前，组内置顶优先，其余按创建时间新到旧；POST /envs 不指定 position 时自动追加到环境变量末尾，GET /envs 按置顶、position 和创建时间升序返回。应用 expiresAt 为空表示永久有效；到期后不能再换取 Authorization，已有应用 token 也会失效。

应用 Client Secret 以明文保存在本地 SQLite 以支持管理员随时查看；备份文件同样包含该字段，请保护 data/ 与备份。永久 Authorization 的 sessions.expires 为 0，不参与过期清理；应用自身到期后该令牌仍会失效。

脚本助手内部使用 POST /api/notifications/send，JSON 为 {title,content}。此端点只接受面板注入的临时通知专用 Bearer 凭证；不能使用该凭证访问其他 API，也不能使用管理员会话绕过此端点的专用鉴权。

## 差异

不提供青龙全部私有功能：OTP/二维码登录、SSH 配置、终端 WebSocket、依赖扫描刷新作业、全部通知厂商、仓库钩子 DSL、视图筛选 DSL、容器管理。未实现路径返回 404，不伪装成功。

日志 API 为平铺运行记录，原有脚本列表保持平铺结构；新增 tree 接口提供目录节点，依赖青龙界面内部结构的旧客户端需适配。订阅不执行 sub_before/sub_after、代理设置或自定义安装钩子。pip 和系统包安装本身可能执行第三方构建。Git 私有仓库认证使用宿主 Git 凭据，URL 禁止嵌入密码。

常见 token、环境变量、任务 CRUD / 运行客户端可沿用；不承诺所有青龙内部接口或未公开字段兼容。完整迁移前按此表运行契约测试。
