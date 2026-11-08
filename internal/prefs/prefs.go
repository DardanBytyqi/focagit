// Package prefs stores what focagit remembers between runs.
package prefs

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// maxRecents is how many repositories the welcome screen remembers.
const maxRecents = 20

// Recent is one repository the user has opened.
type Recent struct {
	Path string    `json:"path"`
	Name string    `json:"name"`
	Last time.Time `json:"last"`
	// Missing is filled in on read, not stored: the folder may be gone or unmounted.
	Missing bool `json:"missing"`
}

// Identity is a name and email to commit under; the email identifies it.
type Identity struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Appearance is what the window looks like; zero means "not chosen yet".
type Appearance struct {
	// Theme is dark, light, or system to follow the desktop.
	Theme string `json:"theme"`
	// FontSize is the base size in pixels; everything else is relative to it.
	FontSize int `json:"fontSize"`
	// Font is "default", "system", "spacemono" or "plexmono".
	Font string `json:"font"`
}

// DefaultAppearance is what a fresh installation looks like.
func DefaultAppearance() Appearance {
	return Appearance{Theme: "dark", FontSize: 14, Font: "default"}
}

// normalise replaces anything unrecognised with the default.
func (a Appearance) normalise() Appearance {
	def := DefaultAppearance()
	switch a.Theme {
	case "dark", "light", "system":
	default:
		a.Theme = def.Theme
	}
	if a.FontSize < 10 || a.FontSize > 20 {
		a.FontSize = def.FontSize
	}
	switch a.Font {
	case "default", "system", "spacemono", "plexmono":
	default:
		a.Font = def.Font
	}
	return a
}

// Data is the whole persisted file.
type Data struct {
	Recents    []Recent   `json:"recents"`
	Identities []Identity `json:"identities"`
	Appearance Appearance `json:"appearance"`
	// Shortcuts holds only the bindings that differ from the defaults.
	Shortcuts map[string]string `json:"shortcuts"`
}

// Store is the config file, guarded so the Wails bridge can call it from anywhere.
type Store struct {
	mu   sync.Mutex
	path string
	data Data
}

// Open reads the config file, treating a missing one as an empty configuration.
func Open() (*Store, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	return openAt(filepath.Join(dir, "focagit", "prefs.json"))
}

func openAt(path string) (*Store, error) {
	s := &Store{path: path, data: Data{Recents: []Recent{}, Identities: []Identity{}, Shortcuts: map[string]string{}}}

	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	// A corrupt file must not stop the app from starting; start over instead.
	if err := json.Unmarshal(b, &s.data); err != nil {
		s.data = Data{Recents: []Recent{}, Identities: []Identity{}, Shortcuts: map[string]string{}}
		return s, nil
	}
	if s.data.Recents == nil {
		s.data.Recents = []Recent{}
	}
	if s.data.Identities == nil {
		s.data.Identities = []Identity{}
	}
	if s.data.Shortcuts == nil {
		s.data.Shortcuts = map[string]string{}
	}
	return s, nil
}

// Recents returns the list newest first, each flagged if its folder has gone.
func (s *Store) Recents() []Recent {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]Recent, 0, len(s.data.Recents))
	for _, r := range s.data.Recents {
		// .git is a directory in a clone and a file in a worktree: only check it exists.
		_, err := os.Stat(filepath.Join(r.Path, ".git"))
		r.Missing = err != nil
		out = append(out, r)
	}
	return out
}

// Remember moves a repository to the front of the recent list.
func (s *Store) Remember(path, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	kept := make([]Recent, 0, len(s.data.Recents)+1)
	kept = append(kept, Recent{Path: path, Name: name, Last: time.Now()})
	for _, r := range s.data.Recents {
		if r.Path != path && len(kept) < maxRecents {
			kept = append(kept, r)
		}
	}
	s.data.Recents = kept
	return s.save()
}

// Forget drops one repository from the list.
func (s *Store) Forget(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	kept := s.data.Recents[:0]
	for _, r := range s.data.Recents {
		if r.Path != path {
			kept = append(kept, r)
		}
	}
	s.data.Recents = kept
	return s.save()
}

// Identities returns the saved identities, in the order they were added.
func (s *Store) Identities() []Identity {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]Identity, len(s.data.Identities))
	copy(out, s.data.Identities)
	return out
}

// AddIdentity saves one, replacing any earlier one with the same email.
func (s *Store) AddIdentity(id Identity) (Identity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id.Name = strings.TrimSpace(id.Name)
	id.Email = strings.TrimSpace(id.Email)
	if id.Name == "" || id.Email == "" {
		return id, errors.New("an identity needs both a name and an email")
	}
	if !strings.Contains(id.Email, "@") {
		return id, fmt.Errorf("%q is not an email address", id.Email)
	}
	if strings.ContainsAny(id.Name+id.Email, "\n\r") {
		return id, errors.New("a line break is not allowed here")
	}

	for i, old := range s.data.Identities {
		if old.Email == id.Email {
			s.data.Identities[i] = id
			return id, s.save()
		}
	}
	s.data.Identities = append(s.data.Identities, id)
	return id, s.save()
}

// Appearance returns the stored look, with anything unrecognised defaulted.
func (s *Store) Appearance() Appearance {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.Appearance.normalise()
}

// SaveAppearance stores the look, refusing to write values it would not read back.
func (s *Store) SaveAppearance(a Appearance) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data.Appearance = a.normalise()
	return s.save()
}

// Shortcuts returns the bindings that differ from the defaults.
func (s *Store) Shortcuts() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make(map[string]string, len(s.data.Shortcuts))
	for action, combo := range s.data.Shortcuts {
		out[action] = combo
	}
	return out
}

// SaveShortcuts stores the bindings that differ from the defaults.
func (s *Store) SaveShortcuts(m map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if m == nil {
		m = map[string]string{}
	}
	s.data.Shortcuts = m
	return s.save()
}

// RemoveIdentity drops the identity with this email.
func (s *Store) RemoveIdentity(email string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	kept := make([]Identity, 0, len(s.data.Identities))
	for _, id := range s.data.Identities {
		if id.Email != email {
			kept = append(kept, id)
		}
	}
	s.data.Identities = kept
	return s.save()
}

// save writes through a temporary file so a crash cannot leave a half-written config.
func (s *Store) save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".prefs-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.path)
}
