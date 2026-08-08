package git

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// maxDiffBytes caps a single file's patch.
const maxDiffBytes = 2 << 20 // 2 MiB

// LineKind is the role of one line inside a hunk.
type LineKind string

const (
	LineContext LineKind = "context"
	LineAdd     LineKind = "add"
	LineDelete  LineKind = "delete"
)

// DiffLine is one line of a patch.
type DiffLine struct {
	Kind LineKind `json:"kind"`
	Old  int      `json:"old"`
	New  int      `json:"new"`
	Text string   `json:"text"`
	// NoNewline marks a line followed by "\ No newline at end of file".
	NoNewline bool `json:"noNewline,omitempty"`
}

// Hunk is one @@ block.
type Hunk struct {
	Header   string     `json:"header"` // the text after the closing @@, if any
	OldStart int        `json:"oldStart"`
	OldLines int        `json:"oldLines"`
	NewStart int        `json:"newStart"`
	NewLines int        `json:"newLines"`
	Lines    []DiffLine `json:"lines"`
}

// FileDiff is the patch for a single file.
type FileDiff struct {
	Path      string `json:"path"`
	OldPath   string `json:"oldPath,omitempty"`
	Binary    bool   `json:"binary"`
	Truncated bool   `json:"truncated"`
	Added     int    `json:"added"`
	Removed   int    `json:"removed"`
	Hunks     []Hunk `json:"hunks"`

	// Header is every line git wrote before the first "---", verbatim.
	Header []string `json:"header,omitempty"`
}

// DiffTarget selects what to compare.
type DiffTarget struct {
	// Commit is the revision to show. Empty means the working tree.
	Commit string `json:"commit"`
	Path   string `json:"path"`
	// Staged applies to the working tree only.
	Staged bool `json:"staged"`
}

func (t DiffTarget) args() []string {
	switch {
	case t.Commit != "":
		// --first-parent is what makes merge commits show anything at all.
		return []string{"show", "--format=", "--patch", "-M", "--first-parent", t.Commit, "--", t.Path}
	case t.Staged:
		return []string{"diff", "--cached", "-M", "--", t.Path}
	default:
		return []string{"diff", "-M", "--", t.Path}
	}
}

// Diff returns the patch for one file.
func (r *Repo) Diff(ctx context.Context, t DiffTarget) (FileDiff, error) {
	out, err := r.run.output(ctx, t.args()...)
	if err != nil {
		return FileDiff{}, err
	}

	truncated := false
	if len(out) > maxDiffBytes {
		// Cut on a line boundary so the parser never sees half a line.
		if i := bytes.LastIndexByte(out[:maxDiffBytes], '\n'); i > 0 {
			out = out[:i+1]
		} else {
			out = out[:maxDiffBytes]
		}
		truncated = true
	}

	files, err := parseDiff(out)
	if err != nil {
		return FileDiff{}, err
	}
	if len(files) == 0 {
		// No changes for this path is a legitimate answer, not a failure.
		return FileDiff{Path: t.Path, Truncated: truncated, Hunks: []Hunk{}}, nil
	}

	d := files[0]
	d.Truncated = truncated
	if d.Hunks == nil {
		d.Hunks = []Hunk{}
	}
	return d, nil
}

var hunkRe = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@(?: (.*))?$`)

// parseDiff turns unified diff output into structured files.
func parseDiff(data []byte) ([]FileDiff, error) {
	var (
		files = []FileDiff{}
		cur   *FileDiff
		hunk  *Hunk
		oldNo int
		newNo int
	)

	// Closing the hunk then the file happens in enough places to be worth naming.
	closeHunk := func() {
		if cur != nil && hunk != nil {
			cur.Hunks = append(cur.Hunks, *hunk)
		}
		hunk = nil
	}
	closeFile := func() {
		closeHunk()
		if cur != nil {
			files = append(files, *cur)
		}
		cur = nil
	}

	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), maxDiffBytes)

	for sc.Scan() {
		line := sc.Text()

		switch {
		case strings.HasPrefix(line, "diff --git "):
			closeFile()
			cur = &FileDiff{Header: []string{line}}

		case cur == nil:
			// Anything before the first "diff --git" header is noise.
			continue

		case strings.HasPrefix(line, "--- "):
			if p := gitPath(strings.TrimPrefix(line, "--- ")); p != "" {
				cur.OldPath = p
			}

		case strings.HasPrefix(line, "+++ "):
			if p := gitPath(strings.TrimPrefix(line, "+++ ")); p != "" {
				cur.Path = p
			}

		case strings.HasPrefix(line, "Binary files ") || strings.HasPrefix(line, "GIT binary patch"):
			cur.Binary = true

		case strings.HasPrefix(line, "rename from "):
			cur.OldPath = strings.TrimPrefix(line, "rename from ")
			cur.Header = append(cur.Header, line)

		case strings.HasPrefix(line, "rename to "):
			cur.Path = strings.TrimPrefix(line, "rename to ")
			cur.Header = append(cur.Header, line)

		case strings.HasPrefix(line, "@@"):
			m := hunkRe.FindStringSubmatch(line)
			if m == nil {
				return nil, fmt.Errorf("git diff: malformed hunk header %q", line)
			}
			closeHunk()
			hunk = &Hunk{
				OldStart: atoi(m[1]),
				OldLines: atoiDefault(m[2], 1),
				NewStart: atoi(m[3]),
				NewLines: atoiDefault(m[4], 1),
				Header:   m[5],
			}
			oldNo, newNo = hunk.OldStart, hunk.NewStart

		case hunk == nil:
			// Lines between "diff --git" and the first hunk: index, file modes, similarity.
			cur.Header = append(cur.Header, line)

		case strings.HasPrefix(line, `\`):
			// "\ No newline at end of file" belongs to the line before it.
			if n := len(hunk.Lines); n > 0 {
				hunk.Lines[n-1].NoNewline = true
			}

		default:
			l, ok := parseDiffLine(line, &oldNo, &newNo)
			if !ok {
				// A line that belongs to no hunk means the patch ended and something else follows.
				continue
			}
			switch l.Kind {
			case LineAdd:
				cur.Added++
			case LineDelete:
				cur.Removed++
			}
			hunk.Lines = append(hunk.Lines, l)
		}
	}
	closeFile()

	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}
	return files, nil
}

// parseDiffLine reads one body line and advances the line counters.
func parseDiffLine(line string, oldNo, newNo *int) (DiffLine, bool) {
	if line == "" {
		// git writes a bare newline for an empty context line.
		l := DiffLine{Kind: LineContext, Old: *oldNo, New: *newNo}
		*oldNo++
		*newNo++
		return l, true
	}

	text := line[1:]
	switch line[0] {
	case ' ':
		l := DiffLine{Kind: LineContext, Old: *oldNo, New: *newNo, Text: text}
		*oldNo++
		*newNo++
		return l, true
	case '+':
		l := DiffLine{Kind: LineAdd, New: *newNo, Text: text}
		*newNo++
		return l, true
	case '-':
		l := DiffLine{Kind: LineDelete, Old: *oldNo, Text: text}
		*oldNo++
		return l, true
	default:
		return DiffLine{}, false
	}
}

// gitPath strips the a/ or b/ prefix git puts on diff paths.
func gitPath(s string) string {
	if s == "/dev/null" {
		return ""
	}
	if len(s) > 2 && (strings.HasPrefix(s, "a/") || strings.HasPrefix(s, "b/")) {
		return s[2:]
	}
	return s
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// atoiDefault handles the unified-diff shorthand where a count of 1 is omitted.
func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	return atoi(s)
}
