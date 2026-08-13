package git

import (
	"bufio"
	"context"
	"strings"
	"testing"
	"time"
)

// record assembles one git log record the way our --format produces it.
func record(fields ...string) string {
	return strings.Join(fields, string(fieldSep)) + string(recordSep) + "\n"
}

func TestParseCommit(t *testing.T) {
	tests := []struct {
		name   string
		fields []string
		want   Commit
	}{
		{
			name: "root commit has no parents",
			fields: []string{
				"16892e646940afd3f1ef61082dc4eba055d0034a",
				"", // a root commit reports an empty parent field
				"Dardan B", "d@example.com", "2026-08-07T15:37:07+02:00",
				"Dardan B", "d@example.com", "2026-08-07T15:37:07+02:00",
				"primo commit", "",
			},
			want: Commit{
				Hash:    "16892e646940afd3f1ef61082dc4eba055d0034a",
				Parents: nil,
				Subject: "primo commit",
				Body:    "",
			},
		},
		{
			name: "merge commit has two parents",
			fields: []string{
				"df4ccf47cde62f72b0b64194c9a09ab07bcfd504",
				"cf07e8839ea2f3ff29a0fc69aa64ebb6a7434049 e7f09c5d040fb2edbc5f07b0b74b758d62aa0f48",
				"Dardan B", "d@example.com", "2026-08-07T15:37:07+02:00",
				"Dardan B", "d@example.com", "2026-08-07T15:37:07+02:00",
				"Merge branch 'feature'", "",
			},
			want: Commit{
				Hash: "df4ccf47cde62f72b0b64194c9a09ab07bcfd504",
				Parents: []string{
					"cf07e8839ea2f3ff29a0fc69aa64ebb6a7434049",
					"e7f09c5d040fb2edbc5f07b0b74b758d62aa0f48",
				},
				Subject: "Merge branch 'feature'",
			},
		},
		{
			name: "multi-line body keeps its newlines but loses the trailing one",
			fields: []string{
				"f0b3e5b57fed4af3add7c945024d16f90881a363",
				"16892e646940afd3f1ef61082dc4eba055d0034a",
				"Dardan B", "d@example.com", "2026-08-07T15:37:07+02:00",
				"Dardan B", "d@example.com", "2026-08-07T15:37:07+02:00",
				"secondo commit", "Con un body\nsu piu righe.\n",
			},
			want: Commit{
				Hash:    "f0b3e5b57fed4af3add7c945024d16f90881a363",
				Parents: []string{"16892e646940afd3f1ef61082dc4eba055d0034a"},
				Subject: "secondo commit",
				Body:    "Con un body\nsu piu righe.",
			},
		},
		{
			// A subject containing the separator characters is impossible.
			name: "subject with unicode and punctuation survives intact",
			fields: []string{
				"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				"Ada Lovelace", "ada@example.com", "2026-08-07T15:37:07+02:00",
				"Ada Lovelace", "ada@example.com", "2026-08-07T15:37:07+02:00",
				"fix: gestisce «virgolette» e emoji 🎉", "",
			},
			want: Commit{
				Hash:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				Parents: []string{"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
				Subject: "fix: gestisce «virgolette» e emoji 🎉",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCommit([]byte(strings.Join(tt.fields, string(fieldSep))))
			if err != nil {
				t.Fatalf("parseCommit: %v", err)
			}
			if got.Hash != tt.want.Hash {
				t.Errorf("Hash = %q, want %q", got.Hash, tt.want.Hash)
			}
			if got.Subject != tt.want.Subject {
				t.Errorf("Subject = %q, want %q", got.Subject, tt.want.Subject)
			}
			if got.Body != tt.want.Body {
				t.Errorf("Body = %q, want %q", got.Body, tt.want.Body)
			}
			if len(got.Parents) != len(tt.want.Parents) {
				t.Fatalf("Parents = %v, want %v", got.Parents, tt.want.Parents)
			}
			for i := range got.Parents {
				if got.Parents[i] != tt.want.Parents[i] {
					t.Errorf("Parents[%d] = %q, want %q", i, got.Parents[i], tt.want.Parents[i])
				}
			}
		})
	}
}

func TestParseCommitRejectsBadInput(t *testing.T) {
	tests := []struct {
		name string
		rec  string
	}{
		{"too few fields", "abc" + string(fieldSep) + "def"},
		{
			"unparseable author date",
			strings.Join([]string{
				"abc", "", "N", "e@x", "non-una-data", "N", "e@x", "2026-08-07T15:37:07+02:00", "s", "",
			}, string(fieldSep)),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseCommit([]byte(tt.rec)); err == nil {
				t.Fatal("expected an error, got nil")
			}
		})
	}
}

