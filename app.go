package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/dardanbytyqi/focagit/internal/git"
	"github.com/dardanbytyqi/focagit/internal/patch"
	"github.com/dardanbytyqi/focagit/internal/prefs"
	"github.com/dardanbytyqi/focagit/internal/session"
	"github.com/dardanbytyqi/focagit/internal/sshkeys"
)

// App is the surface exposed to the frontend.
type App struct {
	ctx     context.Context
	session *session.Session
	prefs   *prefs.Store

	// mu guards cloneCancel, which the clone and its cancel button share.
	mu          sync.Mutex
	cloneCancel context.CancelFunc
}

func NewApp() *App {
	// Preferences are a convenience; not having them must not stop the app.
	store, err := prefs.Open()
	if err != nil {
		slog.Warn("preferences unavailable", slog.String("err", err.Error()))
	}
	return &App{session: session.New(), prefs: store}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// InitialRepo returns FOCAGIT_OPEN, which lets dev builds skip the welcome screen.
func (a *App) InitialRepo() string { return os.Getenv("FOCAGIT_OPEN") }

// InitialView is the companion knob.
func (a *App) InitialView() string { return os.Getenv("FOCAGIT_VIEW") }

// Progress is pushed to the frontend as the history streams in.
type Progress struct {
	ID string `json:"id"`
	// Gen changes on every reload, telling the frontend its cached pages are stale.
	Gen    int    `json:"gen"`
	Loaded int    `json:"loaded"`
	Width  int    `json:"width"`
	Done   bool   `json:"done"`
	Error  string `json:"error,omitempty"`
}

// OpenRepo registers a repository and starts loading its history in the background.
func (a *App) OpenRepo(path string) (session.Info, error) {
	repo, err := a.session.Open(a.ctx, path)
	if err != nil {
		return session.Info{}, err
	}

	a.reload(repo)
	a.startWatch(repo)
	if a.prefs != nil {
		if err := a.prefs.Remember(repo.Info.Path, repo.Info.Name); err != nil {
			slog.Warn("could not save the recent list", slog.String("err", err.Error()))
		}
	}
	return repo.Info, nil
}

// reload walks the history again in the background, reporting progress as it goes.
func (a *App) reload(repo *session.Repo) {
	go func() {
		err := repo.Load(a.ctx, func(gen, loaded, width int, done bool) {
			runtime.EventsEmit(a.ctx, "repo:progress", Progress{
				ID: repo.Info.ID, Gen: gen, Loaded: loaded, Width: width, Done: done,
			})
		})
		// A cancelled walk was replaced by a newer one, which reports for itself.
		if err != nil && !errors.Is(err, context.Canceled) {
			runtime.EventsEmit(a.ctx, "repo:progress", Progress{
				ID: repo.Info.ID, Gen: repo.Gen(), Done: true, Error: err.Error(),
			})
		}
	}()
}

// Changed says the repository moved under us, so the frontend re-reads what it shows.
type Changed struct {
	ID string `json:"id"`
}

// startWatch reports later changes on disk; listing the ignored folders can be slow.
func (a *App) startWatch(repo *session.Repo) {
	go func() {
		err := repo.Watch(a.ctx, func(refs bool) {
			// A moved branch changes the graph, which only a fresh walk can rebuild.
			if refs {
				a.reload(repo)
				return
			}
			runtime.EventsEmit(a.ctx, "repo:changed", Changed{ID: repo.Info.ID})
		})
		if err != nil {
			slog.Warn("changes made outside focagit will go unnoticed",
				slog.String("repo", repo.Info.Name), slog.String("err", err.Error()))
		}
	}()
}

// PathInfo answers what a folder is before we try to open it as a repository.
type PathInfo struct {
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
	IsDir  bool   `json:"isDir"`
	IsRepo bool   `json:"isRepo"`
}

// Inspect classifies a path so the UI can offer to initialise it.
func (a *App) Inspect(path string) PathInfo {
	info := PathInfo{Path: path}

	st, err := os.Stat(path)
	if err != nil {
		return info
	}
	info.Exists = true
	info.IsDir = st.IsDir()

	if _, err := git.Open(a.ctx, path); err == nil {
		info.IsRepo = true
	}
	return info
}

// InitRepo runs `git init` in path and opens the result.
func (a *App) InitRepo(path string) (session.Info, error) {
	if err := git.Init(a.ctx, path); err != nil {
		return session.Info{}, err
	}
	return a.OpenRepo(path)
}

// SelectRepo opens the native directory picker and returns the chosen path, or an empty.
func (a *App) SelectRepo() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Open repository",
	})
}

