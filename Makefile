# Fedora 44 ships webkit2gtk 4.1 only, while Wails v2 defaults to 4.0, so on
# Linux every wails command needs this build tag. macOS links the system WebKit
# and wants no tag at all, so the flag stays empty there.
ifeq ($(shell uname -s),Linux)
WAILS_FLAGS := -tags webkit2_41
endif
WAILS := $(shell go env GOPATH)/bin/wails

.PHONY: test cli dev build package lint fmt tidy

## test: run the whole suite with the race detector
test:
	go test -race ./...

## cli: build the stage 1 terminal harness
cli:
	go build -o bin/focagit ./cmd/focagit

## dev: live-reloading Wails app (from stage 2 on)
dev:
	$(WAILS) dev $(WAILS_FLAGS)

## build: production desktop binary (from stage 2 on)
build:
	$(WAILS) build $(WAILS_FLAGS)

# The latest tag without its v, so a tagged commit packages as its own version.
VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || true)
NFPM := $(shell go env GOPATH)/bin/nfpm

## package: .deb, .rpm and .tar.gz of the Linux build in dist/
package: build
	mkdir -p dist
	VERSION=$(or $(VERSION),0.0.0) $(NFPM) pkg -f build/linux/nfpm.yaml -p deb -t dist/
	VERSION=$(or $(VERSION),0.0.0) $(NFPM) pkg -f build/linux/nfpm.yaml -p rpm -t dist/
	tar -czf dist/focagit-$(or $(VERSION),0.0.0)-linux-amd64.tar.gz -C build/bin focagit -C ../linux focagit.desktop -C .. appicon.svg

lint:
	go vet ./...

fmt:
	gofmt -l -w .

tidy:
	go mod tidy
