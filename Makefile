DATABASE_URL ?= postgres://hidro:hidro@localhost:5432/hidro?sslmode=disable
GOBIN := $(shell go env GOPATH)/bin

.PHONY: up down logs build run test sqlc migrate-up migrate-down migrate-create tidy

up:
	docker-compose up -d

down:
	docker-compose down

logs:
	docker-compose logs -f

build:
	go build -o bin/server ./cmd/server

run:
	go run ./cmd/server

test:
	go test ./...

sqlc:
	$(GOBIN)/sqlc generate

migrate-up:
	$(GOBIN)/migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	$(GOBIN)/migrate -path migrations -database "$(DATABASE_URL)" down 1

# usage: make migrate-create name=add_something
migrate-create:
	$(GOBIN)/migrate create -ext sql -dir migrations -seq $(name)

tidy:
	go mod tidy
