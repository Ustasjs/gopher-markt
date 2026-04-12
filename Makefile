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

migrate-create:
	@if [ -z "$(name)" ]; then \
		echo "Error: name is required. Usage: make migrate-create name=create_users_table"; \
		exit 1; \
	fi
	migrate create -ext sql -dir ./migrations -seq $(name)

.PHONY: migrate-create
migrate-create:
ifndef name
	$(error name is not set. Usage: make migrate-create name=your_migration_name)
endif
	migrate create -ext sql -dir ./migrations -seq $(name)
