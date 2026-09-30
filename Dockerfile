FROM golang:1.26-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

# Code generators pinned to the versions used in CI (generated code is not committed).
RUN go install github.com/a-h/templ/cmd/templ@v0.3.1020 \
 && go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1

# Standalone Tailwind CLI (no Node.js) so the image ships a built stylesheet.
# The release binary is glibc-linked, so Alpine needs libc6-compat to run it.
ARG TARGETARCH
RUN apk add --no-cache libc6-compat libgcc libstdc++; \
    [ -n "${TARGETARCH}" ] || TARGETARCH=amd64; \
    case "${TARGETARCH}" in \
      amd64) twarch=x64 ;; \
      arm64) twarch=arm64 ;; \
      *) echo "unsupported arch ${TARGETARCH}" >&2; exit 1 ;; \
    esac; \
    wget -qO /usr/local/bin/tailwindcss \
      "https://github.com/tailwindlabs/tailwindcss/releases/download/v4.3.3/tailwindcss-linux-${twarch}"; \
    chmod +x /usr/local/bin/tailwindcss

COPY . .
RUN templ generate \
 && sqlc generate \
 && tailwindcss -i assets/css/input.css -o assets/css/app.css --minify

# modernc.org/sqlite is pure Go, so no CGO toolchain is needed.
RUN CGO_ENABLED=0 go build -o /varels-cms ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates wget \
 && adduser -D -u 10001 appuser

COPY --from=builder /varels-cms /usr/local/bin/
COPY --from=builder /app/assets /app/assets

WORKDIR /app
RUN mkdir -p /app/data && chown -R appuser:appuser /app
USER appuser

ENV DB_PATH=/app/data/app.db
VOLUME ["/app/data"]
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- "http://127.0.0.1:${PORT:-8080}/healthz" >/dev/null 2>&1 || exit 1

CMD ["varels-cms"]
