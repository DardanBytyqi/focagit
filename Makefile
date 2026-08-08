# Fedora 44 ships webkit2gtk 4.1 only, while Wails v2 defaults to 4.0.
# Every wails command therefore needs this build tag.
WAILS_TAGS := webkit2_41
WAILS      := $(shell go env GOPATH)/bin/wails

.PHONY: test cli dev build lint fmt tidy

## test: run the whole suite with the race detector
test:
	go test -race ./...

## cli: build the stage 1 terminal harness
cli:
	go build -o bin/focagit ./cmd/focagit

## dev: live-reloading Wails app (from stage 2 on)
dev:
	$(WAILS) dev -tags $(WAILS_TAGS)

## build: production desktop binary (from stage 2 on)
build:
	$(WAILS) build -tags $(WAILS_TAGS)

lint:
	go vet ./...

fmt:
	gofmt -l -w .

tidy:
	go mod tidy
