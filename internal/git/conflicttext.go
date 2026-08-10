package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Conflict markers are always exactly seven characters.
const (
	markerOurs   = "<<<<<<<"
	markerBase   = "|||||||"
	markerSplit  = "======="
	markerTheirs = ">>>>>>>"
)

// ConflictPick is what to keep for one disagreement.
type ConflictPick string

const (
	PickOurs   ConflictPick = "ours"
	PickTheirs ConflictPick = "theirs"
	PickBoth   ConflictPick = "both"
	PickNone   ConflictPick = "none"
)

// ConflictSegment is one run of the file: either text both sides agree on, or
// one disagreement to settle.
type ConflictSegment struct {
	Conflict bool `json:"conflict"`
	// Lines is the agreed text; empty for a conflict.
	Lines  []string `json:"lines"`
	Ours   []string `json:"ours"`
	Theirs []string `json:"theirs"`
	// Base is what both sides started from, present only in the diff3 styles.
	Base []string `json:"base"`
}

// ConflictText is a conflicted file cut into segments the UI can walk through.
type ConflictText struct {
	Path     string            `json:"path"`
	Segments []ConflictSegment `json:"segments"`
	// Count is how many segments are disagreements, which is what gets counted down.
	Count int `json:"count"`
	// Binary is true when the file cannot be shown as lines at all.
	Binary bool `json:"binary"`
}

// ConflictText reads a conflicted file and splits it on the markers git wrote.
func (r *Repo) ConflictText(ctx context.Context, path string) (ConflictText, error) {
	full, err := r.inside(path)
	if err != nil {
		return ConflictText{}, err
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return ConflictText{}, err
	}
	// A NUL byte means git treated it as binary, and so must we.
	if strings.ContainsRune(string(data), 0) {
		return ConflictText{Path: path, Binary: true}, nil
	}

	segs := parseConflictText(string(data))
	out := ConflictText{Path: path, Segments: segs}
	for _, s := range segs {
		if s.Conflict {
			out.Count++
		}
	}
	return out, nil
}

// ResolveLines rebuilds the file from one pick per disagreement and stages it.
func (r *Repo) ResolveLines(ctx context.Context, path string, picks []ConflictPick) error {
	full, err := r.inside(path)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return err
	}

	// Parsed again here rather than trusting the picks to still line up: the file
	// may have been edited since it was shown.
	segs := parseConflictText(string(data))
	want := 0
	for _, s := range segs {
		if s.Conflict {
			want++
		}
	}
	if want != len(picks) {
		return fmt.Errorf("%s has %d conflicts, not %d: reopen it", path, want, len(picks))
	}
	if want == 0 {
		return fmt.Errorf("%s has no conflict markers", path)
	}

	var lines []string
	i := 0
	for _, s := range segs {
		if !s.Conflict {
			lines = append(lines, s.Lines...)
			continue
		}
		switch picks[i] {
		case PickOurs:
			lines = append(lines, s.Ours...)
		case PickTheirs:
			lines = append(lines, s.Theirs...)
		case PickBoth:
			lines = append(lines, s.Ours...)
			lines = append(lines, s.Theirs...)
		case PickNone:
		default:
			return fmt.Errorf("%q is not a choice", picks[i])
		}
		i++
	}

	// Keep the file's own permissions rather than inventing new ones.
	mode := os.FileMode(0o644)
	if st, err := os.Stat(full); err == nil {
		mode = st.Mode().Perm()
	}
	body := strings.Join(lines, "\n")
	if len(lines) > 0 {
		body += "\n"
	}
	if err := os.WriteFile(full, []byte(body), mode); err != nil {
		return err
	}

	_, err = r.run.output(ctx, "add", "--", path)
	return err
}

// parseConflictText walks the lines once, tracking which part of a marker block
// it is inside. A line only counts as a marker where one could legally appear,
// so a row of equals signs in ordinary text stays ordinary text.
func parseConflictText(text string) []ConflictSegment {
	text = strings.TrimSuffix(text, "\n")
	lines := strings.Split(text, "\n")

	var segs []ConflictSegment
	var common, ours, base, theirs, raw []string
	// where is 0 outside a conflict, then ours, base, theirs.
	where := 0

	flushCommon := func() {
		if len(common) > 0 {
			segs = append(segs, ConflictSegment{Lines: common})
			common = nil
		}
	}

	for _, line := range lines {
		if where != 0 {
			// Kept verbatim, markers included, in case the block never closes.
			raw = append(raw, line)
		}
		switch {
		case where == 0 && marker(line, markerOurs):
			// The agreed text is only closed off once the block closes too, so an
			// unterminated one rejoins it instead of splitting it in two.
			raw = []string{line}
			where = 1

		case where == 1 && marker(line, markerBase):
			where = 2

		case (where == 1 || where == 2) && marker(line, markerSplit):
			where = 3

		case where == 3 && marker(line, markerTheirs):
			flushCommon()
			segs = append(segs, ConflictSegment{
				Conflict: true,
				Ours:     ours,
				Base:     base,
				Theirs:   theirs,
			})
			ours, base, theirs, raw = nil, nil, nil, nil
			where = 0

		default:
			switch where {
			case 0:
				common = append(common, line)
			case 1:
				ours = append(ours, line)
			case 2:
				base = append(base, line)
			case 3:
				theirs = append(theirs, line)
			}
		}
	}

	// A block that never closes is text, not a conflict. It goes back exactly as
	// it came in: guessing at half a marker block would lose the user's lines.
	if where != 0 {
		common = append(common, raw...)
	}
	flushCommon()
	return segs
}

// marker reports whether the line opens with one of git's seven-character marks.
func marker(line, mark string) bool {
	return line == mark || strings.HasPrefix(line, mark+" ")
}

// inside resolves a repository-relative path and refuses to leave the worktree.
func (r *Repo) inside(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("no path given")
	}
	full := filepath.Clean(filepath.Join(r.root, path))
	if full != r.root && !strings.HasPrefix(full, r.root+string(filepath.Separator)) {
		return "", fmt.Errorf("%q is outside the repository", path)
	}
	return full, nil
}
