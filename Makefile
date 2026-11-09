# Enchanted Waters - build helpers
#
# The mandatory build of the project is:
#
#     CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build
#
# which is what the `build` target does.

GO      ?= go
BINARY  ?= enchantedwaters
GOFLAGS ?=
LDFLAGS ?= -s -w

# The build is always static: CGO is disabled and no external library is
# linked by the Go code.
export CGO_ENABLED = 0

.PHONY: all build build-debug test vet fmt clean validate run import-check

all: build

## build: the release binary.
build:
	$(GO) build $(GOFLAGS) -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/

## build-debug: the binary with the ImGui debug panel.
build-debug:
	$(GO) build $(GOFLAGS) -tags debugui -trimpath -o $(BINARY)-debug ./cmd/

## test: the whole test suite.
test:
	$(GO) test ./...

## vet: the static analysis of the Go code.
vet:
	$(GO) vet ./...

## fmt: the formatting of the Go code.
fmt:
	$(GO) fmt ./...

## validate: the check of the configuration tables.
validate: build
	./$(BINARY) validate

## run: the playable game.
run: build
	./$(BINARY) run

## clean: the removal of the build artefacts.
clean:
	rm -f $(BINARY) $(BINARY)-debug
	$(GO) clean -cache -testcache

## import-check: the dry run of the importer over the raw directory.
import-check: build
	./$(BINARY) import --raw=assets/raw --out=assets/config
