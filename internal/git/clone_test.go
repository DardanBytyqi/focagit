package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRepoNameFromURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://github.com/owner/project.git", "project"},
		{"https://github.com/owner/project", "project"},
		{"https://github.com/owner/project/", "project"},
		{"git@github.com:owner/project.git", "project"},
		{"ssh://git@github.com:2222/owner/project.git", "project"},
		{"file:///srv/repos/project.git", "project"},
		{"/srv/repos/project/.git", "project"},
		{"/srv/repos/project", "project"},
		// Nothing usable: the caller must ask for a name.
		{"", ""},
		{"https://github.com/", ""},
		{".git", ""},
		{"/", ""},
	}
	for _, tt := range tests {
		if got := RepoNameFromURL(tt.url); got != tt.want {
			t.Errorf("RepoNameFromURL(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestClone(t *testing.T) {
	ctx := context.Background()
	origin, _, _ := netRepos(t)
	parent := t.TempDir()

	var lines []string
	dest, err := Clone(ctx, CloneOptions{URL: origin, Parent: parent}, func(l string) {
		lines = append(lines, l)
	})
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}

	// The folder is named after the address, with the .git suffix dropped.
	if want := filepath.Join(parent, "origin"); dest != want {
		t.Errorf("dest = %q, want %q", dest, want)
	}
	repo, err := Open(ctx, dest)
	if err != nil {
		t.Fatalf("Open the clone: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo.Root(), "f.txt")); err != nil {
		t.Errorf("the working tree is not checked out: %v", err)
	}
	if len(lines) == 0 {
		t.Error("no progress was reported")
	}
}

func TestCloneName(t *testing.T) {
	ctx := context.Background()
	origin, _, _ := netRepos(t)
	parent := t.TempDir()

	dest, err := Clone(ctx, CloneOptions{URL: origin, Parent: parent, Name: "altro"}, nil)
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}
	if want := filepath.Join(parent, "altro"); dest != want {
		t.Errorf("dest = %q, want %q", dest, want)
	}
}

func TestCloneRefusesABusyFolder(t *testing.T) {
	ctx := context.Background()
	origin, _, _ := netRepos(t)
	parent := t.TempDir()

	taken := filepath.Join(parent, "origin")
	if err := os.MkdirAll(taken, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(taken, "mio.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Clone(ctx, CloneOptions{URL: origin, Parent: parent}, nil); !errors.Is(err, ErrDirNotEmpty) {
		t.Fatalf("err = %v, want ErrDirNotEmpty", err)
	}
	// The refusal must leave the user's file where it was.
	if _, err := os.Stat(filepath.Join(taken, "mio.txt")); err != nil {
		t.Errorf("the existing file is gone: %v", err)
	}
}

func TestCloneRejectsBadInput(t *testing.T) {
	ctx := context.Background()
	parent := t.TempDir()

	tests := []struct {
		name string
		opts CloneOptions
	}{
		{"no url", CloneOptions{Parent: parent}},
		{"url that is an option", CloneOptions{URL: "--upload-pack=touch /tmp/x", Parent: parent}},
		{"no name in the url", CloneOptions{URL: "https://github.com/", Parent: parent}},
		{"name that escapes the parent", CloneOptions{URL: "x", Parent: parent, Name: "../fuori"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Clone(ctx, tt.opts, nil); !errors.Is(err, ErrInvalidURL) {
				t.Fatalf("err = %v, want ErrInvalidURL", err)
			}
		})
	}
}

func TestCloneLeavesNothingBehindOnFailure(t *testing.T) {
	ctx := context.Background()
	parent := t.TempDir()
	missing := filepath.Join(t.TempDir(), "non-esiste")

	if _, err := Clone(ctx, CloneOptions{URL: missing, Parent: parent}, nil); err == nil {
		t.Fatal("expected a failure cloning a missing repository")
	}
	// A half-written folder would make the retry fail for the wrong reason.
	if _, err := os.Stat(filepath.Join(parent, "non-esiste")); !os.IsNotExist(err) {
		t.Errorf("the failed clone left its folder behind: %v", err)
	}
}

func TestCloneCancelled(t *testing.T) {
	origin, _, _ := netRepos(t)
	parent := t.TempDir()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := Clone(ctx, CloneOptions{URL: origin, Parent: parent}, nil); !errors.Is(err, ErrCancelled) {
		t.Fatalf("err = %v, want ErrCancelled", err)
	}
	if _, err := os.Stat(filepath.Join(parent, "origin")); !os.IsNotExist(err) {
		t.Error("the cancelled clone left its folder behind")
	}
}

func TestPreviewClone(t *testing.T) {
	parent := t.TempDir()
	if err := os.Mkdir(filepath.Join(parent, "occupato"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "occupato", "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := PreviewClone(CloneOptions{URL: "git@github.com:owner/libero.git", Parent: parent})
	if p.Name != "libero" || p.Path != filepath.Join(parent, "libero") || p.Taken {
		t.Errorf("preview = %+v", p)
	}

	p = PreviewClone(CloneOptions{URL: "git@github.com:owner/occupato.git", Parent: parent})
	if !p.Taken {
		t.Errorf("preview = %+v, want Taken", p)
	}

	// Nothing to show yet is not an error, it is an empty preview.
	if p := PreviewClone(CloneOptions{URL: "", Parent: parent}); p.Path != "" {
		t.Errorf("preview = %+v, want empty", p)
	}
}
