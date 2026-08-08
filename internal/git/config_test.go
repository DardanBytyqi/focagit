package git

import (
	"context"
	"errors"
	"testing"
)

func TestSignature(t *testing.T) {
	repo := branchRepo(t)
	ctx := context.Background()

	t.Run("the fixture inherits its identity", func(t *testing.T) {
		sig, err := repo.Signature(ctx)
		if err != nil {
			t.Fatalf("Signature: %v", err)
		}
		// The fixture commits with GIT_AUTHOR_* env vars and never writes a config,
		// so whatever it reports here comes from outside the repository.
		if sig.IsLocal() {
			t.Errorf("IsLocal() = true, but nothing was written locally: %+v", sig)
		}
	})

	t.Run("setting one makes it local", func(t *testing.T) {
		if err := repo.SetSignature(ctx, "Dardan", "personale@example.com"); err != nil {
			t.Fatalf("SetSignature: %v", err)
		}

		sig, err := repo.Signature(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if sig.Name != "Dardan" || sig.Email != "personale@example.com" {
			t.Errorf("Signature = %+v", sig)
		}
		if !sig.IsLocal() {
			t.Errorf("IsLocal() = false, want true: %+v", sig)
		}
		if !sig.IsComplete() {
			t.Error("IsComplete() = false")
		}
	})

	t.Run("the identity is what the commit is attributed to", func(t *testing.T) {
		if err := repo.Commit(ctx, CommitOptions{Message: "con identita locale\n", AllowEmpty: true}); err != nil {
			t.Fatalf("Commit: %v", err)
		}
		commits, err := repo.Commits(ctx, LogOptions{Limit: 1})
		if err != nil {
			t.Fatal(err)
		}
		if commits[0].Author.Email != "personale@example.com" {
			t.Errorf("author = %+v, want the local identity", commits[0].Author)
		}
	})

	t.Run("clearing it falls back", func(t *testing.T) {
		if err := repo.ClearSignature(ctx); err != nil {
			t.Fatalf("ClearSignature: %v", err)
		}
		sig, err := repo.Signature(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if sig.NameScope == "local" || sig.EmailScope == "local" {
			t.Errorf("still local after clearing: %+v", sig)
		}
		// Clearing what is already cleared is not an error.
		if err := repo.ClearSignature(ctx); err != nil {
			t.Errorf("second ClearSignature: %v", err)
		}
	})

	t.Run("values that would corrupt the config file are refused", func(t *testing.T) {
		for _, tt := range []struct{ name, email string }{
			{"", "a@b.c"},
			{"Dardan", ""},
			{"Dar\ndan", "a@b.c"},
			{"Dardan", "a@b.c\n[core] editor = rm"},
		} {
			if err := repo.SetSignature(ctx, tt.name, tt.email); !errors.Is(err, ErrInvalidSignature) {
				t.Errorf("SetSignature(%q, %q): got %v, want ErrInvalidSignature", tt.name, tt.email, err)
			}
		}
	})

	t.Run("the global identity is readable through a local one", func(t *testing.T) {
		// A private HOME so the test never touches the real global config.
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("XDG_CONFIG_HOME", home)

		if _, err := repo.run.output(ctx, "config", "--global", "user.email", "globale@example.com"); err != nil {
			t.Fatal(err)
		}
		if err := repo.SetSignature(ctx, "Locale", "locale@example.com"); err != nil {
			t.Fatal(err)
		}

		global, err := repo.GlobalSignature(ctx)
		if err != nil {
			t.Fatalf("GlobalSignature: %v", err)
		}
		if global.Email != "globale@example.com" {
			t.Errorf("global email = %q, want the global one", global.Email)
		}
		// The local one must still be what the repository actually commits as.
		sig, err := repo.Signature(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if sig.Email != "locale@example.com" {
			t.Errorf("current email = %q, want the local one", sig.Email)
		}
	})

	t.Run("a name starting with a dash is a name, not an option", func(t *testing.T) {
		if err := repo.SetSignature(ctx, "-Dardan", "-d@example.com"); err != nil {
			t.Fatalf("SetSignature: %v", err)
		}
		sig, err := repo.Signature(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if sig.Name != "-Dardan" || sig.Email != "-d@example.com" {
			t.Errorf("Signature = %+v", sig)
		}
	})
}
