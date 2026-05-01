# Makefile for the JSON-file tl fork

.PHONY: all build test test-cover smoke clean install help

all: build

build:
	@echo "Building tl..."
	go build -buildvcs=false -o tl ./cmd/tl

test:
	@echo "Running tests..."
	GOFLAGS=-buildvcs=false go test ./... -count=1

test-cover:
	@echo "Running tests with coverage..."
	GOFLAGS=-buildvcs=false go test ./... -covermode=atomic -coverprofile=/tmp/task-ledger.coverage.out -count=1
	go tool cover -func=/tmp/task-ledger.coverage.out | tail -1

smoke: build
	@tmp=$$(mktemp -d); \
	cd "$$tmp"; \
	$(CURDIR)/tl init --quiet; \
	fr=$$($(CURDIR)/tl create FR "Smoke feature" --description "Smoke" --json --silent | jq -r .id); \
	ep=$$($(CURDIR)/tl create epic "Smoke epic" --parent "$$fr" --description "Smoke" --json --silent | jq -r .id); \
	tk=$$($(CURDIR)/tl create task "Smoke task" --parent "$$ep" --description "Smoke" --json --silent | jq -r .id); \
	$(CURDIR)/tl show "$$tk" --json | jq -e '.issue_type == "task"' >/dev/null; \
	echo "Smoke OK in $$tmp"

install:
	@echo "Installing tl to $$(go env GOPATH)/bin..."
	GOFLAGS=-buildvcs=false go install ./cmd/tl

clean:
	@echo "Cleaning..."
	rm -f tl

help:
	@echo "tl JSON-file fork targets:"
	@echo "  make build       - Build ./tl"
	@echo "  make test        - Run all Go tests"
	@echo "  make test-cover  - Run tests and print total coverage"
	@echo "  make smoke       - Build and run a small CLI smoke test (requires jq)"
	@echo "  make install     - Install tl with go install"
	@echo "  make clean       - Remove local build artifacts"
