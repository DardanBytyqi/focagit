package git

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestIgnoredDirs(t *testing.T) {
	repo := testRepo(t)
	root := repo.Root()

	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("build/\n*.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "build", "obj"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "build", "obj", "a.o"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "run.log"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	dirs, err := repo.IgnoredDirs(context.Background())
	if err != nil {
		t.Fatalf("IgnoredDirs: %v", err)
	}

	// The whole folder is one entry, not one per file inside it.
	if want := filepath.Join(root, "build"); !slices.Contains(dirs, want) {
		t.Errorf("dirs = %v, want it to contain %q", dirs, want)
	}
	// An ignored file is not a folder, and watching its parent is still needed.
	if bad := filepath.Join(root, "run.log"); slices.Contains(dirs, bad) {
		t.Errorf("dirs = %v, should not contain the file %q", dirs, bad)
	}
}
