// 接口文档清单：内容以 docs/API.md 与 internal/panel 实际路由为准。
// tabs: open = 开放应用接口（/open），api = 内置管理接口（/api）。
// absolute: true 表示 path 已是完整路径，不再拼接前缀。
// 未声明的路径参数会从 path 中的 :name 自动生成输入框。

const openAndApi = ['open', 'api']
const apiOnly = ['api']
const openOnly = ['open']

export const methodTone = { GET: 'get', POST: 'post', PUT: 'put', DELETE: 'delete' }

export const apiGroups = [
  {
    id: 'auth',
    label: '鉴权与账号',
    icon: 'ShieldCheck',
    intro: '网页使用同源 HttpOnly Cookie；开放应用先换取 Bearer token，再访问 /open 资源。成功响应为 {code:200,data:…}，错误为对应 HTTP 状态与 {code,message}。',
    endpoints: [
      {
        id: 'healthz', method: 'GET', path: '/healthz', absolute: true, noAuth: true, tabs: apiOnly,
        summary: '健康检查', description: '无鉴权，返回服务是否存活。','response': { code: 200, data: { status: 'ok' } },
      },
      {
        id: 'system-init', method: 'GET', path: '/system/init', noAuth: true, tabs: apiOnly,
        summary: '初始化状态与版本', description: '公开接口，用于登录页判断是否已完成首次初始化。',
        response: { code: 200, data: { isInitialized: true, version: '1.1.0' } },
      },
      {
        id: 'user-init', method: 'POST', path: '/user/init', noAuth: true, tabs: apiOnly,
        summary: '初始化管理员', description: '仅首次使用；setupToken 来自启动日志或 data/setup.key。用户名 2–64 字节，密码 6–72 字节。',
        body: { setupToken: 'Xk3…', username: 'admin', password: 'a-strong-password' },
        response: { code: 200, data: { initialized: true } },
      },
      {
        id: 'user-login', method: 'POST', path: '/user/login', noAuth: true, tabs: apiOnly,
        summary: '管理员登录', description: '成功后写入 HttpOnly 会话 Cookie，同时返回 Bearer token。15 分钟内失败过多会被限流。',
        body: { username: 'admin', password: 'a-strong-password' },
        response: { code: 200, data: { token: '…', token_type: 'Bearer', expiration: 1790000000, username: 'admin' } },
      },
      {
        id: 'user-logout', method: 'POST', path: '/user/logout', tabs: apiOnly,
        summary: '退出登录', description: '撤销当前会话令牌并清除 Cookie。',
        response: { code: 200, data: null },
      },
      {
        id: 'user-get', method: 'GET', path: '/user', tabs: apiOnly,
        summary: '当前账号', description: '仅管理员。',
        response: { code: 200, data: { username: 'admin', avatar: '', twoFactorActivated: false, role: 'admin' } },
      },
      {
        id: 'user-put', method: 'PUT', path: '/user', tabs: apiOnly,
        summary: '修改账号与密码', description: '需要当前密码；保存后撤销全部登录与应用令牌。',
        body: { username: 'admin', oldPassword: 'old-password', password: 'new-strong-password' },
        response: { code: 200, data: { username: 'admin' } },
      },
      {
        id: 'user-login-log', method: 'GET', path: '/user/login-log', tabs: apiOnly,
        summary: '登录日志', description: '仅管理员，最多保留最近 100 条，形状与青龙 login-log 一致。status：0 成功、1 失败。address 为 IP 归属地：内网标「内网IP」，公网异步解析为「国家 省 市 运营商」并回写缓存。',
        response: { code: 200, data: [{ timestamp: 1790000000000, address: '内网IP', ip: '192.168.1.9', platform: 'desktop', status: 0 }] },
      },
      {
        id: 'system-notify-log', method: 'GET', path: '/system/notify-log', tabs: apiOnly,
        summary: '通知日志', description: '仅管理员，最多保留最近 100 条。source 为 system（系统通知）或 script（脚本通知）；status：0 成功、1 失败；notifyType 为推送渠道类型。',
        response: { code: 200, data: [{ id: 1, timestamp: 1794400064694, title: '大白菜面板', content: '这是一条测试通知。', status: 0, notifyType: 'webhook', source: 'system', scriptName: '' }] },
      },
      {
        id: 'system-log', method: 'GET', path: '/system/log', tabs: apiOnly,
        summary: '系统日志', description: '返回系统运行日志纯文本（末尾最多 2MB）；DELETE 清空。记录启动、任务执行与异常事件。',
        response: { code: 200, data: '[info 2026-11-11 20:27:33]:\t[boot] 大白菜面板 v1.1.0 已启动' },
      },
      {
        id: 'user-ip-blacklist', method: 'GET', path: '/user/ip-blacklist', tabs: apiOnly,
        summary: 'IP 黑名单', description: '仅管理员。GET 返回 IP 数组；PUT 添加 {ip}；DELETE 移除 {ip}。命中黑名单的 IP 无法登录。支持单个 IP 与 CIDR。',
        response: { code: 200, data: ['203.0.113.5'] },
      },
      {
        id: 'open-token', method: 'GET', path: '/open/auth/token', absolute: true, noAuth: true, tabs: openOnly,
        summary: '换取开放接口令牌', description: '用应用的 client_id / client_secret 换取 Bearer token。可选 expiresIn（60–2592000 秒，默认 7200）或 permanent=1。应用过期后不能再换取。此接口属于开放 API，内置 API 没有对应路径。',
        params: [
          { name: 'client_id', in: 'query', required: true, type: 'string', description: '应用 Client ID', example: 'app_1a2b3c4d5e6f' },
          { name: 'client_secret', in: 'query', required: true, type: 'string', description: '应用 Client Secret', example: 'a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6' },
          { name: 'expiresIn', in: 'query', type: 'number', description: '有效期秒数 60–2592000，默认 7200', example: '7200' },
          { name: 'permanent', in: 'query', type: 'string', description: '传 1 或 true 表示永久有效', example: '1' },
        ],
        response: { code: 200, data: { token: '…', token_type: 'Bearer', expiration: 1790000000, expiresIn: 7200, permanent: false } },
      },
    ],
  },
  {
    id: 'crons',
    label: '定时任务',
    icon: 'Clock3',
    intro: '任务支持五段 / 六段 Cron、@every 和留空手动执行。status：0 有在途实例、1 空闲；isDisabled / isPinned：0 / 1。应用需要 crons 权限。',
    endpoints: [
      {
        id: 'crons-list', method: 'GET', path: '/crons', scope: 'crons', tabs: openAndApi,
        summary: '获取任务列表', description: 'data 直接是 CronItem 数组，不嵌套分页对象；page/size 在服务端切片。CronItem 含 _id（字符串）、timeoutMs、notifyOnFinish、nextRunTime、log_name、task_before、task_after、work_dir；extra_schedules 未设置时为 []。',
        params: [
          { name: 'searchValue', in: 'query', type: 'string', description: '按任意字段模糊搜索', example: '备份' },
          { name: 'page', in: 'query', type: 'number', description: '页码，从 1 开始', example: '1' },
          { name: 'size', in: 'query', type: 'number', description: '每页数量，上限 10000', example: '20' },
        ],
        response: { code: 200, data: [{ id: 1, _id: '1', name: '每日备份', command: 'task backup.py', schedule: '0 8 * * *', timestamp: 'Sun Oct 04 02:58:24 GMT+08:00 2026', saved: true, status: 1, isSystem: 0, pid: null, isDisabled: 0, log_path: '/data/log/每日备份_1.log', isPinned: 0, labels: [], last_running_time: 1794240076618, last_execution_time: 1794240439703, sub_id: null, extra_schedules: [], task_before: null, task_after: null, log_name: '每日备份_1.log', allow_multiple_instances: 0, work_dir: null, timeoutMs: 0, notifyOnFinish: 0, nextRunTime: 1794326460000 }] },
      },
      {
        id: 'crons-get', method: 'GET', path: '/crons/:id', scope: 'crons', tabs: openAndApi,
        summary: '任务详情', description: '数字 ID。',
        params: [{ name: 'id', in: 'path', required: true, type: 'number', description: '任务 ID', example: '1' }],
        response: { code: 200, data: { id: 1, _id: '1', name: '每日备份', command: 'task backup.py', schedule: '0 8 * * *', status: 1, isDisabled: 0, isPinned: 0, labels: [], timeoutMs: 0, notifyOnFinish: 0, nextRunTime: 1794326460000 } },
      },
      {
        id: 'crons-create', method: 'POST', path: '/crons', scope: 'crons', tabs: openAndApi,
        summary: '创建任务', description: '请求体为单个对象，返回创建后的任务；也接受对象数组批量创建。命令统一用 task 前缀：task 脚本.py / .js / .ts / .sh。并发用 task 脚本.js conc 变量名 [序号]，指定账号用 task 脚本.js desi 变量名 序号。支持 work_dir 工作目录、log_name 日志名、task_before / task_after 前后置脚本、notifyOnFinish 完成通知；timeout 为秒。',
        body: { name: '每日备份', command: 'task backup.py', schedule: '0 8 * * *', labels: ['日常'], timeout: 600, allowConcurrent: false, work_dir: null, log_name: 'backup', task_before: null, task_after: null, notifyOnFinish: 0 },
        response: { code: 200, data: { id: 2, name: '每日备份', status: 1 } },
      },
      {
        id: 'crons-update', method: 'PUT', path: '/crons', scope: 'crons', tabs: openAndApi,
        summary: '更新任务', description: '只接受单个对象，必须携带 id；未提供的字段保持不变。支持青龙的 timeoutMs 与 allow_multiple_instances 写入字段。',
        body: { id: 2, name: '每日备份', command: 'task backup.py', schedule: '0 9 * * *', timeout: 900 },
        response: { code: 200, data: { id: 2, name: '每日备份', schedule: '0 9 * * *' } },
      },
      {
        id: 'crons-delete', method: 'DELETE', path: '/crons', scope: 'crons', tabs: openAndApi,
        summary: '删除任务', description: '请求体为 ID 数组，1–500 个。',
        body: [2],
        response: { code: 200, data: null },
      },
      {
        id: 'crons-run', method: 'PUT', path: '/crons/run', scope: 'crons', tabs: openAndApi,
        summary: '立即运行', description: '请求体为 ID 数组；返回每个任务的 runId 与状态。执行异步进行，日志从 /logs 查询。',
        body: [1],
        response: { code: 200, data: [{ id: 1, runId: 88, status: 'queued' }] },
      },
      {
        id: 'crons-stop', method: 'PUT', path: '/crons/stop', scope: 'crons', tabs: openAndApi,
        summary: '停止运行', description: '请求体为 ID 数组，停止全部在途实例。',
        body: [1],
        response: { code: 200, data: null },
      },
      {
        id: 'crons-state', method: 'PUT', path: '/crons/:action', scope: 'crons', tabs: openAndApi,
        summary: '批量启用 / 停用 / 置顶', description: 'action 取 enable、disable、pin、unpin；请求体为 ID 数组。',
        params: [{ name: 'action', in: 'path', required: true, type: 'string', description: '操作动作', options: ['enable', 'disable', 'pin', 'unpin'], example: 'disable' }],
        body: [1],
        response: { code: 200, data: null },
      },
      {
        id: 'crons-log', method: 'GET', path: '/crons/:id/log', scope: 'crons', tabs: openAndApi,
        summary: '最新日志（分块）', description: '顶层返回日志内容与 logStatus / offset / nextOffset / total / truncated，支持 offset、limit、tail 分块。',
        params: [
          { name: 'id', in: 'path', required: true, type: 'number', description: '任务 ID', example: '1' },
          { name: 'offset', in: 'query', type: 'number', description: '起始字节偏移', example: '0' },
          { name: 'limit', in: 'query', type: 'number', description: '读取字节数，最大 1MB', example: '65536' },
          { name: 'tail', in: 'query', type: 'string', description: '传 true 从文件末尾读取', example: 'true' },
        ],
        response: { code: 200, data: '你好，大白菜！\n任务完成', logStatus: 'success', offset: 0, nextOffset: 182, total: 182, truncated: false },
      },
      {
        id: 'crons-runs', method: 'GET', path: '/crons/:id/logs', scope: 'crons', tabs: openAndApi,
        summary: '最近运行记录', description: '返回最近 100 条运行数组；/instances 为同义路径。',
        params: [{ name: 'id', in: 'path', required: true, type: 'number', description: '任务 ID', example: '1' }],
        response: { code: 200, data: [{ id: 88, status: 'success', started: '2026-10-04T08:00:00Z', duration: 1.42, exitCode: 0 }] },
      },
      {
        id: 'crons-instance-stop', method: 'POST', path: '/crons/:id/instances/:runId/stop', scope: 'crons', tabs: openAndApi,
        summary: '停止指定实例', description: '精确停止一次运行。',
        params: [
          { name: 'id', in: 'path', required: true, type: 'number', description: '任务 ID', example: '1' },
          { name: 'runId', in: 'path', required: true, type: 'number', description: '运行记录 ID', example: '88' },
        ],
        response: { code: 200, data: null },
      },
      {
        id: 'crons-source', method: 'GET', path: '/crons/:id/source', scope: 'crons + scripts', tabs: openAndApi,
        summary: '读取任务源码', description: '返回 {path,content}。应用同时需要 crons 与 scripts 权限；复杂命令会返回明确错误。',
        params: [{ name: 'id', in: 'path', required: true, type: 'number', description: '任务 ID', example: '1' }],
        response: { code: 200, data: { path: 'backup.py', content: 'print("hello")' } },
      },
      {
        id: 'crons-views-list', method: 'GET', path: '/crons/views', scope: 'crons', tabs: openAndApi,
        summary: '视图列表', description: '命名视图记录；不应用青龙私有筛选 DSL。',
        response: { code: 200, data: [{ id: 1, name: '我的视图', data: {} }] },
      },
      {
        id: 'crons-views-create', method: 'POST', path: '/crons/views', scope: 'crons', tabs: openAndApi,
        summary: '创建视图', description: '保存一份命名视图；PUT 更新、DELETE 删除同在 /crons/views。',
        body: { name: '我的视图', data: {} },
        response: { code: 200, data: { id: 1, name: '我的视图' } },
      },
      {
        id: 'crons-views-update', method: 'PUT', path: '/crons/views', scope: 'crons', tabs: openAndApi,
        summary: '更新视图', description: '必须携带 id。',
        body: { id: 1, name: '重命名视图', data: {} },
        response: { code: 200, data: { id: 1, name: '重命名视图' } },
      },
      {
        id: 'crons-views-delete', method: 'DELETE', path: '/crons/views', scope: 'crons', tabs: openAndApi,
        summary: '删除视图', description: '请求体为 ID 数组。',
        body: [1],
        response: { code: 200, data: null },
      },
      {
        id: 'crons-views-move', method: 'PUT', path: '/crons/views/move', scope: 'crons', tabs: openAndApi,
        summary: '调整视图顺序', description: 'fromIndex / toIndex 为视图列表下标，id 为被移动视图。',
        body: { id: 1, fromIndex: 2, toIndex: 0 },
        response: { code: 200, data: null },
      },
      {
        id: 'crons-views-state', method: 'PUT', path: '/crons/views/:action', scope: 'crons', tabs: openAndApi,
        summary: '启用 / 停用视图', description: 'action 取 enable、disable；请求体为 ID 数组。',
        params: [{ name: 'action', in: 'path', required: true, type: 'string', description: '操作动作', options: ['enable', 'disable'], example: 'disable' }],
        body: [1],
        response: { code: 200, data: null },
      },
      {
        id: 'crons-import', method: 'GET', path: '/crons/import', scope: 'crons', tabs: openAndApi,
        summary: '导入系统 crontab', description: '青龙兼容占位：面板没有系统 crontab，固定返回空数组。',
        response: { code: 200, data: [] },
      },
      {
        id: 'crons-labels-add', method: 'POST', path: '/crons/labels', scope: 'crons', tabs: openAndApi,
        summary: '添加标签', description: '为多个任务追加标签。',
        body: { ids: [1, 2], labels: ['日常'] },
        response: { code: 200, data: null },
      },
      {
        id: 'crons-labels-remove', method: 'DELETE', path: '/crons/labels', scope: 'crons', tabs: openAndApi,
        summary: '移除标签', description: '从多个任务移除标签。',
        body: { ids: [1], labels: ['日常'] },
        response: { code: 200, data: null },
      },
    ],
  },
  {
    id: 'envs',
    label: '环境变量',
    icon: 'KeyRound',
    intro: '变量在任务运行时注入进程；同名变量按排序靠后的启用值覆盖。status：0 启用、1 停用。对齐青龙 Env 模型：对象只有数字 id，没有 _id。应用需要 envs 权限。',
    endpoints: [
      {
        id: 'envs-list', method: 'GET', path: '/envs', scope: 'envs', tabs: openAndApi,
        summary: '变量列表', description: '按置顶、position 和创建时间升序返回；不输出 isDisabled 与 _id，停用状态由 status 表示。',
        params: [{ name: 'searchValue', in: 'query', type: 'string', description: '模糊搜索', example: 'TOKEN' }],
        response: { code: 200, data: [{ id: 1, name: 'MY_TOKEN', value: 'abc', remarks: '', labels: [], position: 0, status: 0, isPinned: 0, timestamp: 'Wed Nov 11 2026 13:29:58 GMT+0800 (China Standard Time)' }] },
      },
      {
        id: 'envs-get', method: 'GET', path: '/envs/:id', scope: 'envs', tabs: openAndApi,
        summary: '变量详情', description: '数字 ID。',
        params: [{ name: 'id', in: 'path', required: true, type: 'number', description: '变量 ID', example: '1' }],
        response: { code: 200, data: { id: 1, name: 'MY_TOKEN', value: 'abc' } },
      },
      {
        id: 'envs-create', method: 'POST', path: '/envs', scope: 'envs', tabs: openAndApi,
        summary: '新增变量', description: '可传对象数组批量新增；未指定 position 时自动追加到末尾。变量名需匹配 [A-Za-z_][A-Za-z0-9_]*。',
        body: [{ name: 'MY_TOKEN', value: 'abc', remarks: '示例变量', labels: ['测试'] }],
        response: { code: 200, data: [{ id: 1, name: 'MY_TOKEN' }] },
      },
      {
        id: 'envs-update', method: 'PUT', path: '/envs', scope: 'envs', tabs: openAndApi,
        summary: '修改变量', description: '只接受单个对象，必须携带 id。',
        body: { id: 1, name: 'MY_TOKEN', value: 'new-value', remarks: '已更新' },
        response: { code: 200, data: { id: 1, name: 'MY_TOKEN', value: 'new-value' } },
      },
      {
        id: 'envs-delete', method: 'DELETE', path: '/envs', scope: 'envs', tabs: openAndApi,
        summary: '删除变量', description: '请求体为 ID 数组。',
        body: [1],
        response: { code: 200, data: null },
      },
      {
        id: 'envs-state', method: 'PUT', path: '/envs/:action', scope: 'envs', tabs: openAndApi,
        summary: '批量启用 / 停用 / 置顶', description: 'action 取 enable、disable、pin、unpin；请求体为 ID 数组。',
        params: [{ name: 'action', in: 'path', required: true, type: 'string', description: '操作动作', options: ['enable', 'disable', 'pin', 'unpin'], example: 'enable' }],
        body: [1],
        response: { code: 200, data: null },
      },
      {
        id: 'envs-move', method: 'PUT', path: '/envs/:id/move', scope: 'envs', tabs: openAndApi,
        summary: '调整变量顺序', description: 'fromIndex / toIndex 为在排序后列表中的下标。',
        params: [{ name: 'id', in: 'path', required: true, type: 'number', description: '变量 ID', example: '1' }],
        body: { fromIndex: 2, toIndex: 0 },
        response: { code: 200, data: null },
      },
      {
        id: 'envs-labels-add', method: 'POST', path: '/envs/labels', scope: 'envs', tabs: openAndApi,
        summary: '添加标签', description: '为多个变量追加标签。',
        body: { ids: [1], labels: ['生产'] },
        response: { code: 200, data: null },
      },
      {
        id: 'envs-labels-remove', method: 'DELETE', path: '/envs/labels', scope: 'envs', tabs: openAndApi,
        summary: '移除标签', description: '从多个变量移除标签。',
        body: { ids: [1], labels: ['生产'] },
        response: { code: 200, data: null },
      },
      {
        id: 'envs-rename', method: 'PUT', path: '/envs/name', scope: 'envs', tabs: openAndApi,
        summary: '批量改名', description: '把多个变量统一改为同一名称；名称需匹配 [A-Za-z_][A-Za-z0-9_]*。',
        body: { ids: [1, 2], name: 'MY_TOKEN' },
        response: { code: 200, data: null },
      },
      {
        id: 'envs-upload', method: 'POST', path: '/envs/upload', scope: 'envs', tabs: openAndApi, upload: true,
        summary: '上传变量文件', description: 'multipart/form-data 的 env 字段，内容为变量对象数组 JSON；逐条校验 name / value。',
        response: { code: 200, data: [{ id: 3, name: 'IMPORTED', value: 'v1' }] },
      },
    ],
  },
  {
    id: 'scripts',
    label: '脚本',
    icon: 'SquareTerminal',
    intro: '脚本工作目录为数据目录下的 scripts/。应用需要 scripts 权限；旧版平铺列表与 v1.1 树接口并存。路径为相对 scripts 的目录或文件。',
    endpoints: [
      {
        id: 'scripts-list', method: 'GET', path: '/scripts', scope: 'scripts', tabs: openAndApi,
        summary: '平铺脚本列表', description: '对齐青龙：返回 {title,key,type,parent,size,createTime}，同时附带面板的 name/path/isLeaf/updatedAt；可用 searchValue 过滤路径。',
        params: [{ name: 'searchValue', in: 'query', type: 'string', description: '按相对路径模糊过滤', example: 'tools/' }],
        response: { code: 200, data: [{ title: 'hello.py', key: 'hello.py', type: 'file', parent: '.', size: 128, createTime: 1794376396494, name: 'hello.py', path: 'hello.py', isLeaf: true, updatedAt: '2026-10-04T08:00:00Z' }] },
      },
      {
        id: 'scripts-tree', method: 'GET', path: '/scripts/tree', scope: 'scripts', tabs: openAndApi,
        summary: '目录树', description: '包含目录与空目录节点。',
        response: { code: 200, data: [{ name: 'demo', path: 'demo', isLeaf: false, size: 0 }, { name: 'hello.py', path: 'hello.py', isLeaf: true, size: 128 }] },
      },
      {
        id: 'scripts-content', method: 'GET', path: '/scripts/:file', scope: 'scripts', tabs: openAndApi,
        summary: '读取脚本内容', description: 'file 为文件名，path 为所在目录。文件名含斜杠时改用 /scripts/detail。',
        params: [
          { name: 'file', in: 'path', required: true, type: 'string', description: '文件名', example: 'hello.py' },
          { name: 'path', in: 'query', type: 'string', description: '所在目录（留空为根目录）', example: 'tools' },
        ],
        response: { code: 200, data: 'print("你好，大白菜！")\n' },
      },
      {
        id: 'scripts-detail', method: 'GET', path: '/scripts/detail', scope: 'scripts', tabs: openAndApi,
        summary: '精确读取', description: 'path 为完整相对路径（目录/文件），也支持 filename 参数。',
        params: [
          { name: 'path', in: 'query', required: true, type: 'string', description: '完整相对路径', example: 'demo/hello.py' },
          { name: 'filename', in: 'query', type: 'string', description: '可选文件名（与 path 拼接）', example: 'hello.py' },
        ],
        response: { code: 200, data: 'print("你好，大白菜！")\n' },
      },
      {
        id: 'scripts-create', method: 'POST', path: '/scripts', scope: 'scripts', tabs: openAndApi,
        summary: '新建脚本', description: '拒绝覆盖已有同名文件。',
        body: { filename: 'hello.py', path: '', content: 'print("你好，大白菜！")' },
        response: { code: 200, data: null },
      },
      {
        id: 'scripts-save', method: 'PUT', path: '/scripts', scope: 'scripts', tabs: openAndApi,
        summary: '保存脚本', description: '保存到已有文件，并记录历史版本。',
        body: { filename: 'hello.py', path: '', content: 'print("你好，大白菜！")' },
        response: { code: 200, data: null },
      },
      {
        id: 'scripts-delete', method: 'DELETE', path: '/scripts', scope: 'scripts', tabs: openAndApi,
        summary: '删除脚本', description: '仅支持单个文件。',
        body: { filename: 'hello.py', path: '' },
        response: { code: 200, data: null },
      },
      {
        id: 'scripts-folder', method: 'POST', path: '/scripts/folder', scope: 'scripts', tabs: openAndApi,
        summary: '新建目录', description: 'path 为目录/子目录，拒绝覆盖已有同名项。',
        body: { path: 'demo/sub' },
        response: { code: 200, data: null },
      },
      {
        id: 'scripts-operations', method: 'POST', path: '/scripts/operations', scope: 'scripts', tabs: openAndApi,
        summary: '批量复制 / 移动 / 重命名 / 删除', description: 'operation 取 copy、move、rename、delete；rename 的 destination 是完整新路径。返回逐项 success / error / updatedTasks。',
        body: { operation: 'move', paths: ['hello.py'], destination: 'demo' },
        response: { code: 200, data: [{ path: 'hello.py', success: true, updatedTasks: 0 }] },
      },
      {
        id: 'scripts-run', method: 'PUT', path: '/scripts/run', scope: 'scripts', tabs: openAndApi,
        summary: '直接运行脚本', description: '生成独立的 scripts 运行记录，不创建定时任务。',
        body: { filename: 'hello.py', path: '' },
        response: { code: 200, data: { id: 88, runId: 88, status: 'queued' } },
      },
      {
        id: 'scripts-stop', method: 'PUT', path: '/scripts/stop', scope: 'scripts', tabs: openAndApi,
        summary: '停止脚本', description: '停止该脚本的在途运行。',
        body: { filename: 'hello.py', path: '' },
        response: { code: 200, data: null },
      },
      {
        id: 'scripts-debug', method: 'POST', path: '/scripts/debug', scope: 'scripts', tabs: openAndApi,
        summary: '调试未保存内容', description: '把未保存源码写入临时文件并运行，返回 {id,runId,status}；支持 py / js / mjs / cjs / sh / bash，结束后清理临时文件。',
        body: { filename: 'demo/hello.py', content: 'print("debug")' },
        response: { code: 200, data: { id: 88, runId: 88, status: 'queued' } },
      },
      {
        id: 'scripts-download', method: 'POST', path: '/scripts/download', scope: 'scripts', tabs: openAndApi,
        summary: '下载远程脚本', description: '从 HTTP(S) URL 拉取脚本保存到脚本目录。',
        body: { url: 'https://example.com/hello.py', filename: 'hello.py', path: '' },
        response: { code: 200, data: null },
      },
      {
        id: 'scripts-rename', method: 'PUT', path: '/scripts/rename', scope: 'scripts', tabs: openAndApi,
        summary: '重命名脚本', description: 'newName 为新文件名。',
        body: { filename: 'hello.py', path: '', newName: 'hello2.py' },
        response: { code: 200, data: null },
      },
      {
        id: 'scripts-upload', method: 'POST', path: '/scripts/upload', scope: 'scripts', tabs: openAndApi, upload: true,
        summary: '上传本地文件', description: 'multipart/form-data 的 file 字段，可选 path 指定目标文件夹；同名文件不会被覆盖，最大 32MB。',
        response: { code: 200, data: { path: 'tools/example.py', size: 1024 } },
      },
      {
        id: 'scripts-archive', method: 'POST', path: '/scripts/archive', scope: 'scripts', tabs: openAndApi,
        summary: '批量打包下载', description: '请求体为路径数组，返回 ZIP 附件。',
        body: ['notify.py', 'tools/example.py'],
      },
      {
        id: 'scripts-history', method: 'GET', path: '/scripts/history', scope: 'scripts', tabs: openAndApi,
        summary: '脚本历史', description: '返回历史元数据列表。',
        params: [{ name: 'path', in: 'query', required: true, type: 'string', description: '完整相对路径', example: 'hello.py' }],
        response: { code: 200, data: [{ id: 3, path: 'hello.py', created: '2026-10-04T08:00:00Z', size: 128 }] },
      },
      {
        id: 'scripts-history-detail', method: 'GET', path: '/scripts/history/detail', scope: 'scripts', tabs: openAndApi,
        summary: '历史内容', description: '返回指定历史版本对象。',
        params: [{ name: 'id', in: 'query', required: true, type: 'number', description: '历史记录 ID', example: '3' }],
        response: { code: 200, data: { id: 3, path: 'hello.py', content: 'print("old")' } },
      },
      {
        id: 'scripts-history-restore', method: 'PUT', path: '/scripts/history/restore', scope: 'scripts', tabs: openAndApi,
        summary: '恢复历史版本', description: '恢复前会保存当前内容。',
        body: { id: 3 },
        response: { code: 200, data: null },
      },
    ],
  },
  {
    id: 'configs',
    label: '内置配置',
    icon: 'Braces',
    intro: '内置配置与脚本共用文件接口，包含虚拟文件 system.json、notifications.json，保存时会校验并立即写入 SQLite。应用需要 configs 权限。',
    endpoints: [
      {
        id: 'configs-files', method: 'GET', path: '/configs/files', scope: 'configs', tabs: openAndApi,
        summary: '配置文件列表', description: '对齐青龙：返回 [{title,value}] 文件名数组；同时附带面板的 name/path/isLeaf/size/builtin 字段。',
        response: { code: 200, data: [{ title: 'config.sh', value: 'config.sh', name: 'config.sh', path: 'config.sh', isLeaf: true, size: 320 }] },
      },
      {
        id: 'configs-samples', method: 'GET', path: '/configs/samples', scope: 'configs', tabs: openAndApi,
        summary: '示例配置文件', description: '对齐青龙：返回示例配置文件名数组。',
        response: { code: 200, data: ['config.sh', 'extra.sh', 'task_before.sh', 'task_after.sh', 'task_before.js', 'task_before.py'] },
      },
      {
        id: 'configs-tree', method: 'GET', path: '/configs/tree', scope: 'configs', tabs: openAndApi,
        summary: '配置目录树', description: '目录与文件节点，内置配置带 builtin=true。',
        response: { code: 200, data: [{ name: 'config.sh', path: 'config.sh', isLeaf: true, size: 320, builtin: true }] },
      },
      {
        id: 'configs-detail', method: 'GET', path: '/configs/detail', scope: 'configs', tabs: openAndApi,
        summary: '读取配置内容', description: 'path 为相对路径，也可用 filename 追加文件名；/configs/:file 同义。',
        params: [
          { name: 'path', in: 'query', required: true, type: 'string', description: '相对路径', example: 'config.sh' },
          { name: 'filename', in: 'query', type: 'string', description: '可选文件名（与 path 拼接）', example: 'hello.py' },
        ],
        response: { code: 200, data: 'export TZ=Asia/Shanghai\n' },
      },
      {
        id: 'configs-save', method: 'POST', path: '/configs/save', scope: 'configs', tabs: openAndApi,
        summary: '保存配置', description: '只保存，不自动执行。system.json 与 notifications.json 会校验字段并立即更新设置；保存前记录历史版本。',
        body: { filename: 'config.sh', content: 'export TZ=Asia/Shanghai\n' },
        response: { code: 200, data: { path: 'config.sh' } },
      },
      {
        id: 'configs-put', method: 'PUT', path: '/configs', scope: 'configs', tabs: openAndApi,
        summary: '保存配置（PUT 同义）', description: '与 /configs/save 等价，请求体 {filename,path?,content}。',
        body: { filename: 'config.sh', content: 'export TZ=Asia/Shanghai\n' },
        response: { code: 200, data: { path: 'config.sh' } },
      },
      {
        id: 'configs-folder', method: 'POST', path: '/configs/folder', scope: 'configs', tabs: openAndApi,
        summary: '新建配置目录', description: '拒绝覆盖已有同名项。',
        body: { path: 'hooks/daily' },
        response: { code: 200, data: null },
      },
      {
        id: 'configs-operations', method: 'POST', path: '/configs/operations', scope: 'configs', tabs: openAndApi,
        summary: '批量文件操作', description: 'operation 取 copy、move、rename、delete。',
        body: { operation: 'rename', paths: ['config.sh'], destination: 'config-old.sh' },
        response: { code: 200, data: [{ path: 'config.sh', success: true, updatedTasks: 0 }] },
      },
    ],
  },
  {
    id: 'logs',
    label: '运行日志',
    icon: 'FileText',
    intro: 'GET /logs 返回青龙形状的日志目录树；面板自身的运行记录在 /logs/runs。kind 取 crons、scripts、subscriptions、dependencies。应用需要 logs 权限。',
    endpoints: [
      {
        id: 'logs-list', method: 'GET', path: '/logs', scope: 'logs', tabs: openAndApi,
        summary: '日志目录树', description: '对齐青龙：按任务分组返回 {title,key,type,parent,createTime,size,children} 的目录树，可直接供青龙前端浏览。',
        response: { code: 200, data: [{ title: '每日备份', key: '每日备份', type: 'directory', parent: '', createTime: 1794376396494, children: [{ title: '1.log', key: '每日备份/1.log', type: 'file', parent: '每日备份', size: 333, createTime: 1794376396494 }] }] },
      },
      {
        id: 'logs-runs', method: 'GET', path: '/logs/runs', scope: 'logs', tabs: openAndApi,
        summary: '运行记录列表', description: '面板扩展接口：返回平铺运行记录，最大 1000 条。',
        params: [
          { name: 'kind', in: 'query', type: 'string', description: '任务类型', example: 'crons' },
          { name: 'taskId', in: 'query', type: 'number', description: '任务 / 记录 ID', example: '1' },
          { name: 'limit', in: 'query', type: 'number', description: '最大条数，默认 200，上限 1000', example: '200' },
        ],
        response: { code: 200, data: [{ id: 88, name: '每日备份', kind: 'crons', taskId: 1, status: 'success', started: '2026-10-04T08:00:00Z', duration: 1.42, exitCode: 0 }] },
      },
      {
        id: 'logs-run', method: 'GET', path: '/logs/run/:runId', scope: 'logs', tabs: openAndApi,
        summary: '运行日志内容', description: '返回 {content,status}，尾部最多 2MB。',
        params: [{ name: 'runId', in: 'path', required: true, type: 'number', description: '运行记录 ID', example: '88' }],
        response: { code: 200, data: { content: '你好，大白菜！\n', status: 'success' } },
      },
      {
        id: 'logs-detail', method: 'GET', path: '/logs/detail', scope: 'logs', tabs: openAndApi,
        summary: '按路径分块读取日志', description: '对齐青龙：path + file 定位日志，支持 offset / limit / tail 分块，顶层返回 offset、nextOffset、total、truncated。',
        params: [
          { name: 'path', in: 'query', type: 'string', description: '目录树里的 parent（任务名）', example: '每日备份' },
          { name: 'file', in: 'query', required: true, type: 'string', description: '日志文件名', example: '1.log' },
          { name: 'offset', in: 'query', type: 'number', description: '起始字节偏移', example: '0' },
          { name: 'limit', in: 'query', type: 'number', description: '读取字节数，默认 2MB', example: '65536' },
          { name: 'tail', in: 'query', type: 'string', description: '传 true 从文件末尾读取', example: 'true' },
        ],
        response: { code: 200, data: '你好，大白菜！\n', offset: 0, nextOffset: 333, total: 333, truncated: false },
      },
      {
        id: 'logs-input', method: 'POST', path: '/logs/input', scope: 'logs', tabs: openAndApi,
        summary: '调试交互输入', description: '向调试任务的标准输入写入一行。仅对通过 /scripts/debug 启动、且仍在等待输入的运行有效。',
        body: { id: 88, input: '张三' },
        response: { code: 200, data: null },
      },
      {
        id: 'logs-stream', method: 'GET', path: '/logs/stream', scope: 'logs', tabs: openAndApi, streaming: true,
        summary: 'SSE 实时日志', description: 'Server-Sent Events，推送 log 与 status 事件，运行结束后自动结束。浏览器原生不显示在文档内，请用 EventSource 或 curl 调用。',
        params: [{ name: 'id', in: 'query', required: true, type: 'number', description: '运行记录 ID', example: '88' }],
      },
      {
        id: 'logs-download', method: 'GET', path: '/logs/download/:runId', scope: 'logs', tabs: openAndApi,
        summary: '下载完整日志', description: '以附件形式返回完整 UTF-8 日志。',
        params: [{ name: 'runId', in: 'path', required: true, type: 'number', description: '运行记录 ID', example: '88' }],
      },
      {
        id: 'logs-download-file', method: 'POST', path: '/logs/download', scope: 'logs', tabs: openAndApi,
        summary: '按路径下载日志', description: '对齐青龙：请求体 {filename, path}，以附件形式返回指定日志文件。',
        body: { filename: '1.log', path: '每日备份' },
      },
      {
        id: 'logs-delete', method: 'DELETE', path: '/logs', scope: 'logs', tabs: openAndApi,
        summary: '删除日志', description: '青龙语义：请求体 {filename, path} 删除单个日志；也兼容面板旧用法（ID 数组，运行中或排队中的记录不能删除）。',
        body: { filename: '1.log', path: '每日备份' },
        response: { code: 200, data: null },
      },
    ],
  },
  {
    id: 'subscriptions',
    label: '订阅同步',
    icon: 'GitBranch',
    intro: '订阅 HTTP(S) Git 仓库或单文件。type 取 git-repo、file；alias 只能包含字母、数字、下划线和连字符。应用需要 subscriptions 权限。',
    endpoints: [
      {
        id: 'subs-list', method: 'GET', path: '/subscriptions', scope: 'subscriptions', tabs: openAndApi,
        summary: '订阅列表', description: '返回订阅数组并附带运行状态。',
        response: { code: 200, data: [{ id: 1, name: '脚本仓库', type: 'git-repo', url: 'https://github.com/owner/repo.git', alias: 'repo', branch: 'main', schedule: '0 */6 * * *', isDisabled: 0 }] },
      },
      {
        id: 'subs-get', method: 'GET', path: '/subscriptions/:id', scope: 'subscriptions', tabs: openAndApi,
        summary: '订阅详情', description: '数字 ID。',
        params: [{ name: 'id', in: 'path', required: true, type: 'number', description: '订阅 ID', example: '1' }],
        response: { code: 200, data: { id: 1, name: '脚本仓库', type: 'git-repo', alias: 'repo' } },
      },
      {
        id: 'subs-create', method: 'POST', path: '/subscriptions', scope: 'subscriptions', tabs: openAndApi,
        summary: '新增订阅', description: 'URL 必须是不含内嵌凭证的 HTTP(S) 地址；schedule 留空表示仅手动同步。',
        body: { name: '脚本仓库', type: 'git-repo', url: 'https://github.com/owner/repo.git', alias: 'repo', branch: 'main', schedule: '0 */6 * * *', whitelist: '', blacklist: '', autoAddCron: true, autoDelCron: false },
        response: { code: 200, data: { id: 1, name: '脚本仓库' } },
      },
      {
        id: 'subs-update', method: 'PUT', path: '/subscriptions', scope: 'subscriptions', tabs: openAndApi,
        summary: '更新订阅', description: '必须携带 id。',
        body: { id: 1, name: '脚本仓库', schedule: '0 2 * * *' },
        response: { code: 200, data: { id: 1, name: '脚本仓库', schedule: '0 2 * * *' } },
      },
      {
        id: 'subs-delete', method: 'DELETE', path: '/subscriptions', scope: 'subscriptions', tabs: openAndApi,
        summary: '删除订阅', description: '请求体为 ID 数组。',
        body: [1],
        response: { code: 200, data: null },
      },
      {
        id: 'subs-action', method: 'PUT', path: '/subscriptions/:action', scope: 'subscriptions', tabs: openAndApi,
        summary: '同步 / 停止 / 启停', description: 'action 取 run、stop、enable、disable；请求体为 ID 数组。',
        params: [{ name: 'action', in: 'path', required: true, type: 'string', description: '操作动作', options: ['run', 'stop', 'enable', 'disable'], example: 'run' }],
        body: [1],
        response: { code: 200, data: null },
      },
      {
        id: 'subs-log', method: 'GET', path: '/subscriptions/:id/log', scope: 'subscriptions', tabs: openAndApi,
        summary: '最新同步日志', description: '返回最新日志文本。',
        params: [{ name: 'id', in: 'path', required: true, type: 'number', description: '订阅 ID', example: '1' }],
        response: { code: 200, data: 'Cloning into repo…\n完成' },
      },
      {
        id: 'subs-runs', method: 'GET', path: '/subscriptions/:id/logs', scope: 'subscriptions', tabs: openAndApi,
        summary: '同步运行记录', description: '返回最近运行数组。',
        params: [{ name: 'id', in: 'path', required: true, type: 'number', description: '订阅 ID', example: '1' }],
        response: { code: 200, data: [{ id: 12, status: 'success', started: '2026-10-04T02:00:00Z' }] },
      },
    ],
  },
  {
    id: 'dependencies',
    label: '依赖管理',
    icon: 'Package',
    intro: 'type：0 Node.js（npm）、1 Python（独立 venv）、2 Linux 系统包。POST 接受数组，PUT 接受对象。应用需要 dependencies 权限。',
    endpoints: [
      {
        id: 'deps-list', method: 'GET', path: '/dependencies', scope: 'dependencies', tabs: openAndApi,
        summary: '依赖列表', description: '可用 type 过滤。status：0 安装成功、1 等待或安装中、2 未就绪。log 为安装日志字符串数组。',
        params: [{ name: 'type', in: 'query', type: 'number', description: '依赖类型 0/1/2', example: '1' }],
        response: { code: 200, data: [{ id: 1, name: 'requests', type: 1, timestamp: 'Wed Nov 11 2026 17:40:08 GMT+0800 (China Standard Time)', status: 1, log: ['开始安装依赖 requests，开始时间 2026-11-11 17:40:08\n\n', '依赖安装成功，结束时间 2026-11-11 17:40:09，耗时 0 秒'], remark: '', createdAt: '2026-11-11T09:40:08.299Z', updatedAt: '2026-11-11T09:40:09.184Z' }] },
      },
      {
        id: 'deps-get', method: 'GET', path: '/dependencies/:id', scope: 'dependencies', tabs: openAndApi,
        summary: '依赖详情', description: '返回记录与安装日志数组。',
        params: [{ name: 'id', in: 'path', required: true, type: 'number', description: '依赖 ID', example: '1' }],
        response: { code: 200, data: { id: 1, name: 'requests', type: 1, status: 0, log: ['Successfully installed requests'] } },
      },
      {
        id: 'deps-create', method: 'POST', path: '/dependencies', scope: 'dependencies', tabs: openAndApi,
        summary: '添加依赖', description: '请求体为数组，每条安装一个包，提交后异步安装。',
        body: [{ name: 'requests==2.32.5', type: 1, remark: 'HTTP 客户端' }],
        response: { code: 200, data: [{ id: 1, name: 'requests==2.32.5', status: 1 }] },
      },
      {
        id: 'deps-update', method: 'PUT', path: '/dependencies', scope: 'dependencies', tabs: openAndApi,
        summary: '修改依赖', description: '仅修改名称、类型或备注，不触发安装。',
        body: { id: 1, name: 'requests==2.32.5', type: 1, remark: '已更新备注' },
        response: { code: 200, data: { id: 1, name: 'requests==2.32.5' } },
      },
      {
        id: 'deps-delete', method: 'DELETE', path: '/dependencies', scope: 'dependencies', tabs: openAndApi,
        summary: '卸载依赖', description: '请求体为 ID 数组；会真实卸载已安装文件。',
        body: [1],
        response: { code: 200, data: null },
      },
      {
        id: 'deps-action', method: 'PUT', path: '/dependencies/:action', scope: 'dependencies', tabs: openAndApi,
        summary: '重新安装 / 取消安装', description: 'action 取 reinstall、cancel；请求体为 ID 数组。',
        params: [{ name: 'action', in: 'path', required: true, type: 'string', description: '操作动作', options: ['reinstall', 'cancel'], example: 'reinstall' }],
        body: [1],
        response: { code: 200, data: null },
      },
      {
        id: 'deps-force', method: 'DELETE', path: '/dependencies/force', scope: 'dependencies', tabs: openAndApi,
        summary: '仅移除记录', description: '请求体为 ID 数组，不卸载文件。',
        body: [1],
        response: { code: 200, data: null },
      },
    ],
  },
  {
    id: 'system',
    label: '系统与设置',
    icon: 'Settings2',
    intro: '仅管理员可用；开放应用（/open）没有 system 权限，调用会返回 403。',
    endpoints: [
      {
        id: 'system-stats', method: 'GET', path: '/system/stats', tabs: apiOnly,
        summary: '资源与执行统计', description: '真实 CPU、内存、磁盘、面板堆内存、执行统计、近 7 天系列和最近 6 条运行。',
        response: { code: 200, data: { version: '1.1.0', uptime: 3600, platform: 'windows/amd64', tasks: 6, running: 1, queued: 0, today: 12, success: 300, failed: 4, successRate: 98.68, cpu: 12.4, memory: 43.1, disk: 55.2, series: [{ date: '2026-11-10', success: 5, failed: 0 }], recent: [] } },
      },
      {
        id: 'system-runtimes', method: 'GET', path: '/system/runtimes', tabs: apiOnly,
        summary: '运行时检测', description: '检测 Python、Node、Bash、Git、npm 是否可用。',
        response: { code: 200, data: [{ name: 'Python', available: true, path: '/usr/bin/python3' }] },
      },
      {
        id: 'system-config-get', method: 'GET', path: '/system/config', tabs: apiOnly,
        summary: '读取系统配置（青龙）', description: '对齐青龙：返回 {id,type,info:{timezone,lang,...},signedUpdates}；info 同时带面板的 concurrency/timeout 等设置。',
        response: { code: 200, data: { id: 1, type: 'systemConfig', info: { timezone: 'Asia/Shanghai', lang: 'zh', concurrency: '4', timeout: '3600' }, signedUpdates: false } },
      },
      {
        id: 'system-settings-get', method: 'GET', path: '/system/settings', tabs: apiOnly,
        summary: '读取系统设置', description: '与系统配置文件共用数据；signedUpdates 表示当前构建是否嵌入了发布公钥。',
        response: { code: 200, data: { concurrency: '4', timeout: '3600', retentionDays: '30', githubRepo: 'bucai2417/cabbage-panel', webhookURL: '', notifyFailures: 'true', signedUpdates: false } },
      },
      {
        id: 'system-settings-put', method: 'PUT', path: '/system/settings', tabs: apiOnly,
        summary: '保存系统设置', description: '支持部分更新：concurrency（1–32）/ timeout（1–604800 秒）/ retentionDays（1–3650）/ githubRepo / webhookURL / notifyFailures / npmMirror / pythonMirror / linuxMirror。镜像为 HTTP(S) URL，留空使用默认源。未知键会返回 400。',
        body: { concurrency: '8', timeout: '3600', retentionDays: '30', npmMirror: 'https://registry.npmmirror.com', pythonMirror: 'https://pypi.tuna.tsinghua.edu.cn/simple', linuxMirror: 'https://mirrors.tuna.tsinghua.edu.cn/ubuntu/' },
        response: { code: 200, data: null },
      },
      {
        id: 'system-cleanup', method: 'PUT', path: '/system/cleanup', tabs: apiOnly,
        summary: '清理过期日志', description: '按保留天数立即清理，返回删除条数。',
        response: { code: 200, data: { removed: 0 } },
      },
      {
        id: 'system-notify', method: 'POST', path: '/system/notify', tabs: apiOnly,
        summary: '测试已保存通知', description: '向已保存且启用的通知渠道发送一条测试消息。',
        response: { code: 200, data: null },
      },
      {
        id: 'system-notifications-get', method: 'GET', path: '/system/notifications', tabs: apiOnly,
        summary: '读取通知配置', description: '仅管理员。',
        response: { code: 200, data: { notifyFailures: true, channels: [] } },
      },
      {
        id: 'system-notifications-put', method: 'PUT', path: '/system/notifications', tabs: apiOnly,
        summary: '保存通知配置', description: 'channels 项字段：type（webhook/wecom/dingtalk/feishu/bark/telegram/pushplus/serverchan）、name、enabled、url、token、secret、chatId。',
        body: { notifyFailures: true, channels: [{ type: 'wecom', name: '企业微信', enabled: true, url: '', token: '', secret: '', chatId: '' }] },
        response: { code: 200, data: null },
      },
      {
        id: 'system-notifications-test', method: 'POST', path: '/system/notifications', tabs: apiOnly,
        summary: '测试通知渠道', description: '向已保存且启用的渠道发送测试通知，返回逐渠道 name / success / error。',
        response: { code: 200, data: [{ name: '企业微信', success: true }] },
      },
      {
        id: 'system-notify-helper', method: 'POST', path: '/api/notifications/send', absolute: true, tabs: apiOnly,
        summary: '脚本内发送通知（helper）', description: '供任务脚本调用，不属于常规接口。Authorization 必须使用任务运行时注入的 CABBAGE_NOTIFY_TOKEN，普通登录会话无效；请求体 {title, content}，返回逐渠道 name / success / error。Python 用 bin/notify.py、Node 用 bin/sendNotify.js 封装。',
        body: { title: '任务完成', content: '备份已生成' },
        response: { code: 200, data: [{ name: '企业微信', success: true }] },
      },
      {
        id: 'system-update-check', method: 'GET', path: '/system/update', tabs: apiOnly,
        summary: '检查更新', description: '查询 GitHub 最新稳定 Release，返回当前版本、最新版本、是否有更新与发行说明。',
        response: { code: 200, data: { current: '1.1.0', latest: '1.1.0', available: false, body: '', url: '', signedUpdates: false, staged: '' } },
      },
      {
        id: 'system-update-download', method: 'POST', path: '/system/update/download', tabs: apiOnly,
        summary: '下载并验证更新', description: '后台下载，验证 Ed25519 签名与 SHA-256，返回 {id,runId,status}。未嵌入发布公钥时返回 409。',
        response: { code: 200, data: { id: 1, runId: 12, status: 'queued' } },
      },
      {
        id: 'system-update-apply', method: 'POST', path: '/system/update/apply', tabs: apiOnly,
        summary: '应用已验证更新', description: '替换程序并返回 restartRequired=true；旧程序保存为 .previous 用于回滚。有任务运行时返回 409。',
        response: { code: 200, data: { version: '1.1.1', restartRequired: true, message: '已替换程序，重启服务后生效；旧程序保存为 .previous' } },
      },
    ],
  },
  {
    id: 'apps',
    label: '开放应用',
    icon: 'Plug',
    intro: '仅管理员。scopes 取值 crons、envs、scripts、logs、configs、subscriptions、dependencies、mcp。Client Secret 可在列表中直接查看。',
    endpoints: [
      {
        id: 'apps-list', method: 'GET', path: '/apps', tabs: apiOnly,
        summary: '应用列表', description: '返回应用，包括可用于显示的 client_secret。',
        response: { code: 200, data: [{ id: 1, name: 'MCP 客户端', client_id: 'Abc123…', client_secret: '…', scopes: ['crons', 'logs', 'mcp'], expiresAt: '' }] },
      },
      {
        id: 'apps-create', method: 'POST', path: '/apps', tabs: apiOnly,
        summary: '创建应用', description: '返回 client_id 与 client_secret；expiresAt 为空表示永久。',
        body: { name: 'MCP 客户端', scopes: ['crons', 'logs', 'mcp'], expiresAt: '' },
        response: { code: 200, data: { id: 1, name: 'MCP 客户端', client_id: 'Abc123…', client_secret: 'Xyz789…' } },
      },
      {
        id: 'apps-update', method: 'PUT', path: '/apps', tabs: apiOnly,
        summary: '更新应用', description: '必须携带 id；修改权限或有效期。',
        body: { id: 1, name: 'MCP 客户端', scopes: ['crons', 'logs', 'mcp'], expiresAt: '2026-12-31' },
        response: { code: 200, data: { id: 1, name: 'MCP 客户端' } },
      },
      {
        id: 'apps-delete', method: 'DELETE', path: '/apps', tabs: apiOnly,
        summary: '删除应用', description: '请求体为 ID 数组，同时撤销该应用全部令牌。',
        body: [1],
        response: { code: 200, data: null },
      },
      {
        id: 'apps-reset-secret', method: 'PUT', path: '/apps/:id/reset-secret', tabs: apiOnly,
        summary: '重置 Client Secret', description: '撤销已有 token，返回新 secret 并保存为可查看密钥。',
        params: [{ name: 'id', in: 'path', required: true, type: 'number', description: '应用 ID', example: '1' }],
        response: { code: 200, data: { id: 1, client_id: 'Abc123…', client_secret: 'NewSecret…' } },
      },
      {
        id: 'apps-token', method: 'POST', path: '/apps/:id/token', tabs: apiOnly,
        summary: '创建 Authorization', description: 'expiresIn 秒或 permanent:true；到期返回 403。Authorization 与 Client ID 用途不同。',
        params: [{ name: 'id', in: 'path', required: true, type: 'number', description: '应用 ID', example: '1' }],
        body: { expiresIn: 86400, permanent: false },
        response: { code: 200, data: { token: '…', token_type: 'Bearer', expiration: 1790000000, expiresIn: 86400, permanent: false, id: 9, client_id: 'Abc123…' } },
      },
      {
        id: 'apps-tokens', method: 'GET', path: '/apps/:id/tokens', tabs: apiOnly,
        summary: 'Authorization 列表', description: '管理员读取该应用的全部令牌登记，刷新页面后可恢复显示。',
        params: [{ name: 'id', in: 'path', required: true, type: 'number', description: '应用 ID', example: '1' }],
        response: { code: 200, data: [{ id: 9, appId: 1, token: '…', name: 'MCP 客户端', created: '2026-10-04T08:00:00Z', expires: 1790000000, permanent: false, active: true }] },
      },
      {
        id: 'apps-token-delete', method: 'DELETE', path: '/apps/:id/tokens/:tokenId', tabs: apiOnly,
        summary: '撤销 Authorization', description: '同时删除令牌登记与会话，立即失效。',
        params: [
          { name: 'id', in: 'path', required: true, type: 'number', description: '应用 ID', example: '1' },
          { name: 'tokenId', in: 'path', required: true, type: 'number', description: '令牌记录 ID', example: '9' },
        ],
        response: { code: 200, data: null },
      },
    ],
  },
  {
    id: 'backups',
    label: '备份恢复',
    icon: 'DatabaseBackup',
    intro: '仅管理员。备份包含任务、变量、应用、账号、配置、脚本和最近运行记录；导入前自动创建安全备份。',
    endpoints: [
      {
        id: 'backups-list', method: 'GET', path: '/backups', tabs: apiOnly,
        summary: '本地备份列表', description: '返回备份文件名、大小和创建时间。',
        response: { code: 200, data: [{ name: 'cabbage-20261004.zip', size: 1048576, created: '2026-10-04T08:00:00Z' }] },
      },
      {
        id: 'backups-export', method: 'POST', path: '/backups/export', tabs: apiOnly,
        summary: '创建备份', description: '生成 ZIP，返回备份文件名。运行和排队任务需清空。',
        response: { code: 200, data: { name: 'cabbage-20261004.zip' } },
      },
      {
        id: 'backups-download', method: 'GET', path: '/backups/:name.zip', tabs: apiOnly,
        summary: '下载备份', description: '以附件形式下载 ZIP。',
        params: [{ name: 'name', in: 'path', required: true, type: 'string', description: '备份文件名（不含 .zip）', example: 'cabbage-20261004' }],
      },
      {
        id: 'backups-delete', method: 'DELETE', path: '/backups/:name.zip', tabs: apiOnly,
        summary: '删除备份', description: '从磁盘永久删除。',
        params: [{ name: 'name', in: 'path', required: true, type: 'string', description: '备份文件名（不含 .zip）', example: 'cabbage-20261004' }],
        response: { code: 200, data: null },
      },
      {
        id: 'backups-import', method: 'POST', path: '/backups/import', tabs: apiOnly, upload: true,
        summary: '导入备份', description: 'multipart/form-data 的 file 字段，或直接提交 application/zip。导入会覆盖账号、任务、变量、应用、脚本和配置，并撤销所有会话。',
      },
    ],
  },
  {
    id: 'mcp',
    label: 'MCP',
    icon: 'Plug',
    intro: 'POST /mcp 为无状态 Streamable HTTP，JSON-RPC 2.0。应用需要 mcp 权限以及对应工具的数据权限；无权限工具不会出现在 tools/list。',
    endpoints: [
      {
        id: 'mcp-call', method: 'POST', path: '/mcp', absolute: true, tabs: apiOnly,
        summary: 'MCP JSON-RPC', description: '无状态 Streamable HTTP，支持 initialize、ping、tools/list、tools/call，以及返回空列表的 resources/list、prompts/list。当前提供 18 个工具（任务、变量、脚本、日志、订阅、依赖），复用应用与资源权限。Header 需带 Authorization: Bearer <token>，Content-Type 必须为 application/json。',
        body: { jsonrpc: '2.0', id: 1, method: 'tools/list', params: {} },
        response: { code: 200, data: { jsonrpc: '2.0', id: 1, result: { tools: [{ name: 'list_crons', description: '列出定时任务' }] } } },
      },
    ],
  },
]
