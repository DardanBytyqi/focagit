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

// ErrNoOperation means there is nothing half-finished to continue or abort.
var ErrNoOperation = errors.New("no operation in progress")

// ErrAlreadyApplied means the change is already in this branch.
var ErrAlreadyApplied = errors.New("that change is already in this branch")

// ErrMainlineNeeded means a merge commit was given with no side to apply against.
var ErrMainlineNeeded = errors.New("a merge commit needs a side to apply against")

// Operation is one left open by a conflict; the values are git's own subcommands.
type Operation string

const (
	OpNone       Operation = ""
	OpMerge      Operation = "merge"
	OpRevert     Operation = "revert"
	OpCherryPick Operation = "cherry-pick"
	OpRebase     Operation = "rebase"
)

// opHeads is the file git leaves in .git for each one, most specific first.
var opHeads = []struct {
	op   Operation
	file string
}{
	{OpRevert, "REVERT_HEAD"},
	{OpCherryPick, "CHERRY_PICK_HEAD"},
	{OpMerge, "MERGE_HEAD"},
}

// Operation reports what is half-finished, and the commit it is applying.
func (r *Repo) Operation() (Operation, string) {
	// Asked first: a rebase is open as long as its directory is, even between commits.
	if dir := r.rebaseDir(); dir != "" {
		return OpRebase, r.rebaseStopped(dir)
	}
	for _, h := range opHeads {
		b, err := os.ReadFile(filepath.Join(r.gitDir, h.file))
		if err != nil {
			continue
		}
		if hash := strings.TrimSpace(string(b)); hash != "" {
			return h.op, hash
		}
	}
	return OpNone, ""
}

// operationFrom names what the pending operation is applying, for the UI to label.
func (r *Repo) operationFrom(ctx context.Context, op Operation, hash string) string {
	if op == OpMerge {
		return r.MergeSource(ctx)
	}
	if hash == "" {
		return ""
	}
	out, err := r.run.outputQuiet(ctx, "log", "-1", "--format=%h %s", hash)
	if line := strings.TrimSpace(string(out)); err == nil && line != "" {
		return line
	}
	if len(hash) > 7 {
		return hash[:7]
	}
	return hash
}

// ApplyKind is how applying a commit turned out.
type ApplyKind string

const (
	ApplyDone     ApplyKind = "done"
	ApplyConflict ApplyKind = "conflict"
)

// ApplyResult is what a revert or a cherry-pick left behind.
type ApplyResult struct {
	Kind      ApplyKind `json:"kind"`
	Conflicts []string  `json:"conflicts"`
	// Head is the commit HEAD ended on.
	Head string `json:"head"`
}

// ApplyOptions describes one revert or one cherry-pick.
type ApplyOptions struct {
	// Rev is the commit to undo, or to replay.
	Rev string `json:"rev"`
	// NoCommit leaves the change staged instead of recording it.
	NoCommit bool `json:"noCommit"`
	// Mainline is which side of a merge to apply against, 1-based; 0 for a plain commit.
	Mainline int `json:"mainline"`
}

// Revert records the opposite of a commit on top of HEAD, leaving it in the history.
func (r *Repo) Revert(ctx context.Context, opts ApplyOptions) (ApplyResult, error) {
	return r.apply(ctx, OpRevert, opts)
}

// CherryPick replays the change a commit made on top of HEAD.
func (r *Repo) CherryPick(ctx context.Context, opts ApplyOptions) (ApplyResult, error) {
	return r.apply(ctx, OpCherryPick, opts)
}

