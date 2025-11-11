# Makefile для локальной разработки booking-svc
# Используется для разработки одного сервиса

.PHONY: build test run tidy migrate

# Update dependencies
tidy:
	@echo "📦 Updating dependencies..."
	@go mod tidy

# Build service
build:
	@echo "🔨 Building booking-svc..."
	@go build -o bin/booking-svc ./cmd/booking-svc

# Run tests
test:
	@echo "🧪 Running tests..."
	@go test ./...

# Run tests with coverage
test-coverage:
	@echo "🧪 Running tests with coverage..."
	@go test -cover ./...

# Run service locally (requires infrastructure to be running)
run:
	@echo "🚀 Running booking-svc locally..."
	@go run ./cmd/booking-svc

# Run migrations for this service
migrate:
	@echo "📦 Running migrations..."
	@if [ -f migrations/*.sql ]; then \
		echo "⚠️  Migrations should be run via booker-infra"; \
	fi

