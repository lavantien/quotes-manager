# quotes-manager — common commands. The SQLite driver needs CGO, so every Go
# command runs with CGO_ENABLED=1.
export CGO_ENABLED := 1

ADDR ?= :8080
DB   ?= database/quotes.db

.PHONY: help all test vet fmt run server extract seed tidy clean

help: ## show this help
	@awk 'BEGIN {FS = ":.*##"; printf "Usage: make <target>\n\nTargets:\n"} \
	      /^[a-zA-Z_-]+:.*##/ { printf "  %-10s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

all: test ## run the full test suite (default)

test: ## run the full test suite
	go test ./...

vet: ## go vet
	go vet ./...

fmt: ## gofmt all Go files
	gofmt -w .

run server: ## run the web server (http://localhost$(ADDR))
	go run ./cmd/server -addr $(ADDR) -db $(DB)

extract: ## regenerate database/seed.sql + exports/shortest-first.md from dumps/
	go run ./cmd/extract

seed: ## delete the database so the next 'make run' re-seeds it from seed.sql
	rm -f $(DB)
	@echo "database removed; the next 'make run' will re-seed it"

tidy: ## go mod tidy
	go mod tidy

clean: ## remove generated artifacts (database, binaries)
	rm -f $(DB) *.exe *.test *.out
