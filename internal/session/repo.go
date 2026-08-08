package session

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/dardanbytyqi/focagit/internal/git"
	"github.com/dardanbytyqi/focagit/internal/graph"
	"github.com/dardanbytyqi/focagit/internal/patch"
)

// progressBatch is how many commits accumulate before the loader reports progress.
const progressBatch = 500

// Entry is one line of the history: the commit and its place in the graph.
type Entry struct {
	Commit git.Commit `json:"commit"`
	Graph  graph.Row  `json:"graph"`
}

// Repo is one open repository plus the history cached from it.
type Repo struct {
	Info Info
	git  *git.Repo

	mu      sync.RWMutex
	entries []Entry
	width   int
	loading bool
	loadErr error
	cancel  context.CancelFunc
	// gen counts reloads, so the frontend knows its cached pages are stale.
	gen int
}

// Gen is the current history generation; it changes on every reload.
func (r *Repo) Gen() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.gen
}

// Git exposes the underlying repository for operations that do not go through the cache.
func (r *Repo) Git() *git.Repo { return r.git }

// Load walks the whole history into the cache, calling onProgress every few hundred.
func (r *Repo) Load(ctx context.Context, onProgress func(gen, loaded, width int, done bool)) error {
	r.cancelLoad()

	ctx, cancel := context.WithCancel(ctx)

	r.mu.Lock()
	r.entries = nil
	r.width = 0
	r.loading = true
	r.loadErr = nil
	r.cancel = cancel
	r.gen++
	gen := r.gen
	r.mu.Unlock()

	defer cancel()

	start := time.Now()
	builder := graph.New()

	var batch []Entry
	flush := func(done bool) {
		r.mu.Lock()
		r.entries = append(r.entries, batch...)
		r.width = builder.Width()
		total, width := len(r.entries), r.width
		r.mu.Unlock()

		batch = batch[:0]
		if onProgress != nil {
			onProgress(gen, total, width, done)
		}
	}

	err := r.git.Log(ctx, git.LogOptions{All: true}, func(c git.Commit) error {
		batch = append(batch, Entry{
			Commit: c,
			Graph:  builder.Add(c.Hash, c.Parents),
		})
		if len(batch) >= progressBatch {
			flush(false)
		}
		return nil
	})

	r.mu.Lock()
	r.loading = false
	r.loadErr = err
	r.mu.Unlock()

	// Flush whatever is left even on failure: a partial history is more useful than none.
	flush(true)

	attrs := []any{
		slog.String("repo", r.Info.Name),
		slog.Int("commits", r.Count()),
		slog.Int("lanes", builder.Width()),
		slog.Duration("dur", time.Since(start)),
	}
	if err != nil {
		slog.Warn("history load interrupted", append(attrs, slog.String("err", err.Error()))...)
	} else {
		slog.Info("history loaded", attrs...)
	}
	return err
}

// cancelLoad stops an in-flight walk, if there is one.
func (r *Repo) cancelLoad() {
	r.mu.Lock()
	cancel := r.cancel
	r.cancel = nil
	r.mu.Unlock()

	if cancel != nil {
		cancel()
	}
}

// Range returns entries in [offset, offset+count), clamped to what is loaded.
func (r *Repo) Range(offset, count int) []Entry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if offset < 0 {
		offset = 0
	}
	if offset >= len(r.entries) || count <= 0 {
		return []Entry{}
	}
	end := min(offset+count, len(r.entries))

	// Copy rather than return a subslice: the backing array keeps growing underneath.
	out := make([]Entry, end-offset)
	copy(out, r.entries[offset:end])
	return out
}

// Count is how many commits are cached right now.
func (r *Repo) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.entries)
}

// GraphWidth is how many lanes the graph column needs.
func (r *Repo) GraphWidth() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.width
}

// State reports load progress for the status bar.
func (r *Repo) State() (count int, loading bool, err error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.entries), r.loading, r.loadErr
}

// Refs and Status are not cached: they are cheap.
func (r *Repo) Refs(ctx context.Context) ([]git.Ref, error) { return r.git.Refs(ctx) }

