package git

import (
	"strings"
	"testing"
)

// nul joins entries the way git status -z emits them.
func nul(entries ...string) []byte {
	return []byte(strings.Join(entries, "\x00") + "\x00")
}

func TestParseStatus(t *testing.T) {
	// Captured verbatim from git 2.55 on a repository with a modified file, a staged.
	data := nul(
		"# branch.oid df4ccf47cde62f72b0b64194c9a09ab07bcfd504",
		"# branch.head main",
		"1 .M N... 100644 100644 100644 fb85c0a2eacf48a66ef1b6802bbaacfd179e1181 fb85c0a2eacf48a66ef1b6802bbaacfd179e1181 a.txt",
		"2 R. N... 100644 100644 100644 c7747099cf9e073babc68f52cdfb4d280ba5689f c7747099cf9e073babc68f52cdfb4d280ba5689f R100 b_rinominato.txt",
		"b.txt", // the rename's original path, a separate entry
		"1 A. N... 000000 100644 100644 0000000000000000000000000000000000000000 19d9cc8584ac2c7dcf57d2680375e80f099dc481 c.txt",
		"? untracked.txt",
	)

	st, err := parseStatus(data)
	if err != nil {
		t.Fatalf("parseStatus: %v", err)
	}

	if st.Head != "main" {
		t.Errorf("Head = %q, want %q", st.Head, "main")
	}
	if st.Detached {
		t.Error("Detached = true, want false")
	}
	if st.Oid != "df4ccf47cde62f72b0b64194c9a09ab07bcfd504" {
		t.Errorf("Oid = %q", st.Oid)
	}
	if len(st.Files) != 4 {
		t.Fatalf("got %d files, want 4: %+v", len(st.Files), st.Files)
	}

	want := []FileStatus{
		{Path: "a.txt", Staged: ChangeNone, Unstaged: ChangeModified},
		{Path: "b_rinominato.txt", OrigPath: "b.txt", Staged: ChangeRenamed, Unstaged: ChangeNone, Score: 100},
		{Path: "c.txt", Staged: ChangeAdded, Unstaged: ChangeNone},
		{Path: "untracked.txt", Untracked: true},
	}
	for i, w := range want {
		g := st.Files[i]
		if g.Path != w.Path || g.OrigPath != w.OrigPath ||
			g.Staged != w.Staged || g.Unstaged != w.Unstaged ||
			g.Untracked != w.Untracked || g.Score != w.Score {
			t.Errorf("Files[%d] = %+v, want %+v", i, g, w)
		}
	}
}

func TestParseStatusBranchHeaders(t *testing.T) {
	tests := []struct {
		name     string
		entries  []string
		head     string
		detached bool
		upstream string
		ahead    int
		behind   int
	}{
		{
			name: "tracking branch ahead and behind",
			entries: []string{
				"# branch.oid abc123",
				"# branch.head main",
				"# branch.upstream origin/main",
				"# branch.ab +3 -7",
			},
			head: "main", upstream: "origin/main", ahead: 3, behind: 7,
		},
		{
			name: "up to date with upstream",
			entries: []string{
				"# branch.head dev",
				"# branch.upstream origin/dev",
				"# branch.ab +0 -0",
			},
			head: "dev", upstream: "origin/dev",
		},
		{
			name:     "detached HEAD reports no branch name",
			entries:  []string{"# branch.oid abc123", "# branch.head (detached)"},
			head:     "",
			detached: true,
		},
		{
			name:    "branch with no upstream",
			entries: []string{"# branch.head feature/x", "# branch.oid abc123"},
			head:    "feature/x",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st, err := parseStatus(nul(tt.entries...))
			if err != nil {
				t.Fatalf("parseStatus: %v", err)
			}
			if st.Head != tt.head {
				t.Errorf("Head = %q, want %q", st.Head, tt.head)
			}
			if st.Detached != tt.detached {
				t.Errorf("Detached = %v, want %v", st.Detached, tt.detached)
			}
			if st.Upstream != tt.upstream {
				t.Errorf("Upstream = %q, want %q", st.Upstream, tt.upstream)
			}
			if st.Ahead != tt.ahead || st.Behind != tt.behind {
				t.Errorf("ahead/behind = %d/%d, want %d/%d", st.Ahead, st.Behind, tt.ahead, tt.behind)
			}
		})
	}
}

