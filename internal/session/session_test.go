package session

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

// newRepo builds a throwaway repository with n commits on main.
func newRepo(t *testing.T, n int) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}

	dir := t.TempDir()
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME=Tester", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=Tester", "GIT_COMMITTER_EMAIL=t@example.com",
		"GIT_AUTHOR_DATE=2026-01-01T10:00:00+01:00",
		"GIT_COMMITTER_DATE=2026-01-01T10:00:00+01:00",
	)
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	run("init", "-q", "-b", "main")
	for i := range n {
		name := filepath.Join(dir, "f.txt")
		if err := os.WriteFile(name, fmt.Appendf(nil, "%d\n", i), 0o644); err != nil {
			t.Fatal(err)
		}
		run("add", ".")
		run("commit", "-qm", fmt.Sprintf("commit %d", i))
	}
	return dir
}

func TestOpenIsIdempotent(t *testing.T) {
	dir := newRepo(t, 3)
	s := New()
	ctx := context.Background()

	a, err := s.Open(ctx, dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	// Opening the same working tree twice must reuse the entry, not reload it.
	b, err := s.Open(ctx, dir)
	if err != nil {
		t.Fatalf("Open again: %v", err)
	}
	if a != b {
		t.Error("Open returned a different *Repo for the same path")
	}
	if got := len(s.List()); got != 1 {
		t.Errorf("List() has %d entries, want 1", got)
	}
}

func TestOpenFromSubdirectoryIsTheSameRepo(t *testing.T) {
	dir := newRepo(t, 2)
	sub := filepath.Join(dir, "nested")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	s := New()
	ctx := context.Background()
	a, _ := s.Open(ctx, dir)
	b, err := s.Open(ctx, sub)
	if err != nil {
		t.Fatalf("Open subdirectory: %v", err)
	}
	if a != b {
		t.Error("a subdirectory opened as a separate repository")
	}
}

func TestGetUnknownRepo(t *testing.T) {
	if _, err := New().Get("/nope"); err == nil {
		t.Fatal("expected an error for an unopened repository")
	}
}

func TestLoadAndRange(t *testing.T) {
	const n = 25
	dir := newRepo(t, n)

	s := New()
	repo, err := s.Open(context.Background(), dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	var lastLoaded, lastGen int
	var sawDone bool
	if err := repo.Load(context.Background(), func(gen, loaded, width int, done bool) {
		lastLoaded, lastGen = loaded, gen
		if done {
			sawDone = true
		}
	}); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !sawDone {
		t.Error("progress callback never reported done")
	}
	if lastLoaded != n {
		t.Errorf("final progress = %d, want %d", lastLoaded, n)
	}
	if repo.Count() != n {
		t.Fatalf("Count() = %d, want %d", repo.Count(), n)
	}

	// The generation is what tells the frontend to drop its cached pages.
	if lastGen != repo.Gen() {
		t.Errorf("progress reported gen %d, but Gen() is %d", lastGen, repo.Gen())
	}
	before := repo.Gen()
	if err := repo.Load(context.Background(), nil); err != nil {
		t.Fatalf("second Load: %v", err)
	}
	if repo.Gen() <= before {
		t.Errorf("Gen() = %d after a reload, want more than %d", repo.Gen(), before)
	}

	// Newest first: git log --topo-order puts children before parents.
	page := repo.Range(0, 10)
	if len(page) != 10 {
		t.Fatalf("Range(0,10) returned %d commits", len(page))
	}
	if want := fmt.Sprintf("commit %d", n-1); page[0].Commit.Subject != want {
		t.Errorf("first commit = %q, want %q", page[0].Commit.Subject, want)
	}

	// The graph is built in the same pass, so every entry already carries its lane.
	if repo.GraphWidth() != 1 {
		t.Errorf("GraphWidth() = %d, want 1 for linear history", repo.GraphWidth())
	}
	for i, e := range page {
		if e.Graph.Lane != 0 {
			t.Errorf("entry %d sits in lane %d, want 0", i, e.Graph.Lane)
		}
	}

	tests := []struct {
		name          string
		offset, count int
		want          int
	}{
		{"exact page", 0, 10, 10},
		{"partial tail", 20, 10, 5},
		{"past the end", 100, 10, 0},
		{"negative offset clamps to zero", -5, 3, 3},
		{"zero count", 0, 0, 0},
		{"negative count", 0, -1, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := len(repo.Range(tt.offset, tt.count)); got != tt.want {
				t.Errorf("Range(%d,%d) returned %d, want %d", tt.offset, tt.count, got, tt.want)
			}
		})
	}
}

func TestRangeDoesNotAliasTheCache(t *testing.T) {
	dir := newRepo(t, 5)
	s := New()
	repo, _ := s.Open(context.Background(), dir)
	if err := repo.Load(context.Background(), nil); err != nil {
		t.Fatalf("Load: %v", err)
	}

	page := repo.Range(0, 3)
	page[0].Commit.Subject = "mutated by the caller"

	if again := repo.Range(0, 3); again[0].Commit.Subject == "mutated by the caller" {
		t.Error("Range handed out a slice aliasing the cache")
	}
}

// TestConcurrentReadsDuringLoad is the reason this package exists: the UI scrolls.
func TestConcurrentReadsDuringLoad(t *testing.T) {
	dir := newRepo(t, 40)
	s := New()
	repo, _ := s.Open(context.Background(), dir)

	var wg sync.WaitGroup
	stop := make(chan struct{})

	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_ = repo.Range(0, 20)
					_ = repo.Count()
					_, _, _ = repo.State()
				}
			}
		}()
	}

	if err := repo.Load(context.Background(), func(int, int, int, bool) {}); err != nil {
		t.Fatalf("Load: %v", err)
	}
	close(stop)
	wg.Wait()

	if repo.Count() != 40 {
		t.Errorf("Count() = %d, want 40", repo.Count())
	}
}

