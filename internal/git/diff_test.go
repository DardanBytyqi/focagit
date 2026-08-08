package git

import (
	"strings"
	"testing"
)

func TestParseDiffModification(t *testing.T) {
	patch := `diff --git a/a.txt b/a.txt
index fb85c0a..2c6d4c5 100644
--- a/a.txt
+++ b/a.txt
@@ -1,3 +1,4 @@
 uno
-due
+DUE
 tre
+quattro
`
	files, err := parseDiff([]byte(patch))
	if err != nil {
		t.Fatalf("parseDiff: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("got %d files, want 1", len(files))
	}

	f := files[0]
	if f.Path != "a.txt" || f.OldPath != "a.txt" {
		t.Errorf("paths = %q / %q", f.OldPath, f.Path)
	}
	if f.Added != 2 || f.Removed != 1 {
		t.Errorf("added/removed = %d/%d, want 2/1", f.Added, f.Removed)
	}
	if len(f.Hunks) != 1 {
		t.Fatalf("got %d hunks, want 1", len(f.Hunks))
	}

	h := f.Hunks[0]
	if h.OldStart != 1 || h.OldLines != 3 || h.NewStart != 1 || h.NewLines != 4 {
		t.Errorf("hunk range = -%d,%d +%d,%d", h.OldStart, h.OldLines, h.NewStart, h.NewLines)
	}

	// Line numbers are what stage 5 will turn a selection into a patch with.
	want := []DiffLine{
		{Kind: LineContext, Old: 1, New: 1, Text: "uno"},
		{Kind: LineDelete, Old: 2, New: 0, Text: "due"},
		{Kind: LineAdd, Old: 0, New: 2, Text: "DUE"},
		{Kind: LineContext, Old: 3, New: 3, Text: "tre"},
		{Kind: LineAdd, Old: 0, New: 4, Text: "quattro"},
	}
	if len(h.Lines) != len(want) {
		t.Fatalf("got %d lines, want %d: %+v", len(h.Lines), len(want), h.Lines)
	}
	for i, w := range want {
		g := h.Lines[i]
		if g.Kind != w.Kind || g.Old != w.Old || g.New != w.New || g.Text != w.Text {
			t.Errorf("line %d = %+v, want %+v", i, g, w)
		}
	}
}

func TestParseDiffNewFile(t *testing.T) {
	patch := `diff --git a/b.txt b/b.txt
new file mode 100644
index 0000000..3b18e51
--- /dev/null
+++ b/b.txt
@@ -0,0 +1,2 @@
+riga uno
+riga due
`
	files, err := parseDiff([]byte(patch))
	if err != nil {
		t.Fatalf("parseDiff: %v", err)
	}
	f := files[0]
	if f.Path != "b.txt" {
		t.Errorf("Path = %q", f.Path)
	}
	// /dev/null means the file did not exist before, so there is no old path.
	if f.OldPath != "" {
		t.Errorf("OldPath = %q, want empty for a new file", f.OldPath)
	}
	if f.Added != 2 || f.Removed != 0 {
		t.Errorf("added/removed = %d/%d, want 2/0", f.Added, f.Removed)
	}
	if h := f.Hunks[0]; h.OldStart != 0 || h.OldLines != 0 {
		t.Errorf("old range = %d,%d, want 0,0", h.OldStart, h.OldLines)
	}
}

func TestParseDiffDeletedFile(t *testing.T) {
	patch := `diff --git a/gone.txt b/gone.txt
deleted file mode 100644
index 3b18e51..0000000
--- a/gone.txt
+++ /dev/null
@@ -1,2 +0,0 @@
-riga uno
-riga due
`
	files, _ := parseDiff([]byte(patch))
	f := files[0]
	if f.OldPath != "gone.txt" {
		t.Errorf("OldPath = %q", f.OldPath)
	}
	if f.Path != "" {
		t.Errorf("Path = %q, want empty for a deleted file", f.Path)
	}
	if f.Removed != 2 {
		t.Errorf("Removed = %d, want 2", f.Removed)
	}
}

func TestParseDiffRename(t *testing.T) {
	patch := `diff --git a/vecchio.txt b/nuovo.txt
similarity index 87%
rename from vecchio.txt
rename to nuovo.txt
index fb85c0a..2c6d4c5 100644
--- a/vecchio.txt
+++ b/nuovo.txt
@@ -1,2 +1,2 @@
 uguale
-prima
+dopo
`
	files, _ := parseDiff([]byte(patch))
	f := files[0]
	if f.OldPath != "vecchio.txt" || f.Path != "nuovo.txt" {
		t.Errorf("rename = %q -> %q", f.OldPath, f.Path)
	}
	if f.Added != 1 || f.Removed != 1 {
		t.Errorf("added/removed = %d/%d, want 1/1", f.Added, f.Removed)
	}
}

func TestParseDiffBinary(t *testing.T) {
	patch := `diff --git a/logo.png b/logo.png
index 1234567..89abcde 100644
Binary files a/logo.png and b/logo.png differ
`
	files, _ := parseDiff([]byte(patch))
	f := files[0]
	if !f.Binary {
		t.Error("Binary = false, want true")
	}
	if len(f.Hunks) != 0 {
		t.Errorf("got %d hunks for a binary file, want 0", len(f.Hunks))
	}
}

func TestParseDiffNoNewlineAtEOF(t *testing.T) {
	patch := `diff --git a/a.txt b/a.txt
--- a/a.txt
+++ b/a.txt
@@ -1 +1 @@
-prima
\ No newline at end of file
+dopo
\ No newline at end of file
`
	files, err := parseDiff([]byte(patch))
	if err != nil {
		t.Fatalf("parseDiff: %v", err)
	}
	lines := files[0].Hunks[0].Lines
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2: %+v", len(lines), lines)
	}
	// The marker attaches to the line before it and must not become a line.
	for i, l := range lines {
		if !l.NoNewline {
			t.Errorf("line %d: NoNewline = false, want true", i)
		}
	}
}

