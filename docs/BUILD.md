# 本地构建与打包

本文对应 v1.0.0，适用于从源码构建和生成本地发行包。签名与 GitHub Release 发布见 [RELEASE.md](RELEASE.md)，开发接手见 [DEVELOPMENT.md](DEVELOPMENT.md)。以下构建与打包命令不会发布到 GitHub。

## 环境准备

| 工具 | 要求与用途 |
| --- | --- |
| Go | 1.26+；以 go.mod 为准，编译后端 |
| Node.js / npm | Node.js 22.12+，安装并构建 Vue 前端 |
| Python | 3.9+，打包脚本使用标准库 zipfile，无需 pip 安装打包依赖 |
| Bash / Git | 构建 Shell 脚本及运行面板中的 Shell、Git 订阅功能时需要 |

所有命令在源码根目录执行。Go 依赖由 go.mod/go.sum 锁定，前端依赖由 web/package-lock.json 锁定。不要用手工复制的 node_modules 代替 npm ci。

运行发行程序不需要 Go。Python、Node.js、Bash 和 Git 不包含在原生 ZIP 中，运行相应脚本或同步 Git 订阅时需自行安装。Dockerfile 包含这些运行工具。

### Docker 与 Python 版本

运行时镜像基于官方 `python:${PYTHON_VERSION}-slim-bookworm`，这是构建过程中唯一需要拉取的镜像。Node 和 Go 不拉官方镜像，改为从国内镜像下载二进制包。默认 Python 3.10，构建时可切换：

```
docker build --build-arg PYTHON_VERSION=3.11 -t cabbage-panel:1.0.1 .
```

compose 通过环境变量覆盖：

```
PYTHON_VERSION=3.12 docker compose build
```

说明：Debian bookworm 官方源只有 Python 3.11，因此运行时基底改为官方 python 镜像而非在 node 镜像里 apt 安装。镜像会略大一些，换来 3.10 / 3.11 / 3.12 一行参数切换。

### 源码来源

**Docker 构建时从 GitHub 下载完整源码 tarball，不使用构建上下文里的源码。** 因此：

- 服务器上的源码树不完整、缺目录、权限异常都不影响构建。
- `.dockerignore` 只保留 `Dockerfile`，构建上下文几乎为空，传输最快。
- 改了本地代码要构建进镜像，必须先 push 到 GitHub。

默认取 `main` 分支，可指定 tag 或 commit：

```
docker build --build-arg SOURCE_REF=v1.0.1 -t cabbage-panel:1.0.1 .
SOURCE_REF=v1.0.1 docker compose build
```

构建阶段会校验 12 个关键文件（`go.mod`、`cmd/cabbage/main.go`、`web/src/components/Icon.vue` 等），缺失时直接报出文件名并退出，不会等到编译阶段才失败。

### Docker 构建镜像源

Dockerfile 默认使用国内依赖源，避免构建阶段直连境外卡住：

| 阶段 | 默认源 | 覆盖参数 |
| --- | --- | --- |
| 源码 tarball | `codeload.github.com` | `SOURCE_MIRROR` / `SOURCE_REPO` / `SOURCE_REF` |
| Node 构建工具链 | 清华 `nodejs-release` | `NODE_MIRROR` / `NODE_VERSION` |
| Go 构建工具链 | 阿里云 `golang` | `GO_MIRROR` / `GO_VERSION` |
| 前端 `npm ci` | `https://registry.npmmirror.com` | `NPM_REGISTRY` |
| 后端 `go mod download` | `https://goproxy.cn,direct` | `GOPROXY` |
| 运行时 `apt-get` | 清华 Debian 源 | `DEBIAN_MIRROR` / `DEBIAN_SECURITY_MIRROR` |
| 基础镜像 | `docker.1ms.run/` 加速站 | `DOCKER_MIRROR`（留空 = Docker Hub 官方名） |

境外网络可覆盖为官方源：

```
docker compose build \
  --build-arg DOCKER_MIRROR= \
  --build-arg SOURCE_MIRROR=https://codeload.github.com \
  --build-arg NODE_MIRROR=https://nodejs.org/dist \
  --build-arg GO_MIRROR=https://go.dev/dl \
  --build-arg NPM_REGISTRY=https://registry.npmjs.org \
  --build-arg GOPROXY=https://proxy.golang.org,direct \
  --build-arg DEBIAN_MIRROR=http://deb.debian.org/debian \
  --build-arg DEBIAN_SECURITY_MIRROR=http://security.debian.org/debian-security
```

### 构建慢或卡住

**为什么不用 node / golang 官方镜像**：实测国内镜像加速站下载层只有 0.1–0.4 MB/s，拉 `node:24-alpine` + `golang:1.27-alpine` 约 350MB 要几十分钟。清华 Node 源和阿里云 Go 源实测 4–5 MB/s，因此构建阶段改为下二进制包，只保留一个必须的 `python:slim-bookworm` 基础镜像（通常本机已缓存）。产物仍是 `CGO_ENABLED=0` 静态二进制。