func TestLoadCancellation(t *testing.T) {
	dir := newRepo(t, 10)
	s := New()
	repo, _ := s.Open(context.Background(), dir)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := repo.Load(ctx, nil); err == nil {
		t.Fatal("expected an error from a cancelled context")
	}
	// Even a failed load must leave the repository readable rather than in a permanently.
	_, loading, loadErr := repo.State()
	if loading {
		t.Error("still reports loading after a cancelled load")
	}
	if loadErr == nil {
		t.Error("State() lost the load error")
	}
}

func TestReloadReplacesTheCache(t *testing.T) {
	dir := newRepo(t, 6)
	s := New()
	repo, _ := s.Open(context.Background(), dir)

	if err := repo.Load(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	first := repo.Count()

	// A second Load must start over, not append to what is already cached.
	if err := repo.Load(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if repo.Count() != first {
		t.Errorf("Count() = %d after reload, want %d", repo.Count(), first)
	}
}

func TestCloseForgetsTheRepo(t *testing.T) {
	dir := newRepo(t, 2)
	s := New()
	repo, _ := s.Open(context.Background(), dir)

	s.Close(repo.Info.ID)
	if _, err := s.Get(repo.Info.ID); err == nil {
		t.Error("Get succeeded after Close")
	}
	if got := len(s.List()); got != 0 {
		t.Errorf("List() has %d entries after Close, want 0", got)
	}
}

func TestOverlappingLoadsDoNotMix(t *testing.T) {
	// Stashing starts two walks at once: one from the binding, one from the watcher.
	dir := newRepo(t, 400)
	s := New()
	repo, _ := s.Open(context.Background(), dir)

	if err := repo.Load(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	want := repo.Count()

	for range 20 {
		var wg sync.WaitGroup
		for range 2 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = repo.Load(context.Background(), nil)
			}()
		}
		wg.Wait()

		if got := repo.Count(); got != want {
			t.Fatalf("Count() = %d after two overlapping loads, want %d", got, want)
		}
	}
}