func TestParseCommitDates(t *testing.T) {
	rec := strings.Join([]string{
		"abc", "",
		"A", "a@x", "2026-08-07T15:37:07+02:00",
		"C", "c@x", "2026-08-08T09:00:00Z",
		"subject", "",
	}, string(fieldSep))

	got, err := parseCommit([]byte(rec))
	if err != nil {
		t.Fatalf("parseCommit: %v", err)
	}

	wantAuthor := time.Date(2026, 8, 7, 15, 37, 7, 0, time.FixedZone("", 2*60*60))
	if !got.Author.When.Equal(wantAuthor) {
		t.Errorf("Author.When = %v, want %v", got.Author.When, wantAuthor)
	}
	// The offset must be preserved, not normalised to UTC.
	if _, off := got.Author.When.Zone(); off != 2*60*60 {
		t.Errorf("author zone offset = %ds, want 7200s", off)
	}
	if !got.Committer.When.Equal(time.Date(2026, 8, 8, 9, 0, 0, 0, time.UTC)) {
		t.Errorf("Committer.When = %v", got.Committer.When)
	}
}

func TestSplitRecords(t *testing.T) {
	stream := record("a", "1") + record("b", "2") + record("c", "3")

	sc := bufio.NewScanner(strings.NewReader(stream))
	sc.Split(splitRecords)

	var got []string
	for sc.Scan() {
		got = append(got, sc.Text())
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scanner: %v", err)
	}

	want := []string{
		"a" + string(fieldSep) + "1",
		"b" + string(fieldSep) + "2",
		"c" + string(fieldSep) + "3",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d records %q, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("record %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestSplitRecordsEmptyInput(t *testing.T) {
	// An empty repository produces no output at all; that is not an error.
	sc := bufio.NewScanner(strings.NewReader(""))
	sc.Split(splitRecords)
	if sc.Scan() {
		t.Fatalf("expected no records, got %q", sc.Text())
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scanner: %v", err)
	}
}

func TestLogOptionsArgs(t *testing.T) {
	tests := []struct {
		name string
		opts LogOptions
		want []string
	}{
		{"empty", LogOptions{}, []string{}},
		{"all", LogOptions{All: true}, []string{"--exclude=refs/stash", "--all"}},
		{"limit and skip", LogOptions{Limit: 50, Skip: 100}, []string{"-n", "50", "--skip", "100"}},
		{"explicit revs", LogOptions{Revs: []string{"main", "dev"}}, []string{"main", "dev"}},
		{"all wins over revs", LogOptions{All: true, Revs: []string{"main"}}, []string{"--exclude=refs/stash", "--all"}},
		{
			"paths are separated by --",
			LogOptions{All: true, Paths: []string{"src/", "README.md"}},
			[]string{"--exclude=refs/stash", "--all", "--", "src/", "README.md"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.opts.args()
			if strings.Join(got, " ") != strings.Join(tt.want, " ") {
				t.Errorf("args() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUnpushedCount(t *testing.T) {
	ctx := context.Background()
	_, ours, _ := netRepos(t)

	// A fresh clone has published everything it has.
	if n := ours.UnpushedCount(ctx); n != 0 {
		t.Errorf("UnpushedCount = %d, want 0 on a fresh clone", n)
	}

	gitIn(t, ours.Root(), "commit", "--allow-empty", "-qm", "solo qui")
	if n := ours.UnpushedCount(ctx); n != 1 {
		t.Errorf("UnpushedCount = %d, want 1", n)
	}

	gitIn(t, ours.Root(), "push", "-q", "origin", "HEAD")
	if n := ours.UnpushedCount(ctx); n != 0 {
		t.Errorf("UnpushedCount = %d after pushing, want 0", n)
	}

	// Without a remote, counting every commit would be noise rather than news.
	t.Run("a repository with no remote counts nothing", func(t *testing.T) {
		repo, _, _ := mergeRepo(t)
		if n := repo.UnpushedCount(ctx); n != 0 {
			t.Errorf("UnpushedCount = %d, want 0", n)
		}
	})
}
