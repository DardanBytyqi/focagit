package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// branchRepo is two commits on main plus a branch that changes the same file.
func branchRepo(t *testing.T) *Repo {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}

	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Tester", "GIT_AUTHOR_EMAIL=t@example.com",
			"GIT_COMMITTER_NAME=Tester", "GIT_COMMITTER_EMAIL=t@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	run("init", "-q", "-b", "main")
	write("f.txt", "main\n")
	run("add", ".")
	run("commit", "-qm", "primo")

	run("switch", "-qc", "altro")
	write("f.txt", "altro\n")
	run("commit", "-qam", "su altro")
	run("switch", "-q", "main")

	repo, err := Open(context.Background(), dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return repo
}

// head returns the current branch, empty when detached.
func head(t *testing.T, repo *Repo) string {
	t.Helper()
	st, err := repo.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	return st.Head
}

func TestCheckout(t *testing.T) {
	repo := branchRepo(t)
	ctx := context.Background()

	if err := repo.Checkout(ctx, "altro"); err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	if got := head(t, repo); got != "altro" {
		t.Errorf("Head = %q, want altro", got)
	}

	// The file content must follow the branch, not just the ref.
	b, err := os.ReadFile(filepath.Join(repo.Root(), "f.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "altro\n" {
		t.Errorf("f.txt = %q, want altro", b)
	}
}

func TestCheckoutRefusesToDestroyChanges(t *testing.T) {
	repo := branchRepo(t)
	ctx := context.Background()

	if err := os.WriteFile(filepath.Join(repo.Root(), "f.txt"), []byte("in corso\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := repo.Checkout(ctx, "altro")
	if !errors.Is(err, ErrWouldOverwrite) {
		t.Fatalf("Checkout: got %v, want ErrWouldOverwrite", err)
	}
	if got := head(t, repo); got != "main" {
		t.Errorf("Head = %q; the failed checkout must leave us where we were", got)
	}
}

func TestCheckoutDetached(t *testing.T) {
	repo := branchRepo(t)
	ctx := context.Background()

	commits, err := repo.Commits(ctx, LogOptions{All: true})
	if err != nil {
		t.Fatal(err)
	}
	root := commits[len(commits)-1]

	if err := repo.CheckoutDetached(ctx, root.Hash); err != nil {
		t.Fatalf("CheckoutDetached: %v", err)
	}
	st, err := repo.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Detached {
		t.Error("Detached = false, want true")
	}
	if st.Oid != root.Hash {
		t.Errorf("Oid = %q, want %q", st.Oid, root.Hash)
	}
}

func TestCreateBranch(t *testing.T) {
	repo := branchRepo(t)
	ctx := context.Background()

	t.Run("without checkout we stay put", func(t *testing.T) {
		if err := repo.CreateBranch(ctx, CreateBranchOptions{Name: "solo"}); err != nil {
			t.Fatalf("CreateBranch: %v", err)
		}
		if !repo.HasBranch(ctx, "solo") {
			t.Error("branch solo was not created")
		}
		if got := head(t, repo); got != "main" {
			t.Errorf("Head = %q, want main", got)
		}
	})

	t.Run("with checkout we move", func(t *testing.T) {
		err := repo.CreateBranch(ctx, CreateBranchOptions{Name: "lavoro", Checkout: true})
		if err != nil {
			t.Fatalf("CreateBranch: %v", err)
		}
		if got := head(t, repo); got != "lavoro" {
			t.Errorf("Head = %q, want lavoro", got)
		}
	})

	t.Run("from an explicit start point", func(t *testing.T) {
		err := repo.CreateBranch(ctx, CreateBranchOptions{Name: "da-altro", Start: "altro"})
		if err != nil {
			t.Fatalf("CreateBranch: %v", err)
		}
		refs, err := repo.Refs(ctx)
		if err != nil {
			t.Fatal(err)
		}
		byShort := map[string]Ref{}
		for _, r := range refs {
			byShort[r.Short] = r
		}
		if byShort["da-altro"].Hash != byShort["altro"].Hash {
			t.Errorf("da-altro is at %q, want altro's %q",
				byShort["da-altro"].Hash, byShort["altro"].Hash)
		}
	})

	t.Run("tracking sets the upstream", func(t *testing.T) {
		err := repo.CreateBranch(ctx, CreateBranchOptions{Name: "seguace", Start: "main", Track: true})
		if err != nil {
			t.Fatalf("CreateBranch: %v", err)
		}
		refs, err := repo.Refs(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range refs {
			if r.Short == "seguace" && r.Upstream != "main" {
				t.Errorf("Upstream = %q, want main", r.Upstream)
			}
		}
	})

	t.Run("a duplicate name is reported as such", func(t *testing.T) {
		err := repo.CreateBranch(ctx, CreateBranchOptions{Name: "altro"})
		if !errors.Is(err, ErrBranchExists) {
			t.Errorf("got %v, want ErrBranchExists", err)
		}
	})

	t.Run("git decides which names are legal", func(t *testing.T) {
		for _, name := range []string{"", "  ", "con spazio", "-inizia-con-trattino", "cade..fuori", "finisce.lock"} {
			if err := repo.CreateBranch(ctx, CreateBranchOptions{Name: name}); !errors.Is(err, ErrInvalidBranchName) {
				t.Errorf("CreateBranch(%q): got %v, want ErrInvalidBranchName", name, err)
			}
		}
	})
}

func TestDeleteBranch(t *testing.T) {
	repo := branchRepo(t)
	ctx := context.Background()

	if err := repo.CreateBranch(ctx, CreateBranchOptions{Name: "usa-e-getta"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteBranch(ctx, "usa-e-getta", false); err != nil {
		t.Fatalf("DeleteBranch: %v", err)
	}
	if repo.HasBranch(ctx, "usa-e-getta") {
		t.Error("branch usa-e-getta is still there")
	}

	// altro holds a commit that is on no other branch, so -d must refuse.
	err := repo.DeleteBranch(ctx, "altro", false)
	if !errors.Is(err, ErrBranchNotMerged) {
		t.Fatalf("got %v, want ErrBranchNotMerged", err)
	}
	if !repo.HasBranch(ctx, "altro") {
		t.Error("altro was deleted despite the refusal")
	}

	if err := repo.DeleteBranch(ctx, "altro", true); err != nil {
		t.Fatalf("forced DeleteBranch: %v", err)
	}
	if repo.HasBranch(ctx, "altro") {
		t.Error("forced delete left the branch behind")
	}
}

func TestRenameBranch(t *testing.T) {
	repo := branchRepo(t)
	ctx := context.Background()

	if err := repo.RenameBranch(ctx, "altro", "rinominato"); err != nil {
		t.Fatalf("RenameBranch: %v", err)
	}
	if repo.HasBranch(ctx, "altro") || !repo.HasBranch(ctx, "rinominato") {
		t.Error("the rename did not take")
	}

	// Renaming the branch we are on has to move HEAD with it.
	if err := repo.RenameBranch(ctx, "main", "principale"); err != nil {
		t.Fatalf("RenameBranch: %v", err)
	}
	if got := head(t, repo); got != "principale" {
		t.Errorf("Head = %q, want principale", got)
	}

	if err := repo.RenameBranch(ctx, "principale", "rinominato"); !errors.Is(err, ErrBranchExists) {
		t.Errorf("got %v, want ErrBranchExists", err)
	}
}

func TestCheckoutRemote(t *testing.T) {
	origin := branchRepo(t)
	ctx := context.Background()

	dir := t.TempDir()
	clone := filepath.Join(dir, "clone")
	cmd := exec.Command("git", "clone", "-q", origin.Root(), clone)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("clone: %v\n%s", err, out)
	}
	repo, err := Open(ctx, clone)
	if err != nil {
		t.Fatal(err)
	}

	// origin/altro exists only as a remote ref; checking it out creates the local one.
	if repo.HasBranch(ctx, "altro") {
		t.Fatal("the clone already has a local altro")
	}
	if err := repo.CheckoutRemote(ctx, "origin/altro"); err != nil {
		t.Fatalf("CheckoutRemote: %v", err)
	}
	if got := head(t, repo); got != "altro" {
		t.Fatalf("Head = %q, want altro", got)
	}

	st, err := repo.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.Upstream != "origin/altro" {
		t.Errorf("Upstream = %q, want origin/altro", st.Upstream)
	}

	// Doing it again, with the local branch now present, is a plain switch.
	if err := repo.Checkout(ctx, "main"); err != nil {
		t.Fatal(err)
	}
	if err := repo.CheckoutRemote(ctx, "origin/altro"); err != nil {
		t.Fatalf("second CheckoutRemote: %v", err)
	}
	if got := head(t, repo); got != "altro" {
		t.Errorf("Head = %q, want altro", got)
	}
}
