package git

import (
	"context"
	"errors"
	"strings"
)

// This file holds the first operations that WRITE to the repository.

// Stage adds whole paths to the index.
func (r *Repo) Stage(ctx context.Context, paths ...string) error {
	if len(paths) == 0 {
		return nil
	}
	args := append([]string{"add", "--"}, paths...)
	_, err := r.run.output(ctx, args...)
	return err
}

// Unstage restores paths in the index from HEAD.
func (r *Repo) Unstage(ctx context.Context, paths ...string) error {
	if len(paths) == 0 {
		return nil
	}

	args := append([]string{"restore", "--staged", "--"}, paths...)
	_, err := r.run.output(ctx, args...)
	if err == nil || r.hasHEAD(ctx) {
		return err
	}

	// Before the first commit there is nothing to restore from, so drop the
	// paths out of the index instead.
	args = append([]string{"rm", "--cached", "-r", "--"}, paths...)
	_, err = r.run.output(ctx, args...)
	return err
}

// hasHEAD reports whether the repository has at least one commit.
func (r *Repo) hasHEAD(ctx context.Context) bool {
	_, err := r.run.output(ctx, "rev-parse", "--verify", "--quiet", "HEAD")
	return err == nil
}

// ApplyPatch feeds a patch to `git apply --cached`.
func (r *Repo) ApplyPatch(ctx context.Context, patch string, reverse bool) error {
	args := []string{"apply", "--cached"}
	if reverse {
		args = append(args, "--reverse")
	}
	// Patches assembled from a selection can end up with less context than git produced.
	args = append(args, "--unidiff-zero", "-")

	_, err := r.run.input(ctx, patch, args...)
	return err
}

// ApplyPatchWorktree applies a patch to the working tree rather than the index.
func (r *Repo) ApplyPatchWorktree(ctx context.Context, patch string, reverse bool) error {
	args := []string{"apply"}
	if reverse {
		args = append(args, "--reverse")
	}
	args = append(args, "--unidiff-zero", "-")

	_, err := r.run.input(ctx, patch, args...)
	return err
}

// Discard throws away working tree changes for paths, restoring them from the index.
func (r *Repo) Discard(ctx context.Context, paths ...string) error {
	if len(paths) == 0 {
		return nil
	}
	args := append([]string{"restore", "--worktree", "--"}, paths...)
	_, err := r.run.output(ctx, args...)
	return err
}

// CommitOptions describes a commit to create.
type CommitOptions struct {
	// Message is the full commit message, subject and body together.
	Message string
	Amend   bool
	// AllowEmpty permits a commit that changes nothing.
	AllowEmpty bool
}

// Commit records the staged changes.
func (r *Repo) Commit(ctx context.Context, opts CommitOptions) error {
	if strings.TrimSpace(opts.Message) == "" {
		return errors.New("commit message is empty")
	}

	args := []string{"commit", "-F", "-"}
	if opts.Amend {
		args = append(args, "--amend")
	}
	if opts.AllowEmpty {
		args = append(args, "--allow-empty")
	}

	_, err := r.run.input(ctx, opts.Message, args...)
	return err
}

// UntrackedDiff renders a file git is not tracking as if every line had just been added.
func (r *Repo) UntrackedDiff(ctx context.Context, path string) (FileDiff, error) {
	// Exit 1 means "they differ", which against /dev/null is always true.
	out, err := r.run.outputOK(ctx, []int{1}, "diff", "--no-index", "--", "/dev/null", path)
	if err != nil {
		return FileDiff{}, err
	}

	files, perr := parseDiff(out)
	if perr != nil {
		return FileDiff{}, perr
	}
	if len(files) == 0 {
		return FileDiff{Path: path}, nil
	}

	d := files[0]
	// --no-index writes the path as given rather than repository-relative.
	d.Path = path
	d.OldPath = ""
	return d, nil
}
