package git

import (
	"bytes"
	"context"
	"fmt"
	"strings"
)

// refFields must stay in sync with parseRef.
var refFields = []string{
	"%(refname)",
	"%(objectname)",
	"%(*objectname)", // peeled target, non-empty only for annotated tags
	"%(objecttype)",
	"%(upstream:short)",
	"%(HEAD)", // "*" for the checked out branch, " " otherwise
}

var refFormat = strings.Join(refFields, "%1f")

// Refs lists every branch, remote branch and tag in the repository.
func (r *Repo) Refs(ctx context.Context) ([]Ref, error) {
	out, err := r.run.output(ctx, "for-each-ref", "--format="+refFormat)
	if err != nil {
		return nil, err
	}
	return parseRefs(out)
}

func parseRefs(data []byte) ([]Ref, error) {
	// Empty, not nil: a nil slice crosses the bridge as JSON null.
	refs := []Ref{}
	for _, line := range bytes.Split(data, []byte{'\n'}) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		ref, err := parseRef(line)
		if err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, nil
}

func parseRef(line []byte) (Ref, error) {
	f := bytes.Split(line, []byte{fieldSep})
	if len(f) != len(refFields) {
		return Ref{}, fmt.Errorf("git for-each-ref: want %d fields, got %d in %q", len(refFields), len(f), line)
	}

	name := string(f[0])
	hash := string(f[1])

	// An annotated tag's objectname is the tag object, not the commit.
	if peeled := string(f[2]); peeled != "" {
		hash = peeled
	}

	kind, short := classifyRef(name)

	return Ref{
		Name:     name,
		Kind:     kind,
		Short:    short,
		Hash:     hash,
		Upstream: string(f[4]),
		Head:     string(f[5]) == "*",
	}, nil
}

func classifyRef(name string) (RefKind, string) {
	switch {
	case strings.HasPrefix(name, "refs/heads/"):
		return RefBranch, strings.TrimPrefix(name, "refs/heads/")
	case strings.HasPrefix(name, "refs/remotes/"):
		return RefRemote, strings.TrimPrefix(name, "refs/remotes/")
	case strings.HasPrefix(name, "refs/tags/"):
		return RefTag, strings.TrimPrefix(name, "refs/tags/")
	default:
		return RefOther, name
	}
}