// SelectFolder is the same picker, for choosing where a clone should land.
func (a *App) SelectFolder(title, start string) (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            title,
		DefaultDirectory: start,
	})
}

// CloneParent is where the last repository was opened from, or the home folder.
func (a *App) CloneParent() string {
	if a.prefs != nil {
		if recents := a.prefs.Recents(); len(recents) > 0 {
			return filepath.Dir(recents[0].Path)
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

// PreviewClone says where these options would put the repository.
func (a *App) PreviewClone(opts git.CloneOptions) git.ClonePreview {
	return git.PreviewClone(opts)
}

// CloneRepo copies a remote repository to disk and opens it.
func (a *App) CloneRepo(opts git.CloneOptions) (session.Info, error) {
	ctx, cancel := context.WithCancel(a.ctx)
	a.mu.Lock()
	a.cloneCancel = cancel
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		a.cloneCancel = nil
		a.mu.Unlock()
		cancel()
	}()

	path, err := git.Clone(ctx, opts, a.report("", "clone"))
	runtime.EventsEmit(a.ctx, "repo:network", Network{Op: "clone", Done: true, Error: errText(err)})
	if err != nil {
		return session.Info{}, err
	}
	return a.OpenRepo(path)
}

// CancelClone stops a clone in progress; the half-copied folder is removed.
func (a *App) CancelClone() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cloneCancel != nil {
		a.cloneCancel()
	}
}

// CloseRepo cancels any in-flight load and drops the cache.
func (a *App) CloseRepo(id string) { a.session.Close(id) }

// ListRepos returns the currently open repositories.
func (a *App) ListRepos() []session.Info { return a.session.List() }

// History describes the cached history: how much of it there is and which walk built it.
type History struct {
	Gen   int `json:"gen"`
	Count int `json:"count"`
	Width int `json:"width"`
}

// GetHistory is one call for everything the commit list needs to size itself.
func (a *App) GetHistory(id string) (History, error) {
	repo, err := a.session.Get(id)
	if err != nil {
		return History{}, err
	}
	return History{Gen: repo.Gen(), Count: repo.Count(), Width: repo.GraphWidth()}, nil
}

// CommitRange returns one page of history, each entry with its commit and graph row.
func (a *App) CommitRange(id string, offset, count int) ([]session.Entry, error) {
	repo, err := a.session.Get(id)
	if err != nil {
		return nil, err
	}
	return repo.Range(offset, count), nil
}

// SearchCommits is the rows of the cached history whose commit matches query.
func (a *App) SearchCommits(id, query string) ([]int, error) {
	repo, err := a.session.Get(id)
	if err != nil {
		return nil, err
	}
	return repo.Search(query), nil
}

// CommitIndex is the row a commit sits on, so the list can scroll to it, or -1
// when the history does not hold it.
func (a *App) CommitIndex(id, hash string) (int, error) {
	repo, err := a.session.Get(id)
	if err != nil {
		return -1, err
	}
	return repo.IndexOf(hash), nil
}

// Refs lists branches, remotes and tags for the sidebar.
func (a *App) Refs(id string) ([]git.Ref, error) {
	repo, err := a.session.Get(id)
	if err != nil {
		return nil, err
	}
	return repo.Refs(a.ctx)
}

// CommitFiles lists the files a commit changed, for the detail pane.
func (a *App) CommitFiles(id, hash string) ([]git.FileChange, error) {
	repo, err := a.session.Get(id)
	if err != nil {
		return nil, err
	}
	return repo.CommitFiles(a.ctx, hash)
}

// Diff returns the patch for one file, either inside a commit or in the working tree.
func (a *App) Diff(id string, target git.DiffTarget) (git.FileDiff, error) {
	repo, err := a.session.Get(id)
	if err != nil {
		return git.FileDiff{}, err
	}
	return repo.Diff(a.ctx, target)
}

// Status is the working tree state shown in the Changes row and status bar.
func (a *App) Status(id string) (git.Status, error) {
	repo, err := a.session.Get(id)
	if err != nil {
		return git.Status{}, err
	}
	return repo.Status(a.ctx)
}

// --- Changes view: everything below writes to the repository ----------------

// WorkingDiff returns the diff for one working tree path, handling untracked files.
func (a *App) WorkingDiff(id, path string, staged, untracked bool) (git.FileDiff, error) {
	repo, err := a.session.Get(id)
	if err != nil {
		return git.FileDiff{}, err
	}
	return repo.WorkingDiff(a.ctx, path, staged, untracked)
}

func (a *App) StageFiles(id string, paths []string) error {
	repo, err := a.session.Get(id)
	if err != nil {
		return err
	}
	return repo.Stage(a.ctx, paths...)
}

func (a *App) UnstageFiles(id string, paths []string) error {
	repo, err := a.session.Get(id)
	if err != nil {
		return err
	}
	return repo.Unstage(a.ctx, paths...)
}

// DiscardFiles throws working tree changes away.
func (a *App) DiscardFiles(id string, paths []string) error {
	repo, err := a.session.Get(id)
	if err != nil {
		return err
	}
	return repo.Discard(a.ctx, paths...)
}

// StageLines stages or unstages a selection of lines within one file.
func (a *App) StageLines(id, path string, refs []patch.LineRef, unstage bool) error {
	repo, err := a.session.Get(id)
	if err != nil {
		return err
	}
	return repo.StageLines(a.ctx, path, refs, unstage)
}

// DiscardLines throws away a selection of working tree lines.
func (a *App) DiscardLines(id, path string, refs []patch.LineRef) error {
	repo, err := a.session.Get(id)
	if err != nil {
		return err
	}
	return repo.DiscardLines(a.ctx, path, refs)
}

// Commit records the staged changes, then reloads the history because the new commit.
func (a *App) Commit(id, message string, amend bool) error {
	repo, err := a.session.Get(id)
	if err != nil {
		return err
	}
	if err := repo.Commit(a.ctx, git.CommitOptions{Message: message, Amend: amend}); err != nil {
		return err
	}
	a.reload(repo)
	return nil
}

// --- recent repositories -----------------------------------------------------

// RecentRepos lists what the welcome screen offers, newest first.
func (a *App) RecentRepos() []prefs.Recent {
	if a.prefs == nil {
		return []prefs.Recent{}
	}
	return a.prefs.Recents()
}

// ForgetRepo drops one entry from the recent list.
func (a *App) ForgetRepo(path string) error {
	if a.prefs == nil {
		return nil
	}
	return a.prefs.Forget(path)
}

// --- identities ---------------------------------------------------------------

// IdentityState is everything the identity dialog shows, in one call.
type IdentityState struct {
	// Current is who the next commit here would be attributed to.
	Current git.Signature `json:"current"`
	// Global is what this repository falls back to when it sets nothing.
	Global git.Signature    `json:"global"`
	Saved  []prefs.Identity `json:"saved"`
}

// GetIdentityState reads the repository's identity, the global one, and the saved list.
func (a *App) GetIdentityState(id string) (IdentityState, error) {
	repo, err := a.session.Get(id)
	if err != nil {
		return IdentityState{}, err
	}

	st := IdentityState{Saved: []prefs.Identity{}}
	if st.Current, err = repo.Git().Signature(a.ctx); err != nil {
		return st, err
	}
	if st.Global, err = repo.Git().GlobalSignature(a.ctx); err != nil {
		return st, err
	}
	if a.prefs != nil {
		st.Saved = a.prefs.Identities()
	}
	return st, nil
}

// Signature is who git would attribute the next commit in this repository to.
func (a *App) Signature(id string) (git.Signature, error) {
	repo, err := a.session.Get(id)
	if err != nil {
		return git.Signature{}, err
	}
	return repo.Git().Signature(a.ctx)
}

// AddIdentity saves an identity, replacing any earlier one with the same email.
func (a *App) AddIdentity(ident prefs.Identity) error {
	if a.prefs == nil {
		return errors.New("preferences are unavailable")
	}
	_, err := a.prefs.AddIdentity(ident)
	return err
}

// RemoveIdentity forgets a saved identity; repositories already using it keep it.
func (a *App) RemoveIdentity(email string) error {
	if a.prefs == nil {
		return errors.New("preferences are unavailable")
	}
	return a.prefs.RemoveIdentity(email)
}

// ApplyIdentity writes a name and email into this repository's own config.
func (a *App) ApplyIdentity(id string, ident prefs.Identity) error {
	repo, err := a.session.Get(id)
	if err != nil {
		return err
	}
	return repo.Git().SetSignature(a.ctx, ident.Name, ident.Email)
}

// ClearIdentity drops the repository's own identity so the global one applies.
func (a *App) ClearIdentity(id string) error {
	repo, err := a.session.Get(id)
	if err != nil {
		return err
	}
	return repo.Git().ClearSignature(a.ctx)
}

// --- branches ---------------------------------------------------------------

// Checkout switches to a local branch, or to the local branch behind a remote one.
func (a *App) Checkout(id string, ref git.Ref) error {
	repo, err := a.session.Get(id)
	if err != nil {
		return err
	}
	if ref.Kind == git.RefRemote {
		return repo.CheckoutRemote(a.ctx, ref.Short)
	}
	return repo.Checkout(a.ctx, ref.Short)
}

// CheckoutCommit detaches HEAD onto a commit, as double-clicking a row does in Fork.
func (a *App) CheckoutCommit(id, hash string) error {
	repo, err := a.session.Get(id)
	if err != nil {
		return err
	}
	return repo.CheckoutDetached(a.ctx, hash)
}

// CreateBranch adds a branch, reloading the history because a new tip may be reachable.
func (a *App) CreateBranch(id string, opts git.CreateBranchOptions) error {
	repo, err := a.session.Get(id)
	if err != nil {
		return err
	}
	if err := repo.CreateBranch(a.ctx, opts); err != nil {
		return err
	}
	a.reload(repo)
	return nil
}

// DeleteBranch removes a branch; deleting a tip can make commits unreachable, so reload.
func (a *App) DeleteBranch(id, name string, force bool) error {
	repo, err := a.session.Get(id)
	if err != nil {
		return err
	}
	if err := repo.DeleteBranch(a.ctx, name, force); err != nil {
		return err
	}
	a.reload(repo)
	return nil
}

// RenameBranch renames a branch; the commits it points at do not move.
func (a *App) RenameBranch(id, from, to string) error {
	repo, err := a.session.Get(id)
	if err != nil {
		return err
	}
	return repo.RenameBranch(a.ctx, from, to)
}

// --- stash --------------------------------------------------------------------

// Stashes lists the stack for the sidebar.
func (a *App) Stashes(id string) ([]git.Stash, error) {
	repo, err := a.session.Get(id)
	if err != nil {
		return nil, err
	}
	return repo.Stashes(a.ctx)
}

// Stash puts the working tree away; the entry is a commit, so the graph changes.
func (a *App) Stash(id string, opts git.StashOptions) error {
	repo, err := a.session.Get(id)
	if err != nil {
		return err
	}
	if err := repo.Stash(a.ctx, opts); err != nil {
		return err
	}
	a.reload(repo)
	return nil
}

// ApplyStash restores an entry and leaves it on the stack.
func (a *App) ApplyStash(id, ref string, index bool) error {
	repo, err := a.session.Get(id)
	if err != nil {
		return err
	}
	return repo.ApplyStash(a.ctx, ref, index)
}

// PopStash restores an entry and drops it, which removes its commit from the graph.
func (a *App) PopStash(id, ref string, index bool) error {
	repo, err := a.session.Get(id)
	if err != nil {
		return err
	}
	if err := repo.PopStash(a.ctx, ref, index); err != nil {
		return err
	}
	a.reload(repo)
	return nil
}

// DropStash removes an entry without restoring it.
func (a *App) DropStash(id, ref string) error {
	repo, err := a.session.Get(id)
	if err != nil {
		return err
	}
	if err := repo.DropStash(a.ctx, ref); err != nil {
		return err
	}
	a.reload(repo)
	return nil
}

// StashFiles lists what an entry would change, for the detail pane.
func (a *App) StashFiles(id, ref string) ([]git.FileChange, error) {
	repo, err := a.session.Get(id)
	if err != nil {
		return nil, err
	}
	return repo.StashFiles(a.ctx, ref)
}

// --- remotes ------------------------------------------------------------------

// Network is what a fetch or a push is doing right now, pushed as it happens.
type Network struct {
	ID string `json:"id"`
	// Op is "fetch" or "push", so the UI can say which one is running.
	Op string `json:"op"`
	// Line is git's own progress text, e.g. "Receiving objects:  62% (31/50)".
	Line  string `json:"line"`
	Done  bool   `json:"done"`
	Error string `json:"error"`
}

// GetRemotes lists where this repository can fetch from and push to.
func (a *App) GetRemotes(id string) ([]git.Remote, error) {
	repo, err := a.session.Get(id)
	if err != nil {
		return nil, err
	}
	list, err := repo.Remotes(a.ctx)
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []git.Remote{}
	}
	return list, nil
}

