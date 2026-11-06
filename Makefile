SHELL := /bin/bash
.DEFAULT_GOAL := check

ROOT := $(CURDIR)
TOOLCHAIN := $(ROOT)/.toolchain
GO := $(TOOLCHAIN)/go/bin/go
NODE := $(TOOLCHAIN)/node/bin/node
NPM := $(TOOLCHAIN)/node/bin/npm

export PATH := $(TOOLCHAIN)/go/bin:$(TOOLCHAIN)/node/bin:$(TOOLCHAIN)/bin:$(PATH)
export GOTOOLCHAIN := local
export GOCACHE := $(TOOLCHAIN)/cache/go-build
export GOMODCACHE := $(TOOLCHAIN)/cache/go-mod
export npm_config_cache := $(TOOLCHAIN)/cache/npm

.PHONY: bootstrap tools fmt lint vet test check build web-build run seed stage-a calibrate clean

bootstrap:
	bash scripts/bootstrap.sh

tools: bootstrap
	bash scripts/tools.sh

fmt:
	bash scripts/gofmt.sh -w
	$(GO) mod tidy

lint: tools
	bash scripts/lint.sh

vet: bootstrap
	$(GO) vet ./...

test: bootstrap
	$(GO) test ./...

check: bootstrap tools
	bash scripts/gofmt.sh -l
	$(GO) vet ./...
	bash scripts/lint.sh
	bash scripts/vuln.sh
	$(GO) test ./... -race

build: web-build
	mkdir -p bin
	$(GO) build -trimpath -o bin/asterwaf ./cmd/asterwaf

web-build: bootstrap
	@if [ -f web/package.json ]; then \
		cd web && $(NPM) ci --no-audit --no-fund && $(NPM) run build; \
	else \
		echo "web-build: no web/package.json, keeping committed placeholder"; \
	fi

run: bootstrap
	$(GO) run ./cmd/asterwaf -config config.yaml

seed: bootstrap
	$(GO) run ./cmd/asterwaf seed -config config.yaml

stage-a: bootstrap
	bash scripts/stage-a.sh

calibrate: bootstrap
	$(GO) test -tags calibration ./internal/calibration -count=1 -v

clean:
	rm -rf bin
