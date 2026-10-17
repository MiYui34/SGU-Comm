# Kallisti: the Go server with the built web app. See deploy/docker/compose.yml.

FROM docker.io/library/node:24-alpine AS web
RUN corepack enable && corepack prepare pnpm@11.22.0 --activate
WORKDIR /src
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml tsconfig.base.json ./
COPY packages/protocol/package.json packages/protocol/
COPY packages/devtools/package.json packages/devtools/
COPY apps/web/package.json apps/web/
RUN pnpm install --frozen-lockfile --filter @kallisti/web...
COPY packages/protocol packages/protocol
COPY apps/web apps/web
RUN pnpm --filter @kallisti/web build

FROM docker.io/library/golang:1.26-alpine AS server
WORKDIR /src
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /kallisti ./cmd/server

FROM docker.io/library/alpine:3.21
RUN adduser -D -H kallisti && mkdir -p /data && chown kallisti /data
COPY --from=server /kallisti /app/kallisti
COPY --from=web /src/apps/web/dist /app/web
ENV WEB_DIR=/app/web DB_PATH=/data/kallisti.db UPLOAD_DIR=/data/uploads LISTEN_PORT=8080
VOLUME /data
# 8080: web app and web clients; 64738: Mumble clients and the desktop app.
EXPOSE 8080 64738/tcp 64738/udp
USER kallisti
ENTRYPOINT ["/app/kallisti"]
