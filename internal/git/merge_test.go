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

// mergeRepo builds main and altro so that they have both diverged from a shared
// base, which is the only state where a merge has anything to decide.
func mergeRepo(t *testing.T) (*Repo, func(args ...string), func(name, content string)) {
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
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	run("init", "-q", "-b", "main")
	write("shared.txt", "base\n")
	run("add", ".")
	run("commit", "-qm", "base")

	run("switch", "-qc", "altro")
	write("theirs.txt", "loro\n")
	run("add", ".")
	run("commit", "-qm", "solo su altro")

	run("switch", "-q", "main")
	write("ours.txt", "nostro\n")
	run("add", ".")
	run("commit", "-qm", "solo su main")

	repo, err := Open(context.Background(), dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return repo, run, write
}

func TestMerge(t *testing.T) {
	ctx := context.Background()

	t.Run("diverged branches make a merge commit", func(t *testing.T) {
		repo, run, _ := mergeRepo(t)
		res, err := repo.Merge(ctx, MergeOptions{Ref: "altro"})
		if err != nil {
			t.Fatalf("Merge: %v", err)
		}
		if res.Kind != MergeCommit {
			t.Errorf("Kind = %q, want %q", res.Kind, MergeCommit)
		}
		if _, merging := repo.MergeHead(ctx); merging {
			t.Error("the merge was left unfinished")
		}
		run("cat-file", "-e", "HEAD^2")
	})

	t.Run("a branch that is only ahead fast-forwards", func(t *testing.T) {
		repo, run, _ := mergeRepo(t)
		// Rewind main onto the shared base so altro is strictly ahead of it.
		run("reset", "-q", "--hard", "HEAD~1")

		res, err := repo.Merge(ctx, MergeOptions{Ref: "altro"})
		if err != nil {
			t.Fatalf("Merge: %v", err)
		}
		if res.Kind != MergeFastForward {
			t.Errorf("Kind = %q, want %q", res.Kind, MergeFastForward)
		}
	})

	t.Run("no fast forward records a merge commit anyway", func(t *testing.T) {
		repo, run, _ := mergeRepo(t)
		run("reset", "-q", "--hard", "HEAD~1")

		res, err := repo.Merge(ctx, MergeOptions{Ref: "altro", NoFastForward: true})
		if err != nil {
			t.Fatalf("Merge: %v", err)
		}
		if res.Kind != MergeCommit {
			t.Errorf("Kind = %q, want %q", res.Kind, MergeCommit)
		}
	})

	t.Run("a branch already contained is up to date", func(t *testing.T) {
		repo, _, _ := mergeRepo(t)
		if _, err := repo.Merge(ctx, MergeOptions{Ref: "altro"}); err != nil {
			t.Fatal(err)
		}
		res, err := repo.Merge(ctx, MergeOptions{Ref: "altro"})
		if err != nil {
			t.Fatalf("Merge: %v", err)
		}
		if res.Kind != MergeUpToDate {
			t.Errorf("Kind = %q, want %q", res.Kind, MergeUpToDate)
		}
	})

	t.Run("an unknown ref is refused before anything moves", func(t *testing.T) {
		repo, _, _ := mergeRepo(t)
		before := headHash(t, repo)
		if _, err := repo.Merge(ctx, MergeOptions{Ref: "non-esiste"}); err == nil {
			t.Fatal("Merge accepted a ref that is not there")
		}
		if headHash(t, repo) != before {
			t.Error("HEAD moved for a merge that never ran")
		}
	})

	// A dirty tree is the one case where git would silently eat work.
	t.Run("uncommitted work stops the merge", func(t *testing.T) {
		repo, run, write := mergeRepo(t)
		run("switch", "-q", "altro")
		write("shared.txt", "cambiato su altro\n")
		run("commit", "-qam", "tocca shared")
		run("switch", "-q", "main")

		write("shared.txt", "lavoro non salvato\n")
		_, err := repo.Merge(ctx, MergeOptions{Ref: "altro"})
		if !errors.Is(err, ErrWouldOverwrite) {
			t.Fatalf("Merge: got %v, want ErrWouldOverwrite", err)
		}
		got, _ := os.ReadFile(filepath.Join(repo.Root(), "shared.txt"))
		if string(got) != "lavoro non salvato\n" {
			t.Errorf("the working file was overwritten: %q", got)
		}
	})
}

func TestMergeConflict(t *testing.T) {
	ctx := context.Background()

	// bothTouch makes the same file differ on the two branches.
	bothTouch := func(t *testing.T) (*Repo, func(args ...string)) {
		t.Helper()
		repo, run, write := mergeRepo(t)
		run("switch", "-q", "altro")
		write("shared.txt", "loro\n")
		run("commit", "-qam", "loro su shared")
		run("switch", "-q", "main")
		write("shared.txt", "nostro\n")
		run("commit", "-qam", "nostro su shared")
		return repo, run
	}

	t.Run("the conflicted paths come back with the result", func(t *testing.T) {
		repo, _ := bothTouch(t)
		res, err := repo.Merge(ctx, MergeOptions{Ref: "altro"})
		if err != nil {
			t.Fatalf("Merge: %v", err)
		}
		if res.Kind != MergeConflict {
			t.Fatalf("Kind = %q, want %q", res.Kind, MergeConflict)
		}
		if len(res.Conflicts) != 1 || res.Conflicts[0] != "shared.txt" {
			t.Errorf("Conflicts = %v", res.Conflicts)
		}
		if _, merging := repo.MergeHead(ctx); !merging {
			t.Error("MERGE_HEAD is missing during a conflict")
		}
	})

	t.Run("abort puts everything back", func(t *testing.T) {
		repo, _ := bothTouch(t)
		before := headHash(t, repo)
		if _, err := repo.Merge(ctx, MergeOptions{Ref: "altro"}); err != nil {
			t.Fatal(err)
		}
		if err := repo.AbortMerge(ctx); err != nil {
			t.Fatalf("AbortMerge: %v", err)
		}
		if _, merging := repo.MergeHead(ctx); merging {
			t.Error("the merge is still in progress after abort")
		}
		if headHash(t, repo) != before {
			t.Error("HEAD moved despite the abort")
		}
		got, _ := os.ReadFile(filepath.Join(repo.Root(), "shared.txt"))
		if string(got) != "nostro\n" {
			t.Errorf("shared.txt = %q, want our version back", got)
		}
	})

	t.Run("aborting when nothing is merging says so", func(t *testing.T) {
		repo, _, _ := mergeRepo(t)
		if err := repo.AbortMerge(ctx); !errors.Is(err, ErrNoMerge) {
			t.Errorf("AbortMerge: got %v, want ErrNoMerge", err)
		}
	})

	t.Run("keeping one side resolves the file", func(t *testing.T) {
		repo, _ := bothTouch(t)
		if _, err := repo.Merge(ctx, MergeOptions{Ref: "altro"}); err != nil {
			t.Fatal(err)
		}
		if err := repo.Resolve(ctx, "shared.txt", SideTheirs); err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		got, _ := os.ReadFile(filepath.Join(repo.Root(), "shared.txt"))
		if string(got) != "loro\n" {
			t.Errorf("shared.txt = %q, want their version", got)
		}
		left, _ := repo.ConflictPaths(ctx)
		if len(left) != 0 {
			t.Errorf("still conflicted: %v", left)
		}
		if err := repo.FinishMerge(ctx, ""); err != nil {
			t.Fatalf("FinishMerge: %v", err)
		}
		if _, merging := repo.MergeHead(ctx); merging {
			t.Error("the merge did not finish")
		}
	})

	t.Run("finishing with conflicts left is refused", func(t *testing.T) {
		repo, _ := bothTouch(t)
		if _, err := repo.Merge(ctx, MergeOptions{Ref: "altro"}); err != nil {
			t.Fatal(err)
		}
		if err := repo.FinishMerge(ctx, ""); !errors.Is(err, ErrUnresolved) {
			t.Errorf("FinishMerge: got %v, want ErrUnresolved", err)
		}
	})

	// git checkout --ours cannot restore a file our side deleted, so this path
	// has to remove it instead. It reports success either way.
	t.Run("a file one side deleted resolves both ways", func(t *testing.T) {
		repo, run, write := mergeRepo(t)
		run("switch", "-q", "altro")
		write("shared.txt", "loro\n")
		run("commit", "-qam", "loro su shared")
		run("switch", "-q", "main")
		run("rm", "-q", "shared.txt")
		run("commit", "-qm", "tolto shared")

		res, err := repo.Merge(ctx, MergeOptions{Ref: "altro"})
		if err != nil {
			t.Fatalf("Merge: %v", err)
		}
		if res.Kind != MergeConflict {
			t.Fatalf("Kind = %q", res.Kind)
		}

		list, err := repo.Conflicts(ctx)
		if err != nil || len(list) != 1 {
			t.Fatalf("Conflicts = %v, %v", list, err)
		}
		if list[0].Ours || !list[0].Theirs {
			t.Errorf("Conflict = %+v, want ours missing", list[0])
		}
		if got := list[0].DeletedBy(); got != SideOurs {
			t.Errorf("DeletedBy = %q, want ours", got)
		}

		if err := repo.Resolve(ctx, "shared.txt", SideOurs); err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if _, err := os.Stat(filepath.Join(repo.Root(), "shared.txt")); !os.IsNotExist(err) {
			t.Error("the file our side deleted is still there")
		}
		if err := repo.FinishMerge(ctx, "unito"); err != nil {
			t.Fatalf("FinishMerge: %v", err)
		}
	})
}

func TestPreviewMerge(t *testing.T) {
	ctx := context.Background()

	t.Run("diverged branches need a merge commit", func(t *testing.T) {
		repo, _, _ := mergeRepo(t)
		p, err := repo.PreviewMerge(ctx, "altro", "main")
		if err != nil {
			t.Fatal(err)
		}
		if p.UpToDate || p.FastForward {
			t.Errorf("preview = %+v, want neither", p)
		}
	})

	t.Run("a target that has not moved can fast-forward", func(t *testing.T) {
		repo, run, _ := mergeRepo(t)
		run("reset", "-q", "--hard", "HEAD~1")
		p, err := repo.PreviewMerge(ctx, "altro", "main")
		if err != nil {
			t.Fatal(err)
		}
		if !p.FastForward || p.UpToDate {
			t.Errorf("preview = %+v, want fast-forward", p)
		}
	})

	t.Run("a source already contained is up to date", func(t *testing.T) {
		repo, _, _ := mergeRepo(t)
		// The shared base is behind main, so merging it in would change nothing.
		p, err := repo.PreviewMerge(ctx, "HEAD~1", "main")
		if err != nil {
			t.Fatal(err)
		}
		if !p.UpToDate || p.FastForward {
			t.Errorf("preview = %+v, want up to date", p)
		}
	})
}

func TestPull(t *testing.T) {
	ctx := context.Background()

	t.Run("a branch with no upstream says so", func(t *testing.T) {
		repo, _, _ := mergeRepo(t)
		_, err := repo.Pull(ctx, PullOptions{}, nil)
		if !errors.Is(err, ErrNoUpstream) {
			t.Errorf("Pull: got %v, want ErrNoUpstream", err)
		}
	})

	t.Run("detached HEAD has nothing to pull into", func(t *testing.T) {
		repo, run, _ := mergeRepo(t)
		run("switch", "-q", "--detach", "HEAD")
		_, err := repo.Pull(ctx, PullOptions{}, nil)
		if !errors.Is(err, ErrNoUpstream) {
			t.Errorf("Pull: got %v, want ErrNoUpstream", err)
		}
	})

	t.Run("what the other clone pushed arrives and merges", func(t *testing.T) {
		_, ours, theirs := netRepos(t)

		// The other clone publishes a commit ours has never seen.
		gitIn(t, theirs.Root(), "commit", "--allow-empty", "-qm", "dal secondo clone")
		gitIn(t, theirs.Root(), "push", "-q", "origin", "HEAD")

		res, err := ours.Pull(ctx, PullOptions{}, nil)
		if err != nil {
			t.Fatalf("Pull: %v", err)
		}
		if res.Kind != MergeFastForward {
			t.Errorf("Kind = %q, want %q", res.Kind, MergeFastForward)
		}

		out := gitIn(t, ours.Root(), "log", "-1", "--format=%s")
		if got := strings.TrimSpace(out); got != "dal secondo clone" {
			t.Errorf("subject = %q", got)
		}
	})
}

// gitIn runs git in a directory and returns stdout.
func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=T", "GIT_AUTHOR_EMAIL=t@e.c",
		"GIT_COMMITTER_NAME=T", "GIT_COMMITTER_EMAIL=t@e.c",
	)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out)
}

// headHash is the commit HEAD points at, for checking nothing moved.
func headHash(t *testing.T, repo *Repo) string {
	t.Helper()
	st, err := repo.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return st.Oid
}
