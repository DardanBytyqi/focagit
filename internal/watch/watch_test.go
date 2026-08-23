package watch

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// tree builds a repository-shaped folder and watches it, skipping the named folders.
func tree(t *testing.T, skipNames ...string) (root string, changes chan bool) {
	t.Helper()
	root = t.TempDir()
	gitDir := filepath.Join(root, ".git")

	skip := make([]string, len(skipNames))
	for i, name := range skipNames {
		skip[i] = filepath.Join(root, name)
	}

	for _, dir := range []string{
		filepath.Join(gitDir, "refs", "heads"),
		filepath.Join(gitDir, "objects"),
		filepath.Join(root, "src"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, dir := range skip {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	changes = make(chan bool, 8)
	w, err := Start(Config{
		Root:     root,
		GitDir:   gitDir,
		Skip:     skip,
		Debounce: 40 * time.Millisecond,
		OnChange: func(refs bool) { changes <- refs },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	return root, changes
}

func write(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// waitChange returns whether the change touched the refs, or fails if none arrives.
func waitChange(t *testing.T, changes chan bool) bool {
	t.Helper()
	select {
	case refs := <-changes:
		return refs
	case <-time.After(2 * time.Second):
		t.Fatal("no change was reported")
		return false
	}
}

func expectQuiet(t *testing.T, changes chan bool) {
	t.Helper()
	select {
	case <-changes:
		t.Fatal("a change was reported for something we ignore")
	case <-time.After(300 * time.Millisecond):
	}
}

func TestWatchSeesAWrite(t *testing.T) {
	root, changes := tree(t)

	write(t, filepath.Join(root, "src", "main.go"))
	if refs := waitChange(t, changes); refs {
		t.Error("a working tree write was reported as a ref change")
	}
}

func TestWatchSeesARefMove(t *testing.T) {
	root, changes := tree(t)

	write(t, filepath.Join(root, ".git", "refs", "heads", "main"))
	if refs := waitChange(t, changes); !refs {
		t.Error("a ref move was reported as a plain write")
	}
}

func TestWatchSeesANewFolder(t *testing.T) {
	root, changes := tree(t)

	// A folder created after the start is not watched unless the watcher adds it.
	sub := filepath.Join(root, "src", "nuovo")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	waitChange(t, changes)

	write(t, filepath.Join(sub, "f.txt"))
	waitChange(t, changes)
}

func TestWatchIgnoresTheNoise(t *testing.T) {
	root, changes := tree(t)

	// Objects are written by every fetch, and the lock by every write, ours included.
	write(t, filepath.Join(root, ".git", "objects", "loose"))
	write(t, filepath.Join(root, ".git", "index.lock"))
	expectQuiet(t, changes)
}

func TestWatchSkipsIgnoredFolders(t *testing.T) {
	// The skip list is what a repository's ignored folders would be.
	root, changes := tree(t, "build")

	write(t, filepath.Join(root, "build", "output.bin"))
	expectQuiet(t, changes)
}

func TestWatchSkipsAFolderIgnoredLater(t *testing.T) {
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	if err := os.MkdirAll(filepath.Join(gitDir, "refs"), 0o755); err != nil {
		t.Fatal(err)
	}

	changes := make(chan bool, 8)
	w, err := Start(Config{
		Root:     root,
		GitDir:   gitDir,
		Debounce: 40 * time.Millisecond,
		Ignored:  func(dir string) bool { return filepath.Base(dir) == "node_modules" },
		OnChange: func(refs bool) { changes <- refs },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })

	// git would not show it, so neither its birth nor its contents are a change.
	deps := filepath.Join(root, "node_modules")
	if err := os.Mkdir(deps, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(deps, "pkg", "dist"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(deps, "pkg", "dist", "index.js"))
	expectQuiet(t, changes)
}

func TestWatchCollapsesABurst(t *testing.T) {
	root, changes := tree(t)

	for i := range 20 {
		write(t, filepath.Join(root, "src", string(rune('a'+i))+".txt"))
	}
	waitChange(t, changes)
	// One quiet period, one report: twenty writes must not be twenty refreshes.
	expectQuiet(t, changes)
}
