package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// tagRepo is two commits on main, so a tag can point somewhere other than HEAD.
func tagRepo(t *testing.T) *Repo {
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
	// Annotated tags are objects our runner writes, and it misses the env above.
	run("config", "user.name", "Tester")
	run("config", "user.email", "t@example.com")
	write("f.txt", "uno\n")
	run("add", ".")
	run("commit", "-qm", "primo")
	write("f.txt", "due\n")
	run("add", ".")
	run("commit", "-qm", "secondo")

	repo, err := Open(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestCreateLightweightTag(t *testing.T) {
	ctx := context.Background()
	repo := tagRepo(t)

	if err := repo.CreateTag(ctx, TagOptions{Name: "v1.0"}); err != nil {
		t.Fatalf("CreateTag: %v", err)
	}

	list, err := repo.Tags(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("Tags() has %d entries, want 1", len(list))
	}
	got := list[0]
	if got.Name != "v1.0" || got.Annotated {
		t.Errorf("tag = %+v, want a lightweight v1.0", got)
	}
	// A lightweight tag has no message of its own; the commit's must not leak in.
	if got.Subject != "" {
		t.Errorf("subject = %q, want empty", got.Subject)
	}
	head, _ := repo.resolve(ctx, "HEAD")
	if got.Hash != head {
		t.Errorf("hash = %q, want HEAD %q", got.Hash, head)
	}
}

func TestCreateAnnotatedTag(t *testing.T) {
	ctx := context.Background()
	repo := tagRepo(t)

	if err := repo.CreateTag(ctx, TagOptions{Name: "v2.0", Message: "la prima release"}); err != nil {
		t.Fatalf("CreateTag: %v", err)
	}

	list, _ := repo.Tags(ctx)
	got := list[0]
	if !got.Annotated || got.Subject != "la prima release" {
		t.Errorf("tag = %+v, want an annotated one carrying its message", got)
	}
	if got.Tagger.Name != "Tester" || got.Tagger.When.IsZero() {
		t.Errorf("tagger = %+v, want it filled in", got.Tagger)
	}
	// The hash must be the commit, not the tag object it peels through.
	head, _ := repo.resolve(ctx, "HEAD")
	if got.Hash != head {
		t.Errorf("hash = %q, want the commit %q", got.Hash, head)
	}
}

func TestTagAnEarlierCommit(t *testing.T) {
	ctx := context.Background()
	repo := tagRepo(t)

	if err := repo.CreateTag(ctx, TagOptions{Name: "v0.1", Rev: "HEAD~1"}); err != nil {
		t.Fatalf("CreateTag: %v", err)
	}

	list, _ := repo.Tags(ctx)
	want, _ := repo.resolve(ctx, "HEAD~1")
	if list[0].Hash != want {
		t.Errorf("hash = %q, want the earlier commit %q", list[0].Hash, want)
	}
}

func TestTagNameAlreadyTaken(t *testing.T) {
	ctx := context.Background()
	repo := tagRepo(t)

	if err := repo.CreateTag(ctx, TagOptions{Name: "v1.0"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateTag(ctx, TagOptions{Name: "v1.0"}); !errors.Is(err, ErrTagExists) {
		t.Errorf("creating it twice = %v, want ErrTagExists", err)
	}
	// Force is what turns the refusal into a move.
	if err := repo.CreateTag(ctx, TagOptions{Name: "v1.0", Rev: "HEAD~1", Force: true}); err != nil {
		t.Fatalf("CreateTag with Force: %v", err)
	}
	list, _ := repo.Tags(ctx)
	want, _ := repo.resolve(ctx, "HEAD~1")
	if list[0].Hash != want {
		t.Errorf("the forced tag did not move: %+v", list[0])
	}
}

func TestDeleteTag(t *testing.T) {
	ctx := context.Background()
	repo := tagRepo(t)

	if err := repo.CreateTag(ctx, TagOptions{Name: "v1.0"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteTag(ctx, "v1.0"); err != nil {
		t.Fatalf("DeleteTag: %v", err)
	}
	if list, _ := repo.Tags(ctx); len(list) != 0 {
		t.Errorf("the tag survived deletion: %+v", list)
	}
	if err := repo.DeleteTag(ctx, "non-esiste"); !errors.Is(err, ErrNoTag) {
		t.Errorf("deleting a missing tag = %v, want ErrNoTag", err)
	}
}

func TestTagNamesGitRefuses(t *testing.T) {
	ctx := context.Background()
	repo := tagRepo(t)

	for _, name := range []string{"", "  ", "--force", "con spazio", "v1..0", "tag/"} {
		if err := repo.CreateTag(ctx, TagOptions{Name: name}); !errors.Is(err, ErrInvalidTagName) {
			t.Errorf("CreateTag(%q) = %v, want ErrInvalidTagName", name, err)
		}
	}
}

func TestTagsAreSortedByVersion(t *testing.T) {
	ctx := context.Background()
	repo := tagRepo(t)

	// Created out of order and within the same second, so only the sort can decide.
	for _, n := range []string{"v1.9", "v1.10", "v1.2"} {
		if err := repo.CreateTag(ctx, TagOptions{Name: n}); err != nil {
			t.Fatal(err)
		}
	}

	var got []string
	list, _ := repo.Tags(ctx)
	for _, t := range list {
		got = append(got, t.Name)
	}
	// Sorted as text, v1.9 would come out on top; as versions it is v1.10.
	want := []string{"v1.10", "v1.9", "v1.2"}
	if len(got) != len(want) {
		t.Fatalf("Tags() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Tags() = %v, want %v", got, want)
		}
	}
}