如果仍然很慢，先 `docker compose build --progress=plain` 看卡在哪：

1. **卡在拉取 `python:slim-bookworm`**（`FROM ...` 行显示 `0B / 51.10MB` 不动）——这是唯一需要拉的基础镜像：
   - 在 `.env` 里换加速站，例如 `DOCKER_MIRROR=docker.m.daocloud.io/library/`（仓库自带 `.env.example` 模板）；设 `DOCKER_MIRROR=` 空值则直连 Docker Hub。
   - 给 Docker 配全局镜像加速：Docker Desktop → Settings → Docker Engine 加 `"registry-mirrors": ["https://docker.1ms.run"]`，或在 Linux 的 `/etc/docker/daemon.json` 加同样配置后 `systemctl restart docker`。
   - 先在能联网的机器 `docker pull` 后 `docker save`，拷到服务器 `docker load`。

2. **卡在下载源码 tarball**——`codeload.github.com` 在国内通常可用（约 550KB，5 秒）。不通时用 `SOURCE_MIRROR` 指向代理，或先在能联网的机器构建镜像再 `docker save` / `docker load`。

3. **卡在下载 Node / Go 二进制**——换镜像源，或在 `.env` 里覆盖 `NODE_MIRROR` / `GO_MIRROR`。可用源：
   - Node：清华 `https://mirrors.tuna.tsinghua.edu.cn/nodejs-release`、阿里云 `https://mirrors.aliyun.com/nodejs-release`
   - Go：阿里云 `https://mirrors.aliyun.com/golang`、中科大 `https://mirrors.ustc.edu.cn/golang`

4. **卡在 `npm ci` / `go mod download` / `apt-get update`**——依赖源问题，确认上面的构建参数已传入，或改 Dockerfile 顶部 `ARG` 默认值。

5. **改代码后构建结果没变**——源码来自 GitHub，需要先 `git push`。想用本地未推送的代码构建，改 `SOURCE_REF` 指向对应 commit 并确保已推送。

6. **一次性构建，不做本地开发**——可在开发机构建好镜像再导出：
   ```
   docker build -t cabbage-panel:1.0.1 .
   docker save cabbage-panel:1.0.1 | gzip > cabbage-panel.tar.gz
   # 服务器上
   gunzip -c cabbage-panel.tar.gz | docker load
   ```

原生部署（非 Docker）可在面板「系统设置 → 常规设置 → 运行环境」指定 Python 解释器路径或版本号，切换后点「重建 Python 环境」。

## Windows 构建

