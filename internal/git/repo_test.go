package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The tests in this file drive the real git binary.

// testRepo builds a throwaway repository with a merge, a tag and a dirty working tree.
func testRepo(t *testing.T) *Repo {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}

	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Tester", "GIT_AUTHOR_EMAIL=t@example.com",
			"GIT_COMMITTER_NAME=Tester", "GIT_COMMITTER_EMAIL=t@example.com",
			// Fixed dates keep the fixture deterministic.
			"GIT_AUTHOR_DATE=2026-01-01T10:00:00+01:00",
			"GIT_COMMITTER_DATE=2026-01-01T10:00:00+01:00",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	run("init", "-q", "-b", "main")
	write("a.txt", "uno\n")
	run("add", ".")
	run("commit", "-qm", "primo commit")

	write("a.txt", "uno\ndue\n")
	run("commit", "-qam", "secondo commit\n\nCon un body\nsu piu righe.")

	run("checkout", "-qb", "feature")
	write("b.txt", "feat\n")
	run("add", ".")
	run("commit", "-qm", "aggiunge feature")

	run("checkout", "-q", "main")
	write("a.txt", "uno\ndue\ntre\n")
	run("commit", "-qam", "terzo su main")
	run("merge", "-q", "--no-ff", "feature", "-m", "Merge branch 'feature'")
	run("tag", "v1.0")

	// Leave the working tree dirty in three distinct ways.
	write("a.txt", "uno\ndue\ntre\nmodificato\n")
	write("untracked.txt", "nuovo\n")
	write("c.txt", "staged\n")
	run("add", "c.txt")

	repo, err := Open(context.Background(), dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return repo
}

func TestOpenRejectsNonRepository(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	_, err := Open(context.Background(), t.TempDir())
	if !errors.Is(err, ErrNotARepository) {
		t.Fatalf("Open on a plain directory: got %v, want ErrNotARepository", err)
	}
}

func TestOpenFromSubdirectory(t *testing.T) {
	repo := testRepo(t)
	sub := filepath.Join(repo.Root(), "sub", "deeper")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	// Opening any subdirectory must resolve back to the working tree root, exactly like.
	got, err := Open(context.Background(), sub)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if got.Root() != repo.Root() {
		t.Errorf("Root() = %q, want %q", got.Root(), repo.Root())
	}
	if got.GitDir() == "" {
		t.Error("GitDir() is empty")
	}
}

func TestLogAgainstRealRepo(t *testing.T) {
	repo := testRepo(t)

	commits, err := repo.Commits(context.Background(), LogOptions{All: true})
	if err != nil {
		t.Fatalf("Commits: %v", err)
	}
	if len(commits) != 5 {
		t.Fatalf("got %d commits, want 5: %+v", len(commits), commits)
	}

	// --date-order puts children before parents.
	if !commits[0].IsMerge() {
		t.Errorf("first commit %q is not a merge", commits[0].Subject)
	}
	if len(commits[0].Parents) != 2 {
		t.Errorf("merge has %d parents, want 2", len(commits[0].Parents))
	}
	last := commits[len(commits)-1]
	if !last.IsRoot() {
		t.Errorf("last commit %q has parents %v, want none", last.Subject, last.Parents)
	}
	if last.Subject != "primo commit" {
		t.Errorf("last commit subject = %q", last.Subject)
	}

	// The multi-line body must survive the round trip through the 0x1E record separator.
	var found bool
	for _, c := range commits {
		if c.Subject == "secondo commit" {
			found = true
			if c.Body != "Con un body\nsu piu righe." {
				t.Errorf("Body = %q", c.Body)
			}
		}
	}
	if !found {
		t.Error("commit with the multi-line body not found")
	}

	for _, c := range commits {
		if len(c.Hash) != 40 && len(c.Hash) != 64 { // sha1 or sha256
			t.Errorf("suspicious hash %q", c.Hash)
		}
		if c.Author.When.IsZero() {
			t.Errorf("commit %s has a zero author date", c.Short())
		}
	}
}

