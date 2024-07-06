# Makefile for building and running the Kubernetes operator

# Variables
BINARY_NAME=operator
GO_FILES=$(shell find . -name '*.go')
OUTPUT_DIR=bin

DOCKER_IMAGE=bestgres/operator
# Generate a tag based on the current git sha
GIT_SHA := $(shell git rev-parse --short HEAD)
TAG := dev-$(GIT_SHA)

# Default build platform
BUILDPLATFORM = linux/amd64
GOOS = linux
GOARCH = amd64

# Determine the operating system and architecture
UNAME_S := $(shell uname -s)
UNAME_M := $(shell uname -m)

ifeq ($(UNAME_S),Darwin)
    GOOS = darwin
    ifeq ($(UNAME_M),arm64)
        GOARCH = arm64
		BUILDPLATFORM = linux/arm64
    else
        GOARCH = amd64
    endif
endif



# Default target
all: docker-build

# Build the binary
build: $(GO_FILES)
	mkdir -p $(OUTPUT_DIR)
	cd cmd/operator && \
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o ../../$(OUTPUT_DIR)/$(BINARY_NAME) main.go

# Build for Linux
build-linux: $(GO_FILES)
	mkdir -p $(OUTPUT_DIR)
	cd cmd/operator && \
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ../../$(OUTPUT_DIR)/$(BINARY_NAME)-linux-amd64 main.go

# Run tests
test: $(GO_FILES)
	go test ./... -cover

# Clean up build artifacts
clean:
	rm -rf $(OUTPUT_DIR)

download:
	go mod download

# Build the Docker image
docker-build: build
	docker buildx build --platform $(BUILDPLATFORM) -t $(DOCKER_IMAGE):$(TAG) . \
		--build-arg BUILDPLATFORM=$(BUILDPLATFORM) 

# Help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all                Default target, builds the binary for the current OS/arch"
	@echo "  build              Build the binary for the current OS/arch"
	@echo "  build-linux        Build the binary for Linux/amd64"
	@echo "  test               Run tests"
	@echo "  clean              Clean up build artifacts"
	@echo "  download           Download dependencies"
	@echo "  docker-build       Build the Docker image"
	@echo "  help               Display this help message"

.PHONY: all build build-linux test clean docker-build help