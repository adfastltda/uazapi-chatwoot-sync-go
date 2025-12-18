.PHONY: build run test clean deps

# Build the application
build:
	go build -o bin/chatwoot-sync main.go

# Run the application
run:
	go run main.go

# Run tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -rf bin/
	go clean

# Download dependencies
deps:
	go mod download
	go mod tidy

# Install dependencies
install: deps build

# Run with environment file
run-env:
	@if [ ! -f .env ]; then \
		echo "Error: .env file not found. Copy .env.example to .env and configure it."; \
		exit 1; \
	fi
	go run main.go

