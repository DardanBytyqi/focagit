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

// stashRepo is one commit on main, so every test starts from a clean tree.
func stashRepo(t *testing.T) (*Repo, func(name, content string)) {
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
	// Stashing goes through our own runner, which does not inherit the env above.
	run("config", "user.name", "Tester")
	run("config", "user.email", "t@example.com")
	write("f.txt", "uno\n")
	run("add", ".")
	run("commit", "-qm", "primo")

	repo, err := Open(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	return repo, write
}

func TestStashRoundTrip(t *testing.T) {
	ctx := context.Background()
	repo, write := stashRepo(t)

	write("f.txt", "due\n")
	if err := repo.Stash(ctx, StashOptions{Message: "il mio lavoro"}); err != nil {
		t.Fatalf("Stash: %v", err)
	}

	// The working tree must be back to the commit.
	if b, err := os.ReadFile(filepath.Join(repo.Root(), "f.txt")); err != nil {
		t.Fatal(err)
	} else if string(b) != "uno\n" {
		t.Errorf("after stashing the file is %q, want the committed content", b)
	}

	list, err := repo.Stashes(ctx)
	if err != nil {
		t.Fatalf("Stashes: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("Stashes() has %d entries, want 1", len(list))
	}
	got := list[0]
	if got.Ref != "stash@{0}" || got.Index != 0 {
		t.Errorf("ref = %q index = %d, want stash@{0} and 0", got.Ref, got.Index)
	}
	if got.Message != "il mio lavoro" || got.Auto {
		t.Errorf("message = %q auto = %v, want ours and not automatic", got.Message, got.Auto)
	}
	if got.Branch != "main" {
		t.Errorf("branch = %q, want main", got.Branch)
	}
	if got.Author.When.IsZero() || got.Author.Name != "Tester" || got.Hash == "" {
		t.Errorf("author or hash missing: %+v", got)
	}

	if err := repo.PopStash(ctx, got.Ref, false); err != nil {
		t.Fatalf("PopStash: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(repo.Root(), "f.txt")); string(b) != "due\n" {
		t.Errorf("after popping the file is %q, want the stashed content", b)
	}
	if list, _ := repo.Stashes(ctx); len(list) != 0 {
		t.Errorf("a popped entry is still on the stack: %+v", list)
	}
}

func TestStashWithNoMessage(t *testing.T) {
	ctx := context.Background()
	repo, write := stashRepo(t)

	write("f.txt", "due\n")
	if err := repo.Stash(ctx, StashOptions{}); err != nil {
		t.Fatalf("Stash: %v", err)
	}

	list, err := repo.Stashes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// git writes "WIP on main: <hash> primo"; the branch still has to come out.
	if list[0].Branch != "main" {
		t.Errorf("branch = %q, want main", list[0].Branch)
	}
	if !list[0].Auto {
		t.Error("an entry git named itself must be marked as such")
	}
	// An entry we did not name keeps git's own wording, the way every other client shows it.
	if !strings.HasPrefix(list[0].Message, "WIP on main: ") {
		t.Errorf("message = %q, want git's own wording", list[0].Message)
	}
}

func TestStashCleanTree(t *testing.T) {
	ctx := context.Background()
	repo, _ := stashRepo(t)

	// git exits 0 here, so without our own check this would look like success.
	if err := repo.Stash(ctx, StashOptions{}); !errors.Is(err, ErrNothingToStash) {
		t.Errorf("Stash on a clean tree = %v, want ErrNothingToStash", err)
	}
}

func TestStashUntracked(t *testing.T) {
	ctx := context.Background()
	repo, write := stashRepo(t)

	write("nuovo.txt", "x\n")
	if err := repo.Stash(ctx, StashOptions{}); !errors.Is(err, ErrNothingToStash) {
		t.Errorf("an untracked file alone is not a change to save: %v", err)
	}

	if err := repo.Stash(ctx, StashOptions{Untracked: true}); err != nil {
		t.Fatalf("Stash with Untracked: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo.Root(), "nuovo.txt")); !os.IsNotExist(err) {
		t.Error("the untracked file is still there after stashing it")
	}
}

func TestStashFiles(t *testing.T) {
	ctx := context.Background()
	repo, write := stashRepo(t)

	write("f.txt", "due\n")
	if err := repo.Stash(ctx, StashOptions{}); err != nil {
		t.Fatal(err)
	}

	files, err := repo.StashFiles(ctx, "stash@{0}")
	if err != nil {
		t.Fatalf("StashFiles: %v", err)
	}
	if len(files) != 1 || files[0].Path != "f.txt" {
		t.Errorf("StashFiles = %+v, want the one file we changed", files)
	}
}

func TestApplyKeepsTheEntry(t *testing.T) {
	ctx := context.Background()
	repo, write := stashRepo(t)

	write("f.txt", "due\n")
	if err := repo.Stash(ctx, StashOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := repo.ApplyStash(ctx, "stash@{0}", false); err != nil {
		t.Fatalf("ApplyStash: %v", err)
	}

	if list, _ := repo.Stashes(ctx); len(list) != 1 {
		t.Errorf("apply must leave the entry on the stack, got %d", len(list))
	}
}

func TestDropStash(t *testing.T) {
	ctx := context.Background()
	repo, write := stashRepo(t)

	write("f.txt", "due\n")
	if err := repo.Stash(ctx, StashOptions{Message: "primo"}); err != nil {
		t.Fatal(err)
	}
	write("f.txt", "tre\n")
	if err := repo.Stash(ctx, StashOptions{Message: "secondo"}); err != nil {
		t.Fatal(err)
	}

	// The newest is stash@{0}, so dropping it must leave the older one behind.
	if err := repo.DropStash(ctx, "stash@{0}"); err != nil {
		t.Fatalf("DropStash: %v", err)
	}
	list, err := repo.Stashes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Message != "primo" {
		t.Errorf("after dropping the newest the stack is %+v", list)
	}

	if err := repo.DropStash(ctx, "stash@{7}"); !errors.Is(err, ErrNoStash) {
		t.Errorf("DropStash on a missing entry = %v, want ErrNoStash", err)
	}
}

func TestPopRefusesADirtyTree(t *testing.T) {
	ctx := context.Background()
	repo, write := stashRepo(t)

	write("f.txt", "stash\n")
	if err := repo.Stash(ctx, StashOptions{}); err != nil {
		t.Fatal(err)
	}
	// git checks before it merges, so this never reaches a conflict.
	write("f.txt", "lavoro\n")

	if err := repo.PopStash(ctx, "stash@{0}", false); !errors.Is(err, ErrWouldOverwrite) {
		t.Errorf("PopStash over an uncommitted change = %v, want ErrWouldOverwrite", err)
	}
}

func TestPopConflict(t *testing.T) {
	ctx := context.Background()
	repo, write := stashRepo(t)

	write("f.txt", "stash\n")
	if err := repo.Stash(ctx, StashOptions{}); err != nil {
		t.Fatal(err)
	}
	// The tree is clean but the base moved: now the merge itself cannot succeed.
	write("f.txt", "commit\n")
	if err := repo.Stage(ctx, "f.txt"); err != nil {
		t.Fatal(err)
	}
	if err := repo.Commit(ctx, CommitOptions{Message: "secondo"}); err != nil {
		t.Fatal(err)
	}

	if err := repo.PopStash(ctx, "stash@{0}", false); !errors.Is(err, ErrStashConflict) {
		t.Fatalf("PopStash into a moved base = %v, want ErrStashConflict", err)
	}
	// A conflicted pop must not drop the entry: the work would be unrecoverable.
	if list, _ := repo.Stashes(ctx); len(list) != 1 {
		t.Errorf("the entry was dropped despite the conflict, stack is %+v", list)
	}
}

func TestStashStaysOutOfTheHistory(t *testing.T) {
	ctx := context.Background()
	repo, write := stashRepo(t)

	write("f.txt", "due\n")
	write("nuovo.txt", "x\n")
	if err := repo.Stash(ctx, StashOptions{Message: "via", Untracked: true}); err != nil {
		t.Fatal(err)
	}

	var subjects []string
	err := repo.Log(ctx, LogOptions{All: true}, func(c Commit) error {
		subjects = append(subjects, c.Subject)
		return nil
	})
	if err != nil {
		t.Fatalf("Log: %v", err)
	}

	// An entry is three commits under refs/, and --all alone would draw all three.
	if len(subjects) != 1 || subjects[0] != "primo" {
		t.Errorf("history is %v, want only the real commit", subjects)
	}
}

func TestSplitStashSubject(t *testing.T) {
	tests := []struct {
		subject string
		branch  string
		message string
		auto    bool
	}{
		{"WIP on main: 1a2b3c primo", "main", "1a2b3c primo", true},
		{"On develop: il mio lavoro", "develop", "il mio lavoro", false},
		{"On feature/x: a: b", "feature/x", "a: b", false},
		{"qualcosa di inatteso", "", "qualcosa di inatteso", false},
	}
	for _, tt := range tests {
		t.Run(tt.subject, func(t *testing.T) {
			branch, message, auto := splitStashSubject(tt.subject)
			if branch != tt.branch || message != tt.message || auto != tt.auto {
				t.Errorf("splitStashSubject(%q) = %q, %q, %v, want %q, %q, %v",
					tt.subject, branch, message, auto, tt.branch, tt.message, tt.auto)
			}
		})
	}
}
