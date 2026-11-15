# 大白菜面板 Docker 构建
#
# 设计要点：
# 1. 只拉一个基础镜像（python:slim-bookworm），Node / Go 从国内镜像下二进制。
#    实测清华 Node 源、阿里云 Go 源 4-5 MB/s，镜像加速站只有 0.1-0.4 MB/s。
# 2. 源码优先取自构建上下文，Docker 按内容哈希判断缓存，git pull 后必然重建。
#    上下文源码不完整时自动回退从 GitHub 下载，保证构建仍能成功。
#
# 常用覆盖参数：
#   docker build --build-arg PYTHON_VERSION=3.11 -t cabbage-panel:1.0.1 .
#   docker build --build-arg DOCKER_MIRROR= -t cabbage-panel:1.0.1 .   # 直连 Docker Hub

ARG PYTHON_VERSION=3.10

# 构建工具链版本与下载源。
ARG NODE_VERSION=24.22.3
ARG GO_VERSION=1.27.4
ARG NODE_MIRROR=https://mirrors.tuna.tsinghua.edu.cn/nodejs-release
ARG GO_MIRROR=https://mirrors.aliyun.com/golang

# 基础镜像仓库前缀。境外网络可设 DOCKER_MIRROR= 直连 Docker Hub。
ARG DOCKER_MIRROR=docker.1ms.run/

# 依赖源默认走国内。
ARG NPM_REGISTRY=https://registry.npmmirror.com
ARG GOPROXY=https://goproxy.cn,direct
ARG DEBIAN_MIRROR=https://mirrors.tuna.tsinghua.edu.cn/debian
ARG DEBIAN_SECURITY_MIRROR=https://mirrors.tuna.tsinghua.edu.cn/debian-security

# 版本号写入二进制。
ARG VERSION=1.0.1
ARG COMMIT=dev

# ---------- 基础镜像：运行时需要 glibc，只拉这一个镜像 ----------
FROM ${DOCKER_MIRROR}python:${PYTHON_VERSION}-slim-bookworm AS base
ARG DEBIAN_MIRROR
ARG DEBIAN_SECURITY_MIRROR
RUN if [ -n "$DEBIAN_MIRROR" ]; then \
      sed -i "s|http://deb.debian.org/debian|$DEBIAN_MIRROR|g; s|http://security.debian.org/debian-security|$DEBIAN_SECURITY_MIRROR|g" \
        /etc/apt/sources.list.d/debian.sources /etc/apt/sources.list 2>/dev/null || true; \
    fi \
    && apt-get update && apt-get install -y --no-install-recommends \
        bash git ca-certificates tzdata tini curl tar gosu \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 5700 cabbage && useradd --uid 5700 --gid 5700 --create-home cabbage \
    && mkdir -p /app/data && chown cabbage:cabbage /app/data
ENV PATH=/opt/node/bin:$PATH

# ---------- 源码 ----------
# 优先使用构建上下文里的源码：Docker 按文件内容哈希判断缓存，git pull 后
# 必然重建，不存在「用了旧代码」的问题。
#
# 之前用 RUN curl / ADD 拉远程源码，在飞牛等只提供「构建」按钮的环境下会一直
# 命中缓存：RUN 的 URL 是常量；ADD 靠 Last-Modified 判断，而 codeload.github.com
# 只返回 ETag，缓存同样不失效。
#
# 上下文源码不完整时（缺目录、拷贝不全）自动回退到 GitHub 下载，保证构建仍能成功。
FROM base AS source
ARG SOURCE_REPO=bucai2417/cabbage-panel
ARG SOURCE_REF=main
ARG SOURCE_MIRROR=https://codeload.github.com
COPY . /src
RUN set -eux; \
    check() { \
      for f in go.mod go.sum cmd/cabbage/main.go internal/panel/app.go \
               web/package.json web/package-lock.json web/index.html web/vite.config.js \
               web/src/App.vue web/src/main.js web/src/components/Icon.vue \
               web/src/pages/Dashboard.vue deploy/docker-entrypoint.sh; do \
        [ -f "/src/$f" ] || return 1; \
      done; \
    }; \
    if check; then \
      echo "源码来自构建上下文，文件数: $(find /src -type f | wc -l)"; \
    else \
      echo "构建上下文源码不完整，改为从 GitHub 下载"; \
      missing=""; \
      for f in go.mod cmd/cabbage/main.go internal/panel/app.go \
               web/src/App.vue web/src/components/Icon.vue; do \
        [ -f "/src/$f" ] || missing="$missing $f"; \
      done; \
      echo "缺失:$missing"; \
      curl --http1.1 -fsSL --retry 3 --retry-delay 2 \
        "$SOURCE_MIRROR/$SOURCE_REPO/tar.gz/refs/heads/$SOURCE_REF" -o /tmp/src.tar.gz; \
      rm -rf /src; mkdir -p /src; \
      tar -xzf /tmp/src.tar.gz -C /src --strip-components=1; \
      rm /tmp/src.tar.gz; \
      if ! check; then echo "FATAL: 下载的源码仍不完整" >&2; exit 1; fi; \
      echo "源码来自 GitHub，文件数: $(find /src -type f | wc -l)"; \
    fi; \
    echo "入口脚本 SHA256: $(sha256sum /src/deploy/docker-entrypoint.sh | cut -d' ' -f1)"

