package git

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// conflicted leaves the repository mid-merge with shared.txt disagreeing.
func conflicted(t *testing.T, style string) *Repo {
	t.Helper()
	repo, run, write := mergeRepo(t)
	run("switch", "-q", "altro")
	write("shared.txt", "riga1\nLORO\nriga3\n")
	run("commit", "-qam", "loro")
	run("switch", "-q", "main")
	write("shared.txt", "riga1\nNOSTRO\nriga3\n")
	run("commit", "-qam", "nostro")
	if style != "" {
		run("config", "merge.conflictStyle", style)
	}

	if _, err := repo.Merge(context.Background(), MergeOptions{Ref: "altro"}); err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestConflictText(t *testing.T) {
	ctx := context.Background()

	t.Run("the default style splits into agreed text and one disagreement", func(t *testing.T) {
		repo := conflicted(t, "")
		got, err := repo.ConflictText(ctx, "shared.txt")
		if err != nil {
			t.Fatal(err)
		}
		if got.Count != 1 || len(got.Segments) != 3 {
			t.Fatalf("segments = %+v", got.Segments)
		}
		if s := got.Segments[0]; s.Conflict || strings.Join(s.Lines, "") != "riga1" {
			t.Errorf("first = %+v", s)
		}
		c := firstConflict(t, got)
		if strings.Join(c.Ours, "") != "NOSTRO" || strings.Join(c.Theirs, "") != "LORO" {
			t.Errorf("conflict = %+v", c)
		}
		if len(c.Base) != 0 {
			t.Errorf("Base = %v, want none in the default style", c.Base)
		}
	})

	// diff3 widens the block it reports, so the conflict is found rather than
	// assumed to sit at a particular index.
	t.Run("diff3 carries what both sides started from", func(t *testing.T) {
		repo := conflicted(t, "diff3")
		got, err := repo.ConflictText(ctx, "shared.txt")
		if err != nil {
			t.Fatal(err)
		}
		c := firstConflict(t, got)
		if base := strings.Join(c.Base, "|"); !strings.Contains(base, "base") {
			t.Errorf("Base = %q, want what both sides started from", base)
		}
	})

	// A row of equals signs underlines a heading in markdown, and appears in
	// plenty of plain text. Only inside a block does it mean anything.
	t.Run("marker characters in ordinary text stay ordinary", func(t *testing.T) {
		segs := parseConflictText("Titolo\n=======\ntesto\n")
		if len(segs) != 1 || segs[0].Conflict {
			t.Fatalf("segments = %+v", segs)
		}
		if len(segs[0].Lines) != 3 {
			t.Errorf("lines = %v", segs[0].Lines)
		}
	})

	t.Run("an unfinished block is kept as text rather than eaten", func(t *testing.T) {
		segs := parseConflictText("prima\n<<<<<<< HEAD\nmia\n")
		if len(segs) != 1 || segs[0].Conflict {
			t.Fatalf("segments = %+v", segs)
		}
		if strings.Join(segs[0].Lines, "|") != "prima|<<<<<<< HEAD|mia" {
			t.Errorf("lines = %v", segs[0].Lines)
		}
	})

	t.Run("a file outside the repository is refused", func(t *testing.T) {
		repo := conflicted(t, "")
		if _, err := repo.ConflictText(ctx, "../../etc/passwd"); err == nil {
			t.Error("ConflictText read outside the worktree")
		}
	})
}

func TestResolveLines(t *testing.T) {
	ctx := context.Background()
	read := func(t *testing.T, repo *Repo) string {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(repo.Root(), "shared.txt"))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}

	t.Run("keeping one side rewrites the file and stages it", func(t *testing.T) {
		repo := conflicted(t, "")
		if err := repo.ResolveLines(ctx, "shared.txt", []ConflictPick{PickTheirs}); err != nil {
			t.Fatal(err)
		}
		if got := read(t, repo); got != "riga1\nLORO\nriga3\n" {
			t.Errorf("file = %q", got)
		}
		left, _ := repo.ConflictPaths(ctx)
		if len(left) != 0 {
			t.Errorf("still conflicted: %v", left)
		}
	})

	t.Run("both sides are kept in order", func(t *testing.T) {
		repo := conflicted(t, "")
		if err := repo.ResolveLines(ctx, "shared.txt", []ConflictPick{PickBoth}); err != nil {
			t.Fatal(err)
		}
		if got := read(t, repo); got != "riga1\nNOSTRO\nLORO\nriga3\n" {
			t.Errorf("file = %q", got)
		}
	})

	t.Run("neither side leaves the surrounding text alone", func(t *testing.T) {
		repo := conflicted(t, "")
		if err := repo.ResolveLines(ctx, "shared.txt", []ConflictPick{PickNone}); err != nil {
			t.Fatal(err)
		}
		if got := read(t, repo); got != "riga1\nriga3\n" {
			t.Errorf("file = %q", got)
		}
	})

	// The picks are made against what was on screen; if that has moved on, acting
	// on them would resolve the wrong block.
	t.Run("a pick count that no longer fits is refused", func(t *testing.T) {
		repo := conflicted(t, "")
		err := repo.ResolveLines(ctx, "shared.txt", []ConflictPick{PickOurs, PickOurs})
		if err == nil {
			t.Fatal("ResolveLines accepted two picks for one conflict")
		}
		if got := read(t, repo); !strings.Contains(got, "<<<<<<<") {
			t.Error("the file was rewritten despite the refusal")
		}
	})

	t.Run("an unknown choice is refused", func(t *testing.T) {
		repo := conflicted(t, "")
		if err := repo.ResolveLines(ctx, "shared.txt", []ConflictPick{"qualcosa"}); err == nil {
			t.Error("ResolveLines accepted a choice that is not one")
		}
	})
}

// firstConflict is the disagreement in a file that has exactly one.
func firstConflict(t *testing.T, text ConflictText) ConflictSegment {
	t.Helper()
	if text.Count != 1 {
		t.Fatalf("Count = %d, want 1", text.Count)
	}
	for _, s := range text.Segments {
		if s.Conflict {
			return s
		}
	}
	t.Fatal("no conflict segment")
	return ConflictSegment{}
}
