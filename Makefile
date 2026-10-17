CGO_ENV := CGO_ENABLED=1
TAGS := -tags sqlite_fts5
GO := $(CGO_ENV) go

ADDR ?= 127.0.0.1:8080
DB   ?= database/quotes.db

FUZZ_PARALLEL ?= 4
FUZZ_TIME ?= 20s

MUTATE_PKGS ?= internal/canon/norm,internal/canon/source/bilara,internal/canon/source/editions,internal/canon/source/dpd,internal/canon/triangulate,internal/canon/server,internal/canon/jobs,internal/canon/analytics,internal/canon/export,internal/quote,internal/search,internal/store,internal/server,internal/cli
MUTATE_PARALLEL ?= 8

E2E_TIMEOUT ?= 30m

MEMSOFT ?= 4GiB

GOMEMLIMIT ?= 512MiB

ifeq ($(OS),Windows_NT)
EXE := .exe
endif

WRAP := $(CURDIR)/dist/tools/jobwrap$(EXE)

RACE_PKGS ?= ./internal/quote ./internal/search ./internal/store ./internal/server ./internal/cli ./internal/e2e

STAGE ?=

MEMCAP ?= 16GB
MONITOR_FOR ?= 0

NPM_REGISTRY ?= https://registry.npmjs.org
UNPKG ?= https://unpkg.com
HTMX_VERSION ?= 4.0.0
HTMX_SHA256 ?= e484d9171a9db30a39c8f16e3d709d4137f3211c659f8e6125816635033d593f
CYTOSCAPE_VERSION ?= 3.34.4
CYTOSCAPE_SHA256 ?= 081591ab735b2b6a3b31a3c60f8f3a3e2fee8c92939e0c7820ad1208571b6cd8
HTMX_URL ?= $(UNPKG)/htmx.org@$(HTMX_VERSION)/dist/htmx.min.js
CYTOSCAPE_URL ?= $(UNPKG)/cytoscape@$(CYTOSCAPE_VERSION)/dist/cytoscape.min.js

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

BIN := dist/qm$(EXE)

FZ := $(CGO_ENV) GOMEMLIMIT=$(MEMSOFT) $(WRAP) go test $(TAGS)

.PHONY: help all build test test-pkg race vet fmt lint lint-pkg typecheck sloc fuzz fuzz-one mutate memtest e2e-hermetic e2e verify vendor-assets stage run server extract seed tidy clean coverage screenshot fixture monitor

help: ## show this help
	@awk 'BEGIN {FS = ":.*##"; printf "Usage: make <target>\n\nTargets:\n"} \
	      /^[a-zA-Z_-]+:.*##/ { printf "  %-10s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

all: test ## run the full test suite (default)

build: ## build the qm binary into dist/
	$(GO) build $(TAGS) -o $(BIN) ./cmd/qm

