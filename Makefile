.PHONY: lint test generate build vuln up

SQLC = go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1
GOVULNCHECK = go run golang.org/x/vuln/cmd/govulncheck@v1.8.0

lint:
	golangci-lint run

test:
	go test -race ./...

generate:
	$(SQLC) generate

build:
	CGO_ENABLED=0 go build -trimpath -o bin/collector ./cmd/collector
	CGO_ENABLED=0 go build -trimpath -o bin/api ./cmd/api

vuln:
	$(GOVULNCHECK) ./...

up:
	docker compose -f deploy/compose.yaml --env-file deploy/.env --profile edge up -d --build
