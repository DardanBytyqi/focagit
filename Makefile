# Fedora 44 ships webkit2gtk 4.1 only, while Wails v2 defaults to 4.0, so on
# Linux every wails command needs this build tag. macOS links the system WebKit
# and wants no tag at all, so the flag stays empty there.
ifeq ($(shell uname -s),Linux)
WAILS_FLAGS := -tags webkit2_41
endif
WAILS := $(shell go env GOPATH)/bin/wails

.PHONY: test cli dev build lint fmt tidy

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

lint:
	go vet ./...

fmt:
	gofmt -l -w .

tidy:
	go mod tidy
