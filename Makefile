.PHONY: all build build-windows generate clean test fmt vet lint pre-commit help

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

# Run all pre-commit checks (mirrors CI and release pipeline)
pre-commit:
	@echo "==> Generating..."
	@go generate ./...
	@echo "==> Tidying..."
	@go mod tidy
	@echo "==> Formatting..."
	@go fmt ./...
	@echo "==> Vetting..."
	@go vet ./...
	@echo "==> Linting..."
	@golangci-lint run ./...
	@echo "==> Testing..."
	@go test -race ./...
	@echo "==> Building (linux/amd64, windows/amd64, darwin/arm64)..."
	@mkdir -p $(BUILD_DIR)
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/cli-linux-amd64 ./cmd/cli & \
		pid1=$$!; \
		CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/cli-windows-amd64.exe ./cmd/cli & \
		pid2=$$!; \
		CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o $(BUILD_DIR)/cli-darwin-arm64 ./cmd/cli & \
		pid3=$$!; \
		wait $$pid1 && wait $$pid2 && wait $$pid3
	@rm -rf $(BUILD_DIR)
	@echo "==> All checks passed."

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
	@echo "  pre-commit     - Run all checks (generate, tidy, fmt, lint, test, cross-build)"
	@echo "  clean          - Remove $(BUILD_DIR)/ directory"
	@echo "  all            - Clean, generate, test, and build"
