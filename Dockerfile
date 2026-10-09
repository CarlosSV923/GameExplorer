# Production image: the Go binary with the React app embedded, plus the
# official 7-Zip (with RAR). Built by `task image`; deployed with
# deploy/truenas/compose.yaml.

# --- 1. Web app ---
FROM node:24-alpine AS web
WORKDIR /src
RUN corepack enable
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY apps/web/package.json apps/web/
RUN pnpm install --frozen-lockfile --filter @gameexplorer/web
COPY apps/web apps/web
COPY contracts contracts
RUN pnpm --filter @gameexplorer/web build

# --- 2. Go binary with the SPA embedded ---
FROM golang:1.27.2 AS api
WORKDIR /src/apps/api
COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download
COPY apps/api .
COPY --from=web /src/apps/web/dist internal/platform/web/dist
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/gameexplorer ./cmd/gameexplorer

# --- 3. Runtime ---
FROM debian:13-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates curl xz-utils \
 && rm -rf /var/lib/apt/lists/*
COPY scripts/install-7zip.sh /tmp/install-7zip.sh
RUN sh /tmp/install-7zip.sh && rm /tmp/install-7zip.sh \
 && apt-get purge -y xz-utils && apt-get autoremove -y
COPY --from=api /out/gameexplorer /usr/local/bin/gameexplorer
ENV PORT=8080 LIBRARY_PATH=/library DATA_PATH=/data
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s CMD curl -fsS http://localhost:8080/api/health || exit 1
ENTRYPOINT ["/usr/local/bin/gameexplorer"]
