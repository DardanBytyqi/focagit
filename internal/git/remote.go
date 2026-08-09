package git

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrNoRemote means the repository has nowhere to fetch from or push to.
	ErrNoRemote = errors.New("no remote configured")
	// ErrNoUpstream means the branch was never told which remote branch it follows.
	ErrNoUpstream = errors.New("branch has no upstream")
	// ErrRejected means the remote refused the push because it holds work we lack.
	ErrRejected = errors.New("push rejected")
	// ErrStaleInfo means the remote moved since our last fetch, so the lease expired.
	ErrStaleInfo = errors.New("remote has changed since the last fetch")
	// ErrAuth means git could not prove who we are.
	ErrAuth = errors.New("authentication failed")
	// ErrUnknownHost means the server is not in known_hosts, so ssh refuses.
	ErrUnknownHost = errors.New("the server is not in known_hosts")
	// ErrRemoteExists means a remote by that name is already configured.
	ErrRemoteExists = errors.New("remote already exists")
	// ErrInvalidRemoteName means git will not accept the name.
	ErrInvalidRemoteName = errors.New("invalid remote name")
)

// Remote is a named place to fetch from and push to.
type Remote struct {
	Name string `json:"name"`
	// Fetch and Push are URLs, which a remote is allowed to have separately.
	Fetch string `json:"fetch"`
	Push  string `json:"push"`
}

// Remotes lists the configured remotes, in git's own order.
func (r *Repo) Remotes(ctx context.Context) ([]Remote, error) {
	out, err := r.run.output(ctx, "remote", "-v")
	if err != nil {
		return nil, err
	}

	var list []Remote
	byName := map[string]int{}
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if line == "" {
			continue
		}
		// The format is "<name>\t<url> (fetch)" and a second line for (push).
		name, rest, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		url, kind, ok := strings.Cut(rest, " ")
		if !ok {
			continue
		}

		i, seen := byName[name]
		if !seen {
			list = append(list, Remote{Name: name})
			i = len(list) - 1
			byName[name] = i
		}
		if strings.Contains(kind, "push") {
			list[i].Push = url
		} else {
			list[i].Fetch = url
		}
	}
	return list, nil
}

// AddRemote records a place to fetch from; git never checks the URL here.
func (r *Repo) AddRemote(ctx context.Context, name, url string) error {
	name = strings.TrimSpace(name)
	url = strings.TrimSpace(url)
	if url == "" {
		return fmt.Errorf("%w: the url is empty", ErrInvalidRemoteName)
	}
	// git would read a leading dash as an option, and so would every later command.
	if name == "" || strings.HasPrefix(name, "-") {
		return fmt.Errorf("%w: %q", ErrInvalidRemoteName, name)
	}

	_, err := r.run.outputQuiet(ctx, "remote", "add", "--", name, url)
	return classifyRemoteName(err, name)
}

// RemoveRemote forgets a remote and the remote-tracking branches that came with it.
func (r *Repo) RemoveRemote(ctx context.Context, name string) error {
	_, err := r.run.output(ctx, "remote", "remove", "--", name)
	return err
}

// classifyRemoteName turns git's refusals about the name into sentinels.
func classifyRemoteName(err error, name string) error {
	var ce *CommandError
	if !errors.As(err, &ce) {
		return err
	}
	switch {
	case strings.Contains(ce.Stderr, "already exists"):
		return fmt.Errorf("%s: %w", name, ErrRemoteExists)
	case strings.Contains(ce.Stderr, "is not a valid remote name"):
		return fmt.Errorf("%w: %q", ErrInvalidRemoteName, name)
	}
	return err
}

// FetchOptions says what to bring down and what to tidy up while doing it.
type FetchOptions struct {
	// Remote is empty for every remote at once.
	Remote string `json:"remote"`
	// Prune deletes remote-tracking branches whose remote branch is gone.
	Prune bool `json:"prune"`
	Tags  bool `json:"tags"`
}

// Fetch updates the remote-tracking branches, reporting progress as it goes.
func (r *Repo) Fetch(ctx context.Context, opts FetchOptions, onProgress func(string)) error {
	args := []string{"fetch", "--progress"}
	if opts.Prune {
		args = append(args, "--prune")
	}
	if opts.Tags {
		args = append(args, "--tags")
	}
	if opts.Remote == "" {
		args = append(args, "--all")
	} else {
		args = append(args, "--", opts.Remote)
	}

	_, err := r.run.progress(ctx, onProgress, args...)
	return classifyRemote(err)
}

