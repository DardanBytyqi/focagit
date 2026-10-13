// Package credentials answers git's HTTPS username and password prompts from the app's window.
package credentials

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Question is what the window asks: where to sign in, and the username git already knows, if any.
type Question struct {
	// URL is the origin, scheme and host, with no path or user.
	URL      string `json:"url"`
	Username string `json:"username"`
}

// Answer is what the user typed; Remember keeps it in the keyring once git says it worked.
type Answer struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Remember bool   `json:"remember"`
}

// AskFunc shows the question and waits; false means the user cancelled or ctx ended.
type AskFunc func(ctx context.Context, q Question) (Answer, bool)

// Login is a username and password kept for one origin.
type Login struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// ErrNotFound means the store holds nothing for that origin.
var ErrNotFound = errors.New("no saved login")

// Store keeps one login per origin.
type Store interface {
	Get(origin string) (Login, error)
	Set(origin string, l Login) error
	Delete(origin string) error
}

// pendingTTL bounds how long an answer waits for git's password prompt or its verdict.
const pendingTTL = 2 * time.Minute

// pending is an answer git has not yet confirmed or rejected.
type pending struct {
	answer Answer
	at     time.Time
}

// Broker turns git's prompts and credential-helper calls into questions and keyring entries.
type Broker struct {
	ask   AskFunc
	store Store
	log   *slog.Logger
	now   func() time.Time

	mu      sync.Mutex
	pending map[string]pending
}

func NewBroker(ask AskFunc, store Store, log *slog.Logger) *Broker {
	return &Broker{ask: ask, store: store, log: log, now: time.Now, pending: map[string]pending{}}
}

// Handle answers one request from the askpass or credential-helper side.
func (b *Broker) Handle(ctx context.Context, req Request) Response {
	switch req.Action {
	case actionPrompt:
		return b.prompt(ctx, req.Prompt)
	case "get":
		return b.get(req.Fields)
	case "store":
		b.save(req.Fields)
	case "erase":
		b.erase(req.Fields)
	}
	// Anything else, including actions newer gits add, is a polite no-op.
	return Response{OK: true}
}

// prompt answers "Username for '…': " and "Password for '…': ", asking the window when needed.
func (b *Broker) prompt(ctx context.Context, text string) Response {
	kind, origin, user, ok := parsePrompt(text)
	if !ok {
		b.log.Warn("unrecognised git prompt", slog.String("prompt", text))
		return Response{}
	}

	// The password prompt follows the username one; the window already asked for both.
	if kind == "Password" {
		if p, ok := b.fresh(origin); ok && (user == "" || p.answer.Username == user) {
			return Response{OK: true, Answer: p.answer.Password}
		}
	}

	ans, ok := b.ask(ctx, Question{URL: origin, Username: user})
	if !ok {
		return Response{}
	}
	if user != "" {
		ans.Username = user
	}
	b.mu.Lock()
	b.pending[origin] = pending{answer: ans, at: b.now()}
	b.mu.Unlock()

	if kind == "Username" {
		return Response{OK: true, Answer: ans.Username}
	}
	return Response{OK: true, Answer: ans.Password}
}

// get hands git a saved login, and nothing when there is none so it moves on to prompting.
func (b *Broker) get(f map[string]string) Response {
	origin := originOf(f)
	if origin == "" {
		return Response{OK: true}
	}
	l, err := b.store.Get(origin)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			b.log.Warn("keyring unavailable", slog.String("err", err.Error()))
		}
		return Response{OK: true}
	}
	if u := f["username"]; u != "" && u != l.Username {
		return Response{OK: true}
	}
	return Response{OK: true, Fields: map[string]string{"username": l.Username, "password": l.Password}}
}

// save keeps the login git just used successfully, if it came from the window with Remember on.
func (b *Broker) save(f map[string]string) {
	origin := originOf(f)
	p, ok := b.take(origin)
	if !ok || !p.answer.Remember || p.answer.Username != f["username"] || p.answer.Password != f["password"] {
		return
	}
	if err := b.store.Set(origin, Login{Username: p.answer.Username, Password: p.answer.Password}); err != nil {
		b.log.Warn("could not save the login", slog.String("origin", origin), slog.String("err", err.Error()))
	}
}

// erase forgets a login the server refused, so the next attempt asks again.
func (b *Broker) erase(f map[string]string) {
	origin := originOf(f)
	b.take(origin)
	l, err := b.store.Get(origin)
	if err != nil || (f["username"] != "" && f["username"] != l.Username) {
		return
	}
	if err := b.store.Delete(origin); err != nil {
		b.log.Warn("could not forget the login", slog.String("origin", origin), slog.String("err", err.Error()))
	}
}

// fresh is the pending answer for origin, unless it is too old to belong to this attempt.
func (b *Broker) fresh(origin string) (pending, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	p, ok := b.pending[origin]
	if !ok || b.now().Sub(p.at) > pendingTTL {
		return pending{}, false
	}
	return p, true
}

// take removes and returns the pending answer for origin.
func (b *Broker) take(origin string) (pending, bool) {
	p, ok := b.fresh(origin)
	b.mu.Lock()
	delete(b.pending, origin)
	b.mu.Unlock()
	return p, ok
}

// parsePrompt reads git's prompt into its kind, the origin and the username inside the URL.
func parsePrompt(text string) (kind, origin, user string, ok bool) {
	kind, rest, found := strings.Cut(text, " for '")
	if !found || (kind != "Username" && kind != "Password") {
		return "", "", "", false
	}
	raw, _, found := strings.Cut(rest, "'")
	if !found {
		return "", "", "", false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", "", "", false
	}
	return kind, u.Scheme + "://" + u.Host, u.User.Username(), true
}

// originOf builds the origin from a credential request's protocol and host fields.
func originOf(f map[string]string) string {
	if f["protocol"] == "" || f["host"] == "" {
		return ""
	}
	return f["protocol"] + "://" + f["host"]
}
