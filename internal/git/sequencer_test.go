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

// applyRepo has one commit on main and one on altro, over a shared base.
func applyRepo(t *testing.T) (*Repo, func(args ...string), func(name, content string)) {
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
	// Reverting and picking commit through our runner, which misses the env above.
	run("config", "user.name", "Tester")
	run("config", "user.email", "t@example.com")
	write("shared.txt", "base\n")
	run("add", ".")
	run("commit", "-qm", "base")

	run("switch", "-qc", "altro")
	write("altro.txt", "loro\n")
	run("add", ".")
	run("commit", "-qm", "solo su altro")

	run("switch", "-q", "main")
	write("main.txt", "nostro\n")
	run("add", ".")
	run("commit", "-qm", "solo su main")

	repo, err := Open(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	return repo, run, write
}

func exists(t *testing.T, repo *Repo, name string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(repo.Root(), name))
	return err == nil
}

func TestRevertUndoesTheCommit(t *testing.T) {
	ctx := context.Background()
	repo, _, _ := applyRepo(t)

	res, err := repo.Revert(ctx, ApplyOptions{Rev: "HEAD"})
	if err != nil {
		t.Fatalf("Revert: %v", err)
	}
	if res.Kind != ApplyDone {
		t.Errorf("Kind = %q, want %q", res.Kind, ApplyDone)
	}
	if exists(t, repo, "main.txt") {
		t.Error("the reverted file is still in the working tree")
	}

	// The commit it undoes stays in the history: that is the point of a revert.
	var subjects []string
	if err := repo.Log(ctx, LogOptions{}, func(c Commit) error {
		subjects = append(subjects, c.Subject)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(subjects) != 3 || !strings.HasPrefix(subjects[0], "Revert ") {
		t.Errorf("history is %v, want a revert on top of both commits", subjects)
	}
}

func TestRevertNoCommitOnlyStages(t *testing.T) {
	ctx := context.Background()
	repo, _, _ := applyRepo(t)

	before, err := repo.resolve(ctx, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Revert(ctx, ApplyOptions{Rev: "HEAD", NoCommit: true}); err != nil {
		t.Fatalf("Revert: %v", err)
	}

	if after, _ := repo.resolve(ctx, "HEAD"); after != before {
		t.Error("HEAD moved despite NoCommit")
	}
	st, err := repo.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Files) != 1 || st.Files[0].Staged != ChangeDeleted {
		t.Errorf("status is %+v, want the deletion staged", st.Files)
	}
}

func TestCherryPickBringsTheCommitOver(t *testing.T) {
	ctx := context.Background()
	repo, _, _ := applyRepo(t)

	res, err := repo.CherryPick(ctx, ApplyOptions{Rev: "altro"})
	if err != nil {
		t.Fatalf("CherryPick: %v", err)
	}
	if res.Kind != ApplyDone || res.Head == "" {
		t.Errorf("result is %+v, want a finished pick", res)
	}
	if !exists(t, repo, "altro.txt") {
		t.Error("the picked file never arrived")
	}
	// The commit on altro is copied, not moved: the branch keeps its own.
	if _, err := repo.resolve(ctx, "altro"); err != nil {
		t.Errorf("altro lost its commit: %v", err)
	}
}

func TestCherryPickConflict(t *testing.T) {
	ctx := context.Background()
	repo, run, write := applyRepo(t)

	// Both sides rewrite the same file, which is the only way to force a conflict.
	run("switch", "-q", "altro")
	write("shared.txt", "loro\n")
	run("add", ".")
	run("commit", "-qm", "shared su altro")
	run("switch", "-q", "main")
	write("shared.txt", "nostro\n")
	run("add", ".")
	run("commit", "-qm", "shared su main")

	res, err := repo.CherryPick(ctx, ApplyOptions{Rev: "altro"})
	if err != nil {
		t.Fatalf("CherryPick: %v", err)
	}
	if res.Kind != ApplyConflict {
		t.Fatalf("Kind = %q, want %q", res.Kind, ApplyConflict)
	}
	if len(res.Conflicts) != 1 || res.Conflicts[0] != "shared.txt" {
		t.Errorf("Conflicts = %v", res.Conflicts)
	}

	if op, _ := repo.Operation(); op != OpCherryPick {
		t.Errorf("Operation() = %q, want %q", op, OpCherryPick)
	}
	// The bar has to say what is being applied, and the status is where it reads it.
	st, err := repo.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.Operation != OpCherryPick || !strings.Contains(st.OperationFrom, "shared su altro") {
		t.Errorf("status says %q from %q", st.Operation, st.OperationFrom)
	}

	if err := repo.Resolve(ctx, "shared.txt", SideTheirs); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if err := repo.FinishOperation(ctx, ""); err != nil {
		t.Fatalf("FinishOperation: %v", err)
	}
	if op, _ := repo.Operation(); op != OpNone {
		t.Errorf("the pick did not finish, operation is %q", op)
	}
	if b, _ := os.ReadFile(filepath.Join(repo.Root(), "shared.txt")); string(b) != "loro\n" {
		t.Errorf("shared.txt = %q, want the picked version", b)
	}
}

func TestAbortOperationPutsEverythingBack(t *testing.T) {
	ctx := context.Background()
	repo, run, write := applyRepo(t)

	run("switch", "-q", "altro")
	write("shared.txt", "loro\n")
	run("add", ".")
	run("commit", "-qm", "shared su altro")
	run("switch", "-q", "main")
	write("shared.txt", "nostro\n")
	run("add", ".")
	run("commit", "-qm", "shared su main")

	before, _ := repo.resolve(ctx, "HEAD")
	if _, err := repo.CherryPick(ctx, ApplyOptions{Rev: "altro"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.AbortOperation(ctx); err != nil {
		t.Fatalf("AbortOperation: %v", err)
	}

	if op, _ := repo.Operation(); op != OpNone {
		t.Errorf("the pick is still open after abort, operation is %q", op)
	}
	if after, _ := repo.resolve(ctx, "HEAD"); after != before {
		t.Error("HEAD moved despite the abort")
	}
	if b, _ := os.ReadFile(filepath.Join(repo.Root(), "shared.txt")); string(b) != "nostro\n" {
		t.Errorf("shared.txt = %q, want our version back", b)
	}
}

func TestAbortWithNothingOpen(t *testing.T) {
	ctx := context.Background()
	repo, _, _ := applyRepo(t)

	if err := repo.AbortOperation(ctx); !errors.Is(err, ErrNoOperation) {
		t.Errorf("AbortOperation = %v, want ErrNoOperation", err)
	}
	if err := repo.FinishOperation(ctx, ""); !errors.Is(err, ErrNoOperation) {
		t.Errorf("FinishOperation = %v, want ErrNoOperation", err)
	}
}

func TestCherryPickAlreadyApplied(t *testing.T) {
	ctx := context.Background()
	repo, _, _ := applyRepo(t)

	if _, err := repo.CherryPick(ctx, ApplyOptions{Rev: "altro"}); err != nil {
		t.Fatal(err)
	}
	// git exits 1 and leaves the sequencer open; both halves have to be handled.
	if _, err := repo.CherryPick(ctx, ApplyOptions{Rev: "altro"}); !errors.Is(err, ErrAlreadyApplied) {
		t.Fatalf("picking it twice = %v, want ErrAlreadyApplied", err)
	}
	if op, _ := repo.Operation(); op != OpNone {
		t.Errorf("the empty pick left %q behind", op)
	}
}

func TestApplyAMergeNeedsASide(t *testing.T) {
	ctx := context.Background()
	repo, run, _ := applyRepo(t)

	run("merge", "-q", "--no-ff", "-m", "unione", "altro")

	if _, err := repo.Revert(ctx, ApplyOptions{Rev: "HEAD"}); !errors.Is(err, ErrMainlineNeeded) {
		t.Errorf("reverting a merge with no side = %v, want ErrMainlineNeeded", err)
	}
	// Side 1 is the branch the merge landed on, so this undoes what came in.
	if _, err := repo.Revert(ctx, ApplyOptions{Rev: "HEAD", Mainline: 1}); err != nil {
		t.Fatalf("Revert with Mainline: %v", err)
	}
	if exists(t, repo, "altro.txt") {
		t.Error("what the merge brought in is still there")
	}
}

func TestApplyRefusesADirtyTree(t *testing.T) {
	ctx := context.Background()
	repo, _, write := applyRepo(t)

	// The pick would rewrite this file, and git will not overwrite unsaved work.
	write("altro.txt", "non salvato\n")
	if _, err := repo.CherryPick(ctx, ApplyOptions{Rev: "altro"}); !errors.Is(err, ErrWouldOverwrite) {
		t.Errorf("CherryPick over an uncommitted change = %v, want ErrWouldOverwrite", err)
	}
}

func TestApplyRejectsANonCommit(t *testing.T) {
	ctx := context.Background()
	repo, _, _ := applyRepo(t)

	for _, rev := range []string{"", "  ", "--force"} {
		if _, err := repo.CherryPick(ctx, ApplyOptions{Rev: rev}); err == nil {
			t.Errorf("CherryPick(%q) was accepted", rev)
		}
	}
}
