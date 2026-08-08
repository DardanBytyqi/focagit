# focagit

A desktop Git client for Linux
Go - Wails - Vue

## Development

```bash
make test    # go test -race ./...
make dev     # Wails live reload
make build   # desktop binary → build/bin/focagit
make cli     # terminal harness over the git layer
```

## Requirements

Go 1.26+, git, Node 20+, and on Fedora:

```bash
sudo dnf install gtk3-devel webkit2gtk4.1-devel
```

Fedora 44 ships webkit2gtk 4.1 while Wails v2 defaults to 4.0, so builds need
`-tags webkit2_41`. The Makefile handles it.