// AddRemote records a remote and, if asked, fetches to prove the URL works.
func (a *App) AddRemote(id, name, url string, fetchNow bool) error {
	repo, err := a.session.Get(id)
	if err != nil {
		return err
	}
	if err := repo.AddRemote(a.ctx, name, url); err != nil {
		return err
	}
	if !fetchNow {
		return nil
	}

	// The remote stays even if the fetch fails: the network may be at fault.
	if err := a.Fetch(id, git.FetchOptions{Remote: name}); err != nil {
		return fmt.Errorf("%s was added, but fetching from it failed: %w", name, err)
	}
	return nil
}

// RemoveRemote forgets a remote, then rewalks: its branches are gone from the graph.
func (a *App) RemoveRemote(id, name string) error {
	repo, err := a.session.Get(id)
	if err != nil {
		return err
	}
	if err := repo.RemoveRemote(a.ctx, name); err != nil {
		return err
	}
	a.reload(repo)
	return nil
}

// report emits one network progress line to the frontend.
func (a *App) report(id, op string) func(string) {
	return func(line string) {
		runtime.EventsEmit(a.ctx, "repo:network", Network{ID: id, Op: op, Line: line})
	}
}

// Fetch brings down remote branches, then rewalks: new commits are now reachable.
func (a *App) Fetch(id string, opts git.FetchOptions) error {
	repo, err := a.session.Get(id)
	if err != nil {
		return err
	}

	err = repo.Fetch(a.ctx, opts, a.report(id, "fetch"))
	runtime.EventsEmit(a.ctx, "repo:network", Network{ID: id, Op: "fetch", Done: true, Error: errText(err)})
	if err != nil {
		return err
	}
	a.reload(repo)
	return nil
}