func TestParseStatusPathsWithSpaces(t *testing.T) {
	// The whole reason for -z.
	data := nul(
		"# branch.head main",
		"1 .M N... 100644 100644 100644 aaa bbb my documents/note finali.txt",
		"? nuova cartella/file con spazi.md",
	)

	st, err := parseStatus(data)
	if err != nil {
		t.Fatalf("parseStatus: %v", err)
	}
	if got := st.Files[0].Path; got != "my documents/note finali.txt" {
		t.Errorf("Files[0].Path = %q", got)
	}
	if got := st.Files[1].Path; got != "nuova cartella/file con spazi.md" {
		t.Errorf("Files[1].Path = %q", got)
	}
}

func TestParseStatusUnmerged(t *testing.T) {
	data := nul(
		"# branch.head main",
		"u UU N... 100644 100644 100644 100644 aaa bbb ccc conflitto.txt",
	)
	st, err := parseStatus(data)
	if err != nil {
		t.Fatalf("parseStatus: %v", err)
	}
	if len(st.Files) != 1 {
		t.Fatalf("got %d files, want 1", len(st.Files))
	}
	f := st.Files[0]
	if !f.Unmerged {
		t.Error("Unmerged = false, want true")
	}
	if f.Path != "conflitto.txt" {
		t.Errorf("Path = %q", f.Path)
	}
	if f.Staged != ChangeUnmerged || f.Unstaged != ChangeUnmerged {
		t.Errorf("staged/unstaged = %q/%q, want U/U", f.Staged, f.Unstaged)
	}
}

func TestParseStatusStagedAndUnstagedTogether(t *testing.T) {
	// A file added to the index and then edited again: both halves are set.
	data := nul(
		"# branch.head main",
		"1 MM N... 100644 100644 100644 aaa bbb doppio.txt",
	)
	st, err := parseStatus(data)
	if err != nil {
		t.Fatalf("parseStatus: %v", err)
	}
	f := st.Files[0]
	if f.Staged != ChangeModified || f.Unstaged != ChangeModified {
		t.Errorf("staged/unstaged = %q/%q, want M/M", f.Staged, f.Unstaged)
	}
}

func TestParseStatusEmptyIsClean(t *testing.T) {
	st, err := parseStatus(nul("# branch.oid abc", "# branch.head main"))
	if err != nil {
		t.Fatalf("parseStatus: %v", err)
	}
	if !st.IsClean() {
		t.Errorf("IsClean() = false, want true: %+v", st.Files)
	}
}

func TestParseStatusRejectsTruncatedRename(t *testing.T) {
	// A rename entry whose original path is missing means we misread the stream.
	data := []byte("# branch.head main\x002 R. N... 100644 100644 100644 aaa bbb R100 nuovo.txt\x00")
	if _, err := parseStatus(data); err == nil {
		t.Fatal("expected an error for a rename without its original path")
	}
}

func TestParseXY(t *testing.T) {
	tests := []struct {
		xy               string
		staged, unstaged Change
		wantErr          bool
	}{
		{xy: ".M", staged: ChangeNone, unstaged: ChangeModified},
		{xy: "M.", staged: ChangeModified, unstaged: ChangeNone},
		{xy: "A.", staged: ChangeAdded},
		{xy: ".D", unstaged: ChangeDeleted},
		{xy: "R.", staged: ChangeRenamed},
		{xy: "UU", staged: ChangeUnmerged, unstaged: ChangeUnmerged},
		{xy: "X", wantErr: true},
		{xy: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.xy, func(t *testing.T) {
			s, u, err := parseXY(tt.xy)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseXY: %v", err)
			}
			if s != tt.staged || u != tt.unstaged {
				t.Errorf("= %q/%q, want %q/%q", s, u, tt.staged, tt.unstaged)
			}
		})
	}
}
