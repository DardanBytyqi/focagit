package git

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var (
	// ErrUnrelatedHistories means the two sides share no commit at all.
	ErrUnrelatedHistories = errors.New("the two histories have nothing in common")
	// ErrNoMerge means there is no merge to finish or abort.
	ErrNoMerge = errors.New("no merge in progress")
	// ErrUnresolved means files are still conflicted.
	ErrUnresolved = errors.New("some files are still conflicted")
	// ErrNoSide means the wanted side of a conflict does not exist.
	ErrNoSide = errors.New("that side deleted the file")
)

// MergeKind is what a merge turned out to be, which decides what to show.
type MergeKind string

const (
	MergeUpToDate    MergeKind = "up-to-date"
	MergeFastForward MergeKind = "fast-forward"
	MergeCommit      MergeKind = "commit"
	MergeConflict    MergeKind = "conflict"
)

// MergeOptions describes one merge.
type MergeOptions struct {
	// Ref is the branch or commit to merge into HEAD.
	Ref string `json:"ref"`
	// NoFastForward records a merge commit even when fast-forwarding would do.
	NoFastForward bool `json:"noFastForward"`
	// Message overrides git's own merge message.
	Message string `json:"message"`
}

// MergeResult is what happened, in the terms the UI talks in.
type MergeResult struct {
	Kind      MergeKind `json:"kind"`
	Conflicts []string  `json:"conflicts"`
	// Head is the commit HEAD ended on.
	Head string `json:"head"`
}

// ConflictSide picks whose version of a file to keep.
type ConflictSide string

const (
	SideOurs   ConflictSide = "ours"
	SideTheirs ConflictSide = "theirs"
)

// Conflict is one unmerged path and which sides still have a version of it.
type Conflict struct {
	Path string `json:"path"`
	// Base is false when the file is new on both sides.
	Base   bool `json:"base"`
	Ours   bool `json:"ours"`
	Theirs bool `json:"theirs"`
}

// DeletedBy names the side that removed the file, empty for a content conflict.
func (c Conflict) DeletedBy() ConflictSide {
	switch {
	case !c.Ours:
		return SideOurs
	case !c.Theirs:
		return SideTheirs
	}
	return ""
}

// MergePreview says what a merge would do, so the UI can offer only the
// choices that actually exist.
type MergePreview struct {
	// UpToDate means the source is already contained in the target.
	UpToDate bool `json:"upToDate"`
	// FastForward means the target can simply move, with no merge commit.
	FastForward bool `json:"fastForward"`
}

// PreviewMerge answers what merging source into target would do, without doing it.
func (r *Repo) PreviewMerge(ctx context.Context, source, target string) (MergePreview, error) {
	src, err := r.resolve(ctx, source)
	if err != nil {
		return MergePreview{}, err
	}
	dst, err := r.resolve(ctx, target)
	if err != nil {
		return MergePreview{}, err
	}

	switch {
	case r.isAncestor(ctx, src, dst):
		return MergePreview{UpToDate: true}, nil
	case r.isAncestor(ctx, dst, src):
		return MergePreview{FastForward: true}, nil
	}
	return MergePreview{}, nil
}