func TestParseDiffSingleLineHunkHeader(t *testing.T) {
	// Unified diff omits the count when it is 1: "@@ -3 +3 @@".
	patch := `diff --git a/a.txt b/a.txt
--- a/a.txt
+++ b/a.txt
@@ -3 +3 @@
-vecchia
+nuova
`
	files, err := parseDiff([]byte(patch))
	if err != nil {
		t.Fatalf("parseDiff: %v", err)
	}
	h := files[0].Hunks[0]
	if h.OldStart != 3 || h.OldLines != 1 || h.NewStart != 3 || h.NewLines != 1 {
		t.Errorf("hunk = -%d,%d +%d,%d, want -3,1 +3,1",
			h.OldStart, h.OldLines, h.NewStart, h.NewLines)
	}
	if h.Lines[0].Old != 3 || h.Lines[1].New != 3 {
		t.Errorf("line numbers = %d / %d, want 3 / 3", h.Lines[0].Old, h.Lines[1].New)
	}
}

func TestParseDiffHunkHeading(t *testing.T) {
	// git puts the enclosing function after the closing @@; the UI shows it.
	patch := `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -10,3 +10,3 @@ func main() {
 	a()
-	b()
+	c()
`
	files, _ := parseDiff([]byte(patch))
	if got := files[0].Hunks[0].Header; got != "func main() {" {
		t.Errorf("Header = %q, want %q", got, "func main() {")
	}
}

func TestParseDiffMultipleHunks(t *testing.T) {
	patch := `diff --git a/a.txt b/a.txt
--- a/a.txt
+++ b/a.txt
@@ -1,2 +1,2 @@
 uno
-due
+DUE
@@ -20,2 +20,3 @@
 venti
+ventuno
`
	files, err := parseDiff([]byte(patch))
	if err != nil {
		t.Fatalf("parseDiff: %v", err)
	}
	f := files[0]
	if len(f.Hunks) != 2 {
		t.Fatalf("got %d hunks, want 2", len(f.Hunks))
	}
	// Each hunk restarts numbering from its own header.
	if f.Hunks[1].Lines[0].Old != 20 {
		t.Errorf("second hunk starts at old line %d, want 20", f.Hunks[1].Lines[0].Old)
	}
	if f.Added != 2 || f.Removed != 1 {
		t.Errorf("added/removed = %d/%d, want 2/1", f.Added, f.Removed)
	}
}

