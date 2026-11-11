package prefs

import (
	"os"
	"path/filepath"
	"testing"
)

// store returns a Store backed by a throwaway file.
func store(t *testing.T) *Store {
	t.Helper()
	s, err := openAt(filepath.Join(t.TempDir(), "prefs.json"))
	if err != nil {
		t.Fatalf("openAt: %v", err)
	}
	return s
}

// reopen reads the same file again, which is what the next run of the app does.
func reopen(t *testing.T, s *Store) *Store {
	t.Helper()
	got, err := openAt(s.path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	return got
}

func TestRecents(t *testing.T) {
	s := store(t)

	if got := s.Recents(); len(got) != 0 {
		t.Fatalf("a fresh store has %d recents, want 0", len(got))
	}

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	for _, p := range []string{"/uno", dir, "/tre"} {
		if err := s.Remember(p, filepath.Base(p)); err != nil {
			t.Fatalf("Remember(%q): %v", p, err)
		}
	}

	got := reopen(t, s).Recents()
	if len(got) != 3 {
		t.Fatalf("got %d recents, want 3", len(got))
	}
	// Newest first.
	if got[0].Path != "/tre" {
		t.Errorf("first = %q, want /tre", got[0].Path)
	}
	// A missing folder is flagged, not dropped: the drive may be unplugged.
	if !got[2].Missing {
		t.Errorf("/uno should be flagged missing: %+v", got[2])
	}
	for _, r := range got {
		if r.Path == dir && r.Missing {
			t.Errorf("%q exists but is flagged missing", dir)
		}
	}

	t.Run("reopening moves to the front without duplicating", func(t *testing.T) {
		if err := s.Remember("/uno", "uno"); err != nil {
			t.Fatal(err)
		}
		got := s.Recents()
		if len(got) != 3 {
			t.Errorf("got %d recents, want 3", len(got))
		}
		if got[0].Path != "/uno" {
			t.Errorf("first = %q, want /uno", got[0].Path)
		}
	})

	t.Run("the list is capped", func(t *testing.T) {
		for i := range maxRecents + 10 {
			if err := s.Remember(filepath.Join("/molti", string(rune('a'+i%26)), string(rune('0'+i/26))), "x"); err != nil {
				t.Fatal(err)
			}
		}
		if got := len(s.Recents()); got != maxRecents {
			t.Errorf("got %d recents, want the cap of %d", got, maxRecents)
		}
	})

	t.Run("forget removes one", func(t *testing.T) {
		before := len(s.Recents())
		first := s.Recents()[0].Path
		if err := s.Forget(first); err != nil {
			t.Fatal(err)
		}
		after := reopen(t, s).Recents()
		if len(after) != before-1 {
			t.Errorf("got %d recents, want %d", len(after), before-1)
		}
		for _, r := range after {
			if r.Path == first {
				t.Errorf("%q is still in the list", first)
			}
		}
	})
}

func TestIdentities(t *testing.T) {
	s := store(t)

	work := Identity{Name: "Nome Lavoro", Email: "lavoro@example.com"}
	home := Identity{Name: "Nome Personale", Email: "personale@example.com"}
	for _, id := range []Identity{work, home} {
		if _, err := s.AddIdentity(id); err != nil {
			t.Fatalf("AddIdentity(%q): %v", id.Email, err)
		}
	}

	got := reopen(t, s).Identities()
	if len(got) != 2 {
		t.Fatalf("got %d identities, want 2", len(got))
	}
	// The order they were added in is the order they are offered in.
	if got[0].Email != work.Email {
		t.Errorf("first = %q, want %q", got[0].Email, work.Email)
	}

	t.Run("the same email replaces rather than duplicates", func(t *testing.T) {
		renamed := work
		renamed.Name = "Dardan B."
		if _, err := s.AddIdentity(renamed); err != nil {
			t.Fatal(err)
		}
		got := s.Identities()
		if len(got) != 2 {
			t.Fatalf("got %d identities, want 2", len(got))
		}
		if got[0].Name != "Dardan B." {
			t.Errorf("name = %q, want it updated in place", got[0].Name)
		}
	})

	t.Run("incomplete or impossible identities are refused", func(t *testing.T) {
		bad := []Identity{
			{Name: "senza email"},
			{Email: "senza-nome@y.z"},
			{Name: "non un indirizzo", Email: "non-una-email"},
			{Name: "a capo\nx", Email: "x@y.z"},
		}
		for _, id := range bad {
			if _, err := s.AddIdentity(id); err == nil {
				t.Errorf("AddIdentity accepted %+v", id)
			}
		}
		// A refusal must not have touched what was already stored.
		if got := s.Identities(); len(got) != 2 {
			t.Errorf("the stored list changed to %d entries", len(got))
		}
	})

	t.Run("removing keeps the rest", func(t *testing.T) {
		if err := s.RemoveIdentity(home.Email); err != nil {
			t.Fatal(err)
		}
		for _, id := range reopen(t, s).Identities() {
			if id.Email == home.Email {
				t.Errorf("%q is still stored", home.Email)
			}
		}
	})
}

func TestCorruptFileDoesNotStopTheApp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prefs.json")
	if err := os.WriteFile(path, []byte("{ questo non e json"), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := openAt(path)
	if err != nil {
		t.Fatalf("openAt on a corrupt file: %v", err)
	}
	if len(s.Recents()) != 0 {
		t.Error("a corrupt file should read as empty")
	}
	// And writing must fix it rather than fail forever.
	if err := s.Remember("/qualcosa", "qualcosa"); err != nil {
		t.Fatalf("Remember after a corrupt read: %v", err)
	}
	if len(reopen(t, s).Recents()) != 1 {
		t.Error("the store did not recover")
	}
}

func TestAppearance(t *testing.T) {
	s := store(t)

	// A fresh store has never been written to, so it answers with the default.
	if got := s.Appearance(); got != DefaultAppearance() {
		t.Errorf("Appearance() = %+v, want the default", got)
	}

	want := Appearance{Theme: "light", FontSize: 15, Font: "default"}
	if err := s.SaveAppearance(want); err != nil {
		t.Fatalf("SaveAppearance: %v", err)
	}
	if got := reopen(t, s).Appearance(); got != want {
		t.Errorf("Appearance() = %+v, want %+v", got, want)
	}

	t.Run("values we would not read back are defaulted, not stored", func(t *testing.T) {
		if err := s.SaveAppearance(Appearance{Theme: "fucsia", FontSize: 400}); err != nil {
			t.Fatal(err)
		}
		// A 400px font would make the window unusable and leave no way back.
		if got := reopen(t, s).Appearance(); got != DefaultAppearance() {
			t.Errorf("Appearance() = %+v, want the default", got)
		}
	})
}