func TestLogLimitAndEarlyExit(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()

	got, err := repo.Commits(ctx, LogOptions{All: true, Limit: 2})
	if err != nil {
		t.Fatalf("Commits: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("got %d commits, want 2", len(got))
	}

	// A callback that stops early must surface its own error unchanged, not a "signal.
	stop := errors.New("stop")
	var seen int
	err = repo.Log(ctx, LogOptions{All: true}, func(Commit) error {
		seen++
		if seen == 2 {
			return stop
		}
		return nil
	})
	if !errors.Is(err, stop) {
		t.Fatalf("Log: got %v, want the sentinel returned by the callback", err)
	}
	if seen != 2 {
		t.Errorf("callback ran %d times, want 2", seen)
	}
}

func TestLogCancellation(t *testing.T) {
	repo := testRepo(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled before we start

	err := repo.Log(ctx, LogOptions{All: true}, func(Commit) error { return nil })
	if err == nil {
		t.Fatal("expected an error from a cancelled context")
	}
}

func TestRefsAgainstRealRepo(t *testing.T) {
	repo := testRepo(t)

	refs, err := repo.Refs(context.Background())
	if err != nil {
		t.Fatalf("Refs: %v", err)
	}

	byShort := map[string]Ref{}
	for _, r := range refs {
		byShort[r.Short] = r
	}

	main, ok := byShort["main"]
	if !ok {
		t.Fatalf("branch main missing from %+v", refs)
	}
	if main.Kind != RefBranch {
		t.Errorf("main.Kind = %q, want branch", main.Kind)
	}
	if !main.Head {
		t.Error("main.Head = false, want true (it is the checked out branch)")
	}

	if feature, ok := byShort["feature"]; !ok {
		t.Error("branch feature missing")
	} else if feature.Head {
		t.Error("feature.Head = true, want false")
	}

	tag, ok := byShort["v1.0"]
	if !ok {
		t.Fatal("tag v1.0 missing")
	}
	if tag.Kind != RefTag {
		t.Errorf("v1.0.Kind = %q, want tag", tag.Kind)
	}
	if tag.Hash != main.Hash {
		t.Errorf("tag hash %q != main hash %q", tag.Hash, main.Hash)
	}
}

func TestStatusAgainstRealRepo(t *testing.T) {
	repo := testRepo(t)

	st, err := repo.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.Head != "main" {
		t.Errorf("Head = %q, want main", st.Head)
	}
	if st.Detached {
		t.Error("Detached = true, want false")
	}
	if st.IsClean() {
		t.Fatal("IsClean() = true, but the fixture leaves the tree dirty")
	}

	byPath := map[string]FileStatus{}
	for _, f := range st.Files {
		byPath[f.Path] = f
	}

	if f, ok := byPath["a.txt"]; !ok {
		t.Error("a.txt missing")
	} else if f.Unstaged != ChangeModified || f.Staged != ChangeNone {
		t.Errorf("a.txt = %+v, want unstaged modification only", f)
	}
	if f, ok := byPath["c.txt"]; !ok {
		t.Error("c.txt missing")
	} else if f.Staged != ChangeAdded {
		t.Errorf("c.txt = %+v, want a staged addition", f)
	}
	if f, ok := byPath["untracked.txt"]; !ok {
		t.Error("untracked.txt missing")
	} else if !f.Untracked {
		t.Errorf("untracked.txt = %+v, want Untracked", f)
	}
}

func TestStatusDetachedHead(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()

	commits, err := repo.Commits(ctx, LogOptions{All: true})
	if err != nil {
		t.Fatal(err)
	}
	// -f because the fixture deliberately leaves the working tree dirty.
	cmd := exec.Command("git", "checkout", "-q", "-f", commits[len(commits)-1].Hash)
	cmd.Dir = repo.Root()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("checkout: %v\n%s", err, out)
	}

	st, err := repo.Status(ctx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !st.Detached {
		t.Error("Detached = false, want true")
	}
	if st.Head != "" {
		t.Errorf("Head = %q, want empty on a detached HEAD", st.Head)
	}
}
