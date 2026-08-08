package git

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// emptyRepo is a freshly initialised repository: no commits, no refs, no HEAD.
func emptyRepo(t *testing.T) *Repo {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}

	dir := t.TempDir()
	cmd := exec.Command("git", "init", "-q", "-b", "main")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	repo, err := Open(context.Background(), dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return repo
}

// A repository with no commits is the state every repository starts in, and
// every read has to answer rather than fail.
func TestEmptyRepository(t *testing.T) {
	repo := emptyRepo(t)
	ctx := context.Background()

	t.Run("log is empty, not an error", func(t *testing.T) {
		commits, err := repo.Commits(ctx, LogOptions{All: true})
		if err != nil {
			t.Fatalf("Commits: %v", err)
		}
		if len(commits) != 0 {
			t.Errorf("got %d commits, want 0", len(commits))
		}
	})

	t.Run("refs are empty", func(t *testing.T) {
		refs, err := repo.Refs(ctx)
		if err != nil {
			t.Fatalf("Refs: %v", err)
		}
		if len(refs) != 0 {
			t.Errorf("got %d refs, want 0", len(refs))
		}
	})

	t.Run("status reports the unborn branch", func(t *testing.T) {
		st, err := repo.Status(ctx)
		if err != nil {
			t.Fatalf("Status: %v", err)
		}
		if st.Head != "main" {
			t.Errorf("Head = %q, want main", st.Head)
		}
		// git writes "(initial)" where a hash would go; it is not an object id.
		if st.Oid != "" {
			t.Errorf("Oid = %q, want empty before the first commit", st.Oid)
		}
	})

	t.Run("untracked files still show a diff", func(t *testing.T) {
		if err := os.WriteFile(filepath.Join(repo.Root(), "nuovo.txt"), []byte("riga\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		d, err := repo.UntrackedDiff(ctx, "nuovo.txt")
		if err != nil {
			t.Fatalf("UntrackedDiff: %v", err)
		}
		if d.Added != 1 {
			t.Errorf("Added = %d, want 1", d.Added)
		}
	})

	t.Run("staging and unstaging work without a HEAD", func(t *testing.T) {
		if err := repo.Stage(ctx, "nuovo.txt"); err != nil {
			t.Fatalf("Stage: %v", err)
		}
		// `git restore --staged` needs a HEAD to restore from; Unstage falls
		// back to `git rm --cached` when there is none.
		if err := repo.Unstage(ctx, "nuovo.txt"); err != nil {
			t.Fatalf("Unstage: %v", err)
		}
		st, _ := repo.Status(ctx)
		for _, f := range st.Files {
			if f.Path == "nuovo.txt" && f.Staged != ChangeNone {
				t.Errorf("nuovo.txt still staged: %+v", f)
			}
		}
	})

	t.Run("the first commit can be made", func(t *testing.T) {
		if err := repo.Stage(ctx, "nuovo.txt"); err != nil {
			t.Fatal(err)
		}
		if err := repo.Commit(ctx, CommitOptions{Message: "primo commit\n"}); err != nil {
			t.Fatalf("Commit: %v", err)
		}
		commits, err := repo.Commits(ctx, LogOptions{All: true})
		if err != nil {
			t.Fatal(err)
		}
		if len(commits) != 1 || !commits[0].IsRoot() {
			t.Errorf("got %+v, want one root commit", commits)
		}
	})
}

// A nil slice marshals to JSON null, but the TypeScript generated from these
// structs promises an array — so the frontend calls .filter on null and the
// whole view dies. Nothing crossing the bridge may be nil.
func TestBridgeSlicesAreNeverNil(t *testing.T) {
	repo := emptyRepo(t)
	ctx := context.Background()

	refs, err := repo.Refs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	st, err := repo.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	commits, err := repo.Commits(ctx, LogOptions{All: true})
	if err != nil {
		t.Fatal(err)
	}
	files, err := repo.CommitFiles(ctx, "HEAD")
	// HEAD does not resolve in an empty repository; the error is fine, the
	// nil-ness of the result is what matters elsewhere.
	if err == nil && files == nil {
		t.Error("CommitFiles returned a nil slice")
	}
	diff, err := repo.Diff(ctx, DiffTarget{Path: "qualsiasi.txt"})
	if err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		name string
		v    any
	}{
		{"Refs", refs},
		{"Status.Files", st.Files},
		{"Commits", commits},
		{"FileDiff.Hunks", diff.Hunks},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.v)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(b), "null") {
				t.Errorf("marshals to %s — the frontend expects an array", b)
			}
		})
	}
}

func TestInit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	ctx := context.Background()
	dir := t.TempDir()

	if _, err := Open(ctx, dir); !errors.Is(err, ErrNotARepository) {
		t.Fatalf("a plain folder should report ErrNotARepository, got %v", err)
	}

	if err := Init(ctx, dir); err != nil {
		t.Fatalf("Init: %v", err)
	}

	repo, err := Open(ctx, dir)
	if err != nil {
		t.Fatalf("Open after Init: %v", err)
	}
	if repo.Root() != dir {
		// macOS resolves /var to /private/var, so compare loosely.
		if !strings.HasSuffix(repo.Root(), filepath.Base(dir)) {
			t.Errorf("Root() = %q, want %q", repo.Root(), dir)
		}
	}
	st, err := repo.Status(ctx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.Head == "" {
		t.Error("Head is empty; git init should leave HEAD on a branch name")
	}
}
