# Переменные окружения
POSTGRES_DSN=postgres://postgres:1234@localhost:5432/corpord?sslmode=disable
MIGRATIONS_DIR=db/migrations
BINARY_NAME=app

# Запуск приложения
run:
	go run cmd/server/main.go

# Сборка приложения
build:
	go build -o bin/$(BINARY_NAME) cmd/server/main.go

# Запуск тестов
test:
	go test -v ./...

# Миграции
migrate-status:
	goose -dir $(MIGRATIONS_DIR) postgres "$(POSTGRES_DSN)" status

migrate-up:
	goose -dir $(MIGRATIONS_DIR) postgres "$(POSTGRES_DSN)" up

migrate-down:
	goose -dir $(MIGRINGS_DIR) postgres "$(POSTGRES_DSN)" down

migrate-create:
	@read -p "Введите название миграции: " name; \
	goose -dir $(MIGRATIONS_DIR) create $$name sql

# Очистка
clean:
	rm -rf bin/ coverage.out

# Генерация Swagger документации
swagger:
	swag init -g cmd/server/main.go

# Проверка кода
lint:
	golangci-lint run

.PHONY: run build test migrate-status migrate-up migrate-down migrate-create clean swagger lint