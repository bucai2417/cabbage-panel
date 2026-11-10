FROM node:24-bookworm-slim AS frontend
WORKDIR /source/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.27-bookworm AS backend
WORKDIR /source
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /source/web/dist ./web/dist
ARG VERSION=1.1.0
ARG COMMIT=dev
ARG CABBAGE_UPDATE_PUBLIC_KEY=""
RUN CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags "-s -w -X main.version=$VERSION -X main.commit=$COMMIT -X main.updateKey=$CABBAGE_UPDATE_PUBLIC_KEY" -o /cabbage ./cmd/cabbage

FROM node:24-bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends python3 python3-venv python3-pip bash git ca-certificates tzdata tini curl \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 5700 cabbage && useradd --uid 5700 --gid 5700 --create-home cabbage \
    && mkdir -p /app/data && chown cabbage:cabbage /app/data
WORKDIR /app
COPY --from=backend /cabbage /usr/local/bin/cabbage
ENV TZ=Asia/Shanghai PYTHONUTF8=1 PYTHONIOENCODING=utf-8
USER cabbage
VOLUME ["/app/data"]
EXPOSE 5700
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s CMD curl --fail --silent http://127.0.0.1:5700/healthz || exit 1
ENTRYPOINT ["/usr/bin/tini", "--", "/usr/local/bin/cabbage"]
CMD ["--data", "/app/data", "--addr", ":5700"]