先确认目标程序没有被运行中的服务占用。默认构建会写入 dist/cabbage-windows-amd64.exe；本机现有服务可能正使用此文件。需要并行开发时使用 [独立开发程序](DEVELOPMENT.md#独立开发程序)，或在另一个源码目录构建。

~~~powershell
powershell -ExecutionPolicy Bypass -File .\scripts\build.ps1
.\dist\cabbage-windows-amd64.exe --version
~~~

脚本先运行 npm ci 和前端生产构建，再编译 Go。输出包含前端静态资源，可在同一端口运行整个面板。脚本将 Go、npm 缓存和临时文件放在项目 .tools/ 下；若 PATH 中没有 Go，会尝试 D:\Go\bin\go.exe。

开发调试时也可以给程序加 `--web web/dist`，从磁盘读取前端目录而不再依赖编译时嵌入的资源；修改前端后只需重新执行 npm 构建。该参数不用于正式发布，发行程序仍应使用内嵌前端。

需要时仅为当前终端设置依赖代理：

~~~powershell
$env:GOPROXY = 'https://goproxy.cn,direct'
~~~

生成目前支持的六个平台程序：

~~~powershell
powershell -ExecutionPolicy Bypass -File .\scripts\build.ps1 -TargetOS windows -TargetArch amd64
powershell -ExecutionPolicy Bypass -File .\scripts\build.ps1 -TargetOS windows -TargetArch arm64 -SkipWeb
powershell -ExecutionPolicy Bypass -File .\scripts\build.ps1 -TargetOS linux -TargetArch amd64 -SkipWeb
powershell -ExecutionPolicy Bypass -File .\scripts\build.ps1 -TargetOS linux -TargetArch arm64 -SkipWeb
powershell -ExecutionPolicy Bypass -File .\scripts\build.ps1 -TargetOS darwin -TargetArch amd64 -SkipWeb
powershell -ExecutionPolicy Bypass -File .\scripts\build.ps1 -TargetOS darwin -TargetArch arm64 -SkipWeb
~~~

-SkipWeb 仅适用于已成功构建当前源码的前端且前端没有继续改动的情况。Go 通过 web/embed.go 嵌入 web/dist，直接编译 Go 不会自动重新构建 Vue。

## Linux / macOS 构建

~~~sh
bash scripts/build.sh
~~~

默认构建当前操作系统与架构。交叉编译使用 GOOS、GOARCH：

~~~sh
GOOS=linux GOARCH=amd64 bash scripts/build.sh
GOOS=linux GOARCH=arm64 bash scripts/build.sh
GOOS=windows GOARCH=amd64 bash scripts/build.sh
GOOS=windows GOARCH=arm64 bash scripts/build.sh
GOOS=darwin GOARCH=amd64 bash scripts/build.sh
GOOS=darwin GOARCH=arm64 bash scripts/build.sh
~~~

此脚本每次都会重新安装和构建前端，没有 -SkipWeb 参数。交叉编译成功只说明生成了目标平台程序，仍需在对应系统验证实际运行。

## 版本号与打包

发布新版本前同步这些位置：VERSION、cmd/cabbage/main.go 中的默认 version、web/package.json、web/package-lock.json 的项目版本、Dockerfile 的默认 VERSION、compose.yaml 的镜像版本，以及 CHANGELOG.md 和相关文档。Go 正式构建通过 ldflags 从 VERSION 注入版本。

确认 dist/ 中每个待打包程序都由当前版本源码重新编译，再运行：

~~~sh
node scripts/package.mjs
~~~

打包需要 Windows 的 python 或其他平台的 python3 命令可用。脚本只读取 dist/ 中名称匹配的平台程序，不会编译，也不会检查程序内部版本。不能只修改 VERSION 后把旧程序重新打包；不打算发布的旧平台程序应移到 dist/ 以外的归档目录。

| 平台 | 程序 | v1.0.0 发行包 |
| --- | --- | --- |
| Windows x64 | cabbage-windows-amd64.exe | cabbage-windows-amd64-v1.0.0.zip |
| Linux x64 | cabbage-linux-amd64 | cabbage-linux-amd64-v1.0.0.zip |
| Linux ARM64 | cabbage-linux-arm64 | cabbage-linux-arm64-v1.0.0.zip |
| macOS Apple Silicon | cabbage-darwin-arm64 | cabbage-darwin-arm64-v1.0.0.zip |

全部输出在 dist/。重复打包会覆盖相同版本的 ZIP 和 SHA256SUMS，不会改写程序或运行数据；已对外发布的版本不要覆盖，应递增版本号。

ZIP 白名单包括对应程序、VERSION、README.md、AGENTS.md、CHANGELOG.md、SECURITY.md、THIRD_PARTY_NOTICES.md、API/发布/测试/使用/构建/开发文档，以及 SHA256SUMS。校验文件记录各平台裸程序的 SHA-256，并非 ZIP 的校验和。ZIP 不包含源码、编译工具链或脚本运行依赖。

打包脚本不会收集 data/、dist/data/、.tools/、用户脚本、数据库、备份、.env、私钥或设计文件。保护措施为 Go 去符号和路径裁剪、前端压缩与关闭 source map；这些措施不等于不可逆向。

## 本地验收

1. 在当前平台执行生成程序的 --version，确认与 VERSION 一致。
2. 检查 ZIP 目录，仅包含上述白名单文件，并核对程序 SHA-256。
3. 解压到独立目录，用独立数据目录和空闲端口试运行；不覆盖现有服务的数据。

Windows 示例，位于已解压的发行包目录：

~~~powershell
.\cabbage-windows-amd64.exe --data .\smoke-data --addr 127.0.0.1:15701
~~~

Linux / macOS 先赋予对应程序执行权限，例如：

~~~sh
chmod +x ./cabbage-linux-amd64
./cabbage-linux-amd64 --data ./smoke-data --addr 127.0.0.1:15701
~~~

访问该端口完成首次初始化，再验证页面、中文脚本与任务运行。退出试运行不会影响正式 5700 服务。恢复正式部署时使用 --addr :5700 和自己的持久化数据目录；相对数据路径以启动时的工作目录为准。

## 签名更新与交付

本地 ZIP 可手工部署。在线更新要求程序内嵌发布公钥：`scripts/build.ps1` / `scripts/build.sh` 在未设置 `CABBAGE_UPDATE_PUBLIC_KEY` 时会读取根目录的 `update-public-key.txt`（内容是 Base64 公钥，可公开，不是秘密）。Release 需提供由匹配私钥签名的 manifest.json、manifest.sig 及对应平台程序；只生成 ZIP 不满足在线更新要求。

正式签名身份由项目所有者保管，操作步骤见 [RELEASE.md](RELEASE.md)。当前未授权对外发布，不要推送 tag 来触发发布工作流。

交给其他 Agent 继续开发时，应提供源码目录，保留 go.mod/go.sum、web/package.json/package-lock.json、scripts/、docs/、AGENTS.md 和 .github/。单独提供发行 ZIP 无法还原开发工程。源码交接应排除运行数据、缓存、构建产物、凭据和本机备份。
