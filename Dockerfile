# --- build the Svelte SPA ---
FROM --platform=$BUILDPLATFORM node:22-alpine AS web-build
WORKDIR /web
RUN corepack enable
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm run build

# --- build the Go binary (embeds web/dist from the stage above) ---
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS go-build
ARG TARGETOS TARGETARCH TARGETVARIANT
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web-build /web/dist ./web/dist
ARG VERSION=dev
# Cross-compile natively rather than emulating the target (the SPA is
# platform-independent, and the Node toolchain has no armv7-musl binding).
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /caddy-ui ./cmd/caddy-ui

# --- runtime ---
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=go-build /caddy-ui /usr/local/bin/caddy-ui
VOLUME ["/data"]
ENV DB_PATH=/data/caddy-ui.db
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/caddy-ui"]
