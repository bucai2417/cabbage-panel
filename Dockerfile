# 大白菜面板 Docker 构建
#
# 设计要点：
# 1. 只拉一个基础镜像（python:slim-bookworm），Node / Go 从国内镜像下二进制。
#    实测清华 Node 源、阿里云 Go 源 4-5 MB/s，镜像加速站只有 0.1-0.4 MB/s。
# 2. 源码从 GitHub 下载完整 tarball，不使用构建上下文里的源码。
#    这样即使服务器上的源码树不完整（缺目录 / 权限异常 / 部分文件没上传），
#    构建依然能成功。构建上下文只需要本 Dockerfile 本身。
#
# 常用覆盖参数：
#   docker build --build-arg SOURCE_REF=v1.0.1 -t cabbage-panel:1.0.1 .
#   docker build --build-arg PYTHON_VERSION=3.11 -t cabbage-panel:1.0.1 .
#   docker build --build-arg DOCKER_MIRROR= -t cabbage-panel:1.0.1 .   # 直连 Docker Hub

ARG PYTHON_VERSION=3.10

# 源码来源。默认 main 分支；可指定 tag 或 commit。
ARG SOURCE_REPO=bucai2417/cabbage-panel
ARG SOURCE_REF=main
ARG SOURCE_MIRROR=https://codeload.github.com

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
        bash git ca-certificates tzdata tini curl tar \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 5700 cabbage && useradd --uid 5700 --gid 5700 --create-home cabbage \
    && mkdir -p /app/data && chown cabbage:cabbage /app/data
ENV PATH=/opt/node/bin:$PATH

# ---------- 源码：从 GitHub 下载完整 tarball ----------
FROM base AS source
ARG SOURCE_REPO
ARG SOURCE_REF
ARG SOURCE_MIRROR
RUN set -eux; \
    url="$SOURCE_MIRROR/$SOURCE_REPO/tar.gz/refs/heads/$SOURCE_REF"; \
    echo "下载源码: $url"; \
    if ! curl -fsSL --retry 3 --retry-delay 2 "$url" -o /tmp/src.tar.gz; then \
      echo "分支下载失败，改用 tag 路径重试" >&2; \
      curl -fsSL --retry 3 --retry-delay 2 "$SOURCE_MIRROR/$SOURCE_REPO/tar.gz/refs/tags/$SOURCE_REF" -o /tmp/src.tar.gz; \
    fi; \
    mkdir -p /src; \
    tar -xzf /tmp/src.tar.gz -C /src --strip-components=1; \
    rm /tmp/src.tar.gz; \
    echo "--- 源码完整性检查 ---"; \
    for f in go.mod go.sum cmd/cabbage/main.go internal/panel/app.go \
             web/package.json web/package-lock.json web/index.html web/vite.config.js \
             web/src/App.vue web/src/main.js web/src/components/Icon.vue web/src/pages/Dashboard.vue; do \
      if [ ! -f "/src/$f" ]; then echo "FATAL: 源码缺少 $f" >&2; exit 1; fi; \
    done; \
    echo "源码完整，文件数: $(find /src -type f | wc -l)"

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
WORKDIR /app
ENV TZ=Asia/Shanghai PYTHONUTF8=1 PYTHONIOENCODING=utf-8
USER cabbage
VOLUME ["/app/data"]
EXPOSE 5700
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s \
  CMD curl --fail --silent http://127.0.0.1:5700/healthz || exit 1
ENTRYPOINT ["/usr/bin/tini", "--", "/usr/local/bin/cabbage"]
CMD ["--data", "/app/data", "--addr", ":5700"]