// Push sends commits to a remote and returns what the remote made of each ref.
func (a *App) Push(id string, opts git.PushOptions) ([]git.PushResult, error) {
	repo, err := a.session.Get(id)
	if err != nil {
		return nil, err
	}

	res, err := repo.Push(a.ctx, opts, a.report(id, "push"))
	runtime.EventsEmit(a.ctx, "repo:network", Network{ID: id, Op: "push", Done: true, Error: errText(err)})
	if res == nil {
		res = []git.PushResult{}
	}
	return res, err
}

// errText is "" for no error, which is what the frontend checks.
func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// --- appearance ---------------------------------------------------------------

// GetAppearance is what the window should look like.
func (a *App) GetAppearance() prefs.Appearance {
	if a.prefs == nil {
		return prefs.DefaultAppearance()
	}
	return a.prefs.Appearance()
}

// SaveAppearance stores the look; unrecognised values fall back to the default.
func (a *App) SaveAppearance(look prefs.Appearance) (prefs.Appearance, error) {
	if a.prefs == nil {
		return prefs.DefaultAppearance(), errors.New("preferences are unavailable")
	}
	if err := a.prefs.SaveAppearance(look); err != nil {
		return prefs.DefaultAppearance(), err
	}
	return a.prefs.Appearance(), nil
}

