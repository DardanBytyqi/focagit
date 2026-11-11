package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// RebaseKind is what a rebase turned out to be, which decides what to show.
type RebaseKind string

const (
	RebaseUpToDate    RebaseKind = "up-to-date"
	RebaseFastForward RebaseKind = "fast-forward"
	RebaseDone        RebaseKind = "done"
	RebaseConflict    RebaseKind = "conflict"
)

// RebaseOptions describes one rebase of the current branch.
type RebaseOptions struct {
	// Onto is the branch or commit to replay the current branch on top of.
	Onto string `json:"onto"`
}

// RebaseResult is what a rebase left behind.
type RebaseResult struct {
	Kind      RebaseKind `json:"kind"`
	Conflicts []string   `json:"conflicts"`
	// Head is the commit HEAD ended on.
	Head string `json:"head"`
}

// RebaseState is how far an open rebase has got, for the UI to say so.
type RebaseState struct {
	// Branch is the branch being rebased, empty when the rebase began detached.
	Branch string `json:"branch"`
	// Onto names what the commits are being replayed on top of.
	Onto string `json:"onto"`
	// Step is the commit being replayed, 1-based, out of Steps.
	Step  int `json:"step"`
	Steps int `json:"steps"`
}

// Rebase replays the commits of the current branch on top of another ref.
func (r *Repo) Rebase(ctx context.Context, opts RebaseOptions) (RebaseResult, error) {
	onto := strings.TrimSpace(opts.Onto)
	if onto == "" || strings.HasPrefix(onto, "-") {
		return RebaseResult{}, fmt.Errorf("%q is not a commit or branch", onto)
	}
	target, err := r.resolve(ctx, onto)
	if err != nil {
		return RebaseResult{}, err
	}

	// As with merge, ancestry says what kind it is without reading git's prose.
	if r.isAncestor(ctx, target, "HEAD") {
		head, _ := r.resolve(ctx, "HEAD")
		return RebaseResult{Kind: RebaseUpToDate, Conflicts: []string{}, Head: head}, nil
	}
	fastForward := r.isAncestor(ctx, "HEAD", target)

	if _, err := r.run.outputQuiet(ctx, "rebase", onto); err != nil {
		// A conflict is an outcome rather than a failure: the rebase waits for us.
		if r.rebaseDir() != "" {
			if paths, perr := r.ConflictPaths(ctx); perr == nil && len(paths) > 0 {
				return RebaseResult{Kind: RebaseConflict, Conflicts: paths}, nil
			}
		}
		return RebaseResult{}, classifyRebase(err)
	}

	head, _ := r.resolve(ctx, "HEAD")
	kind := RebaseDone
	if fastForward {
		kind = RebaseFastForward
	}
	return RebaseResult{Kind: kind, Conflicts: []string{}, Head: head}, nil
}

// rebaseDir is where an open rebase keeps its state, for either backend; empty when none is open.
func (r *Repo) rebaseDir() string {
	merge := filepath.Join(r.gitDir, "rebase-merge")
	if _, err := os.Stat(merge); err == nil {
		return merge
	}
	apply := filepath.Join(r.gitDir, "rebase-apply")
	if _, err := os.Stat(filepath.Join(apply, "rebasing")); err == nil {
		return apply
	}
	return ""
}

// rebaseStopped is the commit an open rebase is replaying, empty between commits.
func (r *Repo) rebaseStopped(dir string) string {
	// REBASE_HEAD is written for a conflict; the backends' own files cover other stops.
	for _, f := range []string{
		filepath.Join(r.gitDir, "REBASE_HEAD"),
		filepath.Join(dir, "stopped-sha"),
		filepath.Join(dir, "original-commit"),
	} {
		if hash := readTrimmed(f); hash != "" {
			return hash
		}
	}
	return ""
}

// RebaseState reports how far the open rebase has got, nil when there is none.
func (r *Repo) RebaseState(ctx context.Context) *RebaseState {
	dir := r.rebaseDir()
	if dir == "" {
		return nil
	}
	st := &RebaseState{}

	// "detached HEAD" is what git writes when there was no branch to update.
	if head := readTrimmed(filepath.Join(dir, "head-name")); strings.HasPrefix(head, "refs/heads/") {
		st.Branch = strings.TrimPrefix(head, "refs/heads/")
	}
	if onto := readTrimmed(filepath.Join(dir, "onto")); onto != "" {
		st.Onto = r.nameCommit(ctx, onto)
	}

	// The merge backend counts in msgnum/end, the apply backend in next/last.
	step, steps := "msgnum", "end"
	if filepath.Base(dir) == "rebase-apply" {
		step, steps = "next", "last"
	}
	st.Step, _ = strconv.Atoi(readTrimmed(filepath.Join(dir, step)))
	st.Steps, _ = strconv.Atoi(readTrimmed(filepath.Join(dir, steps)))
	return st
}

// nameCommit labels a commit with a branch that points at it, or its short hash.
func (r *Repo) nameCommit(ctx context.Context, hash string) string {
	out, err := r.run.outputQuiet(ctx, "name-rev", "--name-only",
		"--refs=refs/heads/*", "--refs=refs/remotes/*", hash)
	if name := strings.TrimSpace(string(out)); err == nil && name != "" && name != "undefined" {
		return strings.TrimPrefix(name, "remotes/")
	}
	if len(hash) > 7 {
		return hash[:7]
	}
	return hash
}

// readTrimmed reads one of git's single-value state files, empty when absent.
func readTrimmed(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// classifyRebase turns git's refusals into sentinels the UI can react to.
func classifyRebase(err error) error {
	var ce *CommandError
	if !errors.As(err, &ce) {
		return err
	}
	switch {
	case strings.Contains(ce.Stderr, "cannot rebase: You have unstaged changes"),
		strings.Contains(ce.Stderr, "cannot rebase: Your index contains uncommitted changes"):
		return fmt.Errorf("%w: %s", ErrWouldOverwrite, ce.Stderr)
	}
	return classifyMerge(err)
}
