ifndef VERSION
# Set VERSION to the latest version tag name. Assuming version tags are formatted 'v*'
VERSION := $(shell git describe --always --abbrev=0 --tags --match "v*" 2>/dev/null || echo "v0.0.0")
BUILD := $(shell git rev-parse --short HEAD 2>/dev/null || echo "HEAD")
endif
PROJECTNAME := "projmark"
PROGRAMNAME := $(PROJECTNAME)

# Go related variables.
GOHOSTOS := $(shell go env GOHOSTOS)
GOHOSTARCH := $(shell go env GOHOSTARCH)
GOBASE := $(shell pwd)
GOBIN := $(GOBASE)/bin
GOFILES := $(shell find . -type f -name '*.go' -not -path './vendor/*' -not -path './build/*')
GOOS_DARWIN := "darwin"
GOARCH_AMD64 := "amd64"
GOARCH_ARM64 := "arm64"

MODFLAGS=-mod=readonly

# Use linker flags to provide version/build settings
LDFLAGS=-ldflags "-w -s -X=main.Version=$(VERSION) -X=main.Build=$(BUILD) -X=main.ProgramName=$(PROGRAMNAME)"

# Make is verbose in Linux. Make it silent.
# MAKEFLAGS += --silent

.PHONY: default
default: install lint format test build

## install: Checks for missing dependencies and installs them
.PHONY: install
install: go-get

## format: Formats Go source files
.PHONY: format
format: go-format

## lint: Runs all linters including go vet, golangci-lint and format check
.PHONY: lint
lint: go-lint golangci-lint go-format-check

## build: Builds binaries for all supported platforms
.PHONY: build
build: go-build

## test: Runs all Go tests
.PHONY: test
test: install go-test

## coverage: Runs all Go tests and generates a coverage report
.PHONY: coverage
coverage: install
	@echo "  >  Running tests with coverage (bypassing cache)..."
	go test $(MODFLAGS) -count=1 -coverpkg=./... -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

## coverage-html: Runs tests and opens the coverage report in a browser
.PHONY: coverage-html
coverage-html: coverage
	go tool cover -html=coverage.out

## release: Runs GoReleaser to create a release
.PHONY: release
release:
ifdef GITHUB_TOKEN
	@echo "  >  Releasing..."
	goreleaser release --clean
else
	$(error GITHUB_TOKEN is not set)
endif

## clean: Removes build artifacts
.PHONY: clean
clean:
	@-rm $(GOBIN)/$(PROGRAMNAME)* 2> /dev/null
	@-$(MAKE) go-clean

.PHONY: go-lint
go-lint:
	@echo "  >  Linting source files..."
	go vet $(MODFLAGS) -c=10 `go list $(MODFLAGS) ./...`

## golangci-lint: Runs golangci-lint
.PHONY: golangci-lint
golangci-lint:
	@echo "  >  Running golangci-lint..."
	go tool github.com/golangci/golangci-lint/v2/cmd/golangci-lint run

.PHONY: go-format
go-format:
	@echo "  >  Formatting source files..."
	gofmt -s -w $(GOFILES)

.PHONY: go-format-check
go-format-check:
	@echo "  >  Checking formatting of source files..."
	@if [ -n "$$(gofmt -l $(GOFILES))" ]; then \
		echo "  >  Format check failed for the following files:"; \
		gofmt -l $(GOFILES); \
		exit 1; \
	fi

.PHONY: go-build
go-build: go-get go-build-darwin-amd64 go-build-darwin-arm64

.PHONY: go-build-current
go-build-current:
	@echo "  >  Building $(GOHOSTOS)/$(GOHOSTARCH) binary..."
	@GOOS=$(GOHOSTOS) GOARCH=$(GOHOSTARCH) GOBIN=$(GOBIN) go build $(MODFLAGS) $(LDFLAGS) -o $(GOBIN)/$(PROGRAMNAME) $(GOBASE)/cmd/projmark

.PHONY: go-build-darwin-amd64
go-build-darwin-amd64:
	@echo "  >  Building darwin amd64 binary..."
	@GOOS=$(GOOS_DARWIN) GOARCH=$(GOARCH_AMD64) GOBIN=$(GOBIN) go build $(MODFLAGS) $(LDFLAGS) -o $(GOBIN)/$(PROGRAMNAME)-darwin-$(GOARCH_AMD64) $(GOBASE)/cmd/projmark

.PHONY: go-build-darwin-arm64
go-build-darwin-arm64:
	@echo "  >  Building darwin arm64 binary..."
	@GOOS=$(GOOS_DARWIN) GOARCH=$(GOARCH_ARM64) GOBIN=$(GOBIN) go build $(MODFLAGS) $(LDFLAGS) -o $(GOBIN)/$(PROGRAMNAME)-darwin-$(GOARCH_ARM64) $(GOBASE)/cmd/projmark

.PHONY: go-get
go-get:
	@echo "  >  Checking if there is any missing dependencies..."
	@GOBIN=$(GOBIN) go mod tidy

.PHONY: go-test
go-test:
	@echo "  >  Running tests..."
	go test $(MODFLAGS) ./...

.PHONY: go-clean
go-clean:
	@echo "  >  Cleaning build cache"
	@GOBIN=$(GOBIN) go clean $(MODFLAGS) $(GOBASE)/cmd/projmark

.PHONY: all
all: help

.PHONY: help
help: Makefile
	@echo
	@echo " Choose a command run in "$(PROJECTNAME)":"
	@echo
	@sed -n 's/^##//p' $< | column -t -s ':' |  sed -e 's/^/ /'
	@echo