// --- merge --------------------------------------------------------------------

// PreviewMerge answers what a merge would do, so the UI offers only real choices.
func (a *App) PreviewMerge(id, source, target string) (git.MergePreview, error) {
	repo, err := a.session.Get(id)
	if err != nil {
		return git.MergePreview{}, err
	}
	return repo.PreviewMerge(a.ctx, source, target)
}

// Merge joins a branch into the current one and reloads what changed.
func (a *App) Merge(id string, opts git.MergeOptions) (git.MergeResult, error) {
	repo, err := a.session.Get(id)
	if err != nil {
		return git.MergeResult{}, err
	}
	res, err := repo.Merge(a.ctx, opts)
	if err != nil {
		return git.MergeResult{}, err
	}
	// A conflict changes the working tree too, so the views reload either way.
	a.reload(repo)
	return res, nil
}

// Pull fetches and then merges the upstream, reporting progress like a fetch does.
func (a *App) Pull(id string, opts git.PullOptions) (git.MergeResult, error) {
	repo, err := a.session.Get(id)
	if err != nil {
		return git.MergeResult{}, err
	}

	res, err := repo.Pull(a.ctx, opts, a.report(id, "pull"))
	runtime.EventsEmit(a.ctx, "repo:network", Network{ID: id, Op: "pull", Done: true, Error: errText(err)})
	if err != nil {
		return git.MergeResult{}, err
	}
	a.reload(repo)
	return res, nil
}

