# 大白菜面板脚本写法规则（兼容青龙习惯）

## 运行环境

- 支持 Python（`.py`）、Node.js（`.js` / `.mjs` / `.cjs`）、Shell（`.sh`）。
- 任务命令优先使用 `task 文件名`，面板会自动选择运行时；也可显式写 `python3 x.py`、`node x.js`、`bash x.sh`。
- 脚本以面板进程权限运行。退出码 0 为成功，非 0 为失败；超时由任务配置控制。

## 订阅注释标记

订阅同步只扫描每个脚本前 50 行的注释，支持 Python / Shell 的 `#`、JavaScript 的 `//` 和 `/* ... */` 块注释。

```python
# cron "0 8 * * *"
# cron: 0 8 * * *
# @cron 0 8 * * *
```

```javascript
// cron: 0 8 * * *
/*
 * cron "0 8 * * *"
 */
```

- 支持 5 段或 6 段 cron 表达式；无效表达式不会创建任务。
- 开启 `autoAddCron` 后，只有**本次新增**的脚本才自动创建任务；上次同步已存在的脚本不再自动建任务，避免手动删掉的任务被同步重建。任务名优先取 new Env('名称') 里的名称，没有则用脚本文件名（含扩展名），命令为 `task "别名/文件名"`。同名任务自动加序号后缀。
- 重复同步会更新已有订阅任务的计划，不会重复创建任务。
- 正文代码、字符串、超过前 50 行的注释不参与识别。
- 开启 `autoDelCron` 后，源仓库删除的脚本会同步删除，并清理关联任务。

## new Env / 环境变量

- 注释里的 `new Env("NAME")`、`new Env('NAME')`、`Env("NAME")` 只表示脚本依赖的变量名，面板不会自动创建变量，也不会解析其值。
- `autoAddCron` 开启时，这个名称同时用作自动创建的定时任务名；注释形式和 `const $ = new Env('NAME')` 代码形式都识别。
- 在「环境变量」页创建同名变量并启用后，变量会注入脚本进程。
- 读取方式：Python `os.environ["NAME"]`；Node `process.env.NAME`；Shell `"$NAME"`。
- 不要把 Cookie、Token、密码硬编码在脚本中，使用环境变量。

## 任务与日志

- 任务命令统一使用 `task` 前缀，例如 `task example.py`。
- 标准输出/错误会写入运行日志；建议输出关键步骤，但不要打印敏感变量。
- 任务可配置超时、并发、前后置脚本、日志名和执行结束通知。

## 通知助手

- Shell：`bin/notify "标题" "内容"`
- Python：`from notify import sendNotify; sendNotify("标题", "内容")`
- Node：`const { sendNotify } = require('./sendNotify'); await sendNotify("标题", "内容")`
- 通知渠道在系统设置 → 告警通知中配置。

## 接口调用

- 完整接口清单可通过 MCP 工具 `get_api_docs` 或资源 `cabbage://api-docs` 获取。
- `/api` 是管理接口，需要管理员会话或管理员 Bearer；`/open` 是开放应用接口，需要应用权限和 Bearer token。
- 成功响应为 `{code:200,data:...}`；失败为对应 HTTP 状态和 `{code,message}`。
- 写操作、删除操作和执行操作必须先确认用户授权。

## 示例

```python
# cron "0 8 * * *"
# new Env("YPCK")
import os

print("YPCK=", os.environ.get("YPCK"))
```

```javascript
// cron: 0 8 * * *
// new Env('YPCK')
console.log('YPCK=', process.env.YPCK)
```
