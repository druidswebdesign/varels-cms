.PHONY: dev build generate sqlc css migrate test tidy clean seed

BINARY          := bin/varels_cms
DB_PATH         ?= data/app.db
MIGRATIONS      := internal/db/migrations
TAILWIND_INPUT  := assets/css/input.css
TAILWIND_OUTPUT := assets/css/app.css

generate:
	templ generate

sqlc:
	sqlc generate

css:
	tailwindcss -i $(TAILWIND_INPUT) -o $(TAILWIND_OUTPUT) --minify

migrate:
	goose -dir $(MIGRATIONS) sqlite3 $(DB_PATH) up

dev:
	air

build: generate
	CGO_ENABLED=0 go build -o $(BINARY) ./cmd/server

test:
	go test ./...

tidy:
	go mod tidy

clean:
	rm -rf bin tmp $(TAILWIND_OUTPUT)

seed: clean dev