// AbortOperation throws away the merge, revert or cherry-pick left half-finished.
func (a *App) AbortOperation(id string) error {
	repo, err := a.session.Get(id)
	if err != nil {
		return err
	}
	if err := repo.AbortOperation(a.ctx); err != nil {
		return err
	}
	a.reload(repo)
	return nil
}

// FinishOperation records the commit that ends it, once nothing is conflicted.
func (a *App) FinishOperation(id, message string) error {
	repo, err := a.session.Get(id)
	if err != nil {
		return err
	}
	if err := repo.FinishOperation(a.ctx, message); err != nil {
		return err
	}
	a.reload(repo)
	return nil
}

// Conflicts lists the unmerged files and which side still has each one.
func (a *App) Conflicts(id string) ([]git.Conflict, error) {
	repo, err := a.session.Get(id)
	if err != nil {
		return nil, err
	}
	list, err := repo.Conflicts(a.ctx)
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []git.Conflict{}
	}
	return list, nil
}

// ConflictText reads a conflicted file cut into the blocks that disagree.
func (a *App) ConflictText(id, path string) (git.ConflictText, error) {
	repo, err := a.session.Get(id)
	if err != nil {
		return git.ConflictText{}, err
	}
	return repo.ConflictText(a.ctx, path)
}

// ResolveLines writes one choice per block and stages the result. The picks are
// plain strings because Wails cannot carry a named string type across the bridge.
func (a *App) ResolveLines(id, path string, picks []string) error {
	repo, err := a.session.Get(id)
	if err != nil {
		return err
	}
	choices := make([]git.ConflictPick, len(picks))
	for i, p := range picks {
		choices[i] = git.ConflictPick(p)
	}
	if err := repo.ResolveLines(a.ctx, path, choices); err != nil {
		return err
	}
	a.reload(repo)
	return nil
}

// ResolveConflict settles one file by keeping one side whole. The side is a
// plain string because Wails cannot carry a named string type across the bridge.
func (a *App) ResolveConflict(id, path, side string) error {
	repo, err := a.session.Get(id)
	if err != nil {
		return err
	}
	if err := repo.Resolve(a.ctx, path, git.ConflictSide(side)); err != nil {
		return err
	}
	a.reload(repo)
	return nil
}

// --- tags ----------------------------------------------------------------------

// Tags lists refs/tags for the sidebar, highest version first.
func (a *App) Tags(id string) ([]git.Tag, error) {
	repo, err := a.session.Get(id)
	if err != nil {
		return nil, err
	}
	return repo.Tags(a.ctx)
}

// CreateTag names a commit; the graph gains a label, so the history is re-read.
func (a *App) CreateTag(id string, opts git.TagOptions) error {
	repo, err := a.session.Get(id)
	if err != nil {
		return err
	}
	if err := repo.CreateTag(a.ctx, opts); err != nil {
		return err
	}
	a.reload(repo)
	return nil
}

// DeleteTag removes it here; a copy already pushed stays on the remote.
func (a *App) DeleteTag(id, name string) error {
	repo, err := a.session.Get(id)
	if err != nil {
		return err
	}
	if err := repo.DeleteTag(a.ctx, name); err != nil {
		return err
	}
	a.reload(repo)
	return nil
}

