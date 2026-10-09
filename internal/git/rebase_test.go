package git

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// subjects lists the history of HEAD, newest first.
func subjects(t *testing.T, repo *Repo) []string {
	t.Helper()
	var out []string
	if err := repo.Log(context.Background(), LogOptions{}, func(c Commit) error {
		out = append(out, c.Subject)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return out
}

// conflictingRebase gives main two commits that each clash with altro over
// shared.txt, with a clean commit between them, so a rebase stops twice.
func conflictingRebase(t *testing.T) (*Repo, func(args ...string), func(name, content string)) {
	t.Helper()
	repo, run, write := applyRepo(t)

	run("switch", "-q", "altro")
	write("shared.txt", "loro\n")
	run("add", ".")
	run("commit", "-qm", "shared su altro")

	run("switch", "-q", "main")
	write("shared.txt", "nostro\n")
	run("add", ".")
	run("commit", "-qm", "primo scontro")
	write("pulito.txt", "pulito\n")
	run("add", ".")
	run("commit", "-qm", "pulito")
	write("shared.txt", "ancora nostro\n")
	run("add", ".")
	run("commit", "-qm", "secondo scontro")
	return repo, run, write
}

func TestRebaseReplaysTheBranch(t *testing.T) {
	ctx := context.Background()
	repo, _, _ := applyRepo(t)

	res, err := repo.Rebase(ctx, RebaseOptions{Onto: "altro"})
	if err != nil {
		t.Fatalf("Rebase: %v", err)
	}
	if res.Kind != RebaseDone || res.Head == "" {
		t.Errorf("result is %+v, want a finished rebase", res)
	}

	// The point of a rebase: main's commit now sits on top of altro's, in a line.
	want := []string{"solo su main", "solo su altro", "base"}
	if got := subjects(t, repo); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("history is %v, want %v", got, want)
	}
	st, err := repo.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.Head != "main" || st.Operation != OpNone {
		t.Errorf("status is on %q with %q open, want main with nothing open", st.Head, st.Operation)
	}
}

func TestRebaseOntoAnAncestorIsUpToDate(t *testing.T) {
	ctx := context.Background()
	repo, _, _ := applyRepo(t)
	before, _ := repo.resolve(ctx, "HEAD")

	res, err := repo.Rebase(ctx, RebaseOptions{Onto: "HEAD~1"})
	if err != nil {
		t.Fatalf("Rebase: %v", err)
	}
	if res.Kind != RebaseUpToDate || res.Head != before {
		t.Errorf("result is %+v, want up to date at %s", res, before)
	}
}

func TestRebaseBehindFastForwards(t *testing.T) {
	ctx := context.Background()
	repo, run, _ := applyRepo(t)
	run("switch", "-qc", "dietro", "main~1")

	res, err := repo.Rebase(ctx, RebaseOptions{Onto: "main"})
	if err != nil {
		t.Fatalf("Rebase: %v", err)
	}
	main, _ := repo.resolve(ctx, "main")
	if res.Kind != RebaseFastForward || res.Head != main {
		t.Errorf("result is %+v, want a fast-forward to %s", res, main)
	}
}

func TestRebaseRejectsOptionsAsRefs(t *testing.T) {
	repo, _, _ := applyRepo(t)
	for _, onto := range []string{"", "  ", "--root", "-i"} {
		if _, err := repo.Rebase(context.Background(), RebaseOptions{Onto: onto}); err == nil {
			t.Errorf("Rebase(%q) succeeded", onto)
		}
	}
}

func TestRebaseRefusesADirtyTree(t *testing.T) {
	repo, _, write := applyRepo(t)
	write("main.txt", "a metà\n")

	_, err := repo.Rebase(context.Background(), RebaseOptions{Onto: "altro"})
	if !errors.Is(err, ErrWouldOverwrite) {
		t.Fatalf("err = %v, want ErrWouldOverwrite", err)
	}
	if op, _ := repo.Operation(); op != OpNone {
		t.Errorf("a refused rebase left %q open", op)
	}
}

func TestRebaseConflictContinuesToTheNextOne(t *testing.T) {
	ctx := context.Background()
	repo, _, write := conflictingRebase(t)

	res, err := repo.Rebase(ctx, RebaseOptions{Onto: "altro"})
	if err != nil {
		t.Fatalf("Rebase: %v", err)
	}
	if res.Kind != RebaseConflict || len(res.Conflicts) != 1 || res.Conflicts[0] != "shared.txt" {
		t.Fatalf("result is %+v, want a conflict on shared.txt", res)
	}

	st, err := repo.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.Operation != OpRebase || !strings.Contains(st.OperationFrom, "primo scontro") {
		t.Errorf("status says %q from %q, want the rebase stopped on primo scontro", st.Operation, st.OperationFrom)
	}
	want := RebaseState{Branch: "main", Onto: "altro", Step: 2, Steps: 4}
	if st.Rebase == nil || *st.Rebase != want {
		t.Errorf("Rebase = %+v, want %+v", st.Rebase, want)
	}

	// Resolving and continuing replays the clean commit and stops on the next
	// clash, which the caller learns from the status, not from an error.
	write("shared.txt", "risolto\n")
	if err := repo.Stage(ctx, "shared.txt"); err != nil {
		t.Fatal(err)
	}
	if err := repo.FinishOperation(ctx, ""); err != nil {
		t.Fatalf("FinishOperation: %v", err)
	}
	st, err = repo.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.Operation != OpRebase || st.Rebase == nil || st.Rebase.Step != 4 {
		t.Fatalf("after continuing the status is %q at %+v, want the rebase at step 4", st.Operation, st.Rebase)
	}
	if !strings.Contains(st.OperationFrom, "secondo scontro") {
		t.Errorf("stopped on %q, want secondo scontro", st.OperationFrom)
	}

	if err := repo.Resolve(ctx, "shared.txt", SideTheirs); err != nil {
		t.Fatal(err)
	}
	if err := repo.FinishOperation(ctx, ""); err != nil {
		t.Fatalf("FinishOperation: %v", err)
	}
	if op, _ := repo.Operation(); op != OpNone {
		t.Fatalf("%q is still open after the last commit", op)
	}
	// The rebase put the branch back: git moves it only once everything replayed.
	st, _ = repo.Status(ctx)
	if st.Head != "main" {
		t.Errorf("HEAD is on %q, want main", st.Head)
	}
	got := subjects(t, repo)
	if len(got) != 7 || got[0] != "secondo scontro" || got[4] != "shared su altro" {
		t.Errorf("history is %v", got)
	}
}

func TestRebaseFinishRefusesWhileConflicted(t *testing.T) {
	ctx := context.Background()
	repo, _, _ := conflictingRebase(t)
	if _, err := repo.Rebase(ctx, RebaseOptions{Onto: "altro"}); err != nil {
		t.Fatal(err)
	}

	if err := repo.FinishOperation(ctx, ""); !errors.Is(err, ErrUnresolved) {
		t.Errorf("err = %v, want ErrUnresolved", err)
	}
}

func TestRebaseSkipDropsTheCommit(t *testing.T) {
	ctx := context.Background()
	repo, _, _ := conflictingRebase(t)
	if _, err := repo.Rebase(ctx, RebaseOptions{Onto: "altro"}); err != nil {
		t.Fatal(err)
	}

	// Skipping the first clash moves on to the second, which is still a stop.
	if err := repo.SkipOperation(ctx); err != nil {
		t.Fatalf("SkipOperation: %v", err)
	}
	if err := repo.SkipOperation(ctx); err != nil {
		t.Fatalf("SkipOperation: %v", err)
	}
	if op, _ := repo.Operation(); op != OpNone {
		t.Fatalf("%q is still open after skipping both", op)
	}
	for _, s := range subjects(t, repo) {
		if strings.Contains(s, "scontro") {
			t.Errorf("skipped commit %q is still in the history", s)
		}
	}
}

func TestSkipNeedsSomethingToSkip(t *testing.T) {
	repo, _, _ := applyRepo(t)
	if err := repo.SkipOperation(context.Background()); !errors.Is(err, ErrNoOperation) {
		t.Errorf("err = %v, want ErrNoOperation", err)
	}
}

func TestRebaseAbortPutsTheBranchBack(t *testing.T) {
	ctx := context.Background()
	repo, _, _ := conflictingRebase(t)
	before, _ := repo.resolve(ctx, "main")
	if _, err := repo.Rebase(ctx, RebaseOptions{Onto: "altro"}); err != nil {
		t.Fatal(err)
	}

	if err := repo.AbortOperation(ctx); err != nil {
		t.Fatalf("AbortOperation: %v", err)
	}
	st, err := repo.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.Operation != OpNone || st.Rebase != nil || st.Head != "main" || st.Oid != before {
		t.Errorf("status after abort is %+v, want main back at %s", st, before)
	}
}

// The apply backend keeps its state elsewhere and counts in other files.
func TestRebaseStateWithTheApplyBackend(t *testing.T) {
	ctx := context.Background()
	repo, run, _ := conflictingRebase(t)
	run("config", "rebase.backend", "apply")

	res, err := repo.Rebase(ctx, RebaseOptions{Onto: "altro"})
	if err != nil {
		t.Fatalf("Rebase: %v", err)
	}
	if res.Kind != RebaseConflict {
		t.Fatalf("Kind = %q, want a conflict", res.Kind)
	}
	st, err := repo.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := RebaseState{Branch: "main", Onto: "altro", Step: 2, Steps: 4}
	if st.Operation != OpRebase || st.Rebase == nil || *st.Rebase != want {
		t.Errorf("status says %q at %+v, want %+v", st.Operation, st.Rebase, want)
	}
	if !strings.Contains(st.OperationFrom, "primo scontro") {
		t.Errorf("stopped on %q, want primo scontro", st.OperationFrom)
	}
}
