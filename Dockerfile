# 构建思路：只拉一个基础镜像（运行时必需的 python:slim-bookworm），
# Node 和 Go 从国内镜像下二进制包，避免拉 node/golang 官方镜像。
# 实测清华 Node 源、阿里云 Go 源约 4-5 MB/s，镜像加速站只有 0.1-0.4 MB/s。
#
# 运行时可选择 Python 版本：docker build --build-arg PYTHON_VERSION=3.11 .
ARG PYTHON_VERSION=3.10

# 构建工具链版本与下载源，可用 --build-arg 覆盖。
ARG NODE_VERSION=24.22.3
ARG GO_VERSION=1.27.4
ARG NODE_MIRROR=https://mirrors.tuna.tsinghua.edu.cn/nodejs-release
ARG GO_MIRROR=https://mirrors.aliyun.com/golang

# 仅用于拉取 python 基础镜像。本机已缓存时不会重新下载。
# 境外网络可设 DOCKER_MIRROR= 直连 Docker Hub。
ARG DOCKER_MIRROR=docker.1ms.run/

# 依赖源默认走国内，避免 npm/go/apt 直连境外卡住。
ARG NPM_REGISTRY=https://registry.npmmirror.com
ARG GOPROXY=https://goproxy.cn,direct
ARG DEBIAN_MIRROR=https://mirrors.tuna.tsinghua.edu.cn/debian
ARG DEBIAN_SECURITY_MIRROR=https://mirrors.tuna.tsinghua.edu.cn/debian-security

# 唯一基础镜像：运行时需要 glibc，且本机通常已缓存。
FROM ${DOCKER_MIRROR}python:${PYTHON_VERSION}-slim-bookworm AS base
ARG DEBIAN_MIRROR
ARG DEBIAN_SECURITY_MIRROR
# 先换 Debian 源为国内镜像（默认清华），再装运行时依赖。
RUN if [ -n "$DEBIAN_MIRROR" ]; then \
      sed -i "s|http://deb.debian.org/debian|$DEBIAN_MIRROR|g; s|http://security.debian.org/debian-security|$DEBIAN_SECURITY_MIRROR|g" \
        /etc/apt/sources.list.d/debian.sources /etc/apt/sources.list 2>/dev/null || true; \
    fi \
    && apt-get update && apt-get install -y --no-install-recommends \
        bash git ca-certificates tzdata tini curl \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 5700 cabbage && useradd --uid 5700 --gid 5700 --create-home cabbage \
    && mkdir -p /app/data && chown cabbage:cabbage /app/data
# 运行时需要 node 执行 JS 脚本，PATH 在 base 里设好即可被 runtime 继承。
ENV PATH=/opt/node/bin:$PATH

# 前端构建：下载 Node 二进制后执行 npm ci + build。产物是静态文件，与 libc 无关。
FROM base AS frontend
ARG NODE_VERSION
ARG NODE_MIRROR
ARG NPM_REGISTRY
# 用基础镜像自带的 dpkg 判断架构，不依赖 buildx 的 TARGETARCH。
RUN set -eux; \
    case "$(dpkg --print-architecture)" in \
      amd64) narch=x64 ;; \
      arm64) narch=arm64 ;; \
      *) echo "不支持的架构: $(dpkg --print-architecture)" >&2; exit 1 ;; \
    esac; \
    curl -fsSL --retry 3 --retry-delay 2 "$NODE_MIRROR/v$NODE_VERSION/node-v$NODE_VERSION-linux-$narch.tar.gz" -o /tmp/node.tar.gz; \
    mkdir -p /opt/node; \
    tar -xzf /tmp/node.tar.gz -C /opt/node --strip-components=1; \
    rm /tmp/node.tar.gz; \
    node --version; npm --version; \
    npm config set registry "$NPM_REGISTRY"
WORKDIR /source/web
COPY web/package*.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
# 构建上下文不完整时明确报错。缺子目录时 vite 只会报
# "Could not resolve ./components/X.vue"，容易被误判成代码问题。
RUN set -eux; \
    missing=""; \
    for f in src/App.vue src/main.js src/components/Icon.vue src/pages/Dashboard.vue; do \
      [ -f "$f" ] || missing="$missing $f"; \
    done; \
    if [ -n "$missing" ]; then \
      echo "" >&2; \
      echo "=== 构建上下文缺少文件:$missing ===" >&2; \
      echo "--- src 下实际内容 ---" >&2; \
      find src -maxdepth 2 -print 2>/dev/null | sort >&2 || true; \
      echo "--- 大小写检查（Windows 不敏感、Linux 敏感）---" >&2; \
      find . -maxdepth 3 \( -iname 'components' -o -iname 'pages' \) -print >&2 || true; \
      echo "--- web/ 顶层 ---" >&2; \
      ls -la . >&2 || true; \
      echo "=== 请在构建机上确认 web/src 完整；必要时重新 git clone ===" >&2; \
      exit 1; \
    fi; \
    npm run build

# 后端构建：下载 Go 二进制。CGO_ENABLED=0 + 纯 Go 的 modernc.org/sqlite，
# 产物是静态二进制，可跨 libc 运行。
FROM base AS backend
ARG GO_VERSION
ARG GO_MIRROR
ARG GOPROXY
# go.sum 仍校验每个模块哈希；GOSUMDB 关闭只跳过 sumdb 在线查询，避免国内超时。
ENV GOPROXY=${GOPROXY} GOSUMDB=off CGO_ENABLED=0 PATH=/opt/go/bin:$PATH
RUN set -eux; \
    case "$(dpkg --print-architecture)" in \
      amd64) garch=amd64 ;; \
      arm64) garch=arm64 ;; \
      *) echo "不支持的架构: $(dpkg --print-architecture)" >&2; exit 1 ;; \
    esac; \
    curl -fsSL --retry 3 --retry-delay 2 "$GO_MIRROR/go${GO_VERSION}.linux-${garch}.tar.gz" -o /tmp/go.tar.gz; \
    mkdir -p /opt/go; \
    tar -xzf /tmp/go.tar.gz -C /opt/go --strip-components=1; \
    rm /tmp/go.tar.gz; \
    go version
WORKDIR /source
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /source/web/dist ./web/dist
ARG VERSION=1.0.1
ARG COMMIT=dev
ARG CABBAGE_UPDATE_PUBLIC_KEY=""
RUN KEY="${CABBAGE_UPDATE_PUBLIC_KEY:-$(tr -d '\r\n' < update-public-key.txt 2>/dev/null)}" && \
    go build -trimpath -buildvcs=false -ldflags "-s -w -X main.version=$VERSION -X main.commit=$COMMIT -X main.updateKey=$KEY" -o /cabbage ./cmd/cabbage

# 运行时：复用 base（已装 bash/git/curl/tini 并建好用户），只加 node 二进制和程序。
FROM base AS runtime
COPY --from=frontend /opt/node /opt/node
COPY --from=backend /cabbage /usr/local/bin/cabbage
WORKDIR /app
ENV TZ=Asia/Shanghai PYTHONUTF8=1 PYTHONIOENCODING=utf-8
USER cabbage
VOLUME ["/app/data"]
EXPOSE 5700
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s CMD curl --fail --silent http://127.0.0.1:5700/healthz || exit 1
ENTRYPOINT ["/usr/bin/tini", "--", "/usr/local/bin/cabbage"]
CMD ["--data", "/app/data", "--addr", ":5700"]
