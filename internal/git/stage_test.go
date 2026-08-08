package git

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// `git diff --no-index` exits 1 on any difference, which against /dev/null is always.
func TestUntrackedDiff(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()

	path := filepath.Join(repo.Root(), "nuovo.txt")
	if err := os.WriteFile(path, []byte("riga uno\nriga due\nriga tre\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	d, err := repo.UntrackedDiff(ctx, "nuovo.txt")
	if err != nil {
		t.Fatalf("UntrackedDiff: %v", err)
	}
	if len(d.Hunks) == 0 {
		t.Fatal("no hunks: the whole file should read as added")
	}
	if d.Added != 3 {
		t.Errorf("Added = %d, want 3", d.Added)
	}
	if d.Removed != 0 {
		t.Errorf("Removed = %d, want 0", d.Removed)
	}
	if d.Path != "nuovo.txt" {
		t.Errorf("Path = %q, want the repository-relative path", d.Path)
	}
	// /dev/null on the old side is what marks the file as new.
	if d.OldPath != "" {
		t.Errorf("OldPath = %q, want empty", d.OldPath)
	}
}

func TestUntrackedDiffEmptyFile(t *testing.T) {
	repo := testRepo(t)

	path := filepath.Join(repo.Root(), "vuoto.txt")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	// An empty new file genuinely has no lines; that is an answer, not a bug.
	d, err := repo.UntrackedDiff(context.Background(), "vuoto.txt")
	if err != nil {
		t.Fatalf("UntrackedDiff: %v", err)
	}
	if d.Added != 0 {
		t.Errorf("Added = %d, want 0", d.Added)
	}
}

func TestStageAndUnstage(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()

	// The fixture leaves a.txt modified but unstaged.
	if err := repo.Stage(ctx, "a.txt"); err != nil {
		t.Fatalf("Stage: %v", err)
	}
	st, err := repo.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if f := find(st, "a.txt"); f == nil || f.Staged == ChangeNone {
		t.Fatalf("a.txt not staged: %+v", f)
	}

	if err := repo.Unstage(ctx, "a.txt"); err != nil {
		t.Fatalf("Unstage: %v", err)
	}
	st, err = repo.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if f := find(st, "a.txt"); f == nil || f.Staged != ChangeNone {
		t.Fatalf("a.txt still staged: %+v", f)
	}
}

func TestStageNoPathsIsANoOp(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()

	before, err := repo.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// `git add --` with no paths would be an error.
	if err := repo.Stage(ctx); err != nil {
		t.Fatalf("Stage with no paths: %v", err)
	}
	if err := repo.Unstage(ctx); err != nil {
		t.Fatalf("Unstage with no paths: %v", err)
	}

	after, err := repo.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(before.Files) != len(after.Files) {
		t.Errorf("status changed: %d files before, %d after", len(before.Files), len(after.Files))
	}
	for i := range before.Files {
		if before.Files[i] != after.Files[i] {
			t.Errorf("file %d changed: %+v -> %+v", i, before.Files[i], after.Files[i])
		}
	}
}

func TestCommitRejectsEmptyMessage(t *testing.T) {
	repo := testRepo(t)
	for _, msg := range []string{"", "   ", "\n\n"} {
		if err := repo.Commit(context.Background(), CommitOptions{Message: msg}); err == nil {
			t.Errorf("Commit accepted the empty message %q", msg)
		}
	}
}

func TestCommitMessageWithAwkwardCharacters(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()

	if err := repo.Stage(ctx, "c.txt"); err != nil {
		t.Fatal(err)
	}

	// Quotes, backticks, dollars and newlines would all be hazards on a command line.
	msg := "fix: gestisce \"virgolette\", `backtick` e $VAR\n\nCorpo su\npiù righe.\n"
	if err := repo.Commit(ctx, CommitOptions{Message: msg}); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	commits, err := repo.Commits(ctx, LogOptions{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if got := commits[0].Subject; got != `fix: gestisce "virgolette", `+"`backtick`"+` e $VAR` {
		t.Errorf("Subject = %q", got)
	}
	if !strings.Contains(commits[0].Body, "Corpo su\npiù righe.") {
		t.Errorf("Body = %q", commits[0].Body)
	}
}

func find(st Status, path string) *FileStatus {
	for i := range st.Files {
		if st.Files[i].Path == path {
			return &st.Files[i]
		}
	}
	return nil
}