// PushTag sends one tag to a remote, or removes the remote's copy of it.
func (a *App) PushTag(id, remote, name string, remove bool) ([]git.PushResult, error) {
	repo, err := a.session.Get(id)
	if err != nil {
		return nil, err
	}

	res, err := repo.PushTag(a.ctx, remote, name, remove, a.report(id, "push"))
	runtime.EventsEmit(a.ctx, "repo:network", Network{ID: id, Op: "push", Done: true, Error: errText(err)})
	if res == nil {
		res = []git.PushResult{}
	}
	return res, err
}

// --- revert, cherry-pick and reset ---------------------------------------------

// Revert records the opposite of a commit on top of HEAD.
func (a *App) Revert(id string, opts git.ApplyOptions) (git.ApplyResult, error) {
	repo, err := a.session.Get(id)
	if err != nil {
		return git.ApplyResult{}, err
	}
	res, err := repo.Revert(a.ctx, opts)
	if err != nil {
		return git.ApplyResult{}, err
	}
	a.reload(repo)
	return res, nil
}

// CherryPick replays a commit's change on top of HEAD.
func (a *App) CherryPick(id string, opts git.ApplyOptions) (git.ApplyResult, error) {
	repo, err := a.session.Get(id)
	if err != nil {
		return git.ApplyResult{}, err
	}
	res, err := repo.CherryPick(a.ctx, opts)
	if err != nil {
		return git.ApplyResult{}, err
	}
	a.reload(repo)
	return res, nil
}

// PreviewReset says what the branch would give up, for the window that has to warn.
func (a *App) PreviewReset(id, rev string) (git.ResetPreview, error) {
	repo, err := a.session.Get(id)
	if err != nil {
		return git.ResetPreview{}, err
	}
	return repo.PreviewReset(a.ctx, rev)
}

// Reset moves the branch onto another commit. The mode is a plain string because
// Wails cannot carry a named string type across the bridge.
func (a *App) Reset(id, rev, mode string) error {
	repo, err := a.session.Get(id)
	if err != nil {
		return err
	}
	if err := repo.Reset(a.ctx, rev, git.ResetMode(mode)); err != nil {
		return err
	}
	a.reload(repo)
	return nil
}

// --- ssh keys -----------------------------------------------------------------

// SSHKeys lists the keys in ~/.ssh and says which the agent already holds.
func (a *App) SSHKeys() ([]sshkeys.Key, error) {
	store, err := sshkeys.Open()
	if err != nil {
		return nil, err
	}
	list, err := store.List(a.ctx)
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []sshkeys.Key{}
	}
	return list, nil
}

// GenerateSSHKey makes an ed25519 keypair and hands it to the agent.
func (a *App) GenerateSSHKey(name, comment string) (sshkeys.Key, error) {
	store, err := sshkeys.Open()
	if err != nil {
		return sshkeys.Key{}, err
	}

	key, err := store.Generate(a.ctx, name, comment)
	if err != nil {
		return sshkeys.Key{}, err
	}
	// A key nobody loaded is a key git cannot use, but a warning is enough.
	if err := store.AddToAgent(a.ctx, key.Path); err != nil {
		slog.Warn("the new key was not loaded into the agent", slog.String("err", err.Error()))
	} else {
		key.InAgent = true
	}
	return key, nil
}

// AddSSHKeyToAgent loads an existing key so git can use it without asking.
func (a *App) AddSSHKeyToAgent(path string) error {
	store, err := sshkeys.Open()
	if err != nil {
		return err
	}
	return store.AddToAgent(a.ctx, path)
}

// CopyToClipboard is how the public key gets out of the app and into GitHub.
func (a *App) CopyToClipboard(text string) error {
	return runtime.ClipboardSetText(a.ctx, text)
}

// OpenURL hands a link to the desktop's browser.
func (a *App) OpenURL(url string) {
	runtime.BrowserOpenURL(a.ctx, url)
}

// ScanHostKeys asks a server for its keys. Nothing is trusted by this call.
func (a *App) ScanHostKeys(host string) ([]sshkeys.HostKey, error) {
	store, err := sshkeys.Open()
	if err != nil {
		return nil, err
	}
	return store.ScanHost(a.ctx, host)
}

// TrustHostKeys stores the keys the user approved, never a fresh scan.
func (a *App) TrustHostKeys(host string, keys []sshkeys.HostKey) error {
	store, err := sshkeys.Open()
	if err != nil {
		return err
	}
	return store.TrustHost(a.ctx, host, keys)
}
