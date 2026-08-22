# --- build the Svelte SPA ---
FROM node:22-alpine AS web-build
WORKDIR /web
COPY web/package.json web/package-lock.json* ./
RUN npm install
COPY web/ ./
RUN npm run build

# --- build the Go binary (embeds web/dist from the stage above) ---
FROM golang:1.25-alpine AS go-build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web-build /web/dist ./web/dist
RUN CGO_ENABLED=0 go build -o /caddy-ui ./cmd/caddy-ui

# --- runtime ---
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=go-build /caddy-ui /usr/local/bin/caddy-ui
VOLUME ["/data"]
ENV DB_PATH=/data/caddy-ui.db
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/caddy-ui"]