func (r *Repo) apply(ctx context.Context, op Operation, opts ApplyOptions) (ApplyResult, error) {
	rev := strings.TrimSpace(opts.Rev)
	if rev == "" || strings.HasPrefix(rev, "-") {
		return ApplyResult{}, fmt.Errorf("%q is not a commit", rev)
	}
	hash, err := r.resolve(ctx, rev)
	if err != nil {
		return ApplyResult{}, err
	}
	// A merge has two sides and git will not guess which one the change is against.
	merge, err := r.isMerge(ctx, hash)
	if err != nil {
		return ApplyResult{}, err
	}
	if merge && opts.Mainline < 1 {
		return ApplyResult{}, ErrMainlineNeeded
	}

	args := []string{string(op), "--no-edit"}
	if opts.NoCommit {
		args = append(args, "--no-commit")
	}
	if opts.Mainline > 0 {
		args = append(args, "--mainline", strconv.Itoa(opts.Mainline))
	}
	args = append(args, hash)

	if _, err := r.run.outputQuiet(ctx, args...); err != nil {
		// A conflict is an outcome rather than a failure, and the index shows it.
		if paths, perr := r.ConflictPaths(ctx); perr == nil && len(paths) > 0 {
			return ApplyResult{Kind: ApplyConflict, Conflicts: paths}, nil
		}
		return ApplyResult{}, r.classifyApply(ctx, op, err)
	}

	head, _ := r.resolve(ctx, "HEAD")
	return ApplyResult{Kind: ApplyDone, Conflicts: []string{}, Head: head}, nil
}

// isMerge reports whether a commit has more than one parent.
func (r *Repo) isMerge(ctx context.Context, rev string) (bool, error) {
	out, err := r.run.outputQuiet(ctx, "rev-list", "-1", "--parents", rev)
	if err != nil {
		return false, err
	}
	return len(strings.Fields(string(out))) > 2, nil
}

// AbortOperation puts the working tree back the way it was before the operation.
func (r *Repo) AbortOperation(ctx context.Context) error {
	op, _ := r.Operation()
	if op == OpNone {
		return ErrNoOperation
	}
	_, err := r.run.output(ctx, string(op), "--abort")
	return err
}

// FinishOperation records the commit that ends the operation, once nothing is conflicted.
func (r *Repo) FinishOperation(ctx context.Context, message string) error {
	op, _ := r.Operation()
	if op == OpNone {
		return ErrNoOperation
	}
	left, err := r.ConflictPaths(ctx)
	if err != nil {
		return err
	}
	if len(left) > 0 {
		return fmt.Errorf("%w: %s", ErrUnresolved, strings.Join(left, ", "))
	}

	if op != OpMerge {
		// --continue keeps the message git prepared; there is no way to pass another.
		return r.proceed(ctx, op, "--continue")
	}

	args := []string{"commit"}
	if msg := strings.TrimSpace(message); msg != "" {
		args = append(args, "-m", msg)
	} else {
		// --no-edit keeps the message git already wrote into MERGE_MSG.
		args = append(args, "--no-edit")
	}
	_, err = r.run.output(ctx, args...)
	return err
}

// SkipOperation drops the commit being replayed and moves on to the next one.
func (r *Repo) SkipOperation(ctx context.Context) error {
	switch op, _ := r.Operation(); op {
	case OpNone:
		return ErrNoOperation
	case OpMerge:
		return errors.New("a merge has no commit to skip")
	default:
		return r.proceed(ctx, op, "--skip")
	}
}

// proceed moves an operation on by one commit; stopping on the next conflict is not a failure.
func (r *Repo) proceed(ctx context.Context, op Operation, flag string) error {
	_, err := r.run.outputQuiet(ctx, string(op), flag)
	if err == nil {
		return nil
	}
	if op == OpRebase && r.rebaseDir() != "" {
		if paths, perr := r.ConflictPaths(ctx); perr == nil && len(paths) > 0 {
			return nil
		}
	}
	r.run.log.Error("git failed", "cmd", string(op)+" "+flag, "err", err)
	return err
}

// classifyApply turns git's refusals into sentinels the UI can react to.
func (r *Repo) classifyApply(ctx context.Context, op Operation, err error) error {
	var ce *CommandError
	if !errors.As(err, &ce) {
		return err
	}
	switch {
	case strings.Contains(ce.Stderr, "is now empty"),
		strings.Contains(ce.Stderr, "nothing to commit"):
		// git leaves the sequencer open, and nothing was applied, so end it here.
		_, _ = r.run.outputQuiet(ctx, string(op), "--quit")
		return ErrAlreadyApplied
	case strings.Contains(ce.Stderr, "no -m option was given"):
		return ErrMainlineNeeded
	}
	return classifyMerge(err)
}
