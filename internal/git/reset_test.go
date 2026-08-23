package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestResetSoftKeepsTheChangeStaged(t *testing.T) {
	ctx := context.Background()
	repo, _, _ := applyRepo(t)

	if err := repo.Reset(ctx, "HEAD~1", ResetSoft); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	st, err := repo.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Files) != 1 || st.Files[0].Staged != ChangeAdded {
		t.Errorf("status is %+v, want the undone commit staged", st.Files)
	}
	if !exists(t, repo, "main.txt") {
		t.Error("a soft reset must not touch the working tree")
	}
}

func TestResetMixedUnstages(t *testing.T) {
	ctx := context.Background()
	repo, _, _ := applyRepo(t)

	if err := repo.Reset(ctx, "HEAD~1", ResetMixed); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	st, err := repo.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// The file is still there, but git no longer knows about it at all.
	if len(st.Files) != 1 || !st.Files[0].Untracked {
		t.Errorf("status is %+v, want the file untracked", st.Files)
	}
}

func TestResetHardThrowsTheWorkAway(t *testing.T) {
	ctx := context.Background()
	repo, _, write := applyRepo(t)

	write("shared.txt", "modificato\n")
	if err := repo.Reset(ctx, "HEAD~1", ResetHard); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	if exists(t, repo, "main.txt") {
		t.Error("the commit's file survived a hard reset")
	}
	if b, _ := os.ReadFile(filepath.Join(repo.Root(), "shared.txt")); string(b) != "base\n" {
		t.Errorf("shared.txt = %q, want the uncommitted change gone", b)
	}
}

func TestResetRejectsAnUnknownMode(t *testing.T) {
	ctx := context.Background()
	repo, _, _ := applyRepo(t)

	if err := repo.Reset(ctx, "HEAD", ResetMode("--exec=rm")); err == nil {
		t.Error("an unknown mode was accepted")
	}
	if err := repo.Reset(ctx, "non-esiste", ResetHard); err == nil {
		t.Error("an unknown commit was accepted")
	}
}

func TestPreviewReset(t *testing.T) {
	ctx := context.Background()
	repo, _, write := applyRepo(t)

	got, err := repo.PreviewReset(ctx, "HEAD~1")
	if err != nil {
		t.Fatalf("PreviewReset: %v", err)
	}
	if got.Behind != 1 || got.Ahead != 0 || got.Dirty {
		t.Errorf("PreviewReset = %+v, want one commit given up on a clean tree", got)
	}

	// Untracked files survive a hard reset, so they must not raise the warning.
	write("nuovo.txt", "x\n")
	if got, _ := repo.PreviewReset(ctx, "HEAD~1"); got.Dirty {
		t.Error("an untracked file counted as work a reset would destroy")
	}

	write("shared.txt", "modificato\n")
	if got, _ := repo.PreviewReset(ctx, "HEAD~1"); !got.Dirty {
		t.Error("a modified tracked file did not raise the warning")
	}

	// Moving forward is a reset too, and the counts have to swap round.
	if err := repo.Reset(ctx, "HEAD~1", ResetHard); err != nil {
		t.Fatal(err)
	}
	if got, _ := repo.PreviewReset(ctx, "main@{1}"); got.Ahead != 1 || got.Behind != 0 {
		t.Errorf("PreviewReset going forward = %+v", got)
	}
}
