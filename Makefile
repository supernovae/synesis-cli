.PHONY: build test lint install clean doc docker

# Build configuration
BINARY_NAME=synesis
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
BUILD_GIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "")
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.BuildGit=$(BUILD_GIT)"

# Go configuration
GO=go
GO_BUILD=$(GO) build $(LDFLAGS)
GO_TEST=$(GO) test -v -race
GO_LINT=$(GO) run github.com/golangci/golangci-lint/cmd/golangci-lint@latest
GO_FMT=$(GO) fmt
GO_VET=$(GO) vet ./...

# Directories
CMD_DIR=cmd/synesis
PKG_DIRS=$(shell find . -name 'go.mod' -exec dirname {} \; | head -1)

default: build

build: $(CMD_DIR)/main.go
	$(GO_BUILD) -o bin/$(BINARY_NAME) ./$(CMD_DIR)

test:
	$(GO_TEST) ./...

lint: $(PKG_DIRS)
	$(GO_LINT) run

fmt:
	$(GO_FMT) ./...

vet:
	$(GO_VET) ./...

install: build
	cp bin/$(BINARY_NAME) $$HOME/local/bin/$(BINARY_NAME)

clean:
	rm -rf bin/
	rm -rf cover.out

docker:
	docker build -t $(BINARY_NAME):latest -f Containerfile .

push-docker:
	docker tag $(BINARY_NAME):latest $(DOCKER_REGISTRY)/$(BINARY_NAME):latest
	docker push $(DOCKER_REGISTRY)/$(BINARY_NAME):latest

revive:
	$(GO) run github.com/mgechev/revive@latest ./...

.PHONY: default build test lint fmt vet install clean docker push-docker revive