# Загружаем переменные из .env, если файл существует
ifneq (,$(wildcard ./.env))
    include .env
    export
endif

.PHONY: run
run:
	go run ./cmd/gophermart/main.go

.PHONY: test
test:
	go test ./... -v -count=1

.PHONY: lint
lint:
	golangci-lint run ./...