$(WRAP): $(wildcard tools/jobwrap/*.go) $(wildcard internal/canon/winjob/*.go)
	$(GO) build -o $@ ./tools/jobwrap

test: $(WRAP) ## run the full test suite
	$(CGO_ENV) GOMEMLIMIT=$(MEMSOFT) $(WRAP) go test $(TAGS) ./...

test-pkg: $(WRAP) ## run one package's tests: make test-pkg PKG=./internal/config
	$(CGO_ENV) GOMEMLIMIT=$(MEMSOFT) $(WRAP) go test $(TAGS) $(PKG)

race: $(WRAP) ## go test -race over the merged-app core (RACE_PKGS)
	$(CGO_ENV) GOMEMLIMIT=$(MEMSOFT) $(WRAP) go test $(TAGS) -race -count=1 $(RACE_PKGS)

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

sloc: ## enforce the 1000-line ceiling on every Go file
	$(CGO_ENV) go run ./tools/sloc

fuzz: $(WRAP) ## fuzz every target in the repo for FUZZ_TIME each
	$(FZ) -run '^$$' -fuzz FuzzAnalyticsSQLGate -fuzztime $(FUZZ_TIME) -parallel $(FUZZ_PARALLEL) ./internal/canon/analytics
	$(FZ) -run '^$$' -fuzz FuzzEnsureNode -fuzztime $(FUZZ_TIME) -parallel $(FUZZ_PARALLEL) ./internal/canon/graph
	$(FZ) -run '^$$' -fuzz FuzzPublishPointer -fuzztime $(FUZZ_TIME) -parallel $(FUZZ_PARALLEL) ./internal/canon/graph
	$(FZ) -run '^$$' -fuzz FuzzSearchFTSText -fuzztime $(FUZZ_TIME) -parallel $(FUZZ_PARALLEL) ./internal/canon/graph
	$(FZ) -run '^$$' -fuzz FuzzSegmentsMuid -fuzztime $(FUZZ_TIME) -parallel $(FUZZ_PARALLEL) ./internal/canon/graph
	$(FZ) -run '^$$' -fuzz FuzzTreeDivisionKey -fuzztime $(FUZZ_TIME) -parallel $(FUZZ_PARALLEL) ./internal/canon/graph
	$(FZ) -run '^$$' -fuzz FuzzPaliTokenizer -fuzztime $(FUZZ_TIME) -parallel $(FUZZ_PARALLEL) ./internal/canon/norm
	$(FZ) -run '^$$' -fuzz FuzzRenderJSONRoundTrip -fuzztime $(FUZZ_TIME) -parallel $(FUZZ_PARALLEL) ./internal/canon/report
	$(FZ) -run '^$$' -fuzz FuzzServerGraphCaps -fuzztime $(FUZZ_TIME) -parallel $(FUZZ_PARALLEL) ./internal/canon/server
	$(FZ) -run '^$$' -fuzz FuzzServerLimits -fuzztime $(FUZZ_TIME) -parallel $(FUZZ_PARALLEL) ./internal/canon/server
	$(FZ) -run '^$$' -fuzz FuzzServerReportPath -fuzztime $(FUZZ_TIME) -parallel $(FUZZ_PARALLEL) ./internal/canon/server
	$(FZ) -run '^$$' -fuzz FuzzServerSearch -fuzztime $(FUZZ_TIME) -parallel $(FUZZ_PARALLEL) ./internal/canon/server
	$(FZ) -run '^$$' -fuzz FuzzJobsStage -fuzztime $(FUZZ_TIME) -parallel $(FUZZ_PARALLEL) ./internal/canon/server
	$(FZ) -run '^$$' -fuzz FuzzBilaraSegmentJSON -fuzztime $(FUZZ_TIME) -parallel $(FUZZ_PARALLEL) ./internal/canon/source/bilara
	$(FZ) -run '^$$' -fuzz FuzzSegmentKey -fuzztime $(FUZZ_TIME) -parallel $(FUZZ_PARALLEL) ./internal/canon/source/bilara
	$(FZ) -run '^$$' -fuzz FuzzDpdExtract -fuzztime $(FUZZ_TIME) -parallel $(FUZZ_PARALLEL) ./internal/canon/source/dpd
	$(FZ) -run '^$$' -fuzz FuzzDpdInflections -fuzztime $(FUZZ_TIME) -parallel $(FUZZ_PARALLEL) ./internal/canon/source/dpd
	$(FZ) -run '^$$' -fuzz FuzzHTMLSegments -fuzztime $(FUZZ_TIME) -parallel $(FUZZ_PARALLEL) ./internal/canon/source/editions
	$(FZ) -run '^$$' -fuzz FuzzDpdKnownForms -fuzztime $(FUZZ_TIME) -parallel $(FUZZ_PARALLEL) ./internal/canon/triangulate
	$(FZ) -run '^$$' -fuzz FuzzCapJoin -fuzztime $(FUZZ_TIME) -parallel $(FUZZ_PARALLEL) ./internal/canon/triangulate
	$(FZ) -run '^$$' -fuzz FuzzParseDateTolerant -fuzztime $(FUZZ_TIME) -parallel $(FUZZ_PARALLEL) ./internal/canon/triangulate
	$(FZ) -run '^$$' -fuzz FuzzSplitEnKey -fuzztime $(FUZZ_TIME) -parallel $(FUZZ_PARALLEL) ./internal/canon/triangulate
	$(FZ) -run '^$$' -fuzz FuzzWebEscape -fuzztime $(FUZZ_TIME) -parallel $(FUZZ_PARALLEL) ./internal/canon/web
	$(FZ) -run '^$$' -fuzz FuzzParseCanonical -fuzztime $(FUZZ_TIME) -parallel $(FUZZ_PARALLEL) ./internal/quote
	$(FZ) -run '^$$' -fuzz FuzzSearchParse -fuzztime $(FUZZ_TIME) -parallel $(FUZZ_PARALLEL) ./internal/search

fuzz-one: $(WRAP) ## fuzz one target: make fuzz-one PKG=./internal/quote FUZZ_TARGET=FuzzParseCanonical
	$(FZ) -run '^$$' -fuzz $(FUZZ_TARGET) -fuzztime $(FUZZ_TIME) -parallel $(FUZZ_PARALLEL) $(PKG)

mutate: ## mutation-test MUTATE_PKGS, every surviving mutant fails the build
	$(CGO_ENV) GOMEMLIMIT=$(MEMSOFT) go run ./tools/mutator -packages $(MUTATE_PKGS) -tags sqlite_fts5 -parallel $(MUTATE_PARALLEL)

memtest: $(WRAP) ## hermetic pipeline + merged e2e under a hard GOMEMLIMIT
	$(CGO_ENV) GOMEMLIMIT=$(GOMEMLIMIT) $(WRAP) go test $(TAGS) -count=1 -run 'TestPipelineMemoryBounded|TestE2EHermetic|TestMergedHermeticE2E' ./internal/cli ./internal/e2e -v -timeout 15m

e2e-hermetic: $(WRAP) ## merged-app hermetic e2e (internal/e2e + the cli hermetic pass)
	$(CGO_ENV) $(WRAP) go test $(TAGS) -count=1 ./internal/e2e -v -timeout 15m
	$(CGO_ENV) $(WRAP) go test $(TAGS) -count=1 ./internal/cli -run 'TestE2EHermetic' -v -timeout 10m

e2e: $(WRAP) ## live end-to-end pipeline (network, QM_E2E_LIVE gate)
	QM_E2E_LIVE=1 $(CGO_ENV) $(WRAP) go test $(TAGS) -count=1 ./internal/cli -run 'TestE2ELive' -v -timeout $(E2E_TIMEOUT)

verify: test memtest fmt lint typecheck sloc fuzz mutate e2e-hermetic ## run the full quality chain, fail-fast

vendor-assets: ## verify or refresh the vendored htmx/cytoscape pins in web/static
	@set -e; \
	trap 'rm -f web/static/*.tmp' EXIT; \
	vendor_one() { \
	  pkg=$$1; pinned=$$2; sha=$$3; file=$$4; ver_name=$$5; sha_name=$$6; pinned_url=$$7; \
	  if tags=$$(curl -fsSL $(NPM_REGISTRY)/-/package/$$pkg/dist-tags 2>/dev/null); then \
	    newest=$$(printf '%s\n' "$$tags" | sed 's/[^:]*:"\([^"]*\)"/\1\n/g' | grep -E '^[0-9]' | grep -v -- '-' | sort -V | tail -1); \
	  else \
	    echo "$$pkg: registry unreachable, verifying the local pin only"; \
	    newest=; \
	  fi; \
	  if [ -n "$$newest" ] && [ "$$newest" != "$$pinned" ] && [ "$$(printf '%s\n%s\n' "$$pinned" "$$newest" | sort -V | tail -1)" = "$$newest" ]; then \
	    curl -fsSL "$(UNPKG)/$$pkg@$$newest/dist/$$file" -o web/static/$$file.tmp; \
	    if head -c 1 web/static/$$file.tmp | grep -q '<'; then \
	      echo "$$pkg: $(UNPKG)/$$pkg@$$newest/dist/$$file returned HTML, refusing to move the pin"; \
	      exit 1; \
	    fi; \
	    mv web/static/$$file.tmp web/static/$$file; \
	    sum=$$(sha256sum web/static/$$file | cut -d' ' -f1); \
	    sed -i "s/^$$ver_name ?=.*/$$ver_name ?= $$newest/" Makefile; \
	    sed -i "s/^$$sha_name ?=.*/$$sha_name ?= $$sum/" Makefile; \
	    grep -q "^$$ver_name ?= $$newest\$$" Makefile || { echo "$$pkg: pin rewrite failed"; exit 1; }; \
	    grep -q "^$$sha_name ?= $$sum\$$" Makefile || { echo "$$pkg: sha rewrite failed"; exit 1; }; \
	    echo "$$pkg: pin moved $$pinned -> $$newest (sha256 $$sum)"; \
	  else \
	    if ! echo "$$sha  web/static/$$file" | sha256sum -c --status -; then \
	      curl -fsSL "$$pinned_url" -o web/static/$$file.tmp; \
	      if head -c 1 web/static/$$file.tmp | grep -q '<'; then \
	        echo "$$pkg: $$pinned_url returned HTML, refusing to replace the pinned bytes"; \
	        exit 1; \
	      fi; \
	      mv web/static/$$file.tmp web/static/$$file; \
	    fi; \
	    echo "$$sha  web/static/$$file" | sha256sum -c -; \
	  fi; \
	}; \
	vendor_one htmx.org '$(HTMX_VERSION)' '$(HTMX_SHA256)' htmx.min.js HTMX_VERSION HTMX_SHA256 '$(HTMX_URL)'; \
	vendor_one cytoscape '$(CYTOSCAPE_VERSION)' '$(CYTOSCAPE_SHA256)' cytoscape.min.js CYTOSCAPE_VERSION CYTOSCAPE_SHA256 '$(CYTOSCAPE_URL)'

stage: build ## run one pipeline stage via the built binary: make stage STAGE=parse
	@if [ -z "$(STAGE)" ]; then echo "STAGE required: sync prune parse load triangulate export report all"; exit 2; fi
	./$(BIN) $(STAGE)

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
	$(CGO_ENV) go run ./tools/memcapmon -cap $(MEMCAP) -poll 2s -for $(MONITOR_FOR) -root $(CURDIR)