func (r *Repo) Status(ctx context.Context) (git.Status, error) { return r.git.Status(ctx) }

// CommitFiles and Diff are read on demand as the user clicks around.
func (r *Repo) CommitFiles(ctx context.Context, hash string) ([]git.FileChange, error) {
	return r.git.CommitFiles(ctx, hash)
}

func (r *Repo) Diff(ctx context.Context, t git.DiffTarget) (git.FileDiff, error) {
	return r.git.Diff(ctx, t)
}

// WorkingDiff is the Changes view's diff for one path.
func (r *Repo) WorkingDiff(ctx context.Context, path string, staged, untracked bool) (git.FileDiff, error) {
	if untracked && !staged {
		return r.git.UntrackedDiff(ctx, path)
	}
	return r.git.Diff(ctx, git.DiffTarget{Path: path, Staged: staged})
}

func (r *Repo) Stage(ctx context.Context, paths ...string) error {
	return r.git.Stage(ctx, paths...)
}

func (r *Repo) Unstage(ctx context.Context, paths ...string) error {
	return r.git.Unstage(ctx, paths...)
}

func (r *Repo) Discard(ctx context.Context, paths ...string) error {
	return r.git.Discard(ctx, paths...)
}

// StageLines stages or unstages individual lines of one file.
func (r *Repo) StageLines(ctx context.Context, path string, refs []patch.LineRef, unstage bool) error {
	// Staging reads the unstaged diff; unstaging reads the staged one.
	d, err := r.git.Diff(ctx, git.DiffTarget{Path: path, Staged: unstage})
	if err != nil {
		return err
	}

	p, err := patch.Build(d, refs, unstage)
	if err != nil {
		return err
	}
	return r.git.ApplyPatch(ctx, p, unstage)
}

// DiscardLines throws away a selection of lines from the working tree.
func (r *Repo) DiscardLines(ctx context.Context, path string, refs []patch.LineRef) error {
	d, err := r.git.Diff(ctx, git.DiffTarget{Path: path})
	if err != nil {
		return err
	}
	p, err := patch.Build(d, refs, true)
	if err != nil {
		return err
	}
	return r.git.ApplyPatchWorktree(ctx, p, true)
}

// Commit records the staged changes.
func (r *Repo) Commit(ctx context.Context, opts git.CommitOptions) error {
	return r.git.Commit(ctx, opts)
}

// --- branches ---------------------------------------------------------------

func (r *Repo) Checkout(ctx context.Context, branch string) error {
	return r.git.Checkout(ctx, branch)
}

func (r *Repo) CheckoutRemote(ctx context.Context, remoteRef string) error {
	return r.git.CheckoutRemote(ctx, remoteRef)
}

func (r *Repo) CheckoutDetached(ctx context.Context, rev string) error {
	return r.git.CheckoutDetached(ctx, rev)
}

func (r *Repo) CreateBranch(ctx context.Context, opts git.CreateBranchOptions) error {
	return r.git.CreateBranch(ctx, opts)
}

func (r *Repo) DeleteBranch(ctx context.Context, name string, force bool) error {
	return r.git.DeleteBranch(ctx, name, force)
}

func (r *Repo) RenameBranch(ctx context.Context, from, to string) error {
	return r.git.RenameBranch(ctx, from, to)
}

func (r *Repo) Remotes(ctx context.Context) ([]git.Remote, error) {
	return r.git.Remotes(ctx)
}

func (r *Repo) AddRemote(ctx context.Context, name, url string) error {
	return r.git.AddRemote(ctx, name, url)
}

func (r *Repo) RemoveRemote(ctx context.Context, name string) error {
	return r.git.RemoveRemote(ctx, name)
}

func (r *Repo) Fetch(ctx context.Context, opts git.FetchOptions, onProgress func(string)) error {
	return r.git.Fetch(ctx, opts, onProgress)
}

func (r *Repo) Push(ctx context.Context, opts git.PushOptions, onProgress func(string)) ([]git.PushResult, error) {
	return r.git.Push(ctx, opts, onProgress)
}
