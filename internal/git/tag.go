package git

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrTagExists means a tag of that name is already in the repository.
var ErrTagExists = errors.New("a tag with that name already exists")

// ErrNoTag means the tag is not there.
var ErrNoTag = errors.New("no such tag")

// ErrInvalidTagName means git itself rejected the name.
var ErrInvalidTagName = errors.New("invalid tag name")

// Tag is one entry of refs/tags, with what an annotated one carries of its own.
type Tag struct {
	Name string `json:"name"`
	// Hash is the commit it points at, peeled through the tag object.
	Hash string `json:"hash"`
	// Annotated means it is an object in its own right, with a message and a tagger.
	Annotated bool   `json:"annotated"`
	Subject   string `json:"subject"`
	Tagger    Person `json:"tagger"`
}

// tagFields must stay in sync with the parsing order in parseTag.
var tagFields = []string{
	"%(refname:short)",
	"%(objecttype)",  // "tag" when annotated, "commit" when lightweight
	"%(objectname)",  // the tag object for an annotated one
	"%(*objectname)", // the commit it peels to, empty when lightweight
	"%(contents:subject)",
	"%(taggername)",
	"%(taggeremail:trim)",
	"%(taggerdate:iso-strict)",
}

var tagFormat = strings.Join(tagFields, string(fieldSep))

// Tags lists refs/tags, highest version first.
func (r *Repo) Tags(ctx context.Context) ([]Tag, error) {
	// version:refname, not the name alone: plain text sorting puts v1.9 above v1.10.
	out, err := r.run.output(ctx,
		"for-each-ref", "--sort=-version:refname", "--format="+tagFormat, "refs/tags")
	if err != nil {
		return nil, err
	}

	list := []Tag{}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		t, err := parseTag(line)
		if err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, nil
}

func parseTag(line string) (Tag, error) {
	f := strings.Split(line, string(fieldSep))
	if len(f) != len(tagFields) {
		return Tag{}, fmt.Errorf("tag record has %d fields, want %d", len(f), len(tagFields))
	}

	t := Tag{Name: f[0], Hash: f[2], Annotated: f[1] == "tag"}
	// Only an annotated tag peels; a lightweight one already names the commit.
	if f[3] != "" {
		t.Hash = f[3]
	}
	// A lightweight tag has no message of its own, and git echoes the commit's.
	if t.Annotated {
		t.Subject = f[4]
		when, _ := time.Parse(time.RFC3339, f[7])
		t.Tagger = Person{Name: f[5], Email: f[6], When: when}
	}
	return t, nil
}

// TagOptions describes one tag to create.
type TagOptions struct {
	Name string `json:"name"`
	// Rev is what to tag; empty means HEAD.
	Rev string `json:"rev"`
	// Message makes it annotated; empty leaves it a bare pointer.
	Message string `json:"message"`
	// Force moves a tag that already exists instead of refusing.
	Force bool `json:"force"`
}

// CreateTag names a commit.
func (r *Repo) CreateTag(ctx context.Context, opts TagOptions) error {
	name := strings.TrimSpace(opts.Name)
	if err := r.CheckTagName(ctx, name); err != nil {
		return err
	}

	args := []string{"tag"}
	if opts.Force {
		args = append(args, "--force")
	}
	if msg := strings.TrimSpace(opts.Message); msg != "" {
		args = append(args, "--annotate", "--message", msg)
	}
	args = append(args, "--", name)
	if rev := strings.TrimSpace(opts.Rev); rev != "" {
		args = append(args, rev)
	}

	_, err := r.run.outputQuiet(ctx, args...)
	return classifyTag(err, name)
}

// DeleteTag removes a tag here; the remote keeps its own copy.
func (r *Repo) DeleteTag(ctx context.Context, name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("%w: %q", ErrInvalidTagName, name)
	}
	_, err := r.run.outputQuiet(ctx, "tag", "--delete", "--", name)
	return classifyTag(err, name)
}

// PushTag sends one tag to a remote, or removes the remote's copy of it.
func (r *Repo) PushTag(ctx context.Context, remote, name string, remove bool, onProgress func(string)) ([]PushResult, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("%w: %q", ErrInvalidTagName, name)
	}
	if strings.TrimSpace(remote) == "" {
		remote = "origin"
	}

	args := []string{"push", "--porcelain", "--progress"}
	if remove {
		args = append(args, "--delete")
	}
	// The full ref name, so a tag and a branch sharing a name cannot be confused.
	args = append(args, "--", remote, "refs/tags/"+name)

	out, err := r.run.progress(ctx, onProgress, args...)
	results := parsePush(string(out))
	if err != nil {
		return results, classifyPush(err, results)
	}
	return results, nil
}

// CheckTagName asks git whether the name is one it would accept.
func (r *Repo) CheckTagName(ctx context.Context, name string) error {
	// check-ref-format accepts a leading dash, but every porcelain command refuses it.
	if name == "" || strings.HasPrefix(name, "-") {
		return fmt.Errorf("%w: %q", ErrInvalidTagName, name)
	}
	if _, err := r.run.outputQuiet(ctx, "check-ref-format", "refs/tags/"+name); err != nil {
		return fmt.Errorf("%w: %q", ErrInvalidTagName, name)
	}
	return nil
}

// classifyTag turns git's refusals into sentinels the UI can react to.
func classifyTag(err error, name string) error {
	var ce *CommandError
	if !errors.As(err, &ce) {
		return err
	}
	switch {
	case strings.Contains(ce.Stderr, "already exists"):
		return fmt.Errorf("%s: %w", name, ErrTagExists)
	case strings.Contains(ce.Stderr, "not found"):
		return fmt.Errorf("%s: %w", name, ErrNoTag)
	case strings.Contains(ce.Stderr, "is not a valid tag name"):
		return fmt.Errorf("%w: %q", ErrInvalidTagName, name)
	}
	return err
}
