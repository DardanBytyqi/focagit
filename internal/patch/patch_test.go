package patch

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dardanbytyqi/focagit/internal/git"
)

// twoChanges is one hunk containing two independent edits.
func twoChanges() git.FileDiff {
	return git.FileDiff{
		Path:    "a.txt",
		OldPath: "a.txt",
		Header:  []string{"diff --git a/a.txt b/a.txt", "index 1111111..2222222 100644"},
		Hunks: []git.Hunk{{
			OldStart: 1, OldLines: 5, NewStart: 1, NewLines: 5,
			Lines: []git.DiffLine{
				{Kind: git.LineContext, Old: 1, New: 1, Text: "uno"},
				{Kind: git.LineDelete, Old: 2, Text: "due"},
				{Kind: git.LineAdd, New: 2, Text: "DUE"},
				{Kind: git.LineContext, Old: 3, New: 3, Text: "tre"},
				{Kind: git.LineDelete, Old: 4, Text: "quattro"},
				{Kind: git.LineAdd, New: 4, Text: "QUATTRO"},
				{Kind: git.LineContext, Old: 5, New: 5, Text: "cinque"},
			},
		}},
	}
}

func TestBuildStagingKeepsUnselectedDeletionsAsContext(t *testing.T) {
	// Staging the first edit only.
	got, err := Build(twoChanges(), []LineRef{{Hunk: 0, Line: 1}, {Hunk: 0, Line: 2}}, false)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	want := `diff --git a/a.txt b/a.txt
index 1111111..2222222 100644
--- a/a.txt
+++ b/a.txt
@@ -1,5 +1,5 @@
 uno
-due
+DUE
 tre
 quattro
 cinque
`
	if got != want {
		t.Errorf("patch mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestBuildUnstagingKeepsUnselectedAdditionsAsContext(t *testing.T) {
	// The mirror image: reverse-applied, so the NEW side must match the index.
	got, err := Build(twoChanges(), []LineRef{{Hunk: 0, Line: 1}, {Hunk: 0, Line: 2}}, true)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	want := `diff --git a/a.txt b/a.txt
index 1111111..2222222 100644
--- a/a.txt
+++ b/a.txt
@@ -1,5 +1,5 @@
 uno
-due
+DUE
 tre
 QUATTRO
 cinque
`
	if got != want {
		t.Errorf("patch mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestBuildStagingOnlyAnAddition(t *testing.T) {
	// Selecting the added line alone, without its paired deletion.
	got, err := Build(twoChanges(), []LineRef{{Hunk: 0, Line: 2}}, false)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// Five context lines and one addition.
	if !strings.Contains(got, "@@ -1,5 +1,6 @@") {
		t.Errorf("wrong hunk header in:\n%s", got)
	}
	if !strings.Contains(got, "\n due\n+DUE\n") {
		t.Errorf("the unselected deletion should have become context:\n%s", got)
	}
}

func TestBuildSkipsHunksWithNothingSelected(t *testing.T) {
	d := twoChanges()
	d.Hunks = append(d.Hunks, git.Hunk{
		OldStart: 20, OldLines: 2, NewStart: 20, NewLines: 2,
		Lines: []git.DiffLine{
			{Kind: git.LineContext, Old: 20, New: 20, Text: "venti"},
			{Kind: git.LineDelete, Old: 21, Text: "ventuno"},
			{Kind: git.LineAdd, New: 21, Text: "VENTUNO"},
		},
	})

	got, err := Build(d, []LineRef{{Hunk: 1, Line: 1}, {Hunk: 1, Line: 2}}, false)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if strings.Contains(got, "DUE") {
		t.Errorf("the untouched hunk leaked into the patch:\n%s", got)
	}
	if !strings.Contains(got, "VENTUNO") {
		t.Errorf("the selected hunk is missing:\n%s", got)
	}
	if n := strings.Count(got, "@@ -"); n != 1 {
		t.Errorf("got %d hunks, want 1:\n%s", n, got)
	}
}

func TestBuildAdjustsLaterHunkOffsets(t *testing.T) {
	// Dropping lines from an early hunk shifts every later hunk on the synthesised side.
	d := git.FileDiff{
		Path: "a.txt", OldPath: "a.txt",
		Header: []string{"diff --git a/a.txt b/a.txt"},
		Hunks: []git.Hunk{
			{
				OldStart: 1, OldLines: 1, NewStart: 1, NewLines: 3,
				Lines: []git.DiffLine{
					{Kind: git.LineContext, Old: 1, New: 1, Text: "testa"},
					{Kind: git.LineAdd, New: 2, Text: "primo"},
					{Kind: git.LineAdd, New: 3, Text: "secondo"},
				},
			},
			{
				OldStart: 10, OldLines: 1, NewStart: 12, NewLines: 2,
				Lines: []git.DiffLine{
					{Kind: git.LineContext, Old: 10, New: 12, Text: "coda"},
					{Kind: git.LineAdd, New: 13, Text: "terzo"},
				},
			},
		},
	}

	// Stage one addition from the first hunk and one from the second.
	got, err := Build(d, []LineRef{{Hunk: 0, Line: 1}, {Hunk: 1, Line: 1}}, false)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// First hunk contributes one added line.
	if !strings.Contains(got, "@@ -1,1 +1,2 @@") {
		t.Errorf("first hunk header wrong:\n%s", got)
	}
	if !strings.Contains(got, "@@ -10,1 +11,2 @@") {
		t.Errorf("second hunk header not offset by the first:\n%s", got)
	}
}

func TestBuildNewFile(t *testing.T) {
	d := git.FileDiff{
		Path:   "nuovo.txt",
		Header: []string{"diff --git a/nuovo.txt b/nuovo.txt", "new file mode 100644"},
		Hunks: []git.Hunk{{
			OldStart: 0, OldLines: 0, NewStart: 1, NewLines: 2,
			Lines: []git.DiffLine{
				{Kind: git.LineAdd, New: 1, Text: "riga uno"},
				{Kind: git.LineAdd, New: 2, Text: "riga due"},
			},
		}},
	}

	got, err := Build(d, []LineRef{{Hunk: 0, Line: 0}}, false)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if !strings.Contains(got, "new file mode 100644") {
		t.Errorf("git's own header was not replayed:\n%s", got)
	}
	if !strings.Contains(got, "--- /dev/null") {
		t.Errorf("a new file needs /dev/null on the old side:\n%s", got)
	}
	if strings.Contains(got, "riga due") {
		t.Errorf("the unselected line was included:\n%s", got)
	}
}

func TestBuildPreservesNoNewlineMarker(t *testing.T) {
	d := git.FileDiff{
		Path: "a.txt", OldPath: "a.txt",
		Header: []string{"diff --git a/a.txt b/a.txt"},
		Hunks: []git.Hunk{{
			OldStart: 1, OldLines: 1, NewStart: 1, NewLines: 1,
			Lines: []git.DiffLine{
				{Kind: git.LineDelete, Old: 1, Text: "prima", NoNewline: true},
				{Kind: git.LineAdd, New: 1, Text: "dopo", NoNewline: true},
			},
		}},
	}

	got, err := Build(d, []LineRef{{Hunk: 0, Line: 0}, {Hunk: 0, Line: 1}}, false)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	// Dropping the marker would silently append a newline to the file.
	if n := strings.Count(got, `\ No newline at end of file`); n != 2 {
		t.Errorf("got %d no-newline markers, want 2:\n%s", n, got)
	}
}

func TestBuildRejectsEmptySelection(t *testing.T) {
	tests := []struct {
		name string
		sel  []LineRef
	}{
		{"no refs at all", nil},
		{"refs pointing only at context lines", []LineRef{{Hunk: 0, Line: 0}, {Hunk: 0, Line: 3}}},
		{"refs out of range", []LineRef{{Hunk: 9, Line: 9}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Build(twoChanges(), tt.sel, false); !errors.Is(err, ErrNothingSelected) {
				t.Fatalf("got %v, want ErrNothingSelected", err)
			}
		})
	}
}

func TestBuildRejectsBinary(t *testing.T) {
	d := twoChanges()
	d.Binary = true
	if _, err := Build(d, []LineRef{{Hunk: 0, Line: 1}}, false); err == nil {
		t.Fatal("expected an error for a binary file")
	}
}

// --- against real git -------------------------------------------------------

// The unit tests above pin the patch text.
func TestPatchAppliesForRealAndStagesOnlyTheSelection(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}

	dir := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=T", "GIT_AUTHOR_EMAIL=t@x",
			"GIT_COMMITTER_NAME=T", "GIT_COMMITTER_EMAIL=t@x",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return string(out)
	}
	write := func(content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	run("init", "-q", "-b", "main")
	write("uno\ndue\ntre\nquattro\ncinque\n")
	run("add", ".")
	run("commit", "-qm", "base")

	// Two independent edits in the working tree.
	write("uno\nDUE\ntre\nQUATTRO\ncinque\n")

	ctx := context.Background()
	repo, err := git.Open(ctx, dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	d, err := repo.Diff(ctx, git.DiffTarget{Path: "a.txt"})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}

	// Find the deletion of "due" and the addition of "DUE" wherever git put them.
	var sel []LineRef
	for hi, h := range d.Hunks {
		for li, l := range h.Lines {
			if l.Text == "due" && l.Kind == git.LineDelete {
				sel = append(sel, LineRef{Hunk: hi, Line: li})
			}
			if l.Text == "DUE" && l.Kind == git.LineAdd {
				sel = append(sel, LineRef{Hunk: hi, Line: li})
			}
		}
	}
	if len(sel) != 2 {
		t.Fatalf("expected to find both lines of the first edit, got %d", len(sel))
	}

	p, err := Build(d, sel, false)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if err := repo.ApplyPatch(ctx, p, false); err != nil {
		t.Fatalf("ApplyPatch: %v\n--- patch ---\n%s", err, p)
	}

	staged := run("diff", "--cached")
	if !strings.Contains(staged, "+DUE") {
		t.Errorf("the selected line was not staged:\n%s", staged)
	}
	if strings.Contains(staged, "QUATTRO") {
		t.Errorf("an unselected line was staged too:\n%s", staged)
	}

	unstaged := run("diff")
	if !strings.Contains(unstaged, "+QUATTRO") {
		t.Errorf("the unselected line should still be unstaged:\n%s", unstaged)
	}
	if strings.Contains(unstaged, "+DUE") {
		t.Errorf("the staged line is still showing as unstaged:\n%s", unstaged)
	}

	// Now unstage it again and check we land back where we started.
	staged2, err := repo.Diff(ctx, git.DiffTarget{Path: "a.txt", Staged: true})
	if err != nil {
		t.Fatalf("Diff staged: %v", err)
	}
	var sel2 []LineRef
	for hi, h := range staged2.Hunks {
		for li, l := range h.Lines {
			if l.Kind != git.LineContext {
				sel2 = append(sel2, LineRef{Hunk: hi, Line: li})
			}
		}
	}
	p2, err := Build(staged2, sel2, true)
	if err != nil {
		t.Fatalf("Build reverse: %v", err)
	}
	if err := repo.ApplyPatch(ctx, p2, true); err != nil {
		t.Fatalf("ApplyPatch reverse: %v\n--- patch ---\n%s", err, p2)
	}

	if out := run("diff", "--cached"); strings.TrimSpace(out) != "" {
		t.Errorf("index should be clean after unstaging everything:\n%s", out)
	}
	if out := run("diff"); !strings.Contains(out, "+DUE") || !strings.Contains(out, "+QUATTRO") {
		t.Errorf("both edits should be back in the working tree:\n%s", out)
	}
}
