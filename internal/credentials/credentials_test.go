package credentials

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/cgi"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestMain lets git run this test binary as its askpass and credential helper.
func TestMain(m *testing.M) {
	if code, ok := Main(os.Args[1:]); ok {
		os.Exit(code)
	}
	os.Exit(m.Run())
}

// memStore is a keyring that lives in memory.
type memStore struct {
	mu     sync.Mutex
	logins map[string]Login
}

func newMemStore() *memStore { return &memStore{logins: map[string]Login{}} }

func (s *memStore) Get(origin string) (Login, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.logins[origin]
	if !ok {
		return Login{}, ErrNotFound
	}
	return l, nil
}

func (s *memStore) Set(origin string, l Login) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logins[origin] = l
	return nil
}

func (s *memStore) Delete(origin string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.logins, origin)
	return nil
}

// asker answers every question the same way and counts them.
type asker struct {
	mu        sync.Mutex
	answer    Answer
	ok        bool
	questions []Question
}

func (a *asker) ask(_ context.Context, q Question) (Answer, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.questions = append(a.questions, q)
	return a.answer, a.ok
}

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestParsePrompt(t *testing.T) {
	for _, c := range []struct {
		in, kind, origin, user string
		ok                     bool
	}{
		{"Username for 'https://github.com': ", "Username", "https://github.com", "", true},
		{"Password for 'https://dardo@github.com': ", "Password", "https://github.com", "dardo", true},
		{"Password for 'http://127.0.0.1:8080/x.git': ", "Password", "http://127.0.0.1:8080", "", true},
		{"Enter passphrase for key '/home/x/.ssh/id_ed25519': ", "", "", "", false},
		{"Username for 'not a url': ", "", "", "", false},
	} {
		kind, origin, user, ok := parsePrompt(c.in)
		if kind != c.kind || origin != c.origin || user != c.user || ok != c.ok {
			t.Errorf("parsePrompt(%q) = %q %q %q %v", c.in, kind, origin, user, ok)
		}
	}
}

func TestPasswordPromptReusesTheAnswer(t *testing.T) {
	a := &asker{answer: Answer{Username: "dardo", Password: "segreto"}, ok: true}
	b := NewBroker(a.ask, newMemStore(), quiet())
	ctx := context.Background()

	if r := b.Handle(ctx, Request{Action: actionPrompt, Prompt: "Username for 'https://host': "}); r.Answer != "dardo" {
		t.Fatalf("username answer is %+v", r)
	}
	if r := b.Handle(ctx, Request{Action: actionPrompt, Prompt: "Password for 'https://dardo@host': "}); r.Answer != "segreto" {
		t.Fatalf("password answer is %+v", r)
	}
	if len(a.questions) != 1 {
		t.Errorf("asked %d times, want once for both prompts", len(a.questions))
	}
}

func TestPasswordPromptKeepsTheUsernameInTheURL(t *testing.T) {
	a := &asker{answer: Answer{Username: "altro", Password: "segreto"}, ok: true}
	b := NewBroker(a.ask, newMemStore(), quiet())

	r := b.Handle(context.Background(), Request{Action: actionPrompt, Prompt: "Password for 'https://dardo@host': "})
	if r.Answer != "segreto" || len(a.questions) != 1 || a.questions[0].Username != "dardo" {
		t.Errorf("answer %+v after %+v, want the window told the username is dardo", r, a.questions)
	}
}

func TestStoreOnlyWhatTheUserAskedToRemember(t *testing.T) {
	ctx := context.Background()
	used := map[string]string{"protocol": "https", "host": "host", "username": "dardo", "password": "segreto"}
	for _, remember := range []bool{true, false} {
		store := newMemStore()
		a := &asker{answer: Answer{Username: "dardo", Password: "segreto", Remember: remember}, ok: true}
		b := NewBroker(a.ask, store, quiet())

		b.Handle(ctx, Request{Action: actionPrompt, Prompt: "Username for 'https://host': "})
		b.Handle(ctx, Request{Action: "store", Fields: used})

		_, err := store.Get("https://host")
		if saved := err == nil; saved != remember {
			t.Errorf("remember=%v but saved=%v", remember, saved)
		}
	}
}

func TestStoreIgnoresLoginsFromElsewhere(t *testing.T) {
	store := newMemStore()
	b := NewBroker((&asker{}).ask, store, quiet())

	// Git also tells us about logins another helper supplied; those are not ours to copy.
	b.Handle(context.Background(), Request{Action: "store", Fields: map[string]string{
		"protocol": "https", "host": "host", "username": "dardo", "password": "segreto",
	}})
	if _, err := store.Get("https://host"); err == nil {
		t.Error("a login the window never asked for was saved")
	}
}

