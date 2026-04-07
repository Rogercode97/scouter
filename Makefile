# Scouter — Code analysis engine for AI agents
BIN_DIR = bin
APP_NAME = scouter

.PHONY: all build clean run test

all: clean build

build:
	@mkdir -p $(BIN_DIR)
	@echo "Building Scouter..."
	@go build -o $(BIN_DIR)/$(APP_NAME) ./cmd/scouter
	@echo "Build complete: $(BIN_DIR)/$(APP_NAME)"

clean:
	@rm -rf $(BIN_DIR)
	@echo "Cleaned build artifacts."

run:
	@go run ./cmd/scouter

test:
	@go test ./...
