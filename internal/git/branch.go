package git

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrWouldOverwrite means the working tree has changes the operation would destroy.
var ErrWouldOverwrite = errors.New("local changes would be overwritten")

// ErrBranchNotMerged means the branch still holds commits that are nowhere else.
var ErrBranchNotMerged = errors.New("branch is not fully merged")

// ErrBranchExists means a branch of that name is already there.
var ErrBranchExists = errors.New("branch already exists")

// ErrInvalidBranchName means git itself rejected the name.
var ErrInvalidBranchName = errors.New("invalid branch name")

// Checkout switches the working tree to a local branch.
func (r *Repo) Checkout(ctx context.Context, branch string) error {
	// `switch` refuses to detach HEAD by accident, which `checkout` does silently.
	_, err := r.run.outputQuiet(ctx, "switch", "--no-guess", "--", branch)
	return classifyBranch(err, branch)
}

// CheckoutRemote checks out origin/foo as a local branch foo tracking it.
func (r *Repo) CheckoutRemote(ctx context.Context, remoteRef string) error {
	local := localNameFor(remoteRef)
	if local == "" {
		return fmt.Errorf("%q is not a remote branch", remoteRef)
	}
	// The local branch may already exist from an earlier checkout; just switch to it.
	if r.HasBranch(ctx, local) {
		return r.Checkout(ctx, local)
	}

	_, err := r.run.outputQuiet(ctx, "switch", "--track", "--", remoteRef)
	return classifyBranch(err, local)
}

// CheckoutDetached moves HEAD to a commit without being on a branch.
func (r *Repo) CheckoutDetached(ctx context.Context, rev string) error {
	_, err := r.run.outputQuiet(ctx, "switch", "--detach", rev)
	return classifyBranch(err, rev)
}

// CreateBranchOptions describes a branch to create.
type CreateBranchOptions struct {
	Name string `json:"name"`
	// Start is the commit or ref to branch from; empty means HEAD.
	Start string `json:"start"`
	// Checkout switches to the new branch instead of only creating it.
	Checkout bool `json:"checkout"`
	// Track sets the new branch's upstream to Start.
	Track bool `json:"track"`
}

// CreateBranch creates a branch and optionally switches to it.
func (r *Repo) CreateBranch(ctx context.Context, opts CreateBranchOptions) error {
	name := strings.TrimSpace(opts.Name)
	if err := r.CheckBranchName(ctx, name); err != nil {
		return err
	}

	args := []string{"branch"}
	if opts.Checkout {
		args = []string{"switch"}
	}
	if opts.Track {
		args = append(args, "--track")
	}
	// --create=<name> and -- keep a name starting with "-" from being read as a flag.
	if opts.Checkout {
		args = append(args, "--create="+name)
	} else {
		args = append(args, "--", name)
	}
	if opts.Start != "" {
		args = append(args, opts.Start)
	}

	_, err := r.run.outputQuiet(ctx, args...)
	return classifyBranch(err, name)
}

// DeleteBranch removes a local branch; force discards unmerged commits.
func (r *Repo) DeleteBranch(ctx context.Context, name string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	_, err := r.run.outputQuiet(ctx, "branch", flag, "--", name)
	return classifyBranch(err, name)
}

// RenameBranch renames a local branch.
func (r *Repo) RenameBranch(ctx context.Context, from, to string) error {
	to = strings.TrimSpace(to)
	if err := r.CheckBranchName(ctx, to); err != nil {
		return err
	}
	_, err := r.run.outputQuiet(ctx, "branch", "-m", "--", from, to)
	return classifyBranch(err, to)
}

// HasBranch reports whether a local branch of that name exists.
func (r *Repo) HasBranch(ctx context.Context, name string) bool {
	_, err := r.run.outputQuiet(ctx, "show-ref", "--verify", "--quiet", "refs/heads/"+name)
	return err == nil
}

// CheckBranchName asks git whether the name is usable, so our rules cannot drift.
func (r *Repo) CheckBranchName(ctx context.Context, name string) error {
	// check-ref-format accepts a leading dash, but every porcelain command refuses it.
	if name == "" || strings.HasPrefix(name, "-") {
		return fmt.Errorf("%w: %q", ErrInvalidBranchName, name)
	}
	if _, err := r.run.outputQuiet(ctx, "check-ref-format", "refs/heads/"+name); err != nil {
		return fmt.Errorf("%w: %q", ErrInvalidBranchName, name)
	}
	return nil
}

// localNameFor turns "origin/feature/x" into "feature/x".
func localNameFor(remoteRef string) string {
	_, rest, found := strings.Cut(remoteRef, "/")
	if !found {
		return ""
	}
	return rest
}

// classifyBranch turns git's prose refusals into sentinels the UI can react to.
func classifyBranch(err error, name string) error {
	var ce *CommandError
	if !errors.As(err, &ce) {
		return err
	}

	switch {
	case strings.Contains(ce.Stderr, "would be overwritten"):
		return fmt.Errorf("%w: %s", ErrWouldOverwrite, ce.Stderr)
	case strings.Contains(ce.Stderr, "not fully merged"):
		return fmt.Errorf("%s: %w", name, ErrBranchNotMerged)
	case strings.Contains(ce.Stderr, "already exists"):
		return fmt.Errorf("%s: %w", name, ErrBranchExists)
	case strings.Contains(ce.Stderr, "is not a valid branch name"):
		return fmt.Errorf("%w: %q", ErrInvalidBranchName, name)
	}
	return err
}
