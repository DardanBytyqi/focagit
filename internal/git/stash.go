package git

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrNothingToStash means the working tree was already clean.
var ErrNothingToStash = errors.New("there is nothing to stash")

// ErrNoStash means the entry is gone, or the stash is empty.
var ErrNoStash = errors.New("no such stash entry")

// ErrStashConflict means the entry was restored but left conflicts behind.
var ErrStashConflict = errors.New("the stash conflicts with the working tree")

// Stash is one entry of the stack. It is a commit, so it carries a Person too.
type Stash struct {
	// Ref is what git answers to; it shifts as earlier entries are dropped.
	Ref     string `json:"ref"`
	Index   int    `json:"index"`
	Hash    string `json:"hash"`
	Branch  string `json:"branch"`
	Message string `json:"message"`
	// Auto means git named it, so Message is the commit it was taken from.
	Auto   bool   `json:"auto"`
	Author Person `json:"author"`
}

// stashFields must stay in sync with the parsing order in parseStash.
var stashFields = []string{
	"%gd", // selector, stash@{0}
	"%H",  // full hash
	"%gs", // reflog subject: "WIP on main: 1a2b3c subject"
	"%an", // author name
	"%ae", // author email
	"%aI", // author date, strict ISO 8601
}

var stashFormat = strings.Join(stashFields, string(fieldSep)) + string(recordSep)

// StashOptions describes what to put away.
type StashOptions struct {
	Message string `json:"message"`
	// Untracked stashes new files too; without it they stay in the working tree.
	Untracked bool `json:"untracked"`
	// KeepIndex leaves what is staged staged, so a partial commit can follow.
	KeepIndex bool `json:"keepIndex"`
	// Paths limits the stash to these files; empty means everything.
	Paths []string `json:"paths"`
}

// Stashes lists the stack, newest first.
func (r *Repo) Stashes(ctx context.Context) ([]Stash, error) {
	out, err := r.run.output(ctx, "stash", "list", "--format="+stashFormat)
	if err != nil {
		return nil, err
	}

	list := []Stash{}
	for i, record := range strings.Split(string(out), string(recordSep)) {
		record = strings.TrimLeft(record, "\n")
		if record == "" {
			continue
		}
		s, err := parseStash(record)
		if err != nil {
			return nil, err
		}
		s.Index = i
		list = append(list, s)
	}
	return list, nil
}

func parseStash(record string) (Stash, error) {
	f := strings.Split(record, string(fieldSep))
	if len(f) != len(stashFields) {
		return Stash{}, fmt.Errorf("stash record has %d fields, want %d", len(f), len(stashFields))
	}

	when, err := time.Parse(time.RFC3339, f[5])
	if err != nil {
		return Stash{}, fmt.Errorf("stash date %q: %w", f[5], err)
	}

	branch, message, auto := splitStashSubject(f[2])
	// git already names an entry we did not name; that wording is what it is called.
	if auto {
		message = f[2]
	}
	return Stash{
		Ref:     f[0],
		Hash:    f[1],
		Branch:  branch,
		Message: message,
		Auto:    auto,
		Author:  Person{Name: f[3], Email: f[4], When: when},
	}, nil
}

// splitStashSubject reads the branch out of git's own wording for the entry.
// "WIP on main: 1a2b3c primo" was named by git; "On main: text" carries our message.
func splitStashSubject(subject string) (branch, message string, auto bool) {
	rest, ok := strings.CutPrefix(subject, "WIP on ")
	if ok {
		auto = true
	} else {
		rest, ok = strings.CutPrefix(subject, "On ")
	}
	if !ok {
		return "", subject, false
	}
	name, text, found := strings.Cut(rest, ": ")
	if !found {
		return "", subject, auto
	}
	return name, text, auto
}

// Stash puts the working tree away and leaves it clean.
func (r *Repo) Stash(ctx context.Context, opts StashOptions) error {
	args := []string{"stash", "push"}
	if opts.Untracked {
		args = append(args, "--include-untracked")
	}
	if opts.KeepIndex {
		args = append(args, "--keep-index")
	}
	if m := strings.TrimSpace(opts.Message); m != "" {
		args = append(args, "--message", m)
	}
	if len(opts.Paths) > 0 {
		args = append(args, "--")
		args = append(args, opts.Paths...)
	}

	out, err := r.run.outputQuiet(ctx, args...)
	if err != nil {
		return classifyStash(err, string(out))
	}
	// git says so on stdout and still exits 0, so nothing else would notice.
	if strings.Contains(string(out), "No local changes to save") {
		return ErrNothingToStash
	}
	return nil
}

// ApplyStash restores an entry and leaves it on the stack.
func (r *Repo) ApplyStash(ctx context.Context, ref string, index bool) error {
	return r.restoreStash(ctx, "apply", ref, index)
}

// PopStash restores an entry and drops it, unless it conflicted.
func (r *Repo) PopStash(ctx context.Context, ref string, index bool) error {
	return r.restoreStash(ctx, "pop", ref, index)
}

func (r *Repo) restoreStash(ctx context.Context, verb, ref string, index bool) error {
	args := []string{"stash", verb}
	// --index restores what was staged as staged, and fails when it cannot.
	if index {
		args = append(args, "--index")
	}
	if ref != "" {
		args = append(args, ref)
	}

	out, err := r.run.outputQuiet(ctx, args...)
	if err != nil {
		return classifyStash(err, string(out))
	}
	return nil
}

// DropStash removes an entry without restoring it.
func (r *Repo) DropStash(ctx context.Context, ref string) error {
	out, err := r.run.outputQuiet(ctx, "stash", "drop", ref)
	return classifyStash(err, string(out))
}

// StashFiles lists what an entry would change, against the commit it was taken from.
func (r *Repo) StashFiles(ctx context.Context, ref string) ([]FileChange, error) {
	return r.CommitFiles(ctx, ref)
}

// classifyStash turns git's prose refusals into sentinels the UI can react to.
func classifyStash(err error, stdout string) error {
	var ce *CommandError
	if !errors.As(err, &ce) {
		return err
	}

	// A conflicting apply reports on stdout and fails on exit; both halves matter.
	if strings.Contains(stdout, "CONFLICT") || strings.Contains(ce.Stderr, "CONFLICT") {
		return ErrStashConflict
	}
	switch {
	case strings.Contains(ce.Stderr, "is not a stash"),
		strings.Contains(ce.Stderr, "No stash entries found"),
		// The wording when the index is past the end of the stack.
		strings.Contains(ce.Stderr, "log for 'stash' only has"),
		strings.Contains(ce.Stderr, "not a valid reference"):
		return ErrNoStash
	case strings.Contains(ce.Stderr, "would be overwritten"):
		return fmt.Errorf("%w: %s", ErrWouldOverwrite, ce.Stderr)
	}
	return err
}
