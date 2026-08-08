package git

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
)

// Repo is a handle on one git working tree.
type Repo struct {
	root   string
	gitDir string
	run    runner
}

// config holds what Open's options can change.
type config struct {
	log *slog.Logger
}

// Option customises Open.
type Option func(*config)

// WithLogger routes this repository's git invocations to a specific logger.
func WithLogger(l *slog.Logger) Option {
	return func(c *config) {
		if l != nil {
			c.log = l
		}
	}
}

// Open resolves path to the working tree that contains it.
func Open(ctx context.Context, path string, opts ...Option) (*Repo, error) {
	cfg := config{log: slog.Default()}
	for _, o := range opts {
		o(&cfg)
	}

	bin, err := exec.LookPath("git")
	if err != nil {
		return nil, fmt.Errorf("git binary not found in PATH: %w", err)
	}

	r := runner{bin: bin, dir: path, log: cfg.log}
	// Quiet: probing a plain folder is a question, not a failure.
	out, err := r.outputQuiet(ctx, "rev-parse", "--show-toplevel", "--absolute-git-dir")
	if err != nil {
		var ce *CommandError
		// git exits 128 with "not a git repository" for anything outside a working tree.
		if ok := asCommandError(err, &ce); ok && strings.Contains(ce.Stderr, "not a git repository") {
			return nil, fmt.Errorf("%s: %w", path, ErrNotARepository)
		}
		cfg.log.Error("git failed", slog.String("cmd", "rev-parse"), slog.String("err", err.Error()))
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 2 {
		return nil, fmt.Errorf("git rev-parse: expected 2 lines, got %d", len(lines))
	}

	root := lines[0]
	cfg.log.Info("repository opened",
		slog.String("root", root),
		slog.String("gitdir", lines[1]),
	)

	return &Repo{
		root:   root,
		gitDir: lines[1],
		// Re-anchor at the working tree root so every later command runs from the same place.
		run: runner{bin: bin, dir: root, log: cfg.log},
	}, nil
}

// Root is the absolute path of the working tree.
func (r *Repo) Root() string { return r.root }

// GitDir is the absolute path of the .git directory.
func (r *Repo) GitDir() string { return r.gitDir }

func asCommandError(err error, target **CommandError) bool {
	ce, ok := err.(*CommandError)
	if ok {
		*target = ce
	}
	return ok
}

// Init creates a repository in dir, honouring the user's init.defaultBranch.
func Init(ctx context.Context, dir string, opts ...Option) error {
	cfg := config{log: slog.Default()}
	for _, o := range opts {
		o(&cfg)
	}

	bin, err := exec.LookPath("git")
	if err != nil {
		return fmt.Errorf("git binary not found in PATH: %w", err)
	}

	r := runner{bin: bin, dir: dir, log: cfg.log}
	_, err = r.output(ctx, "init")
	return err
}
