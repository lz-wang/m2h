# =============================================================================
# m2h Makefile
# =============================================================================

.DEFAULT_GOAL := help

.PHONY: \
	build build-all build-os dist \
	test coverage check format setup upgrade clean version help ci \
	web-install web-ci-install web-lint web-build web-test web-format \
	_build-platform _install-go-tools _check-go-format _check-go-mod

BINARY_NAME := m2h
MAIN_PATH := .
DIST_DIR := dist
WEB_DIR := web
COVERAGE_OUT := coverage.out
COVERAGE_HTML := coverage.html

GO := go
NPM := npm
TAR := tar
ZIP := zip
GOIMPORTS_REVISER := goimports-reviser
GOIMPORTS_REVISER_VERSION := v3.12.6

PLATFORMS := \
	linux/amd64 \
	linux/arm64 \
	darwin/amd64 \
	darwin/arm64 \
	windows/amd64 \
	windows/arm64

COMMIT_ID := $(shell git rev-parse --short=7 HEAD 2>/dev/null || echo unknown)
COMMIT_DATE := $(shell git show -s --format=%cd --date=format:%Y%m%d HEAD 2>/dev/null || echo unknown)
EXACT_TAG := $(shell git describe --tags --exact-match --match 'v[0-9]*.[0-9]*.[0-9]*' HEAD 2>/dev/null || true)
TAG_VERSION := $(patsubst v%,%,$(EXACT_TAG))

VERSION ?=
ifeq ($(strip $(VERSION)),)
ifneq ($(strip $(EXACT_TAG)),)
M2H_VERSION := $(TAG_VERSION)
else ifeq ($(COMMIT_ID),unknown)
M2H_VERSION := dev-unknown-unknown
else
M2H_VERSION := dev-$(COMMIT_DATE)-$(COMMIT_ID)
endif
else
M2H_VERSION := $(VERSION)
endif

VERSION_VAR := main.M2HVersion
GO_BUILD_FLAGS := -trimpath -buildvcs=false
GO_LDFLAGS := -X $(VERSION_VAR)=$(M2H_VERSION)
GO_RELEASE_LDFLAGS := -s -w $(GO_LDFLAGS)
HOST_GOEXE := $(shell $(GO) env GOEXE 2>/dev/null)
HOST_BINARY := $(BINARY_NAME)$(HOST_GOEXE)

## Build m2h for the current platform; depends on web-build.
build: web-build
	@echo "[m2h] build $(M2H_VERSION) -> ./$(HOST_BINARY)"
	@CGO_ENABLED=0 $(GO) build \
		$(GO_BUILD_FLAGS) \
		-ldflags "$(GO_LDFLAGS)" \
		-o "./$(HOST_BINARY)" \
		$(MAIN_PATH)

## Cross-compile six platform binaries into dist/.
build-all: web-build
	@set -eu; \
	for platform in $(PLATFORMS); do \
		os=$${platform%/*}; \
		arch=$${platform#*/}; \
		$(MAKE) --no-print-directory _build-platform OS="$$os" ARCH="$$arch"; \
	done

## Build one platform, for example: make build-os OS=linux ARCH=arm64.
build-os: web-build
	@$(MAKE) --no-print-directory _build-platform OS="$(OS)" ARCH="$(ARCH)"

_build-platform:
	@if [ -z "$(OS)" ] || [ -z "$(ARCH)" ]; then \
		echo "Usage: make build-os OS=linux ARCH=amd64"; \
		exit 2; \
	fi
	@target="$(OS)/$(ARCH)"; \
	case " $(PLATFORMS) " in \
		*" $$target "*) ;; \
		*) \
			echo "Error: unsupported platform $$target"; \
			exit 2; \
			;; \
	esac
	@mkdir -p "$(DIST_DIR)"
	@ext=""; \
	if [ "$(OS)" = "windows" ]; then ext=".exe"; fi; \
	output="$(DIST_DIR)/$(BINARY_NAME)_$(OS)_$(ARCH)$$ext"; \
	echo "[m2h] build $(OS)/$(ARCH) -> $$output"; \
	CGO_ENABLED=0 GOOS="$(OS)" GOARCH="$(ARCH)" \
		$(GO) build \
			$(GO_BUILD_FLAGS) \
			-ldflags "$(GO_RELEASE_LDFLAGS)" \
			-o "$$output" \
			$(MAIN_PATH)

