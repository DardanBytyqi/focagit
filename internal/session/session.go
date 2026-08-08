// Package session holds the application state that outlives a single request.
package session

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/dardanbytyqi/focagit/internal/git"
)

// Info is the summary of an open repository sent to the frontend.
type Info struct {
	ID   string `json:"id"`   // stable identifier; the working tree root path
	Path string `json:"path"` // working tree root
	Name string `json:"name"` // last path element, shown on the tab
}

// Session owns every open repository.
type Session struct {
	mu    sync.RWMutex
	repos map[string]*Repo
}

func New() *Session {
	return &Session{repos: make(map[string]*Repo)}
}

// Open resolves path and registers it.
func (s *Session) Open(ctx context.Context, path string) (*Repo, error) {
	g, err := git.Open(ctx, path)
	if err != nil {
		return nil, err
	}
	root := g.Root()

	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.repos[root]; ok {
		return existing, nil
	}

	r := &Repo{
		Info: Info{ID: root, Path: root, Name: filepath.Base(root)},
		git:  g,
	}
	s.repos[root] = r
	return r, nil
}

// Get returns an open repository by id.
func (s *Session) Get(id string) (*Repo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	r, ok := s.repos[id]
	if !ok {
		return nil, fmt.Errorf("repository %q is not open", id)
	}
	return r, nil
}

// Close stops any in-flight load and forgets the repository.
func (s *Session) Close(id string) {
	s.mu.Lock()
	r, ok := s.repos[id]
	delete(s.repos, id)
	s.mu.Unlock()

	if ok {
		r.cancelLoad()
	}
}

// List returns every open repository, for restoring tabs.
func (s *Session) List() []Info {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Info, 0, len(s.repos))
	for _, r := range s.repos {
		out = append(out, r.Info)
	}
	return out
}