// Merge joins another branch into the current one.
func (r *Repo) Merge(ctx context.Context, opts MergeOptions) (MergeResult, error) {
	ref := strings.TrimSpace(opts.Ref)
	if ref == "" || strings.HasPrefix(ref, "-") {
		return MergeResult{}, fmt.Errorf("%w: %q", ErrInvalidBranchName, ref)
	}
	target, err := r.resolve(ctx, ref)
	if err != nil {
		return MergeResult{}, err
	}

	// Asking about ancestry before merging is how the kind is known without
	// reading git's prose, which changes between versions.
	if r.isAncestor(ctx, target, "HEAD") {
		head, _ := r.resolve(ctx, "HEAD")
		return MergeResult{Kind: MergeUpToDate, Head: head}, nil
	}
	fastForward := !opts.NoFastForward && r.isAncestor(ctx, "HEAD", target)

	args := []string{"merge"}
	if opts.NoFastForward {
		args = append(args, "--no-ff")
	}
	if msg := strings.TrimSpace(opts.Message); msg != "" {
		args = append(args, "-m", msg)
	} else {
		args = append(args, "--no-edit")
	}
	args = append(args, "--", ref)

	// Exit 1 is the conflict, which is an outcome rather than a failure.
	if _, err := r.run.outputOK(ctx, []int{1}, args...); err != nil {
		return MergeResult{}, classifyMerge(err)
	}

	if _, merging := r.MergeHead(ctx); merging {
		paths, err := r.ConflictPaths(ctx)
		if err != nil {
			return MergeResult{}, err
		}
		return MergeResult{Kind: MergeConflict, Conflicts: paths}, nil
	}

	head, _ := r.resolve(ctx, "HEAD")
	kind := MergeCommit
	if fastForward {
		kind = MergeFastForward
	}
	return MergeResult{Kind: kind, Head: head}, nil
}

// PullOptions says where to pull from and how to join what arrives.
type PullOptions struct {
	// Remote is empty for the upstream of the current branch.
	Remote string `json:"remote"`
	Prune  bool   `json:"prune"`
	// NoFastForward records a merge commit even when fast-forwarding would do.
	NoFastForward bool `json:"noFastForward"`
}

// Pull is a fetch followed by a merge of the upstream, kept apart on purpose:
// git pull reports one failure for two operations, so which half broke is lost.
func (r *Repo) Pull(ctx context.Context, opts PullOptions, onProgress func(string)) (MergeResult, error) {
	st, err := r.Status(ctx)
	if err != nil {
		return MergeResult{}, err
	}
	if st.Detached {
		return MergeResult{}, fmt.Errorf("%w: HEAD is detached", ErrNoUpstream)
	}
	if st.Upstream == "" {
		return MergeResult{}, fmt.Errorf("%w: %s", ErrNoUpstream, st.Head)
	}

	if err := r.Fetch(ctx, FetchOptions{Remote: opts.Remote, Prune: opts.Prune}, onProgress); err != nil {
		return MergeResult{}, err
	}
	return r.Merge(ctx, MergeOptions{Ref: st.Upstream, NoFastForward: opts.NoFastForward})
}

// AbortMerge puts the working tree back the way it was before the merge.
func (r *Repo) AbortMerge(ctx context.Context) error {
	if _, merging := r.MergeHead(ctx); !merging {
		return ErrNoMerge
	}
	_, err := r.run.output(ctx, "merge", "--abort")
	return err
}

