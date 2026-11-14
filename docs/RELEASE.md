# 发布与更新

仅本地编译和打包请先阅读 [构建与打包](BUILD.md)；交给其他 Agent 继续开发请阅读 [开发接手指南](DEVELOPMENT.md)。本文的 tag 和 Release 流程供所有者授权发布后使用，当前不执行对外发布。

## 签名配置

本仓库根目录的 `update-public-key.txt` 保存 Base64 公钥，`scripts/build.ps1` / `scripts/build.sh` 在未设置环境变量时会自动读取并嵌入。公钥不是秘密，可以随源码分发；私钥必须离线保管，不能提交或随 Release 分发。

首次生成或更换签名身份时，在受信任本机执行：

    node scripts/release-sign.mjs keygen

生成 `release-private.key`（已被 .gitignore 排除）并输出 Base64 公钥。把公钥写入 `update-public-key.txt`，私钥移至离线安全位置。

GitHub Settings → Secrets and variables → Actions：

- Secret RELEASE_PRIVATE_KEY：私钥 PEM 的完整多行内容。
- Variable CABBAGE_UPDATE_PUBLIC_KEY：公钥。

公钥在构建时嵌入 Go 程序，不允许普通后台设置替换。正式签名身份由项目所有者生成与保管，仓库不包含私钥。启用后，带公钥的版本会拒绝未签名或校验失败的更新。

## 发布流程

1. 同步 VERSION、cmd/cabbage/main.go 默认 version、web/package.json、web/package-lock.json、compose.yaml、Dockerfile 默认版本、CHANGELOG.md 和文档版本。
2. 通过 CI 并审查变更。
3. 推送与 VERSION 对应的 tag，例如 v1.0.0。
4. Workflow 构建 Linux amd64/arm64、Windows amd64、macOS arm64。
5. 生成含使用文档的 ZIP 包，发布程序、ZIP、manifest.json、SHA256SUMS；配置了私钥时额外发布 manifest.sig。

流水线检查 tag/version，并要求 `CABBAGE_UPDATE_PUBLIC_KEY` 非空；配置了私钥时校验私钥与公钥匹配。此配置需推送到真实仓库后才能在 GitHub 执行，发布前按组织策略固定镜像 digest 和 Actions commit。

## 在线更新

默认仓库为 bucai2417/cabbage-panel，可在系统设置中修改。检查更新 → 下载并验证。下载在后台执行，设置页显示真实下载进度（已接收字节 / 总字节与百分比）；构建嵌入公钥时验证清单签名，再验证对应平台程序 SHA-256，未嵌入公钥时只校验 SHA-256。

所有任务结束后点「应用并重启」：后端按运行环境选择重启方式——Windows 生成看守批处理、Linux/macOS 生成看守脚本，两者都在当前进程退出后替换程序并重新拉起；容器内直接就地替换程序后退出，依赖容器 restart 策略重启（程序目录只读时请改用新版本镜像重新部署）。替换失败会尝试回滚，旧程序保留为 .previous。原生部署需程序目录写权限；只有管理员可更新，应用 token 和 MCP 无权限。设置页侧栏会在检测到新版本时给出提示，点击直达更新页。

本地构建若确实没有嵌入公钥，更新默认被拒绝，需要在设置页显式开启「允许未签名更新」才会退化为仅校验 SHA-256。启用签名后，更换签名身份需要旧密钥签名的过渡版本或人工分发，避免替换信任根。

数据与程序分开。升级前备份；未来 schema 迁移必须兼顾恢复，不应假设旧程序可直接读取新 schema。

## 打包保护

- Go 使用 -trimpath -s -w，去掉本机路径和调试符号。
- Vue 生产压缩，关闭 source map，只嵌入 web/dist。
- 不打包源码、.git、设计原图、缓存、数据库、脚本、备份、账户、私钥、.env。
- 二进制不是防逆向容器；公开源码后无法隐藏源码，浏览器始终能读前端资源。
- 密钥只进入 GitHub Secrets 或安全存储，不写入前端。

如需额外 garble 混淆，应先验证跨平台、堆栈诊断和更新。当前使用标准 Go 工具链，避免未经测试的混淆影响可靠性。

## 回滚与容器

停止服务，把 .previous 恢复为可执行文件，再重启。不要在进程占用文件时手动覆盖。

容器重新拉取或构建指定镜像，再执行 docker compose up -d。命名数据卷保留，非 root 容器默认不能覆盖系统目录中的程序。