// PushOptions says what to send where.
type PushOptions struct {
	// Remote and Branch are empty to push the current branch to its upstream.
	Remote string `json:"remote"`
	Branch string `json:"branch"`
	// SetUpstream makes the pushed branch follow the one it was pushed to.
	SetUpstream bool `json:"setUpstream"`
	// Force overwrites the remote branch, but only if it is where we last saw it.
	Force bool `json:"force"`
	Tags  bool `json:"tags"`
}

// PushResult is what happened to one ref, read from git's machine-readable output.
type PushResult struct {
	Ref string `json:"ref"`
	// Summary is git's own wording, e.g. "[new branch]" or "[rejected] (fetch first)".
	Summary  string `json:"summary"`
	Rejected bool   `json:"rejected"`
}

// Push sends local commits to a remote and reports what the remote made of them.
func (r *Repo) Push(ctx context.Context, opts PushOptions, onProgress func(string)) ([]PushResult, error) {
	// --porcelain turns the result into something we can read without guessing.
	args := []string{"push", "--porcelain", "--progress"}
	if opts.SetUpstream {
		args = append(args, "--set-upstream")
	}
	if opts.Force {
		// A lease, never a bare --force: it refuses if the remote moved since our fetch.
		args = append(args, "--force-with-lease")
	}
	if opts.Tags {
		args = append(args, "--tags")
	}
	if opts.Remote != "" {
		args = append(args, "--", opts.Remote)
		if opts.Branch != "" {
			args = append(args, opts.Branch)
		}
	}

	out, err := r.run.progress(ctx, onProgress, args...)
	// The porcelain lines say which ref was refused and why, even on failure.
	results := parsePush(string(out))
	if err != nil {
		return results, classifyPush(err, results)
	}
	return results, nil
}

// classifyPush trusts the per-ref summary: stderr only says something failed.
func classifyPush(err error, results []PushResult) error {
	for _, res := range results {
		if res.Rejected && strings.Contains(res.Summary, "stale info") {
			return fmt.Errorf("%w: fetch and look at what arrived before forcing again", ErrStaleInfo)
		}
	}
	return classifyRemote(err)
}

// parsePush reads the "<flag>\t<from>:<to>\t<summary>" lines git prints with --porcelain.
func parsePush(out string) []PushResult {
	var list []PushResult
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		flag, rest, ok := strings.Cut(line, "\t")
		if !ok || len(flag) != 1 {
			// "To <url>" and "Done" have no flag column.
			continue
		}
		refs, summary, _ := strings.Cut(rest, "\t")
		_, to, ok := strings.Cut(refs, ":")
		if !ok {
			to = refs
		}
		list = append(list, PushResult{
			Ref:      strings.TrimPrefix(to, "refs/heads/"),
			Summary:  strings.TrimSpace(summary),
			Rejected: flag == "!",
		})
	}
	return list
}

// classifyRemote turns git's prose refusals into sentinels the UI can react to.
func classifyRemote(err error) error {
	var ce *CommandError
	if !errors.As(err, &ce) {
		return err
	}
	msg := ce.Stderr
	switch {
	case strings.Contains(msg, "does not appear to be a git repository"),
		strings.Contains(msg, "No such remote"),
		strings.Contains(msg, "No configured push destination"):
		return fmt.Errorf("%w: %s", ErrNoRemote, msg)
	case strings.Contains(msg, "has no upstream branch"):
		return ErrNoUpstream
	case strings.Contains(msg, "failed to push some refs"):
		return fmt.Errorf("%w: the remote has commits you do not have yet", ErrRejected)
	case strings.Contains(msg, "Host key verification failed"):
		return ErrUnknownHost
	case strings.Contains(msg, "Authentication failed"),
		strings.Contains(msg, "could not read Username"),
		strings.Contains(msg, "Permission denied"),
		strings.Contains(msg, "terminal prompts disabled"):
		return fmt.Errorf("%w: %s", ErrAuth, msg)
	}
	return err
}
