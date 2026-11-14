/* 大白菜面板 · 纯前端交互演示。所有数据都在浏览器内存中，不会发起任何后端请求。 */
(() => {
  'use strict'

  const icons = {
    dashboard: '<rect x="3" y="3" width="7" height="7" rx="2"/><rect x="14" y="3" width="7" height="7" rx="2"/><rect x="3" y="14" width="7" height="7" rx="2"/><rect x="14" y="14" width="7" height="7" rx="2"/>',
    clock: '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>',
    key: '<circle cx="8" cy="15" r="4"/><path d="m11 12 8-8M16 4l4 4M13 7l4 4"/>',
    terminal: '<rect x="3" y="4" width="18" height="16" rx="2"/><path d="m7 9 3 2-3 2M13 14h4"/>',
    file: '<path d="M6 3h8l4 4v14H6z"/><path d="M14 3v5h5M9 13h6M9 17h6"/>',
    git: '<circle cx="6" cy="5" r="2"/><circle cx="6" cy="19" r="2"/><circle cx="18" cy="7" r="2"/><path d="M6 7v10M18 9c0 4-6 3-6 8"/>',
    package: '<path d="m12 3 8 4.5v9L12 21l-8-4.5v-9z"/><path d="m4 7.5 8 4.5 8-4.5M12 12v9"/>',
    book: '<path d="M4 5.5A2.5 2.5 0 0 1 6.5 3H20v18H6.5A2.5 2.5 0 0 0 4 23z"/><path d="M4 5.5v15M8 7h8M8 11h6"/>',
    settings: '<path d="M4 6h16M4 12h16M4 18h16"/><circle cx="8" cy="6" r="2" fill="var(--surface)"/><circle cx="16" cy="12" r="2" fill="var(--surface)"/><circle cx="10" cy="18" r="2" fill="var(--surface)"/>',
    search: '<circle cx="11" cy="11" r="7"/><path d="m20 20-4.3-4.3"/>',
    bell: '<path d="M6 9a6 6 0 0 1 12 0c0 7 3 7 3 9H3c0-2 3-2 3-9"/><path d="M10 21h4"/>',
    play: '<path d="m8 5 11 7-11 7z" fill="currentColor" stroke="none"/>',
    stop: '<rect x="6" y="6" width="12" height="12" rx="2" fill="currentColor" stroke="none"/>',
    refresh: '<path d="M20 7v5h-5M4 17v-5h5"/><path d="M6.1 9a7 7 0 0 1 11.6-2L20 12M4 12l2.3 5a7 7 0 0 0 11.6-2"/>',
    x: '<path d="M6 6l12 12M18 6 6 18"/>',
    plus: '<path d="M12 5v14M5 12h14"/>',
    check: '<path d="m5 12 4 4L19 6"/>',
    alert: '<path d="M12 3 2 21h20z"/><path d="M12 9v5M12 17h.01"/>',
    menu: '<path d="M4 7h16M4 12h16M4 17h16"/>',
    more: '<circle cx="5" cy="12" r="1.5" fill="currentColor" stroke="none"/><circle cx="12" cy="12" r="1.5" fill="currentColor" stroke="none"/><circle cx="19" cy="12" r="1.5" fill="currentColor" stroke="none"/>',
    trash: '<path d="M4 7h16M9 7V4h6v3M7 7l1 14h8l1-14"/>',
    download: '<path d="M12 3v12M7 10l5 5 5-5M4 21h16"/>',
    upload: '<path d="M12 21V9M7 14l5-5 5 5M4 3h16"/>',
    copy: '<rect x="9" y="9" width="11" height="11" rx="2"/><path d="M5 15V5a2 2 0 0 1 2-2h8"/>',
    shield: '<path d="M12 3 5 6v5c0 5 3 8 7 10 4-2 7-5 7-10V6z"/><path d="m9 12 2 2 4-4"/>',
    cpu: '<rect x="6" y="6" width="12" height="12" rx="2"/><rect x="9" y="9" width="6" height="6" rx="1"/><path d="M9 2v4M15 2v4M9 18v4M15 18v4M2 9h4M2 15h4M18 9h4M18 15h4"/>',
    activity: '<path d="M3 12h4l2-6 4 12 2-6h6"/>',
    folder: '<path d="M3 6a2 2 0 0 1 2-2h5l2 2h7a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/>',
    code: '<path d="m8 8-4 4 4 4M16 8l4 4-4 4"/>',
    chevron: '<path d="m9 6 6 6-6 6"/>',
    eye: '<path d="M2 12s3.5-6 10-6 10 6 10 6-3.5 6-10 6S2 12 2 12z"/><circle cx="12" cy="12" r="2.5"/>',
    eyeOff: '<path d="m3 3 18 18M10.6 6.2A10.8 10.8 0 0 1 12 6c6.5 0 10 6 10 6a17 17 0 0 1-3 3.5M6.6 6.6C3.6 8.5 2 12 2 12s3.5 6 10 6c1.4 0 2.7-.2 3.8-.6"/><path d="M9.9 9.9a3 3 0 0 0 4.2 4.2"/>',
    circle: '<circle cx="12" cy="12" r="8"/>',
    pin: '<path d="M12 17v5M9 3h6l-1 6 3 3H7l3-3z"/>',
    arrowUp: '<path d="M12 19V5M6 11l6-6 6 6"/>',
    arrowDown: '<path d="M12 5v14M6 13l6 6 6-6"/>',
    square: '<rect x="6" y="6" width="12" height="12" rx="2"/>',
    fileCode: '<path d="M6 3h8l4 4v14H6z"/><path d="M14 3v5h5M10 12l-2 2 2 2M14 12l2 2-2 2"/>',
    folderPlus: '<path d="M3 6a2 2 0 0 1 2-2h5l2 2h7a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/><path d="M12 11v6M9 14h6"/>',
    filePlus: '<path d="M6 3h8l4 4v14H6z"/><path d="M14 3v5h5M12 11v6M9 14h6"/>',
    link: '<path d="M10 13a5 5 0 0 0 7 0l2-2a5 5 0 0 0-7-7l-1 1"/><path d="M14 11a5 5 0 0 0-7 0l-2 2a5 5 0 0 0 7 7l1-1"/>',
    maximize: '<path d="M8 3H3v5M16 3h5v5M21 16v5h-5M3 16v5h5"/>',
    undo: '<path d="M9 14 4 9l5-5"/><path d="M4 9h10a6 6 0 0 1 0 12h-3"/>',
    info: '<circle cx="12" cy="12" r="9"/><path d="M12 11v5M12 8h.01"/>',
    pause: '<circle cx="12" cy="12" r="9"/><path d="M10 9v6M14 9v6"/>',
    move: '<path d="M5 9V6a2 2 0 0 1 2-2h3M19 9V6a2 2 0 0 0-2-2h-3M5 15v3a2 2 0 0 0 2 2h3M19 15v3a2 2 0 0 1-2 2h-3"/><path d="M12 8v8M8 12h8"/>',
    chevronDown: '<path d="m6 9 6 6 6-6"/>',
    chevronLeft: '<path d="m15 6-6 6 6 6"/>',
    chevronRight: '<path d="m9 6 6 6-6 6"/>',
    loader: '<path d="M12 3v4M12 17v4M3 12h4M17 12h4M5.6 5.6l2.8 2.8M15.6 15.6l2.8 2.8M18.4 5.6l-2.8 2.8M8.4 15.6l-2.8 2.8"/>',
    edit: '<path d="M4 20h4l10-10a2.8 2.8 0 0 0-4-4L4 16z"/><path d="m13.5 6.5 4 4"/>'
  }

  function icon(name, size = 18) {
    return `<svg width="${size}" height="${size}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${icons[name] || icons.circle}</svg>`
  }

  const navItems = [
    { id: 'dashboard', label: '仪表盘', icon: 'dashboard' },
    { id: 'tasks', label: '定时任务', icon: 'clock' },
    { id: 'envs', label: '环境变量', icon: 'key' },
    { id: 'scripts', label: '脚本管理', icon: 'terminal' },
    { id: 'logs', label: '运行日志', icon: 'file' },
    { id: 'subscriptions', label: '订阅管理', icon: 'git' },
    { id: 'dependencies', label: '依赖管理', icon: 'package' },
    { id: 'api-docs', label: '接口文档', icon: 'book' },
    { id: 'settings', label: '系统设置', icon: 'settings' }
  ]

  const statusLabel = { success: '成功', failed: '失败', running: '运行中', cancelled: '已停止', idle: '待运行', installing: '安装中', installed: '已安装' }

  const tasks = [
    { id: 1, name: '数据备份', command: 'task demo/backup.py', schedule: '0 8 * * *', enabled: true, status: 'idle', last: '今天 08:00', duration: '12.4s', remarks: '每天备份业务数据到归档目录', labels: ['日常', '备份'], timeout: 0, pinned: false, allowConcurrent: false, notifyOnFinish: false, nextRun: '明天 08:00' },
    { id: 2, name: '报表导出', command: 'task demo/report.py', schedule: '5 8 * * *', enabled: true, status: 'success', last: '今天 08:05', duration: '8.1s', remarks: '导出前一日统计报表', labels: ['报表'], timeout: 600, pinned: true, allowConcurrent: false, notifyOnFinish: true, nextRun: '明天 08:05' },
    { id: 3, name: '健康检查', command: 'task demo/health.js', schedule: '0 */6 * * *', enabled: true, status: 'idle', last: '今天 12:00', duration: '31.2s', remarks: '探测接口与依赖可用性', labels: ['监控'], timeout: 0, pinned: false, allowConcurrent: true, notifyOnFinish: false, nextRun: '今天 18:00' },
    { id: 4, name: '订阅同步', command: 'task demo/sync.py', schedule: '0 3 * * *', enabled: false, status: 'idle', last: '昨天 03:00', duration: '4.0s', remarks: '拉取脚本仓库更新', labels: ['同步'], timeout: 0, pinned: false, allowConcurrent: false, notifyOnFinish: false, nextRun: '已停用' },
    { id: 5, name: '日报推送', command: 'task demo/notify.py', schedule: '30 9 * * *', enabled: true, status: 'failed', last: '今天 09:30', duration: '1.2s', remarks: '推送日报到通知渠道', labels: ['通知'], timeout: 60, pinned: false, allowConcurrent: false, notifyOnFinish: true, nextRun: '明天 09:30' }
  ]

  const runs = [
    { id: 101, taskId: 2, name: '报表导出', status: 'success', started: '今天 08:05', duration: 8.1 },
    { id: 102, taskId: 5, name: '日报推送', status: 'failed', started: '今天 09:30', duration: 1.2 },
    { id: 103, taskId: 1, name: '数据备份', status: 'success', started: '今天 08:00', duration: 12.4 },
    { id: 104, taskId: 3, name: '健康检查', status: 'success', started: '今天 12:00', duration: 31.2 }
  ]

  const envs = [
    { id: 1, name: 'DEMO_TOKEN', value: 'demo-value', remarks: '示例令牌', enabled: true, pinned: false, updatedAt: '今天 09:12' },
    { id: 2, name: 'API_ENDPOINT', value: 'https://example.com/api', remarks: '接口地址', enabled: true, pinned: true, updatedAt: '昨天 18:30' },
    { id: 3, name: 'NOTIFY_KEY', value: 'demo-notify-key', remarks: '通知密钥', enabled: false, pinned: false, updatedAt: '11月12日' }
  ]

  const subscriptions = [
    { id: 1, name: '示例脚本仓库', type: 'git-repo', url: 'https://github.com/example/scripts.git', alias: 'example', schedule: '0 */6 * * *', autoAddCron: true, autoDelCron: false, files: 12, lastSync: '今天 12:00', syncing: false, enabled: true },
    { id: 2, name: '单文件订阅', type: 'file', url: 'https://example.com/daily.py', alias: 'daily', schedule: '0 8 * * *', autoAddCron: true, autoDelCron: true, files: 1, lastSync: '昨天 08:00', syncing: false, enabled: true },
    { id: 3, name: '归档脚本仓库', type: 'git-repo', url: 'https://github.com/example/archive.git', alias: 'archive', schedule: '', autoAddCron: false, autoDelCron: false, files: 5, lastSync: '11月10日', syncing: false, enabled: false }
  ]

  const dependencies = [
    { id: 1, name: 'requests', type: 1, version: '2.32.5', status: 'installed', remark: 'HTTP 请求库' },
    { id: 2, name: 'axios', type: 0, version: '1.12.2', status: 'installed', remark: '前端 HTTP 客户端' },
    { id: 3, name: 'pycryptodome', type: 1, version: '3.21.0', status: 'installing', remark: '加密算法库' },
    { id: 4, name: 'curl', type: 2, version: '8.5.0', status: 'installed', remark: '命令行网络工具' },
    { id: 5, name: 'dayjs', type: 0, version: '1.11.13', status: 'installed', remark: '轻量日期库' },
    { id: 6, name: 'pyyaml', type: 1, version: '6.0.2', status: 'failed', remark: 'YAML 解析' }
  ]

  const scriptFiles = [
    {
      path: 'demo/backup.py',
      content: `# cron "0 8 * * *"\n# new Env("DEMO_TOKEN")\n\nimport os\nimport time\n\ntoken = os.environ.get("DEMO_TOKEN", "")\n\nif not token:\n    raise SystemExit("缺少环境变量 DEMO_TOKEN")\n\nprint("开始执行备份任务")\ntime.sleep(1)\nprint("已读取配置，token 长度:", len(token))\nprint("备份完成（演示输出）")\n`,
      size: 312, updatedAt: '今天 09:20'
    },
    {
      path: 'demo/report.py',
      content: `# cron: 5 8 * * *\n# new Env("API_ENDPOINT")\n\nimport os\n\nendpoint = os.environ.get("API_ENDPOINT")\nif not endpoint:\n    raise SystemExit("缺少 API_ENDPOINT")\n\nprint("报表导出任务开始")\nprint("目标接口:", endpoint)\nprint("导出完成（演示输出）")\n`,
      size: 268, updatedAt: '昨天 18:40'
    },
    {
      path: 'demo/health.js',
      content: `// cron: 0 */6 * * *\n// new Env('DEMO_TOKEN')\n\nconst token = process.env.DEMO_TOKEN\nif (!token) {\n  console.error('缺少 DEMO_TOKEN')\n  process.exit(1)\n}\n\nconsole.log('健康检查任务开始')\nsetTimeout(() => console.log('检查完成（演示输出）'), 600)\n`,
      size: 274, updatedAt: '11月13日'
    },
    {
      path: 'demo/sync.py',
      content: `# cron: 0 3 * * *\n\nimport time\n\nprint("开始同步订阅")\ntime.sleep(1)\nprint("同步完成（演示输出）")\n`,
      size: 118, updatedAt: '昨天 03:00'
    },
    {
      path: 'demo/notify.py',
      content: `# cron: 30 9 * * *\n# new Env("NOTIFY_KEY")\n\nimport os\nfrom notify import sendNotify\n\nkey = os.environ.get("NOTIFY_KEY")\nif not key:\n    raise SystemExit("缺少 NOTIFY_KEY")\n\nsendNotify("每日简报", "任务执行完成（演示输出）")\n`,
      size: 214, updatedAt: '今天 09:30'
    },
    {
      path: 'tools/notify.py',
      content: `from notify import sendNotify\n\nsendNotify("大白菜面板", "这是一条演示通知")\n`,
      size: 92, updatedAt: '11月12日'
    },
    {
      path: 'config.sh',
      content: `# 全局 Shell 配置示例\n# 任务执行前加载，变量会自动导出给脚本。\n\nexport CABBAGE_DEMO=1\n`,
      size: 108, updatedAt: '11月11日'
    }
  ]

  const scriptHistory = [
    { id: 1, path: 'demo/backup.py', savedAt: '今天 09:20', size: 312 },
    { id: 2, path: 'demo/backup.py', savedAt: '今天 08:50', size: 298 },
    { id: 3, path: 'demo/report.py', savedAt: '昨天 18:40', size: 268 }
  ]

  const apiGroups = [
    {
      id: 'auth', label: '鉴权与账号', icon: 'shield',
      intro: '网页使用同源 HttpOnly Cookie；开放应用先换取 Bearer token，再访问 /open 资源。成功响应为 {code:200,data:…}，错误为对应 HTTP 状态与 {code,message}。',
      endpoints: [
        {
          id: 'healthz', method: 'GET', path: '/healthz', absolute: true, noAuth: true, tabs: ['api'],
          summary: '健康检查', description: '无鉴权，返回服务是否存活。',
          response: { code: 200, data: { status: 'ok' } }
        },
        {
          id: 'user-login', method: 'POST', path: '/user/login', noAuth: true, tabs: ['api'],
          summary: '管理员登录', description: '成功后写入 HttpOnly 会话 Cookie，同时返回 Bearer token。15 分钟内失败过多会被限流。',
          body: { username: 'admin', password: 'a-strong-password' },
          response: { code: 200, data: { token: '…', token_type: 'Bearer', expiration: 1790000000, username: 'admin' } }
        },
        {
          id: 'user-get', method: 'GET', path: '/user', tabs: ['api'],
          summary: '当前账号', description: '仅管理员。',
          response: { code: 200, data: { username: 'admin', avatar: '', twoFactorActivated: false, role: 'admin' } }
        },
        {
          id: 'open-token', method: 'POST', path: '/auth/token', noAuth: true, tabs: ['open'],
          summary: '应用换取令牌', description: '使用 client_id / client_secret 换取带权限的 Bearer token。',
          body: { client_id: 'example-client', client_secret: 'example-secret' },
          response: { code: 200, data: { token: '…', token_type: 'Bearer', expiration: 1790000000 } }
        },
        {
          id: 'user-login-log', method: 'GET', path: '/user/login-log', tabs: ['api'],
          summary: '登录日志', description: '仅管理员，最多保留最近 100 条，形状与青龙 login-log 一致。status：0 成功、1 失败。',
          response: { code: 200, data: [{ timestamp: 1790000000000, address: '内网IP', ip: '192.168.1.9', platform: 'desktop', status: 0 }] }
        }
      ]
    },
    {
      id: 'crons', label: '定时任务', icon: 'clock',
      intro: '任务命令统一使用 task 前缀；schedule 留空表示仅手动执行。',
      endpoints: [
        {
          id: 'crons-list', method: 'GET', path: '/crons', tabs: ['open', 'api'], scope: 'crons',
          summary: '任务列表', description: '返回任务数组，包含最近运行状态、下次执行时间和运行中数量。',
          params: [{ name: 'searchValue', in: 'query', type: 'string', description: '按名称或命令搜索' }],
          response: { code: 200, data: [{ id: 1, name: '数据备份', command: 'task demo/backup.py', schedule: '0 8 * * *', isDisabled: 0 }] }
        },
        {
          id: 'crons-create', method: 'POST', path: '/crons', tabs: ['open', 'api'], scope: 'crons',
          summary: '创建任务', description: '对象或数组；支持 name、command、schedule、timeout、allowConcurrent、notifyOnFinish。',
          body: { name: '数据备份', command: 'task demo/backup.py', schedule: '0 8 * * *' },
          response: { code: 200, data: { id: 1, name: '数据备份' } }
        },
        {
          id: 'crons-update', method: 'PUT', path: '/crons', tabs: ['open', 'api'], scope: 'crons',
          summary: '更新任务', description: '按 id 合并字段，保留未提交的内容。',
          body: { id: 1, schedule: '0 9 * * *' },
          response: { code: 200, data: { id: 1, schedule: '0 9 * * *' } }
        },
        {
          id: 'crons-run', method: 'PUT', path: '/crons/run', tabs: ['open', 'api'], scope: 'crons',
          summary: '运行任务', description: 'ID 数组，异步执行并返回 runId。',
          body: [1, 2],
          response: { code: 200, data: [{ runId: 101, status: 'queued' }] }
        },
        {
          id: 'crons-delete', method: 'DELETE', path: '/crons', tabs: ['open', 'api'], scope: 'crons',
          summary: '删除任务', description: 'ID 数组，永久删除任务定义。',
          body: [1],
          response: { code: 200, data: null }
        }
      ]
    },
    {
      id: 'envs', label: '环境变量', icon: 'key',
      intro: '启用的变量注入任务进程；同名变量按排序靠后的值生效。',
      endpoints: [
        {
          id: 'envs-list', method: 'GET', path: '/envs', tabs: ['open', 'api'], scope: 'envs',
          summary: '变量列表', description: '返回变量名、值、备注、状态、标签和排序信息。',
          params: [{ name: 'searchValue', in: 'query', type: 'string', description: '按名称或备注搜索' }],
          response: { code: 200, data: [{ id: 1, name: 'DEMO_TOKEN', value: 'demo-value', remarks: '示例令牌', status: 0 }] }
        },
        {
          id: 'envs-create', method: 'POST', path: '/envs', tabs: ['open', 'api'], scope: 'envs',
          summary: '新增变量', description: '对象数组；name 和 value 必填。',
          body: [{ name: 'DEMO_TOKEN', value: 'demo-value', remarks: '示例令牌' }],
          response: { code: 200, data: [{ id: 1, name: 'DEMO_TOKEN' }] }
        },
        {
          id: 'envs-update', method: 'PUT', path: '/envs', tabs: ['open', 'api'], scope: 'envs',
          summary: '修改变量', description: '提交 id 和需要更新的字段。',
          body: { id: 1, value: 'new-value' },
          response: { code: 200, data: { id: 1, value: 'new-value' } }
        },
        {
          id: 'envs-delete', method: 'DELETE', path: '/envs', tabs: ['open', 'api'], scope: 'envs',
          summary: '删除变量', description: 'ID 数组。',
          body: [1],
          response: { code: 200, data: null }
        }
      ]
    },
    {
      id: 'scripts', label: '脚本与文件', icon: 'terminal',
      intro: '脚本目录和配置目录使用同一套文件 API，拒绝路径穿越和符号链接。',
      endpoints: [
        {
          id: 'scripts-tree', method: 'GET', path: '/scripts/tree', tabs: ['open', 'api'], scope: 'scripts',
          summary: '文件树', description: '返回目录、文件、大小和修改时间。',
          response: { code: 200, data: [{ name: 'backup.py', path: 'demo/backup.py', isLeaf: true, size: 512 }] }
        },
        {
          id: 'scripts-detail', method: 'GET', path: '/scripts/detail', tabs: ['open', 'api'], scope: 'scripts',
          summary: '读取脚本', description: '查询参数 path 为相对路径，最大读取 4MB。',
          params: [{ name: 'path', in: 'query', required: true, type: 'string', description: '脚本相对路径' }],
          response: { code: 200, data: "print('hello')" }
        },
        {
          id: 'scripts-save', method: 'PUT', path: '/scripts', tabs: ['open', 'api'], scope: 'scripts',
          summary: '保存脚本', description: '提交 filename 与 content，覆盖前保存历史版本。',
          body: { filename: 'demo/backup.py', content: "print('hello')" },
          response: { code: 200, data: { path: 'demo/backup.py' } }
        },
        {
          id: 'scripts-ops', method: 'POST', path: '/scripts/operations', tabs: ['open', 'api'], scope: 'scripts',
          summary: '文件操作', description: 'move / copy / rename / delete，一次最多 500 项。',
          body: { operation: 'rename', paths: ['demo/backup.py'], destination: 'demo/backup2.py' },
          response: { code: 200, data: [{ path: 'demo/backup.py', success: true }] }
        }
      ]
    },
    {
      id: 'logs', label: '运行日志', icon: 'file',
      intro: '日志目录树与面板运行记录分开：/logs 返回目录树，/logs/runs 返回执行记录。',
      endpoints: [
        {
          id: 'logs-runs', method: 'GET', path: '/logs/runs', tabs: ['open', 'api'], scope: 'logs',
          summary: '运行记录', description: '支持 kind、taskId、limit 参数。',
          params: [
            { name: 'kind', in: 'query', type: 'string', description: 'crons / subscriptions / dependencies' },
            { name: 'limit', in: 'query', type: 'number', description: '默认 200，最大 1000' }
          ],
          response: { code: 200, data: [{ id: 101, name: '数据备份', status: 'success', duration: 12.4 }] }
        },
        {
          id: 'logs-read', method: 'GET', path: '/logs/run/:id', tabs: ['open', 'api'], scope: 'logs',
          summary: '读取日志', description: '按 runId 返回日志内容与运行状态。',
          params: [{ name: 'id', in: 'path', required: true, type: 'number', description: '运行 ID' }],
          response: { code: 200, data: { content: '任务输出…', status: 'success' } }
        },
        {
          id: 'logs-delete', method: 'DELETE', path: '/logs', tabs: ['open', 'api'], scope: 'logs',
          summary: '删除日志', description: 'ID 数组；运行中的日志不能删除。',
          body: [101],
          response: { code: 200, data: null }
        }
      ]
    },
    {
      id: 'subscriptions', label: '订阅同步', icon: 'git',
      intro: '支持 Git 仓库和单文件；脚本头部 cron 注释可自动创建任务。',
      endpoints: [
        {
          id: 'subs-list', method: 'GET', path: '/subscriptions', tabs: ['open', 'api'], scope: 'subscriptions',
          summary: '订阅列表', description: '附带运行状态、上次同步时间和文件数量。',
          response: { code: 200, data: [{ id: 1, name: '示例脚本仓库', type: 'git-repo', alias: 'example', schedule: '0 */6 * * *' }] }
        },
        {
          id: 'subs-create', method: 'POST', path: '/subscriptions', tabs: ['open', 'api'], scope: 'subscriptions',
          summary: '新增订阅', description: 'type 取 git-repo 或 file；URL 必须为 HTTP(S)。',
          body: { name: '示例脚本仓库', type: 'git-repo', url: 'https://github.com/example/scripts.git', alias: 'example', schedule: '0 */6 * * *', autoAddCron: true },
          response: { code: 200, data: { id: 1, name: '示例脚本仓库' } }
        },
        {
          id: 'subs-run', method: 'PUT', path: '/subscriptions/run', tabs: ['open', 'api'], scope: 'subscriptions',
          summary: '同步订阅', description: 'ID 数组，后台拉取并写入 scripts/别名/。',
          body: [1],
          response: { code: 200, data: [{ runId: 201, status: 'queued' }] }
        }
      ]
    },
    {
      id: 'dependencies', label: '依赖管理', icon: 'package',
      intro: 'Node.js 使用独立 node_modules，Python 使用专属 venv，系统依赖仅 Linux。',
      endpoints: [
        {
          id: 'deps-list', method: 'GET', path: '/dependencies', tabs: ['open', 'api'], scope: 'dependencies',
          summary: '依赖列表', description: '包含安装状态和最近安装日志。',
          response: { code: 200, data: [{ id: 1, name: 'requests', type: 1, status: 0 }] }
        },
        {
          id: 'deps-create', method: 'POST', path: '/dependencies', tabs: ['open', 'api'], scope: 'dependencies',
          summary: '安装依赖', description: 'type：0 Node.js、1 Python、2 系统包。',
          body: { name: 'requests', type: 1 },
          response: { code: 200, data: { id: 1, name: 'requests', status: 1 } }
        },
        {
          id: 'deps-reinstall', method: 'PUT', path: '/dependencies/reinstall', tabs: ['open', 'api'], scope: 'dependencies',
          summary: '重新安装', description: 'ID 数组，异步重新安装。',
          body: [1],
          response: { code: 200, data: [{ runId: 301, status: 'queued' }] }
        }
      ]
    },
    {
      id: 'system', label: '系统与 MCP', icon: 'activity',
      intro: '管理接口仅管理员可用；MCP 还需要应用具备 mcp 权限。',
      endpoints: [
        {
          id: 'system-activity', method: 'GET', path: '/system/activity', tabs: ['api'],
          summary: '活动状态', description: '返回运行中 / 排队的定时任务数量，用于侧边栏徽标轮询。',
          response: { code: 200, data: { running: 1, queued: 0, active: 1 } }
        },
        {
          id: 'system-stats', method: 'GET', path: '/system/stats', tabs: ['api'],
          summary: '资源统计', description: 'CPU、内存、磁盘、执行统计和最近运行。',
          response: { code: 200, data: { tasks: 5, running: 1, today: 16, successRate: 96.8, cpu: 12.4, memory: 43.1 } }
        },
        {
          id: 'mcp', method: 'POST', path: '/mcp', absolute: true, tabs: ['api'],
          summary: 'MCP JSON-RPC', description: '支持 initialize、tools/list、tools/call、resources/list、resources/read。Header 需带 Authorization: Bearer <token>。',
          body: { jsonrpc: '2.0', id: 1, method: 'tools/list', params: {} },
          response: { code: 200, data: { jsonrpc: '2.0', id: 1, result: { tools: [{ name: 'list_tasks' }] } } }
        }
      ]
    }
  ]

  const demoModalNotice = `这是纯前端演示。\n\n页面里的运行、同步、安装、保存等操作都只修改当前浏览器内存，不会请求后端，也不会影响任何真实任务。\n\n刷新页面即可恢复初始数据。`

  const state = {
    page: 'dashboard',
    tabs: loadTabs(),
    sidebarOpen: false,
    tabsMenu: false,
    taskSearch: '',
    taskFilter: 'all',
    taskSort: { key: '', dir: 'asc' },
    taskSelected: [],
    taskPage: 1,
    taskPageSize: 20,
    taskMenu: 0,
    envSearch: '',
    envFilter: 'all',
    envSort: { key: '', dir: 'asc' },
    envSelected: [],
    envPage: 1,
    envPageSize: 20,
    revealed: new Set(),
    scriptSearch: '',
    scriptSelected: [],
    scriptExpanded: new Set(['demo']),
    scriptFullscreen: false,
    apiSearch: '',
    apiTab: 'api',
    apiOpenEndpoints: new Set(),
    settingsTab: 'general',
    depCategory: 0,
    depSelected: [],
    subSearch: '',
    subSelected: [],
    activeFile: scriptFiles[0].path,
    scriptContent: scriptFiles[0].content,
    tasks: tasks.map(task => ({ ...task })),
    runs: runs.map(run => ({ ...run })),
    envs: envs.map(env => ({ ...env })),
    subscriptions: subscriptions.map(sub => ({ ...sub })),
    dependencies: dependencies.map(dep => ({ ...dep })),
    timers: new Map(),
    nextRunId: 200
  }

  const view = document.getElementById('view')
  const nav = document.getElementById('nav')
  const tabs = document.getElementById('tabs')
  const tabsMenu = document.getElementById('tabs-menu')
  const searchPop = document.getElementById('search-pop')
  const modalRoot = document.getElementById('modal-root')
  const toastRoot = document.getElementById('toast-root')
  const scrollPositions = new Map()

  function loadTabs() {
    try {
      const saved = JSON.parse(localStorage.getItem('cabbage-demo-tabs') || '[]')
      const valid = (Array.isArray(saved) ? saved : []).filter((id, index, list) => navItems.some(item => item.id === id) && list.indexOf(id) === index)
      return ['dashboard', ...valid.filter(id => id !== 'dashboard')]
    } catch {
      return ['dashboard']
    }
  }

  function saveTabs() {
    try { localStorage.setItem('cabbage-demo-tabs', JSON.stringify(state.tabs)) } catch { /* ignore */ }
  }

  function escapeHTML(value) {
    return String(value ?? '').replace(/[&<>"']/g, char => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[char]))
  }

  function runningCount() {
    return state.tasks.filter(task => task.running).length
  }

  function navItem(id) {
    return navItems.find(item => item.id === id) || navItems[0]
  }

  function statusBadge(status) {
    return `<span class="badge ${status}">${statusLabel[status] || status}</span>`
  }

  function latestRun(taskId) {
    return state.runs.find(run => run.taskId === taskId)
  }

  function pageHeading(eyebrow, title, description, actions = '') {
    return `<div class="page-heading"><div><span class="eyebrow">${eyebrow}</span><h1>${title}</h1><p>${description}</p></div><div class="heading-actions">${actions}</div></div>`
  }

  function renderNav() {
    const running = runningCount()
    nav.innerHTML = navItems.map(item => {
      const active = state.page === item.id
      const showBadge = item.id === 'tasks' && running > 0
      const trailing = showBadge || active
      return `<button type="button" class="${active ? 'active' : ''}" data-nav="${item.id}">${icon(item.icon)}<span>${item.label}</span>${trailing ? `<span class="nav-trailing">${showBadge ? `<span class="nav-badge">${running > 99 ? '99+' : running}</span>` : ''}${active ? '<span class="nav-dot"></span>' : ''}</span>` : ''}</button>`
    }).join('')
  }

  function renderTabs() {
    tabs.innerHTML = state.tabs.map(id => {
      const item = navItem(id)
      return `<div class="page-tab ${state.page === id ? 'active' : ''}" role="presentation"><button type="button" class="page-tab-main" role="tab" aria-selected="${state.page === id}" data-nav="${id}">${icon(item.icon, 14)}<span>${item.label}</span></button>${id !== 'dashboard' ? `<button type="button" class="page-tab-close" data-close-tab="${id}" aria-label="关闭 ${item.label}">${icon('x', 12)}</button>` : ''}</div>`
    }).join('')
    tabsMenu.hidden = !state.tabsMenu
    requestAnimationFrame(scrollActiveTab)
  }

  function scrollActiveTab() {
    const active = tabs.querySelector('.page-tab.active')
    if (!active) return
    const left = active.offsetLeft
    const right = left + active.offsetWidth
    if (left < tabs.scrollLeft) tabs.scrollTo({ left: Math.max(0, left - 10), behavior: 'smooth' })
    else if (right > tabs.scrollLeft + tabs.clientWidth) tabs.scrollTo({ left: right - tabs.clientWidth + 10, behavior: 'smooth' })
  }

  function openPage(id) {
    if (!navItems.some(item => item.id === id)) id = 'dashboard'
    if (state.page !== id) scrollPositions.set(state.page, window.scrollY)
    if (!state.tabs.includes(id)) {
      state.tabs.push(id)
      saveTabs()
    }
    state.page = id
    state.sidebarOpen = false
    state.tabsMenu = false
    renderAll()
    requestAnimationFrame(() => window.scrollTo({ top: scrollPositions.get(id) || 0, behavior: 'auto' }))
    document.title = `${navItem(id).label} · 大白菜面板演示`
  }

  function closeTab(id) {
    if (id === 'dashboard') return
    const index = state.tabs.indexOf(id)
    if (index < 0) return
    const wasActive = state.page === id
    state.tabs.splice(index, 1)
    scrollPositions.delete(id)
    saveTabs()
    if (wasActive) openPage(state.tabs[index - 1] || state.tabs[index] || 'dashboard')
    else renderTabs()
  }

  function closeOtherTabs() {
    state.tabs = state.page === 'dashboard' ? ['dashboard'] : ['dashboard', state.page]
    saveTabs()
    state.tabsMenu = false
    renderTabs()
    toast('已关闭其他标签')
  }

  function closeAllTabs() {
    state.tabs = ['dashboard']
    saveTabs()
    state.tabsMenu = false
    openPage('dashboard')
    toast('已关闭全部标签')
  }

  function renderAll() {
    renderNav()
    renderTabs()
    renderView()
  }

  function renderView() {
    const pages = {
      dashboard: renderDashboard,
      tasks: renderTasks,
      envs: renderEnvs,
      scripts: renderScripts,
      logs: renderLogs,
      subscriptions: renderSubscriptions,
      dependencies: renderDependencies,
      'api-docs': renderApiDocs,
      settings: renderSettings
    }
    view.innerHTML = (pages[state.page] || renderDashboard)()
    if (state.page === 'tasks') {
      const input = document.getElementById('task-search')
      if (input) input.value = state.taskSearch
      applyTaskFilter()
    }
    if (state.page === 'envs') {
      const input = document.getElementById('env-search')
      if (input) input.value = state.envSearch
      applyEnvFilter()
    }
    if (state.page === 'scripts') updateLineNumbers()
    if (state.page === 'api-docs') applyApiFilter()
  }

  /* ---------------- 仪表盘 ---------------- */
  function renderDashboard() {
    const running = runningCount()
    const recent = state.runs.slice(0, 5)
    return `${pageHeading('WORKSPACE', '仪表盘', '今天也把重复的事情交给自动化。', `<span class="live-indicator"><span class="live-dot"></span>演示数据实时更新</span>`)}
      <div class="stats-grid">
        <article class="stat-card"><div class="stat-top"><span class="stat-icon">${icon('clock', 18)}</span><span class="stat-note">全部任务</span></div><div class="stat-value">${state.tasks.length}</div><div class="stat-label">定时任务</div><div class="stat-trend">${state.tasks.filter(task => task.enabled).length} 个已启用</div></article>
        <article class="stat-card blue"><div class="stat-top"><span class="stat-icon">${icon('activity', 18)}</span><span class="stat-note">实时</span></div><div class="stat-value">${running}</div><div class="stat-label">正在运行</div><div class="stat-trend">${running ? '任务正在执行' : '当前没有运行任务'}</div></article>
        <article class="stat-card amber"><div class="stat-top"><span class="stat-icon">${icon('play', 18)}</span><span class="stat-note">今天</span></div><div class="stat-value">${12 + state.runs.length}</div><div class="stat-label">今日执行</div><div class="stat-trend">较昨日 +8.2%</div></article>
        <article class="stat-card"><div class="stat-top"><span class="stat-icon">${icon('shield', 18)}</span><span class="stat-note">稳定性</span></div><div class="stat-value">96.8<small style="font-size:13px">%</small></div><div class="stat-label">执行成功率</div><div class="stat-trend">近 7 天平均</div></article>
      </div>
      <div class="dashboard-grid">
        <section class="card"><div class="card-heading"><h2>${icon('activity', 17)}执行趋势</h2><span class="muted">最近 7 天</span></div><div class="chart-wrap"><div class="chart-legend"><span><i class="legend-dot"></i>成功</span><span><i class="legend-dot blue"></i>运行中</span><span class="muted">单位：次</span></div>${chartSVG()}</div></section>
        <section class="card"><div class="card-heading"><h2>${icon('check', 17)}完成率</h2><span class="muted">累计</span></div><div class="donut-wrap"><div class="donut"><div class="donut-inner"><strong>96.8<small style="font-size:11px">%</small></strong><small>执行成功率</small></div></div><div class="ring-legend"><div><i></i><span>成功</span><strong>342</strong></div><div><i class="blue"></i><span>运行中</span><strong>${running}</strong></div><div><i class="red"></i><span>失败 / 中断</span><strong>11</strong></div></div></div></section>
      </div>
      <section class="card recent-card"><div class="card-heading"><h2>${icon('file', 17)}最近运行</h2><button class="button compact ghost" data-nav="logs">查看全部${icon('chevron', 13)}</button></div><div class="recent-list">${recent.length ? recent.map(runRow).join('') : '<div class="task-empty">还没有运行记录</div>'}</div></section>
      <section class="card resource-card"><div class="card-heading"><h2>${icon('cpu', 17)}运行环境</h2><span class="muted">演示数据</span></div><div class="resource-meters">${[['CPU 使用率', 12.4, 'cpu'], ['内存使用率', 43.1, 'memory'], ['磁盘使用率', 55.2, 'disk']].map(([label, value, kind]) => `<div class="resource-meter ${kind}"><div class="meter-head"><span>${label}</span><strong>${value}%</strong></div><div class="meter-track"><i style="width:${value}%"></i></div></div>`).join('')}</div><div class="resource-facts">${[['Python', '3.11.9', true], ['Node.js', '22.11.0', true], ['Bash', '可用', true], ['Git', '2.45.2', true]].map(([name, version, ok]) => `<div class="resource-fact"><span class="fact-dot ${ok ? 'ok' : 'bad'}"></span><div><strong>${name}</strong><small>${version}</small></div></div>`).join('')}</div></section>
      <div class="info-banner" style="margin-top:15px">${icon('cpu')}<span><strong>演示环境：</strong>Python 3.11 · Node.js 22 · Bash 可用。这里展示的是纯前端交互，不会真正执行脚本。</span></div>`
  }

  function chartSVG() {
    const points = [[20, 132], [110, 112], [200, 118], [290, 88], [380, 96], [470, 62], [560, 48]]
    const line = points.map((point, index) => `${index ? 'L' : 'M'} ${point[0]} ${point[1]}`).join(' ')
    const area = `${line} L 560 158 L 20 158 Z`
    const labels = ['周一', '周二', '周三', '周四', '周五', '周六', '周日']
    return `<svg class="line-chart" viewBox="0 0 580 170" preserveAspectRatio="none" role="img" aria-label="最近 7 天执行趋势"><defs><linearGradient id="chartFill" x1="0" y1="0" x2="0" y2="1"><stop offset="0%" stop-color="#18c98b" stop-opacity=".22"/><stop offset="100%" stop-color="#18c98b" stop-opacity="0"/></linearGradient></defs>${[28, 68, 108, 148].map(y => `<line class="chart-grid-line" x1="20" x2="560" y1="${y}" y2="${y}"/>`).join('')}<path class="chart-area" d="${area}"/><path class="chart-line" d="${line}"/>${points.map(point => `<circle class="chart-dot" cx="${point[0]}" cy="${point[1]}" r="3"/>`).join('')}${labels.map((label, index) => `<text class="chart-axis" x="${19 + index * 90}" y="168">${label}</text>`).join('')}</svg>`
  }

  function runRow(run) {
    return `<div class="run-row"><span class="run-status ${run.status}">${icon(run.status === 'success' ? 'check' : run.status === 'running' ? 'activity' : 'alert', 12)}</span><div class="run-main"><strong>${escapeHTML(run.name)}</strong><small>${run.status === 'running' ? '正在执行 · 演示任务' : '运行记录 · ' + (run.duration || 0).toFixed(1) + ' 秒'}</small></div><span class="run-meta">${escapeHTML(run.started || '刚刚')}</span><button class="icon-btn" data-open-log="${run.id}" aria-label="查看日志">${icon('file', 14)}</button></div>`
  }

  /* ---------------- 定时任务 ---------------- */
  const taskFilters = [
    { id: 'all', label: '全部' },
    { id: 'running', label: '运行中' },
    { id: 'disabled', label: '已停用' },
    { id: 'failed', label: '失败' }
  ]

  function taskCounts() {
    return {
      all: state.tasks.length,
      running: state.tasks.filter(task => task.running).length,
      disabled: state.tasks.filter(task => !task.enabled).length,
      failed: state.tasks.filter(task => ['failed', 'cancelled'].includes(task.status) && !task.running).length
    }
  }

  function filteredTasks() {
    const query = (state.taskSearch || '').trim().toLowerCase()
    let list = state.tasks.slice()
    if (state.taskFilter === 'running') list = list.filter(task => task.running)
    else if (state.taskFilter === 'disabled') list = list.filter(task => !task.enabled)
    else if (state.taskFilter === 'failed') list = list.filter(task => ['failed', 'cancelled'].includes(task.status) && !task.running)
    if (query) list = list.filter(task => `${task.name} ${task.command} ${task.remarks || ''} ${(task.labels || []).join(' ')}`.toLowerCase().includes(query))
    list.sort((left, right) => Number(!left.enabled) - Number(!right.enabled) || Number(!!right.pinned) - Number(!!left.pinned) || right.id - left.id)
    if (state.taskSort.key) {
      const key = state.taskSort.key
      const dir = state.taskSort.dir === 'asc' ? 1 : -1
      list = list.slice().sort((left, right) => {
        const a = key === 'lastRun' ? (left.last || '') : key === 'nextRun' ? (left.nextRun || '') : key === 'duration' ? parseFloat(left.duration) || 0 : String(left[key] || '')
        const b = key === 'lastRun' ? (right.last || '') : key === 'nextRun' ? (right.nextRun || '') : key === 'duration' ? parseFloat(right.duration) || 0 : String(right[key] || '')
        if (a === b) return 0
        return (a > b ? 1 : -1) * dir
      })
    }
    return list
  }

  function renderTasks() {
    const counts = taskCounts()
    const list = filteredTasks()
    const totalPages = Math.max(1, Math.ceil(list.length / state.taskPageSize))
    if (state.taskPage > totalPages) state.taskPage = totalPages
    const start = (state.taskPage - 1) * state.taskPageSize
    const pageItems = list.slice(start, start + state.taskPageSize)
    const enabled = state.tasks.filter(task => task.enabled).length
    const sortClass = key => state.taskSort.key === key ? `sorted ${state.taskSort.dir}` : ''
    return `${pageHeading('AUTOMATION', '定时任务', '管理计划任务、手动执行并查看运行结果。', `<button class="button primary" data-action="add-task">${icon('plus', 15)}新建任务</button>`)}
      <div class="section-note" style="margin-bottom:12px"><span class="live-dot"></span>${enabled} 个任务已启用 · 自动调度已就绪 · 调度依据服务器时区</div>
      <div class="filter-bar"><div class="filter-pills">${taskFilters.map(item => `<button type="button" class="${state.taskFilter === item.id ? 'active' : ''}" data-task-filter="${item.id}">${item.label}<span>${counts[item.id]}</span></button>`).join('')}</div><label class="search-field">${icon('search', 14)}<input id="task-search" placeholder="搜索任务名称、命令或标签…" autocomplete="off"></label></div>
      ${state.taskSelected.length ? `<div class="bulk-bar"><span>已选择 ${state.taskSelected.length} 项</span><button type="button" data-task-bulk="run">运行</button><button type="button" data-task-bulk="stop">停止</button><button type="button" data-task-bulk="pin">置顶</button><button type="button" data-task-bulk="unpin">取消置顶</button><button type="button" data-task-bulk="enable">启用</button><button type="button" data-task-bulk="disable">停用</button><button type="button" class="danger-text" data-task-bulk="delete">删除</button><button type="button" data-task-bulk="clear">取消选择</button></div>` : ''}
      <section class="card task-list">${pageItems.length ? `<div class="task-list-header"><input type="checkbox" aria-label="选择本页全部任务" data-task-select-all ${pageItems.every(task => state.taskSelected.includes(task.id)) ? 'checked' : ''}><span>任务名称</span><button type="button" class="th-sort ${sortClass('name')}" data-task-sort="name">名称</button><button type="button" class="th-sort ${sortClass('schedule')}" data-task-sort="schedule">调度</button><button type="button" class="th-sort ${sortClass('lastRun')}" data-task-sort="lastRun">上次运行</button><button type="button" class="th-sort ${sortClass('nextRun')}" data-task-sort="nextRun">下次运行</button><button type="button" class="th-sort ${sortClass('duration')}" data-task-sort="duration">耗时</button><span class="th-actions">操作</span></div>${pageItems.map(taskRow).join('')}` : `<div class="empty-state"><div class="empty-icon">${icon('clock', 28)}</div><h3>${state.taskSearch || state.taskFilter !== 'all' ? '没有找到匹配的任务' : '您的时间，值得被自动化'}</h3><p>${state.taskSearch || state.taskFilter !== 'all' ? '试试其他关键词或筛选条件。' : '添加一个脚本命令，剩下的就交给大白菜。'}</p>${!state.taskSearch && state.taskFilter === 'all' ? `<button class="button primary" data-action="add-task">${icon('plus', 14)}创建第一个任务</button>` : ''}</div>`}
      <footer class="pagination"><span>共 ${list.length} 个任务，当前第 ${state.taskPage} 页</span><div><button type="button" class="icon-btn" data-task-page="prev" ${state.taskPage <= 1 ? 'disabled' : ''} aria-label="上一页">${icon('chevronLeft', 16)}</button><span class="current-page">${state.taskPage}</span><button type="button" class="icon-btn" data-task-page="next" ${state.taskPage >= totalPages ? 'disabled' : ''} aria-label="下一页">${icon('chevronRight', 16)}</button><select data-task-page-size aria-label="每页数量">${[10, 20, 50].map(size => `<option value="${size}" ${state.taskPageSize === size ? 'selected' : ''}>${size} 条 / 页</option>`).join('')}</select></div></footer></section>`
  }

  function taskRow(task) {
    const run = latestRun(task.id)
    const status = task.running ? 'running' : task.status
    const logRun = run ? run.id : ''
    const failedStatus = ['failed', 'cancelled'].includes(status)
    const stateIcon = task.running ? 'loader' : failedStatus ? 'alert' : !task.enabled ? 'pause' : status === 'success' ? 'check' : 'clock'
    const stateText = task.running ? '正在执行' : !task.enabled ? '已停用' : failedStatus ? '上次失败' : status === 'success' ? '上次成功' : '待运行'
    return `<article class="task-row ${task.running ? 'running' : ''} ${task.enabled ? '' : 'disabled'}" data-task-row data-task-id="${task.id}" data-status="${status}" data-enabled="${task.enabled}">
      <input type="checkbox" data-task-check="${task.id}" ${state.taskSelected.includes(task.id) ? 'checked' : ''} aria-label="选择 ${escapeHTML(task.name)}">
      <span class="task-state ${status}">${icon(stateIcon, 16)}</span>
      <div class="task-main"><div class="task-title"><button type="button" class="task-name-button" data-task-detail="${task.id}"><strong>${escapeHTML(task.name)}</strong>${task.pinned ? `<span class="pin-badge">${icon('pin', 10)}置顶</span>` : ''}</button>${statusBadge(status)}${task.enabled ? '' : '<span class="badge">已停用</span>'}</div>${task.remarks ? `<div class="task-remarks">${escapeHTML(task.remarks)}</div>` : ''}<button type="button" class="task-command" data-task-source="${task.id}" title="查看源码：${escapeHTML(task.command)}"><code>${escapeHTML(task.command)}</code></button>${task.labels && task.labels.length ? `<div class="labels">${task.labels.map(label => `<i>${escapeHTML(label)}</i>`).join('')}</div>` : ''}${task.running ? '<div class="task-progress"><i></i></div>' : ''}</div>
      <div class="task-schedule"><code>${escapeHTML(task.schedule || '手动运行')}</code><small>${task.running ? '正在执行' : !task.enabled ? '已停用' : 'Cron 表达式'}</small></div>
      <div class="task-time"><small>上次运行</small><span>${escapeHTML(task.last || '—')}</span></div>
      <div class="task-time"><small>下次运行</small><span>${task.enabled ? escapeHTML(task.nextRun || '—') : '已停用'}</span></div>
      <div class="task-duration"><small>最近耗时</small><span>${task.duration || '—'}</span></div>
      <label class="switch" title="启用 / 停用"><input type="checkbox" ${task.enabled ? 'checked' : ''} data-toggle-task="${task.id}"><span></span></label>
      <div class="task-actions"><button type="button" class="icon-btn act-run ${task.running ? 'is-stop' : ''}" data-${task.running ? 'stop' : 'run'}-task="${task.id}" aria-label="${task.running ? '停止' : '运行'} ${escapeHTML(task.name)}" title="${task.running ? '停止' : '立即运行'}">${icon(task.running ? 'square' : 'play', 15)}</button><button type="button" class="icon-btn" data-task-menu="${task.id}" aria-label="更多操作" title="更多操作">${icon('more', 15)}</button><button type="button" class="icon-btn" data-task-detail="${task.id}" aria-label="查看日志" title="查看详情">${icon('file', 15)}</button></div>
      ${state.taskMenu === task.id ? `<div class="task-menu" role="menu"><button type="button" data-task-pin="${task.id}">${icon('pin', 14)}${task.pinned ? '取消置顶' : '置顶'}</button><button type="button" data-task-source="${task.id}">${icon('fileCode', 14)}查看源码</button><button type="button" data-task-edit="${task.id}">${icon('edit', 14)}编辑任务</button><button type="button" class="danger" data-task-delete="${task.id}">${icon('trash', 14)}删除任务</button></div>` : ''}
    </article>`
  }

  function applyTaskFilter() {
    const query = (state.taskSearch || '').trim().toLowerCase()
    let visible = 0
    document.querySelectorAll('[data-task-row]').forEach(row => {
      const task = state.tasks.find(item => item.id === Number(row.dataset.taskId))
      if (!task) return
      let match = true
      if (state.taskFilter === 'running') match = task.running
      else if (state.taskFilter === 'disabled') match = !task.enabled
      else if (state.taskFilter === 'failed') match = ['failed', 'cancelled'].includes(task.status) && !task.running
      if (match && query) match = `${task.name} ${task.command} ${task.remarks || ''} ${(task.labels || []).join(' ')}`.toLowerCase().includes(query)
      row.hidden = !match
      if (match) visible++
    })
    const count = document.getElementById('task-visible-count')
    if (count) count.textContent = `${visible} 个任务`
  }

  function openTaskDetail(id) {
    const task = state.tasks.find(item => item.id === Number(id))
    if (!task) return
    const history = state.runs.filter(run => run.taskId === task.id)
    const rows = history.length
      ? history.map(run => `<div class="log-row"><span class="run-status ${run.status}">${icon(run.status === 'success' ? 'check' : run.status === 'running' ? 'activity' : 'alert', 12)}</span><div class="log-info"><strong>${statusLabel[run.status] || run.status}</strong><small>${(run.duration || 0).toFixed(1)} 秒</small></div><span class="log-time">${escapeHTML(run.started || '刚刚')}</span><button type="button" class="button compact ghost" data-open-log="${run.id}">${icon('file', 12)}日志</button></div>`).join('')
      : '<div class="task-empty">还没有运行记录</div>'
    openModal(`任务详情 · ${task.name}`, `
      <div class="detail-grid"><div><span>命令</span><code>${escapeHTML(task.command)}</code></div><div><span>调度</span><code>${escapeHTML(task.schedule || '手动运行')}</code></div><div><span>状态</span><span>${task.enabled ? '已启用' : '已停用'} · ${statusLabel[task.running ? 'running' : task.status] || task.status}</span></div><div><span>上次运行</span><span>${escapeHTML(task.last || '—')}</span></div><div><span>下次运行</span><span>${task.enabled ? escapeHTML(task.nextRun || '—') : '已停用'}</span></div><div><span>最近耗时</span><span>${task.duration || '—'}</span></div><div><span>超时</span><span>${task.timeout ? task.timeout + ' 秒' : '使用系统默认'}</span></div><div><span>允许重叠</span><span>${task.allowConcurrent ? '是' : '否'}</span></div><div><span>结束通知</span><span>${task.notifyOnFinish ? '开启' : '关闭'}</span></div>${task.remarks ? `<div class="detail-wide"><span>备注</span><span>${escapeHTML(task.remarks)}</span></div>` : ''}${task.labels && task.labels.length ? `<div class="detail-wide"><span>标签</span><span class="labels">${task.labels.map(label => `<i>${escapeHTML(label)}</i>`).join('')}</span></div>` : ''}</div>
      <h4 class="detail-section">最近运行</h4><div class="log-list">${rows}</div>
      <div class="actions" style="justify-content:flex-end;margin-top:12px"><button type="button" class="button" data-close-modal>关闭</button><button type="button" class="button" data-task-source="${task.id}">${icon('fileCode', 14)}查看源码</button><button type="button" class="button primary" data-task-edit="${task.id}">${icon('edit', 14)}编辑任务</button></div>`, true)
  }

  function openTaskSource(id) {
    const task = state.tasks.find(item => item.id === Number(id))
    if (!task) return
    const match = task.command.match(/^task\s+(.+)$/)
    const file = match ? scriptFiles.find(item => item.path === match[1].trim()) : null
    const content = file ? file.content : `# 演示模式：未找到 ${escapeHTML(match ? match[1] : task.command)} 对应的脚本文件\n`
    openModal(`源码 · ${task.name}`, `<div class="apidocs-preview"><span class="apidocs-preview-label">脚本路径</span><code>${escapeHTML(file ? file.path : '未知')}</code></div><pre class="terminal" style="max-height:50vh">${escapeHTML(content)}</pre><div class="actions" style="justify-content:flex-end;margin-top:12px"><button type="button" class="button" data-close-modal>关闭</button>${file ? `<button type="button" class="button primary" data-open-script="${escapeHTML(file.path)}">${icon('code', 14)}在编辑器中打开</button>` : ''}</div>`, true)
  }

  function openTaskForm(id) {
    const task = id ? state.tasks.find(item => item.id === Number(id)) : null
    const value = task || { name: '', command: '', schedule: '0 8 * * *', remarks: '', timeout: 0, labels: [], allowConcurrent: false, notifyOnFinish: false }
    openModal(task ? '编辑任务' : '新建任务', `<form data-form="task" class="form-grid">
      <label class="field"><span>任务名称</span><input name="name" required value="${escapeHTML(value.name)}" placeholder="例如：数据备份"></label>
      <label class="field"><span>执行命令</span><textarea name="command" rows="2" required placeholder="task demo/backup.py">${escapeHTML(value.command)}</textarea><small>统一用 task 前缀：task 脚本.py / .js / .sh，按后缀自动选择解释器。</small></label>
      <label class="field"><span>Cron 表达式</span><input name="schedule" value="${escapeHTML(value.schedule || '')}" placeholder="0 8 * * *"><small>支持 5 段、6 段和 @every 30m。留空表示仅手动执行。</small></label>
      <label class="field"><span>备注</span><input name="remarks" value="${escapeHTML(value.remarks || '')}" placeholder="简单描述这个任务"></label>
      <label class="field"><span>标签</span><input name="labels" value="${escapeHTML((value.labels || []).join(', '))}" placeholder="使用逗号分隔，例如：日常,备份"></label>
      <label class="field"><span>执行超时（秒）</span><input name="timeout" type="number" min="0" value="${Number(value.timeout) || 0}"><small>0 表示使用系统默认值。</small></label>
      <div class="switch-row"><div><strong>允许同一任务重叠执行</strong><small>默认关闭：上次未结束时跳过重复提交</small></div><label class="switch"><input type="checkbox" name="allowConcurrent" ${value.allowConcurrent ? 'checked' : ''}><span></span></label></div>
      <div class="switch-row"><div><strong>执行结束通知</strong><small>结束后通过已配置的通知渠道发送结果</small></div><label class="switch"><input type="checkbox" name="notifyOnFinish" ${value.notifyOnFinish ? 'checked' : ''}><span></span></label></div>
      <div class="actions" style="justify-content:flex-end;margin-top:4px"><button type="button" class="button" data-close-modal>取消</button><button type="submit" class="button primary">${icon('check', 14)}${task ? '保存任务' : '创建任务'}</button></div>
      ${task ? `<input type="hidden" name="id" value="${task.id}">` : ''}
    </form>`, true)
  }

  function saveTaskForm(form) {
    const data = new FormData(form)
    const name = String(data.get('name') || '').trim()
    const command = String(data.get('command') || '').trim()
    if (!name || !command) return toast('任务名称和命令不能为空', 'error')
    const id = Number(data.get('id') || 0)
    const payload = {
      name,
      command,
      schedule: String(data.get('schedule') || '').trim(),
      remarks: String(data.get('remarks') || '').trim(),
      labels: String(data.get('labels') || '').split(/[,，]/).map(value => value.trim()).filter(Boolean),
      timeout: Number(data.get('timeout') || 0),
      allowConcurrent: data.get('allowConcurrent') === 'on',
      notifyOnFinish: data.get('notifyOnFinish') === 'on'
    }
    if (id) {
      const task = state.tasks.find(item => item.id === id)
      Object.assign(task, payload)
      toast(`已保存任务「${name}」`)
    } else {
      state.tasks.push({ id: Date.now(), ...payload, enabled: true, status: 'idle', last: '从未', duration: '—', pinned: false, nextRun: payload.schedule ? '待计算' : '手动运行' })
      toast(`已创建任务「${name}」`)
    }
    closeModal()
    renderAll()
  }

  function toggleTaskSelection(id, checked) {
    const numeric = Number(id)
    if (checked) {
      if (!state.taskSelected.includes(numeric)) state.taskSelected.push(numeric)
    } else {
      state.taskSelected = state.taskSelected.filter(item => item !== numeric)
    }
    renderView()
  }

  function bulkTaskOperation(operation) {
    const ids = state.taskSelected.slice()
    if (operation === 'clear') { state.taskSelected = []; renderView(); return }
    if (!ids.length) return toast('请先选择任务', 'error')
    if (operation === 'delete') {
      state.tasks = state.tasks.filter(task => !ids.includes(task.id))
      state.runs = state.runs.filter(run => !ids.includes(run.taskId))
      state.taskSelected = []
      renderAll()
      return toast(`已删除 ${ids.length} 个任务`)
    }
    ids.forEach(id => {
      const task = state.tasks.find(item => item.id === id)
      if (!task) return
      if (operation === 'run') runTask(id)
      else if (operation === 'stop') stopTask(id)
      else if (operation === 'enable') task.enabled = true
      else if (operation === 'disable') task.enabled = false
      else if (operation === 'pin') task.pinned = true
      else if (operation === 'unpin') task.pinned = false
    })
    const label = { run: '运行', stop: '停止', enable: '启用', disable: '停用', pin: '置顶', unpin: '取消置顶' }[operation]
    if (!['run', 'stop'].includes(operation)) { state.taskSelected = []; renderAll(); toast(`已${label} ${ids.length} 个任务`) }
  }

  function runTask(id) {
    const task = state.tasks.find(item => item.id === id)
    if (!task || task.running) return
    task.running = true
    task.status = 'running'
    const run = { id: state.nextRunId++, taskId: task.id, name: task.name, status: 'running', started: '刚刚', duration: 0 }
    state.runs.unshift(run)
    renderAll()
    toast(`已开始运行「${task.name}」`)
    const timer = setInterval(() => {
      run.duration += 0.5
      const target = document.querySelector(`[data-run-duration="${run.id}"]`)
      if (target) target.textContent = `${run.duration.toFixed(1)}s`
    }, 500)
    state.timers.set(run.id, timer)
    const finishTimer = setTimeout(() => finishRun(run.id, 'success'), 3600 + Math.random() * 1200)
    state.timers.set(`${run.id}-finish`, finishTimer)
  }

  function finishRun(runId, status) {
    const run = state.runs.find(item => item.id === Number(runId))
    if (!run || run.status !== 'running') return
    clearInterval(state.timers.get(run.id))
    clearTimeout(state.timers.get(`${run.id}-finish`))
    state.timers.delete(run.id)
    state.timers.delete(`${run.id}-finish`)
    run.status = status
    const task = state.tasks.find(item => item.id === run.taskId)
    if (task) {
      task.running = false
      task.status = status
      task.last = '刚刚'
      task.duration = `${run.duration.toFixed(1)}s`
    }
    renderAll()
    toast(status === 'success' ? `「${run.name}」执行完成` : `「${run.name}」已停止`, status === 'success' ? 'success' : 'error')
  }

  function stopTask(id) {
    const run = state.runs.find(item => item.taskId === id && item.status === 'running')
    if (run) finishRun(run.id, 'cancelled')
  }

  function openTaskLog(runId) {
    const run = state.runs.find(item => item.id === Number(runId))
    if (!run) return
    const task = state.tasks.find(item => item.id === run.taskId)
    const lines = [
      `大白菜面板 · ${run.name}`,
      `开始时间: ${run.started || '刚刚'}`,
      `命令: ${task ? task.command : 'task example.py'}`,
      `状态: ${statusLabel[run.status] || run.status}`,
      '',
      '[INFO] 初始化运行环境…',
      '[INFO] 读取环境变量…',
      '[INFO] 执行脚本（演示输出）',
      run.status === 'running' ? '[INFO] 任务正在运行，等待结束…' : run.status === 'success' ? '[INFO] 任务完成，退出码 0' : run.status === 'cancelled' ? '[WARN] 已收到停止信号，进程结束' : '[ERROR] 任务执行失败，退出码 1',
      '',
      `结束状态: ${statusLabel[run.status] || run.status} · 耗时 ${(run.duration || 0).toFixed(1)}s`
    ]
    openModal(`运行日志 · ${run.name}`, `<pre class="terminal">${escapeHTML(lines.join('\n'))}</pre>`, true)
  }

  /* ---------------- 环境变量 ---------------- */
  function envFiltered() {
    const query = (state.envSearch || '').trim().toLowerCase()
    let list = state.envs.slice()
    if (state.envFilter === 'enabled') list = list.filter(env => env.enabled)
    else if (state.envFilter === 'disabled') list = list.filter(env => !env.enabled)
    else if (state.envFilter === 'pinned') list = list.filter(env => env.pinned)
    if (query) list = list.filter(env => `${env.name} ${env.remarks || ''}`.toLowerCase().includes(query))
    list.sort((left, right) => Number(!left.enabled) - Number(!right.enabled) || Number(!!right.pinned) - Number(!!left.pinned) || left.id - right.id)
    if (state.envSort.key) {
      const key = state.envSort.key, dir = state.envSort.dir === 'asc' ? 1 : -1
      list = list.slice().sort((left, right) => {
        const a = key === 'status' ? Number(left.enabled) : key === 'updatedAt' ? (left.updatedAt || '') : String(left[key] || '')
        const b = key === 'status' ? Number(right.enabled) : key === 'updatedAt' ? (right.updatedAt || '') : String(right[key] || '')
        if (a === b) return 0
        return (a > b ? 1 : -1) * dir
      })
    }
    return list
  }

  function renderEnvs() {
    const list = envFiltered()
    const totalPages = Math.max(1, Math.ceil(list.length / state.envPageSize))
    if (state.envPage > totalPages) state.envPage = totalPages
    const start = (state.envPage - 1) * state.envPageSize
    const pageItems = list.slice(start, start + state.envPageSize)
    const sortClass = key => state.envSort.key === key ? `sorted ${state.envSort.dir}` : ''
    return `${pageHeading('VARIABLES', '环境变量', '启用的变量会注入任务进程，修改即时生效。', `<button class="button primary" data-action="add-env">${icon('plus', 15)}添加变量</button>`)}
      <div class="section-note" style="margin-bottom:12px">共 ${state.envs.length} 条环境变量 · 长按行可拖动排序</div>
      ${state.envSelected.length ? `<div class="bulk-bar"><span>已选择 ${state.envSelected.length} 项</span><button type="button" data-env-bulk="enable">启用</button><button type="button" data-env-bulk="disable">停用</button><button type="button" class="danger-text" data-env-bulk="delete">删除</button><button type="button" data-env-bulk="clear">取消选择</button></div>` : ''}
      <section class="card"><div class="card-heading"><label class="search-field" style="max-width:none;flex:1">${icon('search', 14)}<input id="env-search" placeholder="搜索环境变量…" autocomplete="off"></label><select data-env-filter aria-label="筛选环境变量" style="padding:7px 10px;border:1px solid var(--border-strong);border-radius:7px;font-size:10.5px">${[['all', '全部变量'], ['enabled', '已启用'], ['disabled', '已停用'], ['pinned', '已置顶']].map(([value, label]) => `<option value="${value}" ${state.envFilter === value ? 'selected' : ''}>${label}</option>`).join('')}</select></div>
      <div class="table-wrap"><table class="env-table"><thead><tr><th class="checkbox-column"><input type="checkbox" data-env-select-all ${pageItems.length && pageItems.every(env => state.envSelected.includes(env.id)) ? 'checked' : ''} aria-label="选择本页记录"></th><th><button type="button" class="th-sort ${sortClass('name')}" data-env-sort="name">名称</button></th><th><button type="button" class="th-sort ${sortClass('value')}" data-env-sort="value">变量值</button></th><th><button type="button" class="th-sort ${sortClass('remarks')}" data-env-sort="remarks">备注</button></th><th><button type="button" class="th-sort ${sortClass('updatedAt')}" data-env-sort="updatedAt">最后修改</button></th><th><button type="button" class="th-sort ${sortClass('status')}" data-env-sort="status">启用</button></th><th class="align-right">操作</th></tr></thead><tbody>${pageItems.length ? pageItems.map(envRow).join('') : `<tr><td colspan="7"><div class="empty-state"><div class="empty-icon">${icon('key', 26)}</div><h3>尚无环境变量</h3><p>把脚本所需的配置集中在这里，安全而有序。</p><button class="button" data-action="add-env">${icon('plus', 14)}添加变量</button></div></td></tr>`}</tbody></table></div>
      <footer class="pagination"><span>共 ${list.length} 条 · 第 ${state.envPage} / ${totalPages} 页</span><div><select data-env-page-size aria-label="每页记录数量">${[10, 20, 50].map(size => `<option value="${size}" ${state.envPageSize === size ? 'selected' : ''}>${size} 条 / 页</option>`).join('')}</select><button type="button" class="icon-btn" data-env-page="prev" ${state.envPage <= 1 ? 'disabled' : ''} aria-label="上一页">${icon('chevronLeft', 15)}</button><button type="button" class="icon-btn" data-env-page="next" ${state.envPage >= totalPages ? 'disabled' : ''} aria-label="下一页">${icon('chevronRight', 15)}</button></div></footer></section>`
  }

  function envRow(env) {
    const revealed = state.revealed.has(env.id)
    return `<tr data-env-row data-env-id="${env.id}" class="${env.enabled ? '' : 'row-disabled'} ${state.envSelected.includes(env.id) ? 'selected-row' : ''}">
      <td><input type="checkbox" data-env-check="${env.id}" ${state.envSelected.includes(env.id) ? 'checked' : ''} aria-label="选择 ${escapeHTML(env.name)}"></td>
      <td><div class="resource-name"><span class="resource-icon">${icon('key', 14)}</span><div class="resource-copy"><div class="resource-title-line"><strong>${escapeHTML(env.name)}</strong>${env.pinned ? `<span class="pin-badge">${icon('pin', 10)}置顶</span>` : ''}<span class="record-id">#${env.id}</span></div></div></div></td>
      <td><div class="value-cell"><code>${revealed ? escapeHTML(env.value) : '••••••••'}</code><button type="button" class="reveal-btn" data-reveal="${env.id}" aria-label="${revealed ? '隐藏' : '显示'}变量值">${icon(revealed ? 'eyeOff' : 'eye', 13)}</button><button type="button" class="reveal-btn" data-copy-env="${env.id}" aria-label="复制变量值">${icon('copy', 13)}</button></div></td>
      <td class="muted">${escapeHTML(env.remarks || '—')}</td>
      <td class="muted nowrap">${escapeHTML(env.updatedAt || '—')}</td>
      <td><label class="switch"><input type="checkbox" ${env.enabled ? 'checked' : ''} data-toggle-env="${env.id}" aria-label="启用 ${escapeHTML(env.name)}"><span></span></label></td>
      <td><div class="row-actions"><button type="button" class="icon-btn ${env.pinned ? 'pin-active' : ''}" data-env-pin="${env.id}" aria-label="${env.pinned ? '取消置顶' : '置顶'}" title="${env.pinned ? '取消置顶' : '置顶'}">${icon('pin', 15)}</button><button type="button" class="icon-btn" data-env-move="-1" data-env-id="${env.id}" aria-label="上移" title="上移">${icon('arrowUp', 15)}</button><button type="button" class="icon-btn" data-env-move="1" data-env-id="${env.id}" aria-label="下移" title="下移">${icon('arrowDown', 15)}</button><button type="button" class="icon-btn" data-env-edit="${env.id}" aria-label="编辑" title="编辑">${icon('edit', 15)}</button><button type="button" class="icon-btn" data-delete-env="${env.id}" aria-label="删除" title="删除">${icon('trash', 15)}</button></div></td>
    </tr>`
  }

  function applyEnvFilter() {
    const query = (state.envSearch || '').trim().toLowerCase()
    document.querySelectorAll('[data-env-row]').forEach(row => {
      const env = state.envs.find(item => item.id === Number(row.dataset.envId))
      if (!env) return
      let match = !query || `${env.name} ${env.remarks || ''}`.toLowerCase().includes(query)
      if (match && state.envFilter === 'enabled') match = env.enabled
      if (match && state.envFilter === 'disabled') match = !env.enabled
      if (match && state.envFilter === 'pinned') match = env.pinned
      row.hidden = !match
    })
  }

  function openEnvForm(id) {
    const env = id ? state.envs.find(item => item.id === Number(id)) : null
    const value = env || { name: '', value: '', remarks: '' }
    openModal(env ? '编辑环境变量' : '添加环境变量', `<form data-form="env" class="form-grid">
      <label class="field"><span>变量名称</span><input name="name" required value="${escapeHTML(value.name)}" placeholder="例如：DEMO_TOKEN"></label>
      <label class="field"><span>变量值</span><textarea name="value" rows="3" required placeholder="演示值">${escapeHTML(value.value)}</textarea><small>仅启用的变量会注入脚本进程；同名变量按排序靠后的值生效。</small></label>
      <label class="field"><span>备注</span><input name="remarks" value="${escapeHTML(value.remarks || '')}" placeholder="可选"></label>
      <div class="actions" style="justify-content:flex-end;margin-top:4px"><button type="button" class="button" data-close-modal>取消</button><button type="submit" class="button primary">${icon('check', 14)}${env ? '保存变量' : '添加变量'}</button></div>
      ${env ? `<input type="hidden" name="id" value="${env.id}">` : ''}
    </form>`)
  }

  function saveEnvForm(form) {
    const data = new FormData(form)
    const name = String(data.get('name') || '').trim()
    if (!name) return toast('变量名称不能为空', 'error')
    const id = Number(data.get('id') || 0)
    const payload = { name, value: String(data.get('value') || ''), remarks: String(data.get('remarks') || '').trim(), updatedAt: '刚刚' }
    if (id) {
      Object.assign(state.envs.find(item => item.id === id), payload)
      toast(`已保存变量「${name}」`)
    } else {
      state.envs.push({ id: Date.now(), ...payload, enabled: true, pinned: false })
      toast(`已添加变量「${name}」`)
    }
    closeModal()
    renderView()
  }

  function toggleEnvSelection(id, checked) {
    const numeric = Number(id)
    if (checked) {
      if (!state.envSelected.includes(numeric)) state.envSelected.push(numeric)
    } else {
      state.envSelected = state.envSelected.filter(item => item !== numeric)
    }
    renderView()
  }

  function bulkEnvOperation(operation) {
    const ids = state.envSelected.slice()
    if (operation === 'clear') { state.envSelected = []; renderView(); return }
    if (!ids.length) return toast('请先选择变量', 'error')
    if (operation === 'delete') {
      state.envs = state.envs.filter(env => !ids.includes(env.id))
      state.envSelected = []
      renderView()
      return toast(`已删除 ${ids.length} 个变量`)
    }
    state.envs.forEach(env => {
      if (!ids.includes(env.id)) return
      env.enabled = operation === 'enable'
      env.updatedAt = '刚刚'
    })
    state.envSelected = []
    renderView()
    toast(`已${operation === 'enable' ? '启用' : '停用'} ${ids.length} 个变量`)
  }

  function moveEnv(id, offset) {
    const sorted = state.envs.slice().sort((left, right) => left.id - right.id)
    const index = sorted.findIndex(env => env.id === Number(id))
    const target = sorted[index + Number(offset)]
    if (index < 0 || !target) return
    const current = sorted[index]
    const currentId = current.id
    current.id = target.id
    target.id = currentId
    state.revealed.clear()
    renderView()
    toast('已调整变量顺序')
  }

  /* ---------------- 脚本编辑器 ---------------- */
  function scriptTree() {
    const query = (state.scriptSearch || '').trim().toLowerCase()
    const rows = []
    const children = new Map()
    scriptFiles.forEach(file => {
      const parts = file.path.split('/')
      let prefix = ''
      parts.forEach((part, index) => {
        const isLeaf = index === parts.length - 1
        const path = prefix ? `${prefix}/${part}` : part
        if (isLeaf) children.set(path, { ...file, name: part, isLeaf: true })
        else if (!children.has(path)) children.set(path, { path, name: part, isLeaf: false, size: 0, updatedAt: '' })
        prefix = path
      })
    })
    const all = [...children.values()]
    const included = new Set()
    if (query) {
      all.forEach(file => {
        if (!file.path.toLowerCase().includes(query)) return
        let name = file.path
        while (name) { included.add(name); name = name.includes('/') ? name.slice(0, name.lastIndexOf('/')) : '' }
      })
    }
    const grouped = new Map()
    all.forEach(file => {
      const parent = file.path.includes('/') ? file.path.slice(0, file.path.lastIndexOf('/')) : ''
      if (!grouped.has(parent)) grouped.set(parent, [])
      grouped.get(parent).push(file)
    })
    function walk(directory, depth) {
      const entries = (grouped.get(directory) || []).sort((left, right) => Number(left.isLeaf) - Number(right.isLeaf) || left.path.localeCompare(right.path, 'zh-CN'))
      entries.forEach(entry => {
        if (query && !included.has(entry.path)) return
        rows.push({ ...entry, depth })
        if (!entry.isLeaf && (query || state.scriptExpanded.has(entry.path))) walk(entry.path, depth + 1)
      })
    }
    walk('', 0)
    return rows
  }

  function renderScripts() {
    const current = scriptFiles.find(file => file.path === state.activeFile)
    const rows = scriptTree()
    const selectable = rows.filter(row => row.isLeaf)
    const canLeave = !current || state.scriptContent === current.content
    const language = current ? (current.path.endsWith('.py') ? 'Python' : current.path.endsWith('.js') ? 'Node.js' : 'Shell') : ''
    return `${pageHeading('WORKSPACE', '脚本管理', '编辑、保存并调试 Python / JavaScript / Shell 脚本。', `<button class="button compact" data-action="new-folder">${icon('folderPlus', 14)}新建文件夹</button><button class="button primary" data-action="new-file">${icon('plus', 14)}新建文件</button>`)}
      ${state.scriptSelected.length ? `<div class="bulk-bar"><span>已选择 ${state.scriptSelected.length} 项</span><button type="button" data-script-bulk="download">${icon('download', 13)}下载</button><button type="button" data-script-bulk="move">移动</button><button type="button" data-script-bulk="copy">复制</button><button type="button" class="danger-text" data-script-bulk="delete">删除</button><button type="button" data-script-bulk="clear">取消选择</button></div>` : ''}
      <div class="card editor-layout ${state.scriptFullscreen ? 'workspace-fullscreen' : ''}"><aside class="file-browser"><label class="search-field" style="max-width:none;margin-bottom:8px">${icon('search', 13)}<input id="script-search" placeholder="搜索文件与文件夹…" autocomplete="off"></label><div class="file-browser-heading"><label class="inline-check"><input type="checkbox" data-script-select-all ${selectable.length && selectable.every(file => state.scriptSelected.includes(file.path)) ? 'checked' : ''} aria-label="选择可见文件">${scriptFiles.length} 文件</label><div class="actions"><button type="button" class="icon-btn" data-script-expand="all" aria-label="展开全部" title="展开全部">${icon('folder', 14)}</button><button type="button" class="icon-btn" data-script-expand="none" aria-label="折叠全部" title="折叠全部">${icon('folder', 14)}</button></div></div>
        <div class="file-tree" role="tree">${rows.length ? rows.map(row => `<div class="tree-row ${state.activeFile === row.path ? 'active' : ''} ${state.scriptSelected.includes(row.path) ? 'selected' : ''}" style="padding-left:${6 + row.depth * 13}px" data-script-row="${escapeHTML(row.path)}" data-leaf="${row.isLeaf}">${row.isLeaf ? `<input type="checkbox" data-script-check="${escapeHTML(row.path)}" ${state.scriptSelected.includes(row.path) ? 'checked' : ''} aria-label="选择 ${escapeHTML(row.path)}">` : `<span class="tree-spacer"></span>`}<button type="button" class="tree-file-button" data-script-open="${escapeHTML(row.path)}" data-leaf="${row.isLeaf}">${icon(row.isLeaf ? 'fileCode' : (state.scriptExpanded.has(row.path) || state.scriptSearch) ? 'folder' : 'folder', 14)}<span>${escapeHTML(row.path.split('/').pop())}</span></button>${row.isLeaf && hasCron(row.content) ? '<span class="badge success">cron</span>' : ''}<button type="button" class="icon-btn file-menu-trigger" data-script-menu="${escapeHTML(row.path)}" aria-label="文件菜单">${icon('more', 13)}</button></div>`).join('') : `<p class="file-empty">${state.scriptSearch ? '没有匹配的文件' : '暂无文件，可以创建文件夹或脚本。'}</p>`}</div></aside>
        <div class="editor-main">${state.scriptFullscreen ? `<div class="fullscreen-strip"><span>全屏工作区</span><button type="button" class="button compact" data-script-fullscreen>退出全屏 · Esc</button></div>` : ''}${current ? `<div class="editor-toolbar"><div class="editor-file">${icon('code', 15)}<strong>${escapeHTML(current.path)}</strong><span class="badge">${language}</span>${canLeave ? '' : '<span class="unsaved-dot" title="有未保存的修改"></span>'}</div><div class="actions"><button type="button" class="button compact ghost" data-script-history>${icon('undo', 13)}历史</button><button type="button" class="button compact ghost" data-script-download="${escapeHTML(current.path)}" aria-label="下载文件">${icon('download', 13)}</button><button type="button" class="button compact ghost" data-script-fullscreen aria-label="全屏">${icon('maximize', 13)}</button><button type="button" class="button compact primary" data-action="save-script">${icon('check', 13)}保存</button></div></div><div class="editor-body"><pre class="line-numbers" id="line-numbers"></pre><textarea class="code-area" id="script-editor" spellcheck="false" aria-label="脚本内容"></textarea></div><div class="editor-status"><span>UTF-8 · 纯前端演示编辑器 · 第 1 行</span><span>Ctrl+S 保存 · Esc 退出全屏</span></div>` : `<div class="empty-state editor-empty"><div class="empty-icon">${icon('code', 30)}</div><h3>选择一个脚本，开始编辑</h3><p>支持文件夹树、多选、移动复制与创建定时任务。</p><button type="button" class="button primary" data-action="new-file">${icon('plus', 14)}新建脚本</button></div>`}</div></div>`
  }

  function updateLineNumbers() {
    const editor = document.getElementById('script-editor')
    const numbers = document.getElementById('line-numbers')
    if (editor && numbers) {
      editor.value = state.scriptContent
      numbers.textContent = state.scriptContent.split('\n').map((_, index) => index + 1).join('\n')
    }
  }

  function hasCron(content) {
    return /^\s*(?:#|\/\/|\*)\s*@?cron\b/im.test((content || '').slice(0, 4000))
  }

  function openScript(path) {
    const file = scriptFiles.find(item => item.path === path)
    if (!file) {
      if (state.scriptExpanded.has(path)) state.scriptExpanded.delete(path)
      else state.scriptExpanded.add(path)
      renderView()
      return
    }
    const current = scriptFiles.find(item => item.path === state.activeFile)
    if (current && state.scriptContent !== current.content) {
      return confirmModal('放弃未保存的修改？', '当前脚本有未保存的修改，切换后这些修改会丢失。', () => {
        state.activeFile = file.path
        state.scriptContent = file.content
        renderView()
      })
    }
    state.activeFile = file.path
    state.scriptContent = file.content
    renderView()
  }

  function saveScript() {
    const editor = document.getElementById('script-editor')
    if (editor) state.scriptContent = editor.value
    const file = scriptFiles.find(item => item.path === state.activeFile)
    if (!file) return
    if (file.content === state.scriptContent) return toast('没有需要保存的修改')
    file.content = state.scriptContent
    file.size = state.scriptContent.length
    file.updatedAt = '刚刚'
    scriptHistory.unshift({ id: Date.now(), path: file.path, savedAt: '刚刚', size: file.size })
    renderView()
    toast('脚本已保存（演示）')
  }

  function scriptOperation(operation, paths) {
    if (!paths.length) return
    if (operation === 'delete') {
      return confirmModal('删除选中的 ' + paths.length + ' 项？', '所选脚本或配置会被永久删除。已有任务记录会保留，但引用这些脚本的任务可能无法继续运行。', () => {
        const set = new Set(paths)
        for (let index = scriptFiles.length - 1; index >= 0; index--) {
          if (set.has(scriptFiles[index].path)) scriptFiles.splice(index, 1)
        }
        if (set.has(state.activeFile)) { state.activeFile = ''; state.scriptContent = '' }
        state.scriptSelected = state.scriptSelected.filter(path => !set.has(path))
        renderView()
        toast(`已删除 ${paths.length} 项（演示）`)
      }, '删除')
    }
    const file = scriptFiles.find(item => item.path === paths[0])
    if (!file) return
    const defaultName = operation === 'rename' ? file.path : (file.path.includes('/') ? file.path.slice(0, file.path.lastIndexOf('/')) : '')
    openModal(operation === 'rename' ? '重命名' : operation === 'move' ? '移动所选文件' : '复制所选文件', `<form data-form="script-op" class="form-grid"><label class="field"><span>${operation === 'rename' ? '新路径' : '目标文件夹'}</span><input name="destination" required value="${escapeHTML(operation === 'rename' ? defaultName : '')}" placeholder="${operation === 'rename' ? 'tools/example.py' : '留空表示根目录'}"><small>${operation === 'rename' ? '路径使用 / 分隔，支持中文名称。' : '保留原文件名；已存在的目标不会覆盖。'}</small></label><input type="hidden" name="operation" value="${operation}"><input type="hidden" name="paths" value="${escapeHTML(paths.join('\n'))}"><div class="actions" style="justify-content:flex-end;margin-top:4px"><button type="button" class="button" data-close-modal>取消</button><button type="submit" class="button primary">${icon('check', 14)}确认</button></div></form>`)
  }

  function submitScriptOperation(form) {
    const data = new FormData(form)
    const operation = String(data.get('operation') || '')
    const paths = String(data.get('paths') || '').split('\n').filter(Boolean)
    const destination = String(data.get('destination') || '').trim().replace(/^\/+|\/+$/g, '')
    if (!paths.length) return
    paths.forEach(path => {
      const file = scriptFiles.find(item => item.path === path)
      if (!file) return
      const target = operation === 'rename' ? destination : (destination ? `${destination}/${path.split('/').pop()}` : path.split('/').pop())
      if (!target) return
      if (operation === 'copy') {
        if (scriptFiles.some(item => item.path === target)) return
        scriptFiles.push({ path: target, content: file.content, size: file.size, updatedAt: '刚刚' })
      } else {
        file.path = target
        file.updatedAt = '刚刚'
        if (state.activeFile === path) state.activeFile = target
      }
    })
    closeModal()
    renderView()
    toast(`已${operation === 'rename' ? '重命名' : operation === 'move' ? '移动' : '复制'} ${paths.length} 项（演示）`)
  }

  function openNewScript(kind) {
    openModal(kind === 'folder' ? '新建文件夹' : '新建文件', `<form data-form="script-new" class="form-grid"><label class="field"><span>${kind === 'folder' ? '文件夹路径' : '文件路径'}</span><input name="path" required placeholder="${kind === 'folder' ? 'tools' : 'tools/example.py'}"><small>路径使用 / 分隔，支持中文名称。</small></label><input type="hidden" name="kind" value="${kind}"><div class="actions" style="justify-content:flex-end;margin-top:4px"><button type="button" class="button" data-close-modal>取消</button><button type="submit" class="button primary">${icon('plus', 14)}创建</button></div></form>`)
  }

  function submitNewScript(form) {
    const data = new FormData(form)
    const path = String(data.get('path') || '').trim().replace(/^\/+|\/+$/g, '')
    const kind = String(data.get('kind') || 'file')
    if (!path) return
    if (scriptFiles.some(item => item.path === path)) return toast('该路径已存在', 'error')
    if (kind === 'folder') {
      state.scriptExpanded.add(path)
      closeModal()
      renderView()
      return toast(`已创建文件夹 ${path}（演示）`)
    }
    scriptFiles.push({ path, content: '', size: 0, updatedAt: '刚刚' })
    state.activeFile = path
    state.scriptContent = ''
    closeModal()
    renderView()
    toast(`已创建文件 ${path}（演示）`)
  }

  function openScriptHistory() {
    const file = scriptFiles.find(item => item.path === state.activeFile)
    if (!file) return
    const items = scriptHistory.filter(item => item.path === file.path)
    const rows = items.length
      ? items.map(item => `<div class="log-row"><div class="log-info"><strong>${escapeHTML(item.savedAt)}</strong><small>${item.size} 字节</small></div><button type="button" class="button compact ghost" data-restore-history="${item.id}">${icon('undo', 12)}恢复</button></div>`).join('')
      : '<div class="task-empty">还没有历史版本</div>'
    openModal(`历史版本 · ${file.path}`, `<div class="log-list">${rows}</div><div class="actions" style="justify-content:flex-end;margin-top:12px"><button type="button" class="button" data-close-modal>关闭</button></div>`)
  }

  function openScriptMenu(path) {
    const file = scriptFiles.find(item => item.path === path)
    if (!file) {
      return confirmModal('删除文件夹？', '演示模式不会真正删除文件夹。', () => toast('已删除（演示）'), '删除')
    }
    openModal(`文件操作 · ${path}`, `<div class="menu-list"><button type="button" class="menu-item" data-script-open="${escapeHTML(path)}">${icon('edit', 15)}打开编辑</button><button type="button" class="menu-item" data-script-rename="${escapeHTML(path)}">${icon('edit', 15)}重命名</button><button type="button" class="menu-item" data-script-download="${escapeHTML(path)}">${icon('download', 15)}下载文件</button><button type="button" class="menu-item" data-script-move="${escapeHTML(path)}">${icon('move', 15)}移动</button><button type="button" class="menu-item" data-script-copy="${escapeHTML(path)}">${icon('copy', 15)}复制</button><button type="button" class="menu-item danger" data-script-delete="${escapeHTML(path)}">${icon('trash', 15)}删除</button></div><div class="actions" style="justify-content:flex-end;margin-top:12px"><button type="button" class="button" data-close-modal>关闭</button></div>`)
  }

  /* ---------------- 日志 ---------------- */
  function renderLogs() {
    return `${pageHeading('OBSERVABILITY', '运行日志', '查看任务执行记录与输出，运行中的任务会实时显示。')}
      <section class="card"><div class="card-heading"><h2>${icon('file', 17)}最近运行</h2><span class="muted">${state.runs.length} 条记录</span></div><div class="log-list">${state.runs.length ? state.runs.map(run => `<div class="log-row"><span class="run-status ${run.status}">${icon(run.status === 'success' ? 'check' : run.status === 'running' ? 'activity' : 'alert', 12)}</span><div class="log-info"><strong>${escapeHTML(run.name)}</strong><small>${statusLabel[run.status] || run.status} · ${(run.duration || 0).toFixed(1)} 秒</small></div><span class="log-time">${escapeHTML(run.started || '刚刚')}</span><button class="button compact ghost" data-open-log="${run.id}">${icon('file', 13)}查看日志</button></div>`).join('') : '<div class="task-empty">还没有运行记录</div>'}</div></section>`
  }

  /* ---------------- 订阅 ---------------- */
  function renderSubscriptions() {
    const query = (state.subSearch || '').trim().toLowerCase()
    const list = state.subscriptions.filter(sub => !query || `${sub.name} ${sub.url} ${sub.alias}`.toLowerCase().includes(query))
    return `${pageHeading('SYNC', '订阅管理', '从 Git 仓库或单文件拉取脚本，按头部 cron 注释自动创建任务。', `<button class="button primary" data-action="add-sub">${icon('plus', 15)}添加订阅</button>`)}
      <div class="section-note" style="margin-bottom:12px">共 ${state.subscriptions.length} 条订阅 · 同步后脚本写入 scripts/别名/</div>
      ${state.subSelected.length ? `<div class="bulk-bar"><span>已选择 ${state.subSelected.length} 项</span><button type="button" data-sub-bulk="run">同步</button><button type="button" data-sub-bulk="stop">停止</button><button type="button" data-sub-bulk="enable">启用</button><button type="button" data-sub-bulk="disable">停用</button><button type="button" class="danger-text" data-sub-bulk="delete">删除</button><button type="button" data-sub-bulk="clear">取消选择</button></div>` : ''}
      <section class="card"><div class="card-heading"><label class="search-field" style="max-width:none;flex:1">${icon('search', 14)}<input id="sub-search" placeholder="搜索订阅名称或地址…" autocomplete="off"></label></div>
      <div class="table-wrap"><table><thead><tr><th class="checkbox-column"><input type="checkbox" data-sub-select-all ${list.length && list.every(sub => state.subSelected.includes(sub.id)) ? 'checked' : ''} aria-label="选择全部订阅"></th><th>名称</th><th>同步计划 / 来源</th><th>最近同步</th><th>启用</th><th class="align-right">操作</th></tr></thead><tbody>${list.length ? list.map(subRow).join('') : `<tr><td colspan="6"><div class="empty-state"><div class="empty-icon">${icon('git', 26)}</div><h3>尚无订阅</h3><p>订阅一个仓库，让脚本更新自动发生。</p><button class="button" data-action="add-sub">${icon('plus', 14)}添加订阅</button></div></td></tr>`}</tbody></table></div></section>`
  }

  function subRow(sub) {
    return `<tr data-sub-row data-sub-id="${sub.id}" class="${sub.enabled ? '' : 'row-disabled'} ${state.subSelected.includes(sub.id) ? 'selected-row' : ''}">
      <td><input type="checkbox" data-sub-check="${sub.id}" ${state.subSelected.includes(sub.id) ? 'checked' : ''} aria-label="选择 ${escapeHTML(sub.name)}"></td>
      <td><div class="resource-name"><span class="resource-icon">${icon('git', 14)}</span><div class="resource-copy"><div class="resource-title-line"><strong>${escapeHTML(sub.name)}</strong><span class="record-id">#${sub.id}</span></div><small class="secondary-line">${escapeHTML(sub.alias)}</small></div></div></td>
      <td><code>${escapeHTML(sub.schedule || '手动同步')}</code><span class="secondary-line ellipsis resource-url">${escapeHTML(sub.url)}</span></td>
      <td class="muted nowrap">${escapeHTML(sub.syncing ? '同步中…' : sub.lastSync)}</td>
      <td><label class="switch"><input type="checkbox" ${sub.enabled ? 'checked' : ''} data-toggle-sub="${sub.id}" aria-label="启用 ${escapeHTML(sub.name)}"><span></span></label></td>
      <td><div class="row-actions"><button type="button" class="icon-btn" data-sync-sub="${sub.id}" ${sub.syncing ? 'disabled' : ''} aria-label="同步订阅" title="同步">${icon('refresh', 15)}</button><button type="button" class="icon-btn" data-sub-log="${sub.id}" aria-label="查看执行日志" title="日志">${icon('file', 15)}</button><button type="button" class="icon-btn" data-sub-edit="${sub.id}" aria-label="编辑记录" title="编辑">${icon('edit', 15)}</button><button type="button" class="icon-btn" data-delete-sub="${sub.id}" aria-label="删除记录" title="删除">${icon('trash', 15)}</button></div></td>
    </tr>`
  }

  function openSubLog(id) {
    const sub = state.subscriptions.find(item => item.id === id)
    if (!sub) return
    openModal(`同步日志 · ${sub.name}`, `<pre class="terminal">${escapeHTML(`[INFO] 开始同步 ${sub.url}\n[INFO] 类型: ${sub.type === 'git-repo' ? 'Git 仓库' : '单文件'}\n[INFO] 别名: ${sub.alias}\n[INFO] 拉取完成，共 ${sub.files} 个文件\n[INFO] 写入 scripts/${sub.alias}/\n[INFO] 上次同步: ${sub.lastSync}`)}</pre><div class="actions" style="justify-content:flex-end;margin-top:12px"><button type="button" class="button" data-close-modal>关闭</button></div>`)
  }

  function openSubForm(id) {
    const sub = id ? state.subscriptions.find(item => item.id === Number(id)) : null
    const value = sub || { name: '', type: 'git-repo', url: '', alias: '', schedule: '0 */6 * * *', branch: '', autoAddCron: true, autoDelCron: false }
    openModal(sub ? '编辑订阅' : '添加订阅', `<form data-form="sub" class="form-grid">
      <label class="field"><span>订阅名称</span><input name="name" required value="${escapeHTML(value.name)}" placeholder="例如：示例脚本仓库"></label>
      <label class="field"><span>订阅类型</span><select name="type"><option value="git-repo" ${value.type === 'git-repo' ? 'selected' : ''}>Git 仓库</option><option value="file" ${value.type === 'file' ? 'selected' : ''}>单个脚本文件</option></select></label>
      <label class="field"><span>订阅 URL</span><input name="url" required value="${escapeHTML(value.url)}" placeholder="https://github.com/example/scripts.git"><small>必须是不含内嵌凭证的 HTTP(S) 地址。</small></label>
      <label class="field"><span>目录别名</span><input name="alias" required value="${escapeHTML(value.alias)}" placeholder="example"><small>字母、数字、连字符或下划线，脚本同步到 scripts/别名/。</small></label>
      <label class="field"><span>同步 Cron</span><input name="schedule" value="${escapeHTML(value.schedule || '')}" placeholder="0 */6 * * *"><small>留空表示仅手动同步。</small></label>
      <div class="switch-row"><div><strong>根据脚本头部 cron 注释自动创建任务</strong><small>扫描前 50 行注释，任务名取脚本文件名</small></div><label class="switch"><input type="checkbox" name="autoAddCron" ${value.autoAddCron ? 'checked' : ''}><span></span></label></div>
      <div class="switch-row"><div><strong>同步删除源仓库中已移除的脚本与关联任务</strong><small>谨慎开启</small></div><label class="switch"><input type="checkbox" name="autoDelCron" ${value.autoDelCron ? 'checked' : ''}><span></span></label></div>
      <div class="actions" style="justify-content:flex-end;margin-top:4px"><button type="button" class="button" data-close-modal>取消</button><button type="submit" class="button primary">${icon('check', 14)}${sub ? '保存订阅' : '添加订阅'}</button></div>
      ${sub ? `<input type="hidden" name="id" value="${sub.id}">` : ''}
    </form>`, true)
  }

  function toggleSubSelection(id, checked) {
    const numeric = Number(id)
    if (checked) {
      if (!state.subSelected.includes(numeric)) state.subSelected.push(numeric)
    } else {
      state.subSelected = state.subSelected.filter(item => item !== numeric)
    }
    renderView()
  }

  function bulkSubOperation(operation) {
    const ids = state.subSelected.slice()
    if (operation === 'clear') { state.subSelected = []; renderView(); return }
    if (!ids.length) return toast('请先选择订阅', 'error')
    if (operation === 'delete') {
      state.subscriptions = state.subscriptions.filter(sub => !ids.includes(sub.id))
      state.subSelected = []
      renderView()
      return toast(`已删除 ${ids.length} 个订阅`)
    }
    if (operation === 'run') { ids.forEach(id => syncSubscription(id)); return }
    state.subscriptions.forEach(sub => { if (ids.includes(sub.id)) sub.enabled = operation === 'enable' })
    state.subSelected = []
    renderView()
    toast(`已${operation === 'enable' ? '启用' : '停用'} ${ids.length} 个订阅`)
  }

  function syncSubscription(id) {
    const sub = state.subscriptions.find(item => item.id === id)
    if (!sub || sub.syncing) return
    sub.syncing = true
    renderView()
    toast(`正在同步「${sub.name}」`)
    setTimeout(() => {
      sub.syncing = false
      sub.lastSync = '刚刚'
      sub.files = Math.max(sub.files, 1)
      renderView()
      toast(`「${sub.name}」同步完成，已写入 scripts/${sub.alias}/`)
    }, 1400)
  }

  /* ---------------- 依赖 ---------------- */
  function renderDependencies() {
    const labels = ['Node.js', 'Python', '系统依赖']
    const hints = ['Node 依赖安装到独立目录，默认禁止 npm 生命周期脚本。原生扩展请在可信环境中预装。', 'Python 依赖使用面板专属虚拟环境，不修改宿主 Python 环境。', '系统包安装需要 Linux 宿主具备 apt-get 或 apk 和相应安装权限。']
    const list = state.dependencies.filter(dep => dep.type === state.depCategory)
    return `${pageHeading('RUNTIME', '依赖管理', 'Node.js、Python 与 Linux 系统包统一管理。', `<button class="button primary" data-action="add-dep">${icon('plus', 15)}添加依赖</button>`)}
      <div class="dep-tabs">${labels.map((label, index) => `<button type="button" class="${state.depCategory === index ? 'active' : ''}" data-dep-category="${index}">${label}<span class="muted"> ${state.dependencies.filter(dep => dep.type === index).length}</span></button>`).join('')}</div>
      <div class="info-banner" style="margin-bottom:12px">${icon('info', 15)}<span>${hints[state.depCategory]}</span></div>
      ${state.depSelected.length ? `<div class="bulk-bar"><span>已选择 ${state.depSelected.length} 项</span><button type="button" data-dep-bulk="reinstall">重新安装</button><button type="button" class="danger-text" data-dep-bulk="delete">卸载</button><button type="button" data-dep-bulk="clear">取消选择</button></div>` : ''}
      <section class="card"><div class="table-wrap"><table><thead><tr><th class="checkbox-column"><input type="checkbox" data-dep-select-all ${list.length && list.every(dep => state.depSelected.includes(dep.id)) ? 'checked' : ''} aria-label="选择全部依赖"></th><th>名称</th><th>状态</th><th>备注</th><th class="align-right">操作</th></tr></thead><tbody>${list.length ? list.map(depRow).join('') : `<tr><td colspan="5"><div class="empty-state"><div class="empty-icon">${icon('package', 26)}</div><h3>该分类还没有依赖</h3><p>添加依赖，后台会为您完成安装。</p><button class="button" data-action="add-dep">${icon('plus', 14)}添加依赖</button></div></td></tr>`}</tbody></table></div></section>`
  }

  function depRow(dep) {
    const labels = ['npm', 'pip', '系统包']
    const statusClass = dep.status === 'installed' ? 'success' : dep.status === 'installing' ? 'running' : 'failed'
    const statusText = dep.status === 'installed' ? '已安装' : dep.status === 'installing' ? '等待 / 安装中' : '未就绪'
    return `<tr data-dep-row data-dep-id="${dep.id}" class="${state.depSelected.includes(dep.id) ? 'selected-row' : ''}">
      <td><input type="checkbox" data-dep-check="${dep.id}" ${state.depSelected.includes(dep.id) ? 'checked' : ''} aria-label="选择 ${escapeHTML(dep.name)}"></td>
      <td><div class="resource-name"><span class="resource-icon">${icon('package', 14)}</span><div class="resource-copy"><div class="resource-title-line"><strong>${escapeHTML(dep.name)}</strong><span class="record-id">#${dep.id}</span></div><small class="secondary-line">${escapeHTML(labels[dep.type])}${dep.version ? ' · ' + escapeHTML(dep.version) : ''}</small></div></div></td>
      <td><span class="badge ${statusClass}">${statusText}</span></td>
      <td class="muted">${escapeHTML(dep.remark || '—')}</td>
      <td><div class="row-actions"><button type="button" class="icon-btn" data-reinstall-dep="${dep.id}" ${dep.status === 'installing' ? 'disabled' : ''} aria-label="重新安装" title="重新安装">${icon('refresh', 15)}</button><button type="button" class="icon-btn" data-dep-log="${dep.id}" aria-label="查看安装日志" title="日志">${icon('file', 15)}</button><button type="button" class="icon-btn" data-dep-edit="${dep.id}" aria-label="编辑" title="编辑">${icon('edit', 15)}</button><button type="button" class="icon-btn" data-dep-delete="${dep.id}" aria-label="卸载" title="卸载">${icon('trash', 15)}</button></div></td>
    </tr>`
  }

  function openDepLog(id) {
    const dep = state.dependencies.find(item => item.id === Number(id))
    if (!dep) return
    openModal(`安装日志 · ${dep.name}`, `<pre class="terminal">${escapeHTML(`[INFO] 开始安装 ${dep.name}\n[INFO] 类型: ${['npm', 'pip', '系统包'][dep.type]}\n[INFO] 版本: ${dep.version || 'latest'}\n${dep.status === 'installing' ? '[INFO] 正在下载并安装依赖…' : dep.status === 'installed' ? '[INFO] 安装完成' : '[ERROR] 安装失败，请查看完整日志'}`)}</pre><div class="actions" style="justify-content:flex-end;margin-top:12px"><button type="button" class="button" data-close-modal>关闭</button></div>`)
  }

  function openDepForm(id) {
    const dep = id ? state.dependencies.find(item => item.id === Number(id)) : null
    const value = dep || { name: '', type: state.depCategory, remark: '' }
    openModal(dep ? '编辑依赖' : '添加依赖', `<form data-form="dep" class="form-grid">
      <label class="field"><span>包名称</span><input name="name" required value="${escapeHTML(value.name)}" placeholder="${state.depCategory === 1 ? 'requests==2.32.5' : state.depCategory === 0 ? 'axios@1.12.2' : 'curl'}"><small>每次添加一个包。提交后将异步安装。</small></label>
      <label class="field"><span>依赖类型</span><select name="type">${['Node.js (npm)', 'Python (pip / 独立虚拟环境)', 'Linux 系统包 (apt / apk)'].map((label, index) => `<option value="${index}" ${value.type === index ? 'selected' : ''}>${label}</option>`).join('')}</select></label>
      <label class="field"><span>备注</span><input name="remark" value="${escapeHTML(value.remark || '')}" placeholder="可选"></label>
      <div class="actions" style="justify-content:flex-end;margin-top:4px"><button type="button" class="button" data-close-modal>取消</button><button type="submit" class="button primary">${icon('check', 14)}${dep ? '保存' : '开始安装'}</button></div>
      ${dep ? `<input type="hidden" name="id" value="${dep.id}">` : ''}
    </form>`)
  }

  function toggleDepSelection(id, checked) {
    const numeric = Number(id)
    if (checked) {
      if (!state.depSelected.includes(numeric)) state.depSelected.push(numeric)
    } else {
      state.depSelected = state.depSelected.filter(item => item !== numeric)
    }
    renderView()
  }

  function bulkDepOperation(operation) {
    const ids = state.depSelected.slice()
    if (operation === 'clear') { state.depSelected = []; renderView(); return }
    if (!ids.length) return toast('请先选择依赖', 'error')
    if (operation === 'delete') {
      state.dependencies = state.dependencies.filter(dep => !ids.includes(dep.id))
      state.depSelected = []
      renderView()
      return toast(`已卸载 ${ids.length} 个依赖`)
    }
    ids.forEach(id => reinstallDependency(id))
    state.depSelected = []
  }

  function reinstallDependency(id) {
    const dep = state.dependencies.find(item => item.id === id)
    if (!dep || dep.status === 'installing') return
    dep.status = 'installing'
    renderView()
    toast(`正在重新安装 ${dep.name}`)
    setTimeout(() => {
      dep.status = 'installed'
      renderView()
      toast(`${dep.name} 安装完成`)
    }, 1400)
  }

  /* ---------------- 接口文档 ---------------- */
  function apiVisibleGroups() {
    const tab = state.apiTab
    const query = (state.apiSearch || '').trim().toLowerCase()
    return apiGroups
      .map(group => ({
        ...group,
        endpoints: group.endpoints.filter(ep => {
          if (!(ep.tabs || ['api']).includes(tab)) return false
          if (!query) return true
          return [ep.method, ep.path, ep.summary, ep.description, ep.scope, group.label, group.intro]
            .filter(Boolean).join(' ').toLowerCase().includes(query)
        })
      }))
      .filter(group => group.endpoints.length)
  }

  function apiCount(tab) {
    return apiGroups.reduce((total, group) => total + group.endpoints.filter(ep => (ep.tabs || ['api']).includes(tab)).length, 0)
  }

  function apiPrefix() {
    return state.apiTab === 'open' ? '/open' : '/api'
  }

  function apiFullPath(ep) {
    if (ep.absolute) return ep.path
    return apiPrefix() + ep.path
  }

  function renderApiDocs() {
    const groups = apiVisibleGroups()
    const total = apiCount(state.apiTab)
    const origin = typeof location !== 'undefined' && location.protocol.startsWith('http') ? location.origin : 'https://panel.example.com'
    const banner = state.apiTab === 'open'
      ? '开放应用接口需要 <code>Authorization: Bearer &lt;token&gt;</code>，权限由应用 scope 决定。先在「系统设置 → 开放应用 / MCP」创建应用并生成 Authorization。'
      : '内置接口使用当前登录会话（同源 HttpOnly Cookie）。系统、账号、开放应用与备份接口仅管理员可用。'
    return `<section class="apidocs-hero"><div class="apidocs-hero-main"><span class="eyebrow">DEVELOPER · HTTP API</span><h1>${icon('book', 26)}接口文档</h1><p>开放 API 与内置 API 使用同一套资源路由，区别是前缀与权限：<code>/open</code> 面向开放应用，按应用 scope 授权；<code>/api</code> 面向面板自身，包含仅管理员可用的系统、账号、应用与备份接口。</p></div><div class="apidocs-hero-side"><span class="badge">${escapeHTML(origin)}</span><span class="badge success">当前前缀 ${apiPrefix()}</span></div></section>
      <div class="apidocs-toolbar"><div class="tabs">${[['open', '开放 API'], ['api', '内置 API']].map(([id, label]) => `<button type="button" class="${state.apiTab === id ? 'active' : ''}" data-api-tab="${id}">${label}<span>${apiCount(id)}</span></button>`).join('')}</div><label class="search-field apidocs-search">${icon('search', 14)}<input id="api-search" placeholder="搜索路径、名称或权限…" autocomplete="off"></label><div class="actions"><button type="button" class="button compact ghost" data-api-expand="all">全部展开</button><button type="button" class="button compact ghost" data-api-expand="none">全部折叠</button></div></div>
      <div class="info-banner apidocs-banner">${icon('alert', 16)}<span>${banner}</span></div>
      <div class="apidocs-layout"><aside class="apidocs-nav" aria-label="接口分组">${groups.length ? groups.map(group => `<button type="button" data-api-jump="${group.id}">${icon(group.icon, 16)}<span>${group.label}</span><em>${group.endpoints.length}</em></button>`).join('') : '<p class="apidocs-nav-empty">没有匹配的接口</p>'}</aside>
      <div class="apidocs-body">${groups.length ? groups.map(group => apiGroupCard(group)).join('') : `<div class="card empty-state apidocs-empty"><div class="empty-icon">${icon('search', 28)}</div><h3>没有匹配的接口</h3><p>试试其他关键词，或切换开放 API / 内置 API。</p></div>`}</div></div>
      <p class="settings-note" style="padding:14px 0 0">共 ${total} 个接口 · 演示数据，不会真正发起请求</p>`
  }

  function apiGroupCard(group) {
    return `<section class="apidocs-group" id="apidocs-${group.id}" data-api-group-container="${group.id}"><header class="apidocs-group-head"><h2><span class="apidocs-group-icon">${icon(group.icon, 18)}</span>${group.label}</h2><span class="muted">${group.endpoints.length} 个接口</span></header><p class="apidocs-group-intro">${escapeHTML(group.intro)}</p>${group.endpoints.map(ep => apiEndpointCard(group, ep)).join('')}</section>`
  }

  function apiEndpointCard(group, ep) {
    const key = ep.id || `${group.id}-${ep.method}-${ep.path}`
    const open = state.apiOpenEndpoints.has(key)
    const scope = ep.noAuth ? '公开' : (ep.scope || '管理员')
    const text = `${group.label} ${ep.method} ${ep.path} ${ep.summary} ${ep.description} ${ep.scope || ''}`.toLowerCase()
    const body = open ? `<div class="endpoint-body"><p class="endpoint-desc">${escapeHTML(ep.description)}</p>${ep.method !== 'GET' ? `<p class="apidocs-warn">${icon('alert', 14)}这是写操作或会产生实际影响，发送前请确认参数。</p>` : ''}<div class="apidocs-preview"><span class="apidocs-preview-label">请求地址</span><code>${escapeHTML(ep.method + ' ' + apiFullPath(ep))}</code></div>${apiParamsBlock(ep)}${apiBodyBlock(ep)}<div class="apidocs-actions"><button type="button" class="button compact primary" data-api-send="${key}">${icon('play', 14)}发送请求</button><button type="button" class="button compact" data-copy-curl="${escapeHTML(apiCurl(ep))}">${icon('copy', 14)}复制 cURL</button><span class="apidocs-inline-note">演示模式不会真正请求后端</span></div>${ep.response ? `<details class="apidocs-sample"><summary>示例响应</summary><pre>${escapeHTML(JSON.stringify(ep.response, null, 2))}</pre></details>` : ''}</div>` : ''
    return `<article class="endpoint ${open ? 'open' : ''}" data-api-endpoint="${key}" data-api-text="${escapeHTML(text)}"><button type="button" class="endpoint-head" aria-expanded="${open}" data-api-toggle="${key}"><span class="method ${ep.method.toLowerCase()}">${ep.method}</span><code class="endpoint-path">${escapeHTML(ep.path)}</code><span class="endpoint-summary">${escapeHTML(ep.summary)}</span><span class="endpoint-scope ${ep.noAuth ? 'public' : ''}">${escapeHTML(scope)}</span><span class="endpoint-chevron">${icon('chevron', 16)}</span></button>${body}</article>`
  }

  function apiParamsBlock(ep) {
    const params = ep.params || []
    if (!params.length) return ''
    return `<div class="apidocs-block"><h4>参数</h4><div class="apidocs-fields">${params.map(param => `<label class="apidocs-field"><span><code>${escapeHTML(param.name)}</code>${param.required !== false && param.in === 'path' ? '<em>必填</em>' : ''}</span><input value="${escapeHTML(param.example || '')}" placeholder="${escapeHTML(param.name)}" aria-label="${escapeHTML(param.name)}"><small>${escapeHTML(param.in === 'path' ? '路径参数' : '查询参数')}${param.description ? ' · ' + escapeHTML(param.description) : ''}</small></label>`).join('')}</div></div>`
  }

  function apiBodyBlock(ep) {
    if (ep.body === undefined) return ''
    return `<div class="apidocs-block"><h4>请求体 <span class="muted">JSON</span></h4><textarea class="apidocs-editor" rows="8" spellcheck="false" aria-label="请求体 JSON">${escapeHTML(JSON.stringify(ep.body, null, 2))}</textarea></div>`
  }

  function apiCurl(ep) {
    const base = typeof location !== 'undefined' && location.protocol.startsWith('http') ? location.origin : 'https://panel.example.com'
    const lines = [`curl -X ${ep.method} '${base}${apiFullPath(ep)}'`, "  -H 'Authorization: Bearer <token>'"]
    if (ep.body !== undefined) lines.push("  -H 'Content-Type: application/json'", `  -d '${JSON.stringify(ep.body)}'`)
    return lines.join(' \\\n')
  }

  function applyApiFilter() {
    const query = (state.apiSearch || '').trim().toLowerCase()
    document.querySelectorAll('[data-api-group-container]').forEach(group => {
      let visible = 0
      group.querySelectorAll('[data-api-endpoint]').forEach(endpoint => {
        const match = !query || endpoint.dataset.apiText.includes(query)
        endpoint.hidden = !match
        if (match) visible++
      })
      group.hidden = visible === 0
    })
    const nav = document.querySelector('.apidocs-nav')
    if (!nav) return
    nav.querySelectorAll('[data-api-jump]').forEach(button => {
      const target = document.getElementById('apidocs-' + button.dataset.apiJump)
      button.hidden = !target || target.hidden
    })
  }

  /* ---------------- 设置 ---------------- */
  function renderSettings() {
    const nav = [
      ['general', '常规设置', 'settings'],
      ['alerts', '告警通知', 'bell'],
      ['backups', '备份恢复', 'download'],
      ['security', '账户安全', 'shield'],
      ['about', '关于', 'book']
    ]
    return `${pageHeading('SETTINGS', '系统设置', '演示面板中的常用设置，所有修改仅作用于当前页面。')}
      <div class="settings-layout"><aside class="settings-nav">${nav.map(([id, label, iconName]) => `<button type="button" class="${state.settingsTab === id ? 'active' : ''}" data-settings-tab="${id}">${icon(iconName, 15)}${label}</button>`).join('')}</aside><div class="settings-content">${settingsContent()}</div></div>`
  }

  function settingsContent() {
    if (state.settingsTab === 'alerts') {
      return `<section class="card settings-card"><div class="card-heading"><h2>${icon('bell', 17)}通知渠道</h2><span class="badge success">2 个已启用</span></div><div class="settings-fields"><div class="info-banner">${icon('bell')}<span>告警复用通知渠道；任务失败、连续登录失败和新 IP 登录等事件会按冷却时间推送。</span></div>${[['企业微信机器人', 'wecom'], ['Telegram Bot', 'telegram']].map(([name, type]) => `<div class="switch-row"><div><strong>${name}</strong><small>${type} · 演示渠道</small></div><span class="badge success">已启用</span></div>`).join('')}<div class="switch-row"><div><strong>任务失败 / 超时</strong><small>任务结束后推送告警</small></div><label class="switch"><input type="checkbox" checked><span></span></label></div><div class="switch-row"><div><strong>连续登录失败</strong><small>同一账号 15 分钟内达到阈值</small></div><label class="switch"><input type="checkbox" checked><span></span></label></div><div class="switch-row"><div><strong>新 IP 登录成功</strong><small>只对公网 IP 触发</small></div><label class="switch"><input type="checkbox" checked><span></span></label></div></div><div class="settings-footer"><button class="button primary" data-action="save-settings">${icon('check', 14)}保存告警设置</button></div></section>`
    }
    if (state.settingsTab === 'backups') {
      return `<section class="card settings-card"><div class="card-heading"><h2>${icon('download', 17)}备份与恢复</h2><span class="badge">自动备份已开启</span></div><div class="settings-fields"><p class="muted">备份包含任务、环境变量、应用、账号、脚本和配置；演示中不会真的写入文件。</p><div class="actions"><button class="button primary" data-action="create-backup">${icon('download', 14)}创建备份</button><button class="button" data-action="upload-backup">${icon('upload', 14)}导入备份</button></div><div class="switch-row"><div><strong>每日自动备份</strong><small>每天 03:00 · 保存到 data/backups/auto</small></div><label class="switch"><input type="checkbox" checked><span></span></label></div><div class="switch-row"><div><strong>自动清理</strong><small>保留 30 天 · 每天 04:00 清理</small></div><label class="switch"><input type="checkbox" checked><span></span></label></div></div><div class="settings-note">演示备份：2026-11-14 03:00 · auto-20261114-030000-demo.zip · 12.4 MB</div></section>`
    }
    if (state.settingsTab === 'security') {
      return `<section class="card settings-card"><div class="card-heading"><h2>${icon('shield', 17)}管理员账户</h2></div><div class="settings-fields"><label class="field"><span>用户名</span><input value="admin" readonly></label><label class="field"><span>当前密码</span><input type="password" value="demo-password" readonly></label><label class="field"><span>新密码</span><input type="password" placeholder="演示模式不修改真实账号"></label><div class="info-banner">${icon('shield')}<span>正式环境中密码使用 bcrypt 保存，会话 Cookie 为 HttpOnly + SameSite=Strict。</span></div></div><div class="settings-footer"><button class="button primary" data-action="save-settings">${icon('check', 14)}更新账户</button></div></section>`
    }
    if (state.settingsTab === 'about') {
      return `<section class="card settings-card"><div class="card-heading"><h2>${icon('book', 17)}关于</h2><span class="badge success">纯前端演示</span></div><div class="settings-fields"><div class="info-banner">${icon('book')}<span>大白菜面板 · 轻量自托管自动化任务面板。当前页面是静态演示，不包含后端服务。</span></div><div class="switch-row"><div><strong>版本</strong><small>演示版 · v1.0.0</small></div><span class="badge">DEMO</span></div><div class="switch-row"><div><strong>技术栈</strong><small>Go + Vue 3 + SQLite（正式版）</small></div><span class="muted">本页仅 HTML / CSS / JS</span></div><div class="actions"><button class="button" data-action="reset-demo">${icon('refresh', 14)}重置演示数据</button><button class="button primary" data-action="save-settings">${icon('check', 14)}检查更新</button></div></div></section>`
    }
    return `<section class="card settings-card"><div class="card-heading"><h2>${icon('settings', 17)}执行与存储</h2></div><div class="settings-fields"><label class="field"><span>全局最大并发数</span><input type="number" value="4" min="1" max="32"><small>超出部分自动排队。</small></label><label class="field"><span>默认执行超时（秒）</span><input type="number" value="3600"><small>任务可以单独覆盖。</small></label><label class="field"><span>日志保留天数</span><input type="number" value="30"><small>每天自动清理过期运行记录。</small></label><div class="switch-row"><div><strong>任务失败后自动安装缺失依赖</strong><small>默认关闭，避免信任脚本日志内容</small></div><label class="switch"><input type="checkbox"><span></span></label></div></div><div class="settings-footer"><button class="button primary" data-action="save-settings">${icon('check', 14)}保存设置</button></div></section>`
  }

  /* ---------------- 弹窗 / Toast ---------------- */
  function openModal(title, body, wide = false) {
    modalRoot.innerHTML = `<div class="overlay" data-overlay><section class="modal ${wide ? 'wide' : ''}" role="dialog" aria-modal="true" aria-labelledby="demo-modal-title"><div class="modal-head"><h2 id="demo-modal-title">${escapeHTML(title)}</h2><button type="button" class="icon-btn" data-close-modal aria-label="关闭">${icon('x')}</button></div><div class="modal-body">${body}</div></section></div>`
    const field = modalRoot.querySelector('input, textarea, select')
    if (field) setTimeout(() => field.focus(), 20)
  }

  function closeModal() {
    modalRoot.innerHTML = ''
    state.confirmAction = null
  }

  function confirmModal(title, message, action, confirmText = '确认') {
    state.confirmAction = action
    openModal(title, `<p class="muted" style="margin:0 0 14px;line-height:1.8">${escapeHTML(message)}</p><div class="actions" style="justify-content:flex-end"><button type="button" class="button" data-close-modal>取消</button><button type="button" class="button danger" data-confirm-ok>${escapeHTML(confirmText)}</button></div>`)
  }

  function toast(message, type = 'success') {
    const item = document.createElement('div')
    item.className = `toast ${type === 'error' ? 'error' : ''}`
    item.innerHTML = `${icon(type === 'error' ? 'alert' : 'check', 15)}<span>${escapeHTML(message)}</span>`
    toastRoot.appendChild(item)
    setTimeout(() => item.remove(), 3200)
  }

  function openNotice() {
    openModal('演示说明', `<div class="info-banner" style="margin-bottom:12px">${icon('book')}<span>${escapeHTML(demoModalNotice).replace(/\n/g, '<br>')}</span></div><div class="actions" style="justify-content:flex-end"><button class="button primary" data-close-modal>知道了</button></div>`)
  }

  function addSubFromForm(form) {
    const data = new FormData(form)
    const name = String(data.get('name') || '').trim()
    const url = String(data.get('url') || '').trim()
    const alias = String(data.get('alias') || '').trim()
    if (!name || !url || !alias) return toast('名称、URL 和别名不能为空', 'error')
    const id = Number(data.get('id') || 0)
    const payload = {
      name,
      type: String(data.get('type') || 'git-repo'),
      url,
      alias,
      schedule: String(data.get('schedule') || '').trim(),
      autoAddCron: data.get('autoAddCron') === 'on',
      autoDelCron: data.get('autoDelCron') === 'on'
    }
    if (id) {
      Object.assign(state.subscriptions.find(item => item.id === id), payload)
      toast(`已保存订阅「${name}」`)
    } else {
      state.subscriptions.push({ id: Date.now(), ...payload, files: 0, lastSync: '尚未同步', syncing: false, enabled: true })
      toast(`已添加订阅「${name}」`)
    }
    closeModal()
    renderView()
  }

  function addDepFromForm(form) {
    const data = new FormData(form)
    const name = String(data.get('name') || '').trim()
    if (!name) return toast('包名称不能为空', 'error')
    const type = Number(data.get('type') || 0)
    const remark = String(data.get('remark') || '').trim()
    const id = Number(data.get('id') || 0)
    if (id) {
      Object.assign(state.dependencies.find(item => item.id === id), { name, type, remark })
      state.depCategory = type
      closeModal()
      renderView()
      return toast(`已保存依赖「${name}」`)
    }
    const dep = { id: Date.now(), name, type, version: '待安装', status: 'installing', remark }
    state.dependencies.push(dep)
    state.depCategory = type
    closeModal()
    renderView()
    toast(`正在安装 ${name}`)
    setTimeout(() => {
      dep.status = 'installed'
      dep.version = 'demo'
      renderView()
      toast(`${name} 安装完成`)
    }, 1500)
  }

  /* ---------------- 顶栏搜索 ---------------- */
  function renderSearchPop() {
    const input = document.getElementById('global-search')
    const query = (input ? input.value : '').trim().toLowerCase()
    if (!query) {
      searchPop.hidden = true
      return
    }
    const matches = state.tasks.filter(task => `${task.name} ${task.command}`.toLowerCase().includes(query)).slice(0, 5)
    searchPop.hidden = false
    searchPop.innerHTML = matches.length ? matches.map(task => `<button type="button" class="search-item" data-search-task="${task.id}"><strong>${escapeHTML(task.name)}</strong><small>${escapeHTML(task.command)}</small></button>`).join('') : '<div class="search-empty">没有匹配的任务</div>'
  }

  function openSearchTask(id) {
    const task = state.tasks.find(item => item.id === Number(id))
    if (!task) return
    state.taskSearch = task.name
    searchPop.hidden = true
    openPage('tasks')
    const input = document.getElementById('task-search')
    if (input) input.value = task.name
    applyTaskFilter()
  }

  function updateDate() {
    const label = new Date().toLocaleDateString('zh-CN', { month: 'long', day: 'numeric', weekday: 'long' })
    const target = document.getElementById('today')
    if (target) target.textContent = label
  }

  function copyText(value) {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(value).then(() => toast('已复制到剪贴板')).catch(() => toast('浏览器不允许复制', 'error'))
    } else {
      toast('浏览器不允许复制，请手动选择', 'error')
    }
  }

  /* ---------------- 事件 ---------------- */
  document.addEventListener('click', event => {
    const navTarget = event.target.closest('[data-nav]')
    if (navTarget) {
      event.preventDefault()
      openPage(navTarget.dataset.nav)
      return
    }
    const closeTabButton = event.target.closest('[data-close-tab]')
    if (closeTabButton) {
      event.stopPropagation()
      closeTab(closeTabButton.dataset.closeTab)
      return
    }
    const searchTask = event.target.closest('[data-search-task]')
    if (searchTask) {
      openSearchTask(searchTask.dataset.searchTask)
      return
    }
    const apiTab = event.target.closest('[data-api-tab]')
    if (apiTab) {
      state.apiTab = apiTab.dataset.apiTab
      state.apiOpenEndpoints.clear()
      renderView()
      return
    }
    const apiToggle = event.target.closest('[data-api-toggle]')
    if (apiToggle) {
      const key = apiToggle.dataset.apiToggle
      if (state.apiOpenEndpoints.has(key)) state.apiOpenEndpoints.delete(key)
      else state.apiOpenEndpoints.add(key)
      renderView()
      return
    }
    const apiExpand = event.target.closest('[data-api-expand]')
    if (apiExpand) {
      if (apiExpand.dataset.apiExpand === 'all') {
        apiVisibleGroups().forEach(group => group.endpoints.forEach(ep => state.apiOpenEndpoints.add(ep.id || `${group.id}-${ep.method}-${ep.path}`)))
      } else {
        state.apiOpenEndpoints.clear()
      }
      renderView()
      return
    }
    const apiJump = event.target.closest('[data-api-jump]')
    if (apiJump) {
      const target = document.getElementById('apidocs-' + apiJump.dataset.apiJump)
      if (target) {
        document.querySelectorAll('[data-api-jump]').forEach(button => button.classList.toggle('active', button === apiJump))
        target.scrollIntoView({ behavior: 'smooth', block: 'start' })
      }
      return
    }
    const apiSend = event.target.closest('[data-api-send]')
    if (apiSend) {
      toast('演示模式：不会真正请求后端接口')
      return
    }
    const settingsTab = event.target.closest('[data-settings-tab]')
    if (settingsTab) {
      state.settingsTab = settingsTab.dataset.settingsTab
      renderView()
      return
    }
    const depCategory = event.target.closest('[data-dep-category]')
    if (depCategory) {
      state.depCategory = Number(depCategory.dataset.depCategory)
      renderView()
      return
    }
    const runButton = event.target.closest('[data-run-task]')
    if (runButton) { runTask(Number(runButton.dataset.runTask)); return }
    const stopButton = event.target.closest('[data-stop-task]')
    if (stopButton) { stopTask(Number(stopButton.dataset.stopTask)); return }
    const taskFilter = event.target.closest('[data-task-filter]')
    if (taskFilter) { state.taskFilter = taskFilter.dataset.taskFilter; state.taskPage = 1; renderView(); return }
    const taskSort = event.target.closest('[data-task-sort]')
    if (taskSort) {
      const key = taskSort.dataset.taskSort
      state.taskSort = state.taskSort.key === key ? { key, dir: state.taskSort.dir === 'asc' ? 'desc' : 'asc' } : { key, dir: 'asc' }
      renderView()
      return
    }
    const taskPage = event.target.closest('[data-task-page]')
    if (taskPage) {
      state.taskPage += taskPage.dataset.taskPage === 'next' ? 1 : -1
      if (state.taskPage < 1) state.taskPage = 1
      renderView()
      return
    }
    const taskBulk = event.target.closest('[data-task-bulk]')
    if (taskBulk) { bulkTaskOperation(taskBulk.dataset.taskBulk); return }
    const taskDetail = event.target.closest('[data-task-detail]')
    if (taskDetail) { openTaskDetail(taskDetail.dataset.taskDetail); return }
    const taskSource = event.target.closest('[data-task-source]')
    if (taskSource) { openTaskSource(taskSource.dataset.taskSource); return }
    const taskEdit = event.target.closest('[data-task-edit]')
    if (taskEdit) { openTaskForm(taskEdit.dataset.taskEdit); return }
    const taskMenuButton = event.target.closest('[data-task-menu]')
    if (taskMenuButton) { state.taskMenu = state.taskMenu === Number(taskMenuButton.dataset.taskMenu) ? 0 : Number(taskMenuButton.dataset.taskMenu); renderView(); return }
    const taskPin = event.target.closest('[data-task-pin]')
    if (taskPin) {
      const task = state.tasks.find(item => item.id === Number(taskPin.dataset.taskPin))
      if (task) { task.pinned = !task.pinned; state.taskMenu = 0; renderView(); toast(task.pinned ? '已置顶' : '已取消置顶') }
      return
    }
    const taskDelete = event.target.closest('[data-task-delete]')
    if (taskDelete) {
      const id = Number(taskDelete.dataset.taskDelete)
      const task = state.tasks.find(item => item.id === id)
      state.taskMenu = 0
      if (!task) return
      confirmModal(`删除「${task.name}」？`, '任务会从调度列表中移除，对应的脚本文件不会被删除。历史运行日志仍会保留。此操作不可撤销。', () => {
        state.tasks = state.tasks.filter(item => item.id !== id)
        state.runs = state.runs.filter(run => run.taskId !== id)
        state.taskSelected = state.taskSelected.filter(item => item !== id)
        renderAll()
        toast(`已删除任务「${task.name}」`)
      }, '删除任务')
      return
    }
    const openScriptButton = event.target.closest('[data-open-script]')
    if (openScriptButton) {
      const path = openScriptButton.dataset.openScript
      closeModal()
      openPage('scripts')
      setTimeout(() => openScript(path), 60)
      return
    }
    const envFilterControl = event.target.closest('[data-env-filter]')
    if (envFilterControl) return
    const envSort = event.target.closest('[data-env-sort]')
    if (envSort) {
      const key = envSort.dataset.envSort
      state.envSort = state.envSort.key === key ? { key, dir: state.envSort.dir === 'asc' ? 'desc' : 'asc' } : { key, dir: 'asc' }
      renderView()
      return
    }
    const envPage = event.target.closest('[data-env-page]')
    if (envPage) {
      state.envPage += envPage.dataset.envPage === 'next' ? 1 : -1
      if (state.envPage < 1) state.envPage = 1
      renderView()
      return
    }
    const envBulk = event.target.closest('[data-env-bulk]')
    if (envBulk) { bulkEnvOperation(envBulk.dataset.envBulk); return }
    const envPin = event.target.closest('[data-env-pin]')
    if (envPin) {
      const env = state.envs.find(item => item.id === Number(envPin.dataset.envPin))
      if (env) { env.pinned = !env.pinned; env.updatedAt = '刚刚'; renderView(); toast(env.pinned ? `已置顶 ${env.name}` : `已取消置顶 ${env.name}`) }
      return
    }
    const envMove = event.target.closest('[data-env-move]')
    if (envMove) { moveEnv(envMove.dataset.envId, envMove.dataset.envMove); return }
    const envEdit = event.target.closest('[data-env-edit]')
    if (envEdit) { openEnvForm(envEdit.dataset.envEdit); return }
    const envCopy = event.target.closest('[data-copy-env]')
    if (envCopy) {
      const env = state.envs.find(item => item.id === Number(envCopy.dataset.copyEnv))
      if (env) copyText(env.value)
      return
    }
    const scriptExpand = event.target.closest('[data-script-expand]')
    if (scriptExpand) {
      if (scriptExpand.dataset.scriptExpand === 'all') {
        scriptTree().forEach(row => { if (!row.isLeaf) state.scriptExpanded.add(row.path) })
      } else {
        state.scriptExpanded.clear()
      }
      renderView()
      return
    }
    const scriptOpen = event.target.closest('[data-script-open]')
    if (scriptOpen) { openScript(scriptOpen.dataset.scriptOpen); return }
    const scriptMenu = event.target.closest('[data-script-menu]')
    if (scriptMenu) { openScriptMenu(scriptMenu.dataset.scriptMenu); return }
    const scriptBulk = event.target.closest('[data-script-bulk]')
    if (scriptBulk) {
      const operation = scriptBulk.dataset.scriptBulk
      if (operation === 'clear') { state.scriptSelected = []; renderView(); return }
      if (operation === 'download') { toast(`已打包 ${state.scriptSelected.length} 个文件（演示）`); return }
      scriptOperation(operation, state.scriptSelected.slice())
      return
    }
    const scriptRename = event.target.closest('[data-script-rename]')
    if (scriptRename) { closeModal(); scriptOperation('rename', [scriptRename.dataset.scriptRename]); return }
    const scriptMove = event.target.closest('[data-script-move]')
    if (scriptMove) { closeModal(); scriptOperation('move', [scriptMove.dataset.scriptMove]); return }
    const scriptCopy = event.target.closest('[data-script-copy]')
    if (scriptCopy) { closeModal(); scriptOperation('copy', [scriptCopy.dataset.scriptCopy]); return }
    const scriptDelete = event.target.closest('[data-script-delete]')
    if (scriptDelete) { closeModal(); scriptOperation('delete', [scriptDelete.dataset.scriptDelete]); return }
    const scriptDownload = event.target.closest('[data-script-download]')
    if (scriptDownload) {
      const file = scriptFiles.find(item => item.path === scriptDownload.dataset.scriptDownload)
      if (file) toast(`已下载 ${file.path}（演示）`)
      return
    }
    const scriptFullscreen = event.target.closest('[data-script-fullscreen]')
    if (scriptFullscreen) { state.scriptFullscreen = !state.scriptFullscreen; renderView(); return }
    const scriptHistoryButton = event.target.closest('[data-script-history]')
    if (scriptHistoryButton) { openScriptHistory(); return }
    const restoreHistory = event.target.closest('[data-restore-history]')
    if (restoreHistory) {
      closeModal()
      toast('已恢复到该历史版本（演示）')
      return
    }
    const confirmOk = event.target.closest('[data-confirm-ok]')
    if (confirmOk) {
      const action = state.confirmAction
      closeModal()
      if (typeof action === 'function') action()
      return
    }
    const logButton = event.target.closest('[data-open-log]')
    if (logButton && logButton.dataset.openLog) { openTaskLog(Number(logButton.dataset.openLog)); return }
    const search = event.target.closest('[data-search-task]')
    if (search) { openSearchTask(search.dataset.searchTask); return }
    const fileButton = event.target.closest('[data-file]')
    if (fileButton) {
      const file = scriptFiles.find(item => item.path === fileButton.dataset.file)
      if (file) { state.activeFile = file.path; state.scriptContent = file.content; renderView() }
      return
    }
    const reveal = event.target.closest('[data-reveal]')
    if (reveal) {
      const id = Number(reveal.dataset.reveal)
      if (state.revealed.has(id)) state.revealed.delete(id)
      else state.revealed.add(id)
      renderView()
      return
    }
    const deleteEnv = event.target.closest('[data-delete-env]')
    if (deleteEnv) {
      const id = Number(deleteEnv.dataset.deleteEnv)
      state.envs = state.envs.filter(item => item.id !== id)
      renderView()
      toast('变量已删除')
      return
    }
    const sync = event.target.closest('[data-sync-sub]')
    if (sync) { syncSubscription(Number(sync.dataset.syncSub)); return }
    const deleteSub = event.target.closest('[data-delete-sub]')
    if (deleteSub) {
      const id = Number(deleteSub.dataset.deleteSub)
      const sub = state.subscriptions.find(item => item.id === id)
      if (!sub) return
      confirmModal(`删除订阅「${sub.name}」？`, '订阅将被永久删除，已同步的脚本文件不会被自动移除。此操作不可撤销。', () => {
        state.subscriptions = state.subscriptions.filter(item => item.id !== id)
        state.subSelected = state.subSelected.filter(item => item !== id)
        renderView()
        toast('订阅已删除')
      }, '删除订阅')
      return
    }
    const subEdit = event.target.closest('[data-sub-edit]')
    if (subEdit) { openSubForm(subEdit.dataset.subEdit); return }
    const subLog = event.target.closest('[data-sub-log]')
    if (subLog) { openSubLog(Number(subLog.dataset.subLog)); return }
    const subBulk = event.target.closest('[data-sub-bulk]')
    if (subBulk) { bulkSubOperation(subBulk.dataset.subBulk); return }
    const depLog = event.target.closest('[data-dep-log]')
    if (depLog) { openDepLog(depLog.dataset.depLog); return }
    const depEdit = event.target.closest('[data-dep-edit]')
    if (depEdit) { openDepForm(depEdit.dataset.depEdit); return }
    const depDelete = event.target.closest('[data-dep-delete]')
    if (depDelete) {
      const id = Number(depDelete.dataset.depDelete)
      const dep = state.dependencies.find(item => item.id === id)
      if (!dep) return
      confirmModal(`卸载依赖「${dep.name}」？`, '依赖将从环境中卸载，引用它的脚本可能无法继续运行。已安装的文件会被移除。', () => {
        state.dependencies = state.dependencies.filter(item => item.id !== id)
        state.depSelected = state.depSelected.filter(item => item !== id)
        renderView()
        toast(`已卸载 ${dep.name}`)
      }, '卸载')
      return
    }
    const depBulk = event.target.closest('[data-dep-bulk]')
    if (depBulk) { bulkDepOperation(depBulk.dataset.depBulk); return }
    const reinstall = event.target.closest('[data-reinstall-dep]')
    if (reinstall) { reinstallDependency(Number(reinstall.dataset.reinstallDep)); return }
    const copy = event.target.closest('[data-copy-curl]')
    if (copy) { copyText(copy.dataset.copyCurl); return }
    const action = event.target.closest('[data-action]')
    if (action) {
      const name = action.dataset.action
      if (name === 'menu') {
        state.sidebarOpen = !state.sidebarOpen
        document.getElementById('sidebar').classList.toggle('open', state.sidebarOpen)
        document.getElementById('scrim').classList.toggle('active', state.sidebarOpen)
        return
      }
      if (name === 'close-menu') {
        state.sidebarOpen = false
        document.getElementById('sidebar').classList.remove('open')
        document.getElementById('scrim').classList.remove('active')
        return
      }
      if (name === 'notice') { openNotice(); return }
      if (name === 'tab-menu') { state.tabsMenu = !state.tabsMenu; renderTabs(); return }
      if (name === 'close-others') { closeOtherTabs(); return }
      if (name === 'close-all') { closeAllTabs(); return }
      if (name === 'reset-demo') {
        try { localStorage.removeItem('cabbage-demo-tabs') } catch { /* ignore */ }
        toast('演示数据已重置，正在刷新…')
        setTimeout(() => location.reload(), 450)
        return
      }
      if (name === 'add-task') { openTaskForm(0); return }
      if (name === 'add-env') { openEnvForm(0); return }
      if (name === 'new-file') { openNewScript('file'); return }
      if (name === 'new-folder') { openNewScript('folder'); return }
      if (name === 'add-sub') { openSubForm(0); return }
      if (name === 'add-dep') { openDepForm(0); return }
      if (name === 'save-script') { saveScript(); return }
      if (name === 'save-settings' || name === 'create-backup' || name === 'upload-backup') {
        toast(name === 'create-backup' ? '备份已创建（演示）' : name === 'upload-backup' ? '演示模式不导入真实文件' : '设置已保存（演示）')
        return
      }
    }
    if (event.target.matches('[data-overlay]')) closeModal()
    if (event.target.closest('[data-close-modal]')) closeModal()
    if (!event.target.closest('.tabs-actions')) { state.tabsMenu = false; tabsMenu.hidden = true }
    if (!event.target.closest('.global-search')) searchPop.hidden = true
  })

  document.addEventListener('change', event => {
    const taskToggle = event.target.closest('[data-toggle-task]')
    if (taskToggle) {
      const task = state.tasks.find(item => item.id === Number(taskToggle.dataset.toggleTask))
      if (task) {
        task.enabled = taskToggle.checked
        renderView()
        toast(task.enabled ? `已启用「${task.name}」` : `已停用「${task.name}」`)
      }
      return
    }
    const taskSelectAll = event.target.closest('[data-task-select-all]')
    if (taskSelectAll) {
      const ids = filteredTasks().slice((state.taskPage - 1) * state.taskPageSize, state.taskPage * state.taskPageSize).map(task => task.id)
      state.taskSelected = taskSelectAll.checked ? [...new Set([...state.taskSelected, ...ids])] : state.taskSelected.filter(id => !ids.includes(id))
      renderView()
      return
    }
    const taskCheck = event.target.closest('[data-task-check]')
    if (taskCheck) { toggleTaskSelection(taskCheck.dataset.taskCheck, taskCheck.checked); return }
    const taskPageSize = event.target.closest('[data-task-page-size]')
    if (taskPageSize) { state.taskPageSize = Number(taskPageSize.value); state.taskPage = 1; renderView(); return }
    const envToggle = event.target.closest('[data-toggle-env]')
    if (envToggle) {
      const env = state.envs.find(item => item.id === Number(envToggle.dataset.toggleEnv))
      if (env) {
        env.enabled = envToggle.checked
        env.updatedAt = '刚刚'
        renderView()
        toast(env.enabled ? `已启用 ${env.name}` : `已停用 ${env.name}`)
      }
      return
    }
    const envSelectAll = event.target.closest('[data-env-select-all]')
    if (envSelectAll) {
      const ids = envFiltered().slice((state.envPage - 1) * state.envPageSize, state.envPage * state.envPageSize).map(env => env.id)
      state.envSelected = envSelectAll.checked ? [...new Set([...state.envSelected, ...ids])] : state.envSelected.filter(id => !ids.includes(id))
      renderView()
      return
    }
    const envCheck = event.target.closest('[data-env-check]')
    if (envCheck) { toggleEnvSelection(envCheck.dataset.envCheck, envCheck.checked); return }
    const envPageSize = event.target.closest('[data-env-page-size]')
    if (envPageSize) { state.envPageSize = Number(envPageSize.value); state.envPage = 1; renderView(); return }
    const envFilterSelect = event.target.closest('[data-env-filter]')
    if (envFilterSelect) { state.envFilter = envFilterSelect.value; state.envPage = 1; renderView(); return }
    const scriptSelectAll = event.target.closest('[data-script-select-all]')
    if (scriptSelectAll) {
      const paths = scriptTree().filter(row => row.isLeaf).map(row => row.path)
      state.scriptSelected = scriptSelectAll.checked ? paths : []
      renderView()
      return
    }
    const scriptCheck = event.target.closest('[data-script-check]')
    if (scriptCheck) {
      const path = scriptCheck.dataset.scriptCheck
      state.scriptSelected = scriptCheck.checked ? [...new Set([...state.scriptSelected, path])] : state.scriptSelected.filter(item => item !== path)
      renderView()
      return
    }
    const subSelectAll = event.target.closest('[data-sub-select-all]')
    if (subSelectAll) {
      const query = (state.subSearch || '').trim().toLowerCase()
      const ids = state.subscriptions.filter(sub => !query || `${sub.name} ${sub.url} ${sub.alias}`.toLowerCase().includes(query)).map(sub => sub.id)
      state.subSelected = subSelectAll.checked ? [...new Set([...state.subSelected, ...ids])] : state.subSelected.filter(id => !ids.includes(id))
      renderView()
      return
    }
    const subCheck = event.target.closest('[data-sub-check]')
    if (subCheck) { toggleSubSelection(subCheck.dataset.subCheck, subCheck.checked); return }
    const subToggle = event.target.closest('[data-toggle-sub]')
    if (subToggle) {
      const sub = state.subscriptions.find(item => item.id === Number(subToggle.dataset.toggleSub))
      if (sub) { sub.enabled = subToggle.checked; renderView(); toast(sub.enabled ? `已启用 ${sub.name}` : `已停用 ${sub.name}`) }
      return
    }
    const depSelectAll = event.target.closest('[data-dep-select-all]')
    if (depSelectAll) {
      const ids = state.dependencies.filter(dep => dep.type === state.depCategory).map(dep => dep.id)
      state.depSelected = depSelectAll.checked ? [...new Set([...state.depSelected, ...ids])] : state.depSelected.filter(id => !ids.includes(id))
      renderView()
      return
    }
    const depCheck = event.target.closest('[data-dep-check]')
    if (depCheck) { toggleDepSelection(depCheck.dataset.depCheck, depCheck.checked) }
  })

  document.addEventListener('input', event => {
    const target = event.target
    if (target.id === 'global-search') renderSearchPop()
    if (target.id === 'task-search') {
      state.taskSearch = target.value
      state.taskPage = 1
      renderView()
    }
    if (target.id === 'env-search') {
      state.envSearch = target.value
      state.envPage = 1
      renderView()
    }
    if (target.id === 'script-search') {
      state.scriptSearch = target.value
      const rows = scriptTree()
      const container = document.querySelector('.file-tree')
      if (container) {
        const tree = scriptTree()
        container.innerHTML = tree.length ? tree.map(row => `<div class="tree-row ${state.activeFile === row.path ? 'active' : ''} ${state.scriptSelected.includes(row.path) ? 'selected' : ''}" style="padding-left:${6 + row.depth * 13}px" data-script-row="${escapeHTML(row.path)}" data-leaf="${row.isLeaf}">${row.isLeaf ? `<input type="checkbox" data-script-check="${escapeHTML(row.path)}" ${state.scriptSelected.includes(row.path) ? 'checked' : ''} aria-label="选择 ${escapeHTML(row.path)}">` : `<span class="tree-spacer"></span>`}<button type="button" class="tree-file-button" data-script-open="${escapeHTML(row.path)}" data-leaf="${row.isLeaf}">${icon(row.isLeaf ? 'fileCode' : 'folder', 14)}<span>${escapeHTML(row.path.split('/').pop())}</span></button>${row.isLeaf && hasCron(row.content) ? '<span class="badge success">cron</span>' : ''}<button type="button" class="icon-btn file-menu-trigger" data-script-menu="${escapeHTML(row.path)}" aria-label="文件菜单">${icon('more', 13)}</button></div>`).join('') : `<p class="file-empty">${state.scriptSearch ? '没有匹配的文件' : '暂无文件，可以创建文件夹或脚本。'}</p>`
      }
    }
    if (target.id === 'sub-search') {
      state.subSearch = target.value
      renderView()
    }
    if (target.id === 'api-search') {
      state.apiSearch = target.value
      applyApiFilter()
    }
    if (target.id === 'script-editor') {
      state.scriptContent = target.value
      const numbers = document.getElementById('line-numbers')
      if (numbers) numbers.textContent = state.scriptContent.split('\n').map((_, index) => index + 1).join('\n')
      const file = scriptFiles.find(item => item.path === state.activeFile)
      const dirty = file && file.content !== state.scriptContent
      const marker = document.querySelector('.editor-file .unsaved-dot')
      if (dirty && !marker) document.querySelector('.editor-file')?.insertAdjacentHTML('beforeend', '<span class="unsaved-dot" title="有未保存的修改"></span>')
      if (!dirty && marker) marker.remove()
    }
  })

  document.addEventListener('submit', event => {
    const form = event.target.closest('[data-form]')
    if (!form) return
    event.preventDefault()
    if (form.dataset.form === 'task') saveTaskForm(form)
    if (form.dataset.form === 'env') saveEnvForm(form)
    if (form.dataset.form === 'script-op') submitScriptOperation(form)
    if (form.dataset.form === 'script-new') submitNewScript(form)
    if (form.dataset.form === 'sub') addSubFromForm(form)
    if (form.dataset.form === 'dep') addDepFromForm(form)
  })

  document.addEventListener('keydown', event => {
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') {
      event.preventDefault()
      document.getElementById('global-search')?.focus()
    }
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 's' && state.page === 'scripts') {
      event.preventDefault()
      toast('脚本已保存（演示）')
    }
    if (event.key === 'Escape') {
      const hadModal = !!modalRoot.querySelector('[data-overlay]')
      closeModal()
      state.tabsMenu = false
      tabsMenu.hidden = true
      searchPop.hidden = true
      state.taskMenu = 0
      if (!hadModal && state.scriptFullscreen) { state.scriptFullscreen = false; renderView() }
      if (state.sidebarOpen) {
        state.sidebarOpen = false
        document.getElementById('sidebar').classList.remove('open')
        document.getElementById('scrim').classList.remove('active')
      }
    }
  })

  document.getElementById('global-search-form').addEventListener('submit', event => {
    event.preventDefault()
    const query = document.getElementById('global-search').value.trim()
    if (!query) return
    const match = state.tasks.find(task => `${task.name} ${task.command}`.toLowerCase().includes(query.toLowerCase()))
    if (match) openSearchTask(match.id)
  })

  document.querySelectorAll('[data-icon]').forEach(element => {
    element.innerHTML = icon(element.dataset.icon, Number(element.dataset.size) || 18)
  })

  updateDate()
  renderAll()
})()