# ---------- 前端构建 ----------
FROM source AS frontend
ARG NODE_VERSION
ARG NODE_MIRROR
ARG NPM_REGISTRY
RUN set -eux; \
    case "$(dpkg --print-architecture)" in \
      amd64) narch=x64 ;; \
      arm64) narch=arm64 ;; \
      *) echo "不支持的架构: $(dpkg --print-architecture)" >&2; exit 1 ;; \
    esac; \
    curl -fsSL --retry 3 --retry-delay 2 \
      "$NODE_MIRROR/v$NODE_VERSION/node-v$NODE_VERSION-linux-$narch.tar.gz" -o /tmp/node.tar.gz; \
    mkdir -p /opt/node; \
    tar -xzf /tmp/node.tar.gz -C /opt/node --strip-components=1; \
    rm /tmp/node.tar.gz; \
    node --version; npm --version; \
    npm config set registry "$NPM_REGISTRY"
WORKDIR /src/web
RUN npm ci --no-audit --no-fund && npm run build

# ---------- 后端构建 ----------
FROM source AS backend
ARG GO_VERSION
ARG GO_MIRROR
ARG GOPROXY
ARG VERSION
ARG COMMIT
ARG CABBAGE_UPDATE_PUBLIC_KEY=""
ENV GOPROXY=${GOPROXY} GOSUMDB=off CGO_ENABLED=0 PATH=/opt/go/bin:$PATH
RUN set -eux; \
    case "$(dpkg --print-architecture)" in \
      amd64) garch=amd64 ;; \
      arm64) garch=arm64 ;; \
      *) echo "不支持的架构: $(dpkg --print-architecture)" >&2; exit 1 ;; \
    esac; \
    curl -fsSL --retry 3 --retry-delay 2 \
      "$GO_MIRROR/go${GO_VERSION}.linux-${garch}.tar.gz" -o /tmp/go.tar.gz; \
    mkdir -p /opt/go; \
    tar -xzf /tmp/go.tar.gz -C /opt/go --strip-components=1; \
    rm /tmp/go.tar.gz; \
    go version
WORKDIR /src
COPY --from=frontend /src/web/dist ./web/dist
RUN set -eux; \
    KEY="${CABBAGE_UPDATE_PUBLIC_KEY:-$(tr -d '\r\n' < update-public-key.txt 2>/dev/null || true)}"; \
    go build -trimpath -buildvcs=false \
      -ldflags "-s -w -X main.version=$VERSION -X main.commit=$COMMIT -X main.updateKey=$KEY" \
      -o /cabbage ./cmd/cabbage; \
    ls -lh /cabbage; \
    file /cabbage || true

# ---------- 运行时 ----------
FROM base AS runtime
COPY --from=frontend /opt/node /opt/node
COPY --from=backend /cabbage /usr/local/bin/cabbage
# 入口脚本：bind mount 的数据目录属主通常是宿主机用户，不一定是 uid 5700。
# 以 root 启动，修正属主后降权到 cabbage 运行，避免数据目录不可写。
# 从 source 阶段取：脚本随源码 tarball 下载，不依赖构建上下文。
COPY --from=source --chmod=0755 /src/deploy/docker-entrypoint.sh /usr/local/bin/entrypoint.sh
WORKDIR /app
ENV TZ=Asia/Shanghai PYTHONUTF8=1 PYTHONIOENCODING=utf-8
EXPOSE 5700
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s \
  CMD curl --fail --silent http://127.0.0.1:5700/healthz || exit 1
ENTRYPOINT ["/usr/bin/tini", "--", "/usr/local/bin/entrypoint.sh"]
CMD ["/usr/local/bin/cabbage", "--data", "/app/data", "--addr", ":5700"]
