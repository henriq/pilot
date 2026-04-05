.PHONY: all build build-windows generate clean test fmt vet lint help

# Build output directory
BUILD_DIR := bin

# Default build for current platform
build: generate
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/cli ./cmd/cli

# Build for Windows AMD64
build-windows: generate
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/cli.exe ./cmd/cli

# Generate Wire dependency injection code
generate:
	go generate ./...

# Run all tests
test:
	go test ./...

# Format code
fmt:
	go fmt ./...

# Run go vet
vet:
	go vet ./...

# Run linters (matches CI)
lint: vet
	golangci-lint run ./...

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)

# Build everything
all: clean generate test build

# Show available targets
help:
	@echo "Available targets:"
	@echo "  build          - Build for current platform (output: $(BUILD_DIR)/cli)"
	@echo "  build-windows  - Cross-compile for Windows AMD64 (output: $(BUILD_DIR)/cli.exe)"
	@echo "  generate       - Run go generate (Wire DI)"
	@echo "  test           - Run all tests"
	@echo "  fmt            - Format Go code"
	@echo "  vet            - Run go vet"
	@echo "  lint           - Run go vet and golangci-lint"
	@echo "  clean          - Remove $(BUILD_DIR)/ directory"
	@echo "  all            - Clean, generate, test, and build"
