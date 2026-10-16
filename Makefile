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

# Vendored frontend assets, fetched by `make vendor-assets` into web/static
# and served from there; nothing loads from a CDN at runtime. npm's latest
# dist-tag still resolves htmx to 2.x, so the 4.x line rides the next tag
# and the pins are mandatory: latest-first, when any dist-tag publishes
# above the pin the target fetches it and moves file and pin together.
NPM_REGISTRY ?= https://registry.npmjs.org
HTMX_VERSION ?= 4.0.0
HTMX_SHA256 ?= e484d9171a9db30a39c8f16e3d709d4137f3211c659f8e6125816635033d593f
CYTOSCAPE_VERSION ?= 3.34.4
CYTOSCAPE_SHA256 ?= 081591ab735b2b6a3b31a3c60f8f3a3e2fee8c92939e0c7820ad1208571b6cd8
HTMX_URL ?= https://unpkg.com/htmx.org@$(HTMX_VERSION)/dist/htmx.min.js
CYTOSCAPE_URL ?= https://unpkg.com/cytoscape@$(CYTOSCAPE_VERSION)/dist/cytoscape.min.js

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

.PHONY: help all build test test-pkg vet fmt lint lint-pkg typecheck vendor-assets run server extract seed tidy clean coverage screenshot fixture monitor

help: ## show this help
	@awk 'BEGIN {FS = ":.*##"; printf "Usage: make <target>\n\nTargets:\n"} \
	      /^[a-zA-Z_-]+:.*##/ { printf "  %-10s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

all: test ## run the full test suite (default)

vendor-assets: ## verify or refresh the vendored htmx/cytoscape pins in web/static
	@set -e; \
	vendor_one() { \
	  pkg=$$1; pinned=$$2; sha=$$3; file=$$4; ver_name=$$5; sha_name=$$6; \
	  tags=$$(curl -fsSL $(NPM_REGISTRY)/-/package/$$pkg/dist-tags); \
	  newest=$$(printf '%s\n' "$$tags" | sed 's/[^:]*:"\([^"]*\)"/\1\n/g' | grep -E '^[0-9]' | sort -V | tail -1); \
	  if [ "$$newest" != "$$pinned" ] && [ "$$(printf '%s\n%s\n' "$$pinned" "$$newest" | sort -V | tail -1)" = "$$newest" ]; then \
	    curl -fsSL "https://unpkg.com/$$pkg@$$newest/dist/$$file" -o web/static/$$file.tmp; \
	    mv web/static/$$file.tmp web/static/$$file; \
	    sum=$$(sha256sum web/static/$$file | cut -d' ' -f1); \
	    sed -i "s/^$$ver_name ?=.*/$$ver_name ?= $$newest/" Makefile; \
	    sed -i "s/^$$sha_name ?=.*/$$sha_name ?= $$sum/" Makefile; \
	    echo "$$pkg: pin moved $$pinned -> $$newest (sha256 $$sum)"; \
	  else \
	    if ! echo "$$sha  web/static/$$file" | sha256sum -c --status -; then \
	      curl -fsSL "https://unpkg.com/$$pkg@$$pinned/dist/$$file" -o web/static/$$file.tmp; \
	      mv web/static/$$file.tmp web/static/$$file; \
	    fi; \
	    echo "$$sha  web/static/$$file" | sha256sum -c -; \
	  fi; \
	}; \
	vendor_one htmx.org '$(HTMX_VERSION)' '$(HTMX_SHA256)' htmx.min.js HTMX_VERSION HTMX_SHA256; \
	vendor_one cytoscape '$(CYTOSCAPE_VERSION)' '$(CYTOSCAPE_SHA256)' cytoscape.min.js CYTOSCAPE_VERSION CYTOSCAPE_SHA256

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