// FinishMerge records the merge commit once every conflict is resolved.
func (r *Repo) FinishMerge(ctx context.Context, message string) error {
	if _, merging := r.MergeHead(ctx); !merging {
		return ErrNoMerge
	}
	left, err := r.ConflictPaths(ctx)
	if err != nil {
		return err
	}
	if len(left) > 0 {
		return fmt.Errorf("%w: %s", ErrUnresolved, strings.Join(left, ", "))
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

// MergeHead is the commit being merged in, and whether a merge is under way.
func (r *Repo) MergeHead(ctx context.Context) (string, bool) {
	out, err := r.run.outputQuiet(ctx, "rev-parse", "--verify", "--quiet", "MERGE_HEAD")
	if err != nil {
		return "", false
	}
	hash := strings.TrimSpace(string(out))
	return hash, hash != ""
}

// MergeSource names the branch being merged in, for labelling the two sides.
func (r *Repo) MergeSource(ctx context.Context) string {
	out, err := r.run.outputQuiet(ctx, "name-rev", "--name-only", "--refs=refs/heads/*", "MERGE_HEAD")
	if name := strings.TrimSpace(string(out)); err == nil && name != "" && name != "undefined" {
		return name
	}
	// A merge can come from a commit no branch points at any more.
	if hash, ok := r.MergeHead(ctx); ok && len(hash) >= 7 {
		return hash[:7]
	}
	return ""
}

// Conflicts lists the unmerged paths and which sides still hold the file.
func (r *Repo) Conflicts(ctx context.Context) ([]Conflict, error) {
	out, err := r.run.output(ctx, "ls-files", "-u", "-z")
	if err != nil {
		return nil, err
	}

	byPath := map[string]*Conflict{}
	var order []string
	for _, e := range strings.Split(string(out), "\x00") {
		if e == "" {
			continue
		}
		// The format is "<mode> <hash> <stage>\t<path>".
		meta, path, ok := strings.Cut(e, "\t")
		if !ok {
			continue
		}
		fields := strings.Fields(meta)
		if len(fields) != 3 {
			continue
		}
		stage, err := strconv.Atoi(fields[2])
		if err != nil {
			continue
		}

		c, seen := byPath[path]
		if !seen {
			c = &Conflict{Path: path}
			byPath[path] = c
			order = append(order, path)
		}
		switch stage {
		case 1:
			c.Base = true
		case 2:
			c.Ours = true
		case 3:
			c.Theirs = true
		}
	}

	list := make([]Conflict, 0, len(order))
	for _, p := range order {
		list = append(list, *byPath[p])
	}
	return list, nil
}

// ConflictPaths is Conflicts reduced to the paths, for callers that only count.
func (r *Repo) ConflictPaths(ctx context.Context) ([]string, error) {
	list, err := r.Conflicts(ctx)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(list))
	for _, c := range list {
		paths = append(paths, c.Path)
	}
	return paths, nil
}

// Resolve settles one conflicted file by keeping one side whole.
func (r *Repo) Resolve(ctx context.Context, path string, side ConflictSide) error {
	if side != SideOurs && side != SideTheirs {
		return fmt.Errorf("%q is not a side", side)
	}
	list, err := r.Conflicts(ctx)
	if err != nil {
		return err
	}

	var found *Conflict
	for i := range list {
		if list[i].Path == path {
			found = &list[i]
			break
		}
	}
	if found == nil {
		return fmt.Errorf("%q is not conflicted", path)
	}

	// A side that deleted the file has nothing to check out, so keeping it means
	// removing the file rather than restoring a version that does not exist.
	has := found.Ours
	if side == SideTheirs {
		has = found.Theirs
	}
	if !has {
		_, err := r.run.output(ctx, "rm", "-q", "--", path)
		return err
	}

	if _, err := r.run.output(ctx, "checkout", "--"+string(side), "--", path); err != nil {
		return err
	}
	_, err = r.run.output(ctx, "add", "--", path)
	return err
}

// resolve turns a ref into the commit it names.
func (r *Repo) resolve(ctx context.Context, ref string) (string, error) {
	out, err := r.run.outputQuiet(ctx, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("%q is not a commit or branch", ref)
	}
	return strings.TrimSpace(string(out)), nil
}

// isAncestor reports whether a is reachable from b.
func (r *Repo) isAncestor(ctx context.Context, a, b string) bool {
	_, err := r.run.outputQuiet(ctx, "merge-base", "--is-ancestor", a, b)
	return err == nil
}

// classifyMerge turns git's refusals into sentinels the UI can react to.
func classifyMerge(err error) error {
	var ce *CommandError
	if !errors.As(err, &ce) {
		return err
	}
	switch {
	case strings.Contains(ce.Stderr, "refusing to merge unrelated histories"):
		return ErrUnrelatedHistories
	case strings.Contains(ce.Stderr, "would be overwritten"),
		strings.Contains(ce.Stderr, "Please commit your changes"):
		return fmt.Errorf("%w: %s", ErrWouldOverwrite, ce.Stderr)
	}
	return err
}