func TestParseDiffEmptyContextLine(t *testing.T) {
	// A blank context line is a single space, which some pipelines strip to nothing.
	patch := "diff --git a/a.txt b/a.txt\n--- a/a.txt\n+++ b/a.txt\n@@ -1,3 +1,3 @@\n uno\n\n-due\n+DUE\n"

	files, err := parseDiff([]byte(patch))
	if err != nil {
		t.Fatalf("parseDiff: %v", err)
	}
	lines := files[0].Hunks[0].Lines
	if len(lines) != 4 {
		t.Fatalf("got %d lines, want 4: %+v", len(lines), lines)
	}
	if lines[1].Kind != LineContext || lines[1].Text != "" {
		t.Errorf("line 1 = %+v, want an empty context line", lines[1])
	}
	if lines[2].Old != 3 {
		t.Errorf("line 2 old number = %d, want 3", lines[2].Old)
	}
}

func TestParseDiffMultipleFiles(t *testing.T) {
	patch := `diff --git a/one.txt b/one.txt
--- a/one.txt
+++ b/one.txt
@@ -1 +1 @@
-a
+b
diff --git a/two.txt b/two.txt
--- a/two.txt
+++ b/two.txt
@@ -1 +1 @@
-c
+d
`
	files, err := parseDiff([]byte(patch))
	if err != nil {
		t.Fatalf("parseDiff: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2", len(files))
	}
	if files[0].Path != "one.txt" || files[1].Path != "two.txt" {
		t.Errorf("paths = %q, %q", files[0].Path, files[1].Path)
	}
	// Counters must not leak between files.
	if files[1].Added != 1 || files[1].Removed != 1 {
		t.Errorf("second file added/removed = %d/%d, want 1/1", files[1].Added, files[1].Removed)
	}
}

func TestParseDiffEmptyInput(t *testing.T) {
	// A path with no changes produces no output, which is an answer not a bug.
	files, err := parseDiff(nil)
	if err != nil {
		t.Fatalf("parseDiff: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("got %d files, want 0", len(files))
	}
}

func TestParseDiffRejectsMalformedHunkHeader(t *testing.T) {
	patch := "diff --git a/a.txt b/a.txt\n--- a/a.txt\n+++ b/a.txt\n@@ nonsenso @@\n context\n"
	if _, err := parseDiff([]byte(patch)); err == nil {
		t.Fatal("expected an error for a malformed hunk header")
	}
}

func TestDiffTargetArgs(t *testing.T) {
	tests := []struct {
		name   string
		target DiffTarget
		want   string
	}{
		{
			// --first-parent is the flag that makes merges show anything.
			name:   "commit",
			target: DiffTarget{Commit: "abc123", Path: "main.go"},
			want:   "show --format= --patch -M --first-parent abc123 -- main.go",
		},
		{
			name:   "staged working tree",
			target: DiffTarget{Path: "main.go", Staged: true},
			want:   "diff --cached -M -- main.go",
		},
		{
			name:   "unstaged working tree",
			target: DiffTarget{Path: "main.go"},
			want:   "diff -M -- main.go",
		},
		{
			// Staged must not leak --cached in once a commit is named.
			name:   "commit wins over staged",
			target: DiffTarget{Commit: "abc123", Path: "main.go", Staged: true},
			want:   "show --format= --patch -M --first-parent abc123 -- main.go",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := strings.Join(tt.target.args(), " "); got != tt.want {
				t.Errorf("args() = %q\nwant       %q", got, tt.want)
			}
		})
	}
}
