# Binary name
BINARY_NAME=mail-server
BUILD_DIR=bin

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test

# Build Flags
# -s: Omit the symbol table and debug information
# -w: Omit the DWARF symbol table
LDFLAGS_RELEASE=-ldflags "-s -w"
# -N: Disable optimizations
# -l: Disable inlining
GCFLAGS_DEBUG=-gcflags "all=-N -l"

.PHONY: all build release debug run clean test build-all build-linux build-windows build-mac

all: build

# Default build (Development)
build:
	mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) .

# Release build (Optimized, smaller size)
release:
	mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS_RELEASE) -o $(BUILD_DIR)/$(BINARY_NAME) .

# Debug build (For use with Delve or other debuggers)
debug:
	mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(GCFLAGS_DEBUG) -o $(BUILD_DIR)/$(BINARY_NAME)-debug .

# Run the application
run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

# Cross Compilation Targets

# Linux
build-linux:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS_RELEASE) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 .
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS_RELEASE) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 .

# Windows
build-windows:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS_RELEASE) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe .

# MacOS (Darwin)
build-mac:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS_RELEASE) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 .
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS_RELEASE) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 .

# Build for all platforms
build-all: build-linux build-windows build-mac

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)

