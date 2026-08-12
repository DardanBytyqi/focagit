package git

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	// ErrDirNotEmpty means the destination folder already holds something.
	ErrDirNotEmpty = errors.New("the destination folder is not empty")
	// ErrInvalidURL means no repository address could be read from the input.
	ErrInvalidURL = errors.New("invalid repository url")
	// ErrCancelled means the user stopped the operation before it finished.
	ErrCancelled = errors.New("cancelled")
)

// CloneOptions says what to copy and where to put it.
type CloneOptions struct {
	URL string `json:"url"`
	// Parent is the folder the clone is created in.
	Parent string `json:"parent"`
	// Name is the folder to create; empty takes the name from the URL.
	Name string `json:"name"`
	// Recursive clones the submodules too.
	Recursive bool `json:"recursive"`
}

// ClonePreview is where a clone would land, for the dialog to show before it runs.
type ClonePreview struct {
	Name string `json:"name"`
	Path string `json:"path"`
	// Taken is true when something is already there, so the clone would refuse.
	Taken bool `json:"taken"`
}

// PreviewClone answers where these options would put the repository.
func PreviewClone(opts CloneOptions) ClonePreview {
	name := strings.TrimSpace(opts.Name)
	if name == "" {
		name = RepoNameFromURL(opts.URL)
	}
	parent := strings.TrimSpace(opts.Parent)
	if !validName(name) || parent == "" {
		return ClonePreview{}
	}

	dest := filepath.Join(parent, name)
	_, err := emptyDir(dest)
	return ClonePreview{Name: name, Path: dest, Taken: errors.Is(err, ErrDirNotEmpty)}
}

// RepoNameFromURL is the folder name git would pick for this address.
func RepoNameFromURL(raw string) string {
	s := strings.TrimRight(strings.TrimSpace(raw), "/")
	s = strings.TrimSuffix(s, "/.git")
	s = strings.TrimSuffix(s, ".git")
	s = strings.TrimRight(s, "/")

	// Drop the scheme and the host first, or a bare address would name the folder.
	if i := strings.Index(s, "://"); i >= 0 {
		_, s, _ = strings.Cut(s[i+3:], "/")
	} else if i := strings.Index(s, "@"); i >= 0 && !strings.HasPrefix(s, "/") {
		// The scp form, where the path follows the colon.
		_, s, _ = strings.Cut(s[i+1:], ":")
	}
	if i := strings.LastIndexAny(s, "/:"); i >= 0 {
		s = s[i+1:]
	}
	if !validName(s) {
		return ""
	}
	return s
}

// validName refuses anything that would put the clone somewhere else.
func validName(s string) bool {
	if s == "" || s == "." || s == ".." || strings.HasPrefix(s, "-") {
		return false
	}
	return !strings.ContainsAny(s, `/\`)
}

// Clone copies a repository into opts.Parent and returns the new working tree.
func Clone(ctx context.Context, opts CloneOptions, onProgress func(string), o ...Option) (string, error) {
	cfg := config{log: slog.Default()}
	for _, f := range o {
		f(&cfg)
	}

	url := strings.TrimSpace(opts.URL)
	// A leading dash would be read as an option rather than an address.
	if url == "" || strings.HasPrefix(url, "-") {
		return "", fmt.Errorf("%w: %q", ErrInvalidURL, opts.URL)
	}

	name := strings.TrimSpace(opts.Name)
	if name == "" {
		name = RepoNameFromURL(url)
	}
	if !validName(name) {
		return "", fmt.Errorf("%w: no folder name in %q", ErrInvalidURL, url)
	}

	parent := strings.TrimSpace(opts.Parent)
	if st, err := os.Stat(parent); err != nil || !st.IsDir() {
		return "", fmt.Errorf("%s: not a folder to clone into", parent)
	}

	dest := filepath.Join(parent, name)
	existed, err := emptyDir(dest)
	if err != nil {
		return "", err
	}

	bin, err := exec.LookPath("git")
	if err != nil {
		return "", fmt.Errorf("git binary not found in PATH: %w", err)
	}

	args := []string{"clone", "--progress"}
	if opts.Recursive {
		args = append(args, "--recurse-submodules")
	}
	args = append(args, "--", url, name)

	r := runner{bin: bin, dir: parent, log: cfg.log}
	if _, err := r.progress(ctx, onProgress, args...); err != nil {
		// git leaves a half-clone behind, and that would block the next attempt.
		os.RemoveAll(dest)
		if existed {
			_ = os.Mkdir(dest, 0o755)
		}
		if ctx.Err() != nil {
			return "", ErrCancelled
		}
		return "", classifyClone(err)
	}
	return dest, nil
}

// classifyClone blames the address, since here it is the thing the user typed.
func classifyClone(err error) error {
	var ce *CommandError
	if errors.As(err, &ce) && strings.Contains(ce.Stderr, "does not appear to be a git repository") {
		return fmt.Errorf("%w: no repository at that address", ErrInvalidURL)
	}
	return classifyRemote(err)
}

// emptyDir reports whether dir already exists, and refuses one holding files.
func emptyDir(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if len(entries) > 0 {
		return true, fmt.Errorf("%s: %w", dir, ErrDirNotEmpty)
	}
	return true, nil
}
