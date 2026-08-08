package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// netRepos builds a bare origin with one commit and two clones of it, so that
// one clone can push behind the other's back.
func netRepos(t *testing.T) (origin string, ours, theirs *Repo) {
	t.Helper()
	root := t.TempDir()
	origin = filepath.Join(root, "origin.git")

	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=T", "GIT_AUTHOR_EMAIL=t@e.c",
			"GIT_COMMITTER_NAME=T", "GIT_COMMITTER_EMAIL=t@e.c",
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}

	run(root, "init", "-q", "--bare", "-b", "main", "origin.git")
	seed := filepath.Join(root, "seed")
	run(root, "clone", "-q", origin, "seed")
	if err := os.WriteFile(filepath.Join(seed, "f.txt"), []byte("uno\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(seed, "add", "f.txt")
	run(seed, "commit", "-qm", "primo")
	run(seed, "push", "-q", "origin", "main")

	for name, dst := range map[string]**Repo{"ours": &ours, "theirs": &theirs} {
		run(root, "clone", "-q", origin, name)
		repo, err := Open(context.Background(), filepath.Join(root, name))
		if err != nil {
			t.Fatalf("Open(%s): %v", name, err)
		}
		*dst = repo
	}
	return origin, ours, theirs
}

// commitTo writes a file and commits it through the plain git binary.
func commitTo(t *testing.T, repo *Repo, text, msg string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo.Root(), "f.txt"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := repo.Stage(ctx, "f.txt"); err != nil {
		t.Fatal(err)
	}
	if err := repo.Commit(ctx, CommitOptions{Message: msg + "\n"}); err != nil {
		t.Fatal(err)
	}
}

func TestRemotes(t *testing.T) {
	origin, ours, _ := netRepos(t)

	list, err := ours.Remotes(context.Background())
	if err != nil {
		t.Fatalf("Remotes: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("got %d remotes, want 1: %+v", len(list), list)
	}
	// A remote has two lines in git's output but is one remote.
	if list[0].Name != "origin" {
		t.Errorf("name = %q, want origin", list[0].Name)
	}
	if list[0].Fetch != origin || list[0].Push != origin {
		t.Errorf("urls = %q / %q, want %q", list[0].Fetch, list[0].Push, origin)
	}
}

func TestAddRemote(t *testing.T) {
	origin, _, _ := netRepos(t)
	repo := branchRepo(t)
	ctx := context.Background()

	t.Run("adding one makes it fetchable", func(t *testing.T) {
		if err := repo.AddRemote(ctx, "origin", origin); err != nil {
			t.Fatalf("AddRemote: %v", err)
		}
		list, err := repo.Remotes(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(list) != 1 || list[0].Name != "origin" || list[0].Fetch != origin {
			t.Fatalf("remotes = %+v", list)
		}
		if err := repo.Fetch(ctx, FetchOptions{Remote: "origin"}, nil); err != nil {
			t.Errorf("Fetch from the remote just added: %v", err)
		}
	})

	t.Run("the same name twice is refused", func(t *testing.T) {
		if err := repo.AddRemote(ctx, "origin", origin); !errors.Is(err, ErrRemoteExists) {
			t.Errorf("AddRemote: got %v, want ErrRemoteExists", err)
		}
	})

	t.Run("names git would not accept are refused", func(t *testing.T) {
		for _, name := range []string{"", "  ", "ha spazi", "-trattino"} {
			if err := repo.AddRemote(ctx, name, origin); !errors.Is(err, ErrInvalidRemoteName) {
				t.Errorf("AddRemote(%q): got %v, want ErrInvalidRemoteName", name, err)
			}
		}
	})

	t.Run("an empty url is refused before git sees it", func(t *testing.T) {
		if err := repo.AddRemote(ctx, "vuoto", "  "); !errors.Is(err, ErrInvalidRemoteName) {
			t.Errorf("AddRemote: got %v, want ErrInvalidRemoteName", err)
		}
	})

	t.Run("a url that goes nowhere is accepted, and only fetching finds out", func(t *testing.T) {
		// This is git's own behaviour, and the reason the dialog fetches straight away.
		if err := repo.AddRemote(ctx, "rotto", "/non/esiste"); err != nil {
			t.Fatalf("AddRemote: %v", err)
		}
		if err := repo.Fetch(ctx, FetchOptions{Remote: "rotto"}, nil); !errors.Is(err, ErrNoRemote) {
			t.Errorf("Fetch: got %v, want ErrNoRemote", err)
		}
	})

	t.Run("removing drops it", func(t *testing.T) {
		if err := repo.RemoveRemote(ctx, "rotto"); err != nil {
			t.Fatalf("RemoveRemote: %v", err)
		}
		list, err := repo.Remotes(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, rem := range list {
			if rem.Name == "rotto" {
				t.Error("the remote survived removal")
			}
		}
	})
}

func TestPush(t *testing.T) {
	_, ours, theirs := netRepos(t)
	ctx := context.Background()

	t.Run("a normal push reports what it sent", func(t *testing.T) {
		commitTo(t, ours, "uno\ndue\n", "da noi")

		res, err := ours.Push(ctx, PushOptions{}, nil)
		if err != nil {
			t.Fatalf("Push: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("got %d results, want 1: %+v", len(res), res)
		}
		if res[0].Ref != "main" || res[0].Rejected {
			t.Errorf("result = %+v, want main accepted", res[0])
		}
	})

	t.Run("progress reaches the caller", func(t *testing.T) {
		commitTo(t, ours, "uno\ndue\ntre\n", "ancora")

		var lines []string
		if _, err := ours.Push(ctx, PushOptions{}, func(s string) { lines = append(lines, s) }); err != nil {
			t.Fatal(err)
		}
		if len(lines) == 0 {
			t.Error("no progress lines were reported")
		}
	})

	t.Run("a push that would overwrite someone else is rejected", func(t *testing.T) {
		// theirs is still at the first commit and does not know about ours.
		commitTo(t, theirs, "uno\nloro\n", "da loro")

		res, err := theirs.Push(ctx, PushOptions{}, nil)
		if !errors.Is(err, ErrRejected) {
			t.Fatalf("Push: got %v, want ErrRejected", err)
		}
		// The porcelain lines are printed even on refusal, and say which ref lost.
		if len(res) != 1 || !res[0].Rejected {
			t.Errorf("results = %+v, want one rejected ref", res)
		}
		if !strings.Contains(res[0].Summary, "rejected") {
			t.Errorf("summary = %q, want it to mention the rejection", res[0].Summary)
		}
	})

	t.Run("forcing without fetching first is refused by the lease", func(t *testing.T) {
		_, err := theirs.Push(ctx, PushOptions{Force: true}, nil)
		if !errors.Is(err, ErrStaleInfo) {
			t.Fatalf("Push --force-with-lease: got %v, want ErrStaleInfo", err)
		}
	})

	t.Run("forcing after fetching goes through", func(t *testing.T) {
		if err := theirs.Fetch(ctx, FetchOptions{}, nil); err != nil {
			t.Fatalf("Fetch: %v", err)
		}
		res, err := theirs.Push(ctx, PushOptions{Force: true}, nil)
		if err != nil {
			t.Fatalf("Push --force-with-lease after fetch: %v", err)
		}
		if len(res) != 1 || res[0].Rejected {
			t.Errorf("results = %+v, want the forced push to land", res)
		}
	})
}

func TestPushNewBranch(t *testing.T) {
	_, ours, _ := netRepos(t)
	ctx := context.Background()

	if err := ours.CreateBranch(ctx, CreateBranchOptions{Name: "feature", Checkout: true}); err != nil {
		t.Fatal(err)
	}
	commitTo(t, ours, "feature\n", "su feature")

	t.Run("a branch with no upstream is refused, not guessed at", func(t *testing.T) {
		if _, err := ours.Push(ctx, PushOptions{}, nil); !errors.Is(err, ErrNoUpstream) {
			t.Fatalf("Push: got %v, want ErrNoUpstream", err)
		}
	})

	t.Run("setting the upstream publishes it", func(t *testing.T) {
		res, err := ours.Push(ctx, PushOptions{
			Remote: "origin", Branch: "feature", SetUpstream: true,
		}, nil)
		if err != nil {
			t.Fatalf("Push -u: %v", err)
		}
		if len(res) != 1 || res[0].Ref != "feature" {
			t.Fatalf("results = %+v, want feature", res)
		}
		if !strings.Contains(res[0].Summary, "new branch") {
			t.Errorf("summary = %q, want it to say it is new", res[0].Summary)
		}

		st, err := ours.Status(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if st.Upstream != "origin/feature" {
			t.Errorf("upstream = %q, want origin/feature", st.Upstream)
		}
	})
}

func TestFetch(t *testing.T) {
	_, ours, theirs := netRepos(t)
	ctx := context.Background()

	commitTo(t, theirs, "uno\nloro\n", "da loro")
	if _, err := theirs.Push(ctx, PushOptions{}, nil); err != nil {
		t.Fatal(err)
	}

	t.Run("fetching moves the remote-tracking branch, not ours", func(t *testing.T) {
		var lines []string
		if err := ours.Fetch(ctx, FetchOptions{}, func(s string) { lines = append(lines, s) }); err != nil {
			t.Fatalf("Fetch: %v", err)
		}
		if len(lines) == 0 {
			t.Error("no progress lines were reported")
		}

		st, err := ours.Status(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if st.Behind != 1 {
			t.Errorf("behind = %d, want 1", st.Behind)
		}
		if st.Ahead != 0 {
			t.Errorf("ahead = %d, want the working tree untouched", st.Ahead)
		}
	})

	t.Run("pruning drops a remote branch that is gone", func(t *testing.T) {
		if err := theirs.CreateBranch(ctx, CreateBranchOptions{Name: "usa-e-getta"}); err != nil {
			t.Fatal(err)
		}
		if _, err := theirs.Push(ctx, PushOptions{
			Remote: "origin", Branch: "usa-e-getta",
		}, nil); err != nil {
			t.Fatal(err)
		}
		if err := ours.Fetch(ctx, FetchOptions{}, nil); err != nil {
			t.Fatal(err)
		}
		if !hasRemoteRef(t, ours, "origin/usa-e-getta") {
			t.Fatal("the branch never arrived, so pruning proves nothing")
		}

		// Delete it on the remote by pushing an empty ref.
		if _, err := theirs.run.output(ctx, "push", "origin", "--delete", "usa-e-getta"); err != nil {
			t.Fatal(err)
		}
		if err := ours.Fetch(ctx, FetchOptions{Prune: true}, nil); err != nil {
			t.Fatal(err)
		}
		if hasRemoteRef(t, ours, "origin/usa-e-getta") {
			t.Error("origin/usa-e-getta survived a pruning fetch")
		}
	})
}

func hasRemoteRef(t *testing.T, repo *Repo, name string) bool {
	t.Helper()
	refs, err := repo.Refs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, ref := range refs {
		if ref.Kind == RefRemote && ref.Short == name {
			return true
		}
	}
	return false
}

func TestFetchWithoutRemote(t *testing.T) {
	repo := branchRepo(t)

	// --all over no remotes at all is a no-op, not a failure; asking for one by
	// name is the case where the user is owed an answer.
	err := repo.Fetch(context.Background(), FetchOptions{Remote: "origin"}, nil)
	if !errors.Is(err, ErrNoRemote) {
		t.Errorf("Fetch: got %v, want ErrNoRemote", err)
	}
}
