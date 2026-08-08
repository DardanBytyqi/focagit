package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseNameStatus(t *testing.T) {
	tests := []struct {
		name    string
		entries []string
		want    []FileChange
	}{
		{
			name:    "modification and addition",
			entries: []string{"M", "a.txt", "A", "b.txt"},
			want: []FileChange{
				{Path: "a.txt", Status: ChangeModified},
				{Path: "b.txt", Status: ChangeAdded},
			},
		},
		{
			name:    "root commit adds everything",
			entries: []string{"A", "a.txt"},
			want:    []FileChange{{Path: "a.txt", Status: ChangeAdded}},
		},
		{
			// A rename is three entries, not two: status, old path, new path.
			name:    "rename carries both paths and a score",
			entries: []string{"R100", "b.txt", "b2.txt"},
			want: []FileChange{
				{Path: "b2.txt", OldPath: "b.txt", Status: ChangeRenamed, Score: 100},
			},
		},
		{
			name:    "copy behaves like a rename",
			entries: []string{"C75", "src.txt", "copia.txt"},
			want: []FileChange{
				{Path: "copia.txt", OldPath: "src.txt", Status: ChangeCopied, Score: 75},
			},
		},
		{
			// The three-entry and two-entry shapes have to interleave cleanly.
			name:    "rename between ordinary entries",
			entries: []string{"M", "primo.txt", "R90", "vecchio.txt", "nuovo.txt", "D", "morto.txt"},
			want: []FileChange{
				{Path: "primo.txt", Status: ChangeModified},
				{Path: "nuovo.txt", OldPath: "vecchio.txt", Status: ChangeRenamed, Score: 90},
				{Path: "morto.txt", Status: ChangeDeleted},
			},
		},
		{
			name:    "paths with spaces survive",
			entries: []string{"M", "una cartella/file con spazi.md"},
			want:    []FileChange{{Path: "una cartella/file con spazi.md", Status: ChangeModified}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseNameStatus([]byte(strings.Join(tt.entries, "\x00") + "\x00"))
			if err != nil {
				t.Fatalf("parseNameStatus: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d entries, want %d: %+v", len(got), len(tt.want), got)
			}
			for i, w := range tt.want {
				if got[i] != w {
					t.Errorf("entry %d = %+v, want %+v", i, got[i], w)
				}
			}
		})
	}
}

func TestParseNameStatusEmpty(t *testing.T) {
	// A merge with no first-parent changes produces nothing.
	got, err := parseNameStatus(nil)
	if err != nil {
		t.Fatalf("parseNameStatus: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d entries, want 0", len(got))
	}
}

func TestParseNameStatusRejectsTruncated(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{"status with no path", "M\x00"},
		{"rename missing its second path", "R100\x00vecchio.txt\x00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseNameStatus([]byte(tt.data)); err == nil {
				t.Fatal("expected an error, got nil")
			}
		})
	}
}

// --- against a real repository ---------------------------------------------

func TestCommitFilesAgainstRealRepo(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()

	commits, err := repo.Commits(ctx, LogOptions{All: true})
	if err != nil {
		t.Fatal(err)
	}

	bySubject := map[string]Commit{}
	for _, c := range commits {
		bySubject[c.Subject] = c
	}

	t.Run("ordinary commit", func(t *testing.T) {
		c := bySubject["aggiunge feature"]
		files, err := repo.CommitFiles(ctx, c.Hash)
		if err != nil {
			t.Fatalf("CommitFiles: %v", err)
		}
		if len(files) != 1 || files[0].Path != "b.txt" || files[0].Status != ChangeAdded {
			t.Errorf("got %+v, want b.txt added", files)
		}
	})

	t.Run("root commit", func(t *testing.T) {
		c := bySubject["primo commit"]
		files, err := repo.CommitFiles(ctx, c.Hash)
		if err != nil {
			t.Fatalf("CommitFiles: %v", err)
		}
		if len(files) != 1 || files[0].Status != ChangeAdded {
			t.Errorf("got %+v, want a single addition", files)
		}
	})

	t.Run("merge commit is not empty", func(t *testing.T) {
		// This is the case --first-parent exists for.
		c := bySubject["Merge branch 'feature'"]
		if !c.IsMerge() {
			t.Fatalf("fixture commit is not a merge: %+v", c)
		}
		files, err := repo.CommitFiles(ctx, c.Hash)
		if err != nil {
			t.Fatalf("CommitFiles: %v", err)
		}
		if len(files) == 0 {
			t.Fatal("merge commit reported no changed files")
		}
		if files[0].Path != "b.txt" {
			t.Errorf("merge brought in %+v, want b.txt", files)
		}
	})
}

func TestDiffAgainstRealRepo(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()

	commits, err := repo.Commits(ctx, LogOptions{All: true})
	if err != nil {
		t.Fatal(err)
	}
	var target Commit
	for _, c := range commits {
		if c.Subject == "terzo su main" {
			target = c
		}
	}

	t.Run("commit diff", func(t *testing.T) {
		d, err := repo.Diff(ctx, DiffTarget{Commit: target.Hash, Path: "a.txt"})
		if err != nil {
			t.Fatalf("Diff: %v", err)
		}
		if len(d.Hunks) == 0 {
			t.Fatal("no hunks")
		}
		if d.Added == 0 {
			t.Errorf("Added = 0, want at least one added line")
		}
		if d.Path != "a.txt" {
			t.Errorf("Path = %q", d.Path)
		}
	})

	t.Run("unstaged working tree diff", func(t *testing.T) {
		// The fixture leaves a.txt modified but not staged.
		d, err := repo.Diff(ctx, DiffTarget{Path: "a.txt"})
		if err != nil {
			t.Fatalf("Diff: %v", err)
		}
		if len(d.Hunks) == 0 {
			t.Fatal("no hunks for the modified working tree file")
		}
	})

	t.Run("staged diff", func(t *testing.T) {
		// c.txt was added to the index by the fixture.
		d, err := repo.Diff(ctx, DiffTarget{Path: "c.txt", Staged: true})
		if err != nil {
			t.Fatalf("Diff: %v", err)
		}
		if d.Added == 0 {
			t.Error("staged addition reported no added lines")
		}
	})

	t.Run("path with no changes is not an error", func(t *testing.T) {
		d, err := repo.Diff(ctx, DiffTarget{Commit: target.Hash, Path: "inesistente.txt"})
		if err != nil {
			t.Fatalf("Diff: %v", err)
		}
		if len(d.Hunks) != 0 {
			t.Errorf("got %d hunks, want 0", len(d.Hunks))
		}
	})
}

func TestDiffBinaryAgainstRealRepo(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()

	// A file with a NUL byte is binary as far as git is concerned.
	path := filepath.Join(repo.Root(), "blob.bin")
	if err := os.WriteFile(path, []byte{0x00, 0x01, 0x02, 0xff, 0x00}, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", "blob.bin")
	cmd.Dir = repo.Root()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}

	d, err := repo.Diff(ctx, DiffTarget{Path: "blob.bin", Staged: true})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if !d.Binary {
		t.Errorf("Binary = false for a file containing NUL bytes: %+v", d)
	}
}
