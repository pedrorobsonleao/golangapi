.PHONY: all build run test coverage clean generate

# Binary name
BINARY_NAME=api-server

# Default target
all: build

# Generate code from OpenAPI
generate:
	chmod +x build.sh
	./build.sh

# Build the application local binary
build:
	go build -o $(BINARY_NAME) ./src

# Run the application locally
run: build
	./$(BINARY_NAME)

# Run unit tests
test:
	go test -v ./src/...

# Run unit tests with coverage
coverage:
	go test -v -coverprofile=coverage.out ./src/...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated at coverage.html"

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME) coverage.out coverage.html