func TestEraseForgetsARefusedLogin(t *testing.T) {
	store := newMemStore()
	store.Set("https://host", Login{Username: "dardo", Password: "vecchia"})
	b := NewBroker((&asker{}).ask, store, quiet())

	b.Handle(context.Background(), Request{Action: "erase", Fields: map[string]string{
		"protocol": "https", "host": "host", "username": "dardo",
	}})
	if _, err := store.Get("https://host"); err == nil {
		t.Error("the refused login is still saved")
	}
}

// gitServer serves bare repositories over HTTP behind basic auth, like a forge would.
func gitServer(t *testing.T, user, pass string) (url string, root string) {
	t.Helper()
	execPath, err := exec.Command("git", "--exec-path").Output()
	if err != nil {
		t.Skip("git not available")
	}
	root = t.TempDir()
	bare := filepath.Join(root, "repo.git")
	work := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "--bare", bare},
		{"-C", bare, "config", "http.receivepack", "true"},
		{"init", "-q", work},
		{"-C", work, "-c", "user.name=T", "-c", "user.email=t@e", "commit", "-q", "--allow-empty", "-m", "first"},
		{"-C", work, "push", "-q", bare, "HEAD:refs/heads/main"},
		{"-C", bare, "symbolic-ref", "HEAD", "refs/heads/main"},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	backend := &cgi.Handler{
		Path: filepath.Join(strings.TrimSpace(string(execPath)), "git-http-backend"),
		Env:  []string{"GIT_PROJECT_ROOT=" + root, "GIT_HTTP_EXPORT_ALL=1"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u, p, ok := r.BasicAuth(); !ok || u != user || p != pass {
			w.Header().Set("WWW-Authenticate", `Basic realm="test"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		backend.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv.URL, root
}

// cloneWith runs a real git clone whose prompts and helper calls reach the broker.
func cloneWith(t *testing.T, srv *Server, url string) error {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "clone", "-q", url+"/repo.git", filepath.Join(t.TempDir(), "clone"))
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_CONFIG_NOSYSTEM=1", "HOME="+t.TempDir())
	cmd.Env = append(cmd.Env, srv.Env(exe)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("git clone: %s", out)
	}
	return err
}

func TestCloneOverHTTPAsksThenRemembers(t *testing.T) {
	url, _ := gitServer(t, "dardo", "segreto")
	store := newMemStore()
	a := &asker{answer: Answer{Username: "dardo", Password: "segreto", Remember: true}, ok: true}
	srv, err := Listen(NewBroker(a.ask, store, quiet()).Handle)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	if err := cloneWith(t, srv, url); err != nil {
		t.Fatalf("first clone: %v", err)
	}
	if len(a.questions) != 1 || a.questions[0].URL != url {
		t.Fatalf("questions %+v, want one for %s", a.questions, url)
	}
	if l, err := store.Get(url); err != nil || l.Password != "segreto" {
		t.Fatalf("saved %+v, %v", l, err)
	}

	// The second time the keyring answers, and the window stays shut.
	if err := cloneWith(t, srv, url); err != nil {
		t.Fatalf("second clone: %v", err)
	}
	if len(a.questions) != 1 {
		t.Errorf("asked %d times, want the saved login used", len(a.questions))
	}
}

func TestWrongSavedPasswordIsForgotten(t *testing.T) {
	url, _ := gitServer(t, "dardo", "nuova")
	store := newMemStore()
	store.Set(url, Login{Username: "dardo", Password: "vecchia"})
	srv, err := Listen(NewBroker((&asker{}).ask, store, quiet()).Handle)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	if err := cloneWith(t, srv, url); err == nil {
		t.Fatal("clone with a stale password succeeded")
	}
	if _, err := store.Get(url); err == nil {
		t.Error("the stale password is still saved, so every attempt would fail the same way")
	}
}

func TestCancellingFailsTheClone(t *testing.T) {
	url, _ := gitServer(t, "dardo", "segreto")
	a := &asker{ok: false}
	srv, err := Listen(NewBroker(a.ask, newMemStore(), quiet()).Handle)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	if err := cloneWith(t, srv, url); err == nil {
		t.Fatal("clone succeeded with no credentials")
	}
	if len(a.questions) != 1 {
		t.Errorf("asked %d times, want once", len(a.questions))
	}
}

func TestServerRejectsAWrongToken(t *testing.T) {
	called := false
	srv, err := Listen(func(context.Context, Request) Response { called = true; return Response{OK: true} })
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	if _, err := call(srv.path, Request{Token: "nope", Action: actionPrompt}); err == nil || called {
		t.Errorf("a request with the wrong token was answered (err %v)", err)
	}
}
