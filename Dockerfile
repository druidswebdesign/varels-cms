FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go install github.com/a-h/templ/cmd/templ@latest
RUN templ generate
# modernc.org/sqlite is pure Go, so no CGO toolchain is needed.
RUN CGO_ENABLED=0 go build -o /varels_cms ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /varels_cms /usr/local/bin/
COPY --from=builder /app/assets /app/assets
WORKDIR /app
ENV DB_PATH=/app/data/app.db
EXPOSE 8080
CMD ["varels_cms"]
