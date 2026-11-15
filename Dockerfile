# 运行时可选择 Python 版本：docker build --build-arg PYTHON_VERSION=3.11 .
ARG PYTHON_VERSION=3.10

# 构建阶段默认走国内镜像，避免直连境外源卡住。境外网络可覆盖为空串使用官方源。
#   docker compose build --build-arg NPM_REGISTRY=https://registry.npmjs.org
ARG NPM_REGISTRY=https://registry.npmmirror.com
ARG GOPROXY=https://goproxy.cn,direct
ARG DEBIAN_MIRROR=https://mirrors.tuna.tsinghua.edu.cn/debian
ARG DEBIAN_SECURITY_MIRROR=https://mirrors.tuna.tsinghua.edu.cn/debian-security

FROM node:24-bookworm-slim AS frontend
ARG NPM_REGISTRY
WORKDIR /source/web
COPY web/package*.json ./
RUN npm config set registry "$NPM_REGISTRY" \
    && npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build

FROM golang:1.27-bookworm AS backend
ARG GOPROXY
# go.sum 仍校验每个模块的哈希；GOSUMDB 关闭只跳过 sumdb 在线查询，
# 避免国内访问 sum.golang.org 超时。如需在线校验可覆盖 GOSUMDB。
ENV GOPROXY=${GOPROXY} GOSUMDB=off
WORKDIR /source
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /source/web/dist ./web/dist
ARG VERSION=1.0.1
ARG COMMIT=dev
ARG CABBAGE_UPDATE_PUBLIC_KEY=""
RUN KEY="${CABBAGE_UPDATE_PUBLIC_KEY:-$(tr -d '\r\n' < update-public-key.txt 2>/dev/null)}" && \
    CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags "-s -w -X main.version=$VERSION -X main.commit=$COMMIT -X main.updateKey=$KEY" -o /cabbage ./cmd/cabbage

# 官方 python 镜像自带 pip 与 venv，Node 从 node 镜像复制进来（同为 bookworm，glibc 一致）。
FROM node:24-bookworm-slim AS node-runtime

FROM python:${PYTHON_VERSION}-slim-bookworm
ARG DEBIAN_MIRROR
ARG DEBIAN_SECURITY_MIRROR
# 替换 Debian 源为国内镜像（默认清华），再安装运行时依赖，避免 apt-get 直连境外卡住。
RUN if [ -n "$DEBIAN_MIRROR" ]; then \
      sed -i "s|http://deb.debian.org/debian|$DEBIAN_MIRROR|g; s|http://security.debian.org/debian-security|$DEBIAN_SECURITY_MIRROR|g" \
        /etc/apt/sources.list.d/debian.sources /etc/apt/sources.list 2>/dev/null || true; \
    fi \
    && apt-get update && apt-get install -y --no-install-recommends bash git ca-certificates tzdata tini curl \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 5700 cabbage && useradd --uid 5700 --gid 5700 --create-home cabbage \
    && mkdir -p /app/data && chown cabbage:cabbage /app/data
COPY --from=node-runtime /usr/local/bin/node /usr/local/bin/node
COPY --from=node-runtime /usr/local/lib/node_modules /usr/local/lib/node_modules
RUN ln -sf ../lib/node_modules/npm/bin/npm-cli.js /usr/local/bin/npm \
    && ln -sf ../lib/node_modules/npm/bin/npx-cli.js /usr/local/bin/npx
WORKDIR /app
COPY --from=backend /cabbage /usr/local/bin/cabbage
ENV TZ=Asia/Shanghai PYTHONUTF8=1 PYTHONIOENCODING=utf-8
USER cabbage
VOLUME ["/app/data"]
EXPOSE 5700
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s CMD curl --fail --silent http://127.0.0.1:5700/healthz || exit 1
ENTRYPOINT ["/usr/bin/tini", "--", "/usr/local/bin/cabbage"]
CMD ["--data", "/app/data", "--addr", ":5700"]
