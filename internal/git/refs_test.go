package git

import (
	"strings"
	"testing"
)

// refLine assembles one for-each-ref line the way our --format produces it.
func refLine(fields ...string) string {
	return strings.Join(fields, string(fieldSep))
}

func TestParseRefs(t *testing.T) {
	data := strings.Join([]string{
		// refname, objectname, *objectname, objecttype, upstream, HEAD
		refLine("refs/heads/feature", "e7f09c5", "", "commit", "", " "),
		refLine("refs/heads/main", "df4ccf4", "", "commit", "origin/main", "*"),
		refLine("refs/remotes/origin/main", "df4ccf4", "", "commit", "", " "),
		refLine("refs/tags/v1.0", "df4ccf4", "", "commit", "", " "),
	}, "\n")

	refs, err := parseRefs([]byte(data))
	if err != nil {
		t.Fatalf("parseRefs: %v", err)
	}
	if len(refs) != 4 {
		t.Fatalf("got %d refs, want 4", len(refs))
	}

	want := []Ref{
		{Name: "refs/heads/feature", Short: "feature", Kind: RefBranch, Hash: "e7f09c5"},
		{Name: "refs/heads/main", Short: "main", Kind: RefBranch, Hash: "df4ccf4", Upstream: "origin/main", Head: true},
		{Name: "refs/remotes/origin/main", Short: "origin/main", Kind: RefRemote, Hash: "df4ccf4"},
		{Name: "refs/tags/v1.0", Short: "v1.0", Kind: RefTag, Hash: "df4ccf4"},
	}
	for i, w := range want {
		g := refs[i]
		if g != w {
			t.Errorf("refs[%d] = %+v, want %+v", i, g, w)
		}
	}
}

func TestParseRefPeelsAnnotatedTags(t *testing.T) {
	// An annotated tag's objectname is the tag object.
	line := refLine("refs/tags/v2.0", "7a7a7a7tagobject", "c0mm1tc0mm1t", "tag", "", " ")

	ref, err := parseRef([]byte(line))
	if err != nil {
		t.Fatalf("parseRef: %v", err)
	}
	if ref.Hash != "c0mm1tc0mm1t" {
		t.Errorf("Hash = %q, want the peeled commit %q", ref.Hash, "c0mm1tc0mm1t")
	}
	if ref.Kind != RefTag || ref.Short != "v2.0" {
		t.Errorf("got %+v", ref)
	}
}

func TestParseRefsSkipsBlankLines(t *testing.T) {
	// for-each-ref ends its output with a newline.
	refs, err := parseRefs([]byte("\n"))
	if err != nil {
		t.Fatalf("parseRefs: %v", err)
	}
	if len(refs) != 0 {
		t.Errorf("got %d refs, want 0", len(refs))
	}
}

func TestParseRefRejectsShortLine(t *testing.T) {
	if _, err := parseRef([]byte("refs/heads/main" + string(fieldSep) + "abc")); err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestClassifyRef(t *testing.T) {
	tests := []struct {
		name  string
		kind  RefKind
		short string
	}{
		{"refs/heads/main", RefBranch, "main"},
		{"refs/heads/feature/nested/deep", RefBranch, "feature/nested/deep"},
		{"refs/remotes/origin/main", RefRemote, "origin/main"},
		{"refs/tags/v1.0.0", RefTag, "v1.0.0"},
		{"refs/stash", RefOther, "refs/stash"},
		{"refs/notes/commits", RefOther, "refs/notes/commits"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, short := classifyRef(tt.name)
			if kind != tt.kind || short != tt.short {
				t.Errorf("= %q/%q, want %q/%q", kind, short, tt.kind, tt.short)
			}
		})
	}
}