## Create six distribution archives and checksums in dist/.
dist: clean build-all
	@command -v "$(TAR)" >/dev/null 2>&1 || { echo "Error: tar is required"; exit 2; }
	@command -v "$(ZIP)" >/dev/null 2>&1 || { echo "Error: zip is required"; exit 2; }
	@if ! command -v sha256sum >/dev/null 2>&1 && ! command -v shasum >/dev/null 2>&1; then \
		echo "Error: sha256sum or shasum is required"; \
		exit 2; \
	fi
	@set -eu; \
	stage="$(DIST_DIR)/.stage"; \
	mkdir -p "$$stage"; \
	for platform in $(PLATFORMS); do \
		os=$${platform%/*}; \
		arch=$${platform#*/}; \
		ext=""; \
		if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
		binary="$(DIST_DIR)/$(BINARY_NAME)_$${os}_$${arch}$$ext"; \
		name="$(BINARY_NAME)_$(M2H_VERSION)_$${os}_$${arch}"; \
		package_dir="$$stage/$$name"; \
		mkdir -p "$$package_dir"; \
		cp "$$binary" "$$package_dir/$(BINARY_NAME)$$ext"; \
		cp README.md LICENSE "$$package_dir/"; \
		if [ "$$os" = "windows" ]; then \
			(cd "$$package_dir" && $(ZIP) -qr "$(abspath $(DIST_DIR))/$$name.zip" .); \
		else \
			$(TAR) -C "$$package_dir" -czf "$(DIST_DIR)/$$name.tar.gz" .; \
		fi; \
		rm -f "$$binary"; \
	done; \
	rm -rf "$$stage"
	@cd "$(DIST_DIR)" && { \
		for file in *.tar.gz *.zip; do \
			[ -f "$$file" ] || continue; \
			if command -v sha256sum >/dev/null 2>&1; then \
				sha256sum "$$file"; \
			else \
				shasum -a 256 "$$file"; \
			fi; \
		done; \
	} > "checksums.txt"
	@echo "[m2h] distributions -> $(DIST_DIR)/"

## Run Go and Web tests.
test:
	@echo "[m2h] test Go"
	@$(GO) test -timeout 30s ./...
	@$(MAKE) --no-print-directory web-test

## Generate Go coverage files for Codecov and local inspection.
coverage:
	@rm -f "$(COVERAGE_OUT)" "$(COVERAGE_HTML)"
	@$(GO) test -timeout 30s -covermode=atomic -coverprofile="$(COVERAGE_OUT)" ./...
	@$(GO) tool cover -html="$(COVERAGE_OUT)" -o "$(COVERAGE_HTML)"

## Run read-only static checks and tests.
check: _check-go-format _check-go-mod
	@$(GO) vet ./...
	@$(MAKE) --no-print-directory web-lint
	@$(MAKE) --no-print-directory test

_check-go-format:
	@$(GOIMPORTS_REVISER) \
		-rm-unused \
		-format \
		-imports-order std,general,company,project,blanked,dotted \
		-list-diff \
		-set-exit-status \
		./...

_check-go-mod:
	@$(GO) mod tidy -diff
	@$(GO) mod verify

## Format Go and Web sources.
format:
	@$(GOIMPORTS_REVISER) \
		-rm-unused \
		-format \
		-imports-order std,general,company,project,blanked,dotted \
		./...
	@$(MAKE) --no-print-directory web-format

## Install development tools and project dependencies.
setup: _install-go-tools web-install
	@$(GO) mod download

_install-go-tools:
	@$(GO) install github.com/incu6us/goimports-reviser/v3@$(GOIMPORTS_REVISER_VERSION)

## Install deterministic dependencies and run all checks in CI.
ci: _install-go-tools web-ci-install
	@$(GO) mod download
	@$(MAKE) --no-print-directory check

## Upgrade all Go and Web dependencies, then normalize manifests.
upgrade:
	@$(GO) get -u ./...
	@$(GO) mod tidy
	@cd "$(WEB_DIR)" && $(NPM) update

## Install Web dependencies locally.
web-install:
	@cd "$(WEB_DIR)" && $(NPM) install

## Install exact Web dependencies in CI.
web-ci-install:
	@cd "$(WEB_DIR)" && $(NPM) ci

## Check Web sources with Biome.
web-lint:
	@cd "$(WEB_DIR)" && $(NPM) run lint

## Build React WebUI for Go embedding.
web-build:
	@cd "$(WEB_DIR)" && $(NPM) run build

## Run Web tests with Vitest.
web-test:
	@cd "$(WEB_DIR)" && $(NPM) run test

## Format Web sources with Biome.
web-format:
	@cd "$(WEB_DIR)" && $(NPM) run format

## Remove binaries, logs, coverage files and dist/.
clean:
	@rm -f \
		"./$(BINARY_NAME)" \
		"./$(BINARY_NAME).exe" \
		"$(COVERAGE_OUT)" \
		"$(COVERAGE_HTML)" \
		./*.log \
		./__debug_bin
	@rm -rf ./build ./dist ./logs "$(WEB_DIR)/dist" "$(WEB_DIR)/coverage"

## Print the embedded version string.
version:
	@echo "$(M2H_VERSION)"

## Show available targets.
help:
	@echo "m2h Makefile"
	@echo ""
	@echo "Main:"
	@echo "  make build          Build m2h for the current platform"
	@echo "  make build-all      Cross-compile six platform binaries to dist/"
	@echo "  make build-os       Build one platform with OS= and ARCH="
	@echo "  make dist           Create six archives and checksums"
	@echo "  make test           Run Go and Web tests"
	@echo "  make format         Format Go and Web sources"
	@echo "  make setup          Install tools and dependencies"
	@echo "  make upgrade        Upgrade Go and Web dependencies"
	@echo "  make clean          Remove generated files"
	@echo "  make version        Print the embedded version"
	@echo "  make help           Show this help"
	@echo ""
	@echo "Web:"
	@echo "  make web-install    Install Web dependencies"
	@echo "  make web-lint       Check Web sources"
	@echo "  make web-build      Build WebUI"
	@echo "  make web-test       Run Web tests"
