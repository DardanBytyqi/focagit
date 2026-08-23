package git

import (
	"context"
	"path/filepath"
	"strings"
)

// Ignores reports whether git would ignore this path, for folders that appear later.
func (r *Repo) Ignores(ctx context.Context, path string) bool {
	// check-ignore exits 0 when the path is ignored and 1 when it is not.
	_, err := r.run.outputQuiet(ctx, "check-ignore", "-q", "--", path)
	return err == nil
}

// IgnoredDirs lists the ignored folders, absolute, so a watcher can leave them alone.
func (r *Repo) IgnoredDirs(ctx context.Context) ([]string, error) {
	// --directory collapses a wholly ignored folder to one entry instead of its contents.
	out, err := r.run.output(ctx, "ls-files", "--others", "--ignored",
		"--exclude-standard", "--directory", "--no-empty-directory", "-z")
	if err != nil {
		return nil, err
	}

	var dirs []string
	for _, entry := range strings.Split(string(out), "\x00") {
		// Only the entries git marked as folders; the rest are single ignored files.
		if strings.HasSuffix(entry, "/") {
			dirs = append(dirs, filepath.Join(r.root, entry))
		}
	}
	return dirs, nil
}
