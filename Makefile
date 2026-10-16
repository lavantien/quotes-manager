# quotes-manager — common commands. The SQLite driver needs CGO, so every Go
# command runs with CGO_ENABLED=1 and the sqlite_fts5 build tag.
CGO_ENV := CGO_ENABLED=1
TAGS := -tags sqlite_fts5
GO := $(CGO_ENV) go

ADDR ?= 127.0.0.1:8080
DB   ?= database/quotes.db

# Combined working-set ceiling the memory monitor enforces over every
# project process (compilers, test binaries, mutation) while it runs;
# MONITOR_FOR bounds its runtime for smoke runs, 0 means until killed.
MEMCAP ?= 16GB
MONITOR_FOR ?= 0

# duckdb-go's prebuilt Windows static libs link against UCRT; the default
# msvcrt-flavored mingw cannot. go-sqlite3 builds under either. Override
# via environment (CC/CXX) or GCC_UCRT_BIN on machines with another layout.
ifeq ($(OS),Windows_NT)
GCC_UCRT_BIN ?= $(subst \,/,$(USERPROFILE))/scoop/apps/mingw-winlibs-ucrt/current/bin
ifneq ($(origin CC),command line)
CC := $(GCC_UCRT_BIN)/gcc.exe
endif
ifneq ($(origin CXX),command line)
CXX := $(GCC_UCRT_BIN)/g++.exe
endif
export CC CXX
endif

# Windows binaries carry the .exe suffix; everything below runs the one
# name that actually exists on this platform.
ifeq ($(OS),Windows_NT)
EXE := .exe
endif
BIN := dist/qm$(EXE)

.PHONY: help all build test test-pkg vet fmt lint lint-pkg typecheck run server extract seed tidy clean coverage screenshot fixture monitor

help: ## show this help
	@awk 'BEGIN {FS = ":.*##"; printf "Usage: make <target>\n\nTargets:\n"} \
	      /^[a-zA-Z_-]+:.*##/ { printf "  %-10s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

all: test ## run the full test suite (default)

build: ## build the qm binary into dist/
	$(GO) build $(TAGS) -o $(BIN) ./cmd/qm

test: ## run the full test suite
	$(GO) test $(TAGS) ./...

test-pkg: ## run one package's tests: make test-pkg PKG=./internal/config
	$(GO) test $(TAGS) $(PKG)

vet: ## go vet
	$(GO) vet $(TAGS) ./...

fmt: ## gofumpt all Go files
	gofumpt -w .

lint: ## golangci-lint run
	golangci-lint run

lint-pkg: ## lint one package: make lint-pkg PKG=./internal/config
	golangci-lint run $(PKG)

typecheck: ## compile every package, then go vet
	$(GO) build $(TAGS) ./...
	$(GO) vet $(TAGS) ./...

run server: ## run the web server (http://$(ADDR))
	$(GO) run $(TAGS) ./cmd/qm serve --addr $(ADDR) --db $(DB)

extract: ## regenerate database/seed.sql + exports/shortest-first.md from dumps/
	$(GO) run $(TAGS) ./cmd/qm extract

seed: ## delete the database so the next 'make run' re-seeds it from seed.sql
	rm -f $(DB)
	@echo "database removed; the next 'make run' will re-seed it"

tidy: ## go mod tidy
	go mod tidy

clean: ## remove generated artifacts (database, binaries)
	rm -f $(DB) *.exe *.test *.out
	rm -rf dist

coverage: ## recompute Go test coverage and refresh the README coverage badge
	$(GO) test $(TAGS) -coverpkg=./... -coverprofile=coverage.out ./...
	$(GO) run $(TAGS) ./cmd/qm coverage -profile=coverage.out -readme=readme.md
	rm -f coverage.out

screenshot: ## capture the home page into project-pictures/home.png (needs Chrome or Edge)
	$(GO) run $(TAGS) ./cmd/qm screenshot

fixture: ## regenerate the clone-of-main test fixture (internal/store/storetest/testdata)
	$(GO) run $(TAGS) ./cmd/qm fixture

monitor: ## memory guard over project processes (make monitor MONITOR_FOR=5s)
	$(CGO_ENV) go run ./tools/memcapmon -cap $(MEMCAP) -poll 2s -for $(MONITOR_FOR)
