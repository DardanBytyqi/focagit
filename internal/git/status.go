package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Status reports the state of the working tree and index.
func (r *Repo) Status(ctx context.Context) (Status, error) {
	out, err := r.run.output(ctx,
		"status", "--porcelain=v2", "-z", "--branch", "--untracked-files=all")
	if err != nil {
		return Status{}, err
	}
	st, err := parseStatus(out)
	if err != nil {
		return Status{}, err
	}
	// Read off disk rather than asking git: status is refreshed constantly.
	if _, err := os.Stat(filepath.Join(r.gitDir, "MERGE_HEAD")); err == nil {
		st.Merging = true
		st.MergeFrom = r.MergeSource(ctx)
	}
	return st, nil
}

func parseStatus(data []byte) (Status, error) {
	st := Status{Files: []FileStatus{}}

	entries := bytes.Split(data, []byte{0})
	for i := 0; i < len(entries); i++ {
		e := string(entries[i])
		if e == "" {
			continue
		}

		switch e[0] {
		case '#':
			parseStatusHeader(&st, e)

		case '1': // ordinary change
			f, err := parseOrdinary(e)
			if err != nil {
				return Status{}, err
			}
			st.Files = append(st.Files, f)

		case '2': // rename or copy: the original path is the NEXT NUL entry
			// Splitting on NUL always yields a trailing empty element.
			if i+1 >= len(entries) || len(entries[i+1]) == 0 {
				return Status{}, fmt.Errorf("git status: rename entry %q missing its original path", e)
			}
			i++
			f, err := parseRenamed(e, string(entries[i]))
			if err != nil {
				return Status{}, err
			}
			st.Files = append(st.Files, f)

		case 'u': // unmerged
			f, err := parseUnmerged(e)
			if err != nil {
				return Status{}, err
			}
			st.Files = append(st.Files, f)

		case '?': // untracked
			st.Files = append(st.Files, FileStatus{
				Path:      strings.TrimPrefix(e, "? "),
				Untracked: true,
			})

		case '!': // ignored; only present with --ignored, skipped for now
		}
	}

	return st, nil
}

func parseStatusHeader(st *Status, e string) {
	key, val, ok := strings.Cut(strings.TrimPrefix(e, "# "), " ")
	if !ok {
		return
	}
	switch key {
	case "branch.oid":
		// A repository with no commits reports "(initial)", not a hash.
		if val != "(initial)" {
			st.Oid = val
		}
	case "branch.head":
		// git writes the literal "(detached)" instead of a name when HEAD is not on a branch.
		if val == "(detached)" {
			st.Detached = true
		} else {
			st.Head = val
		}
	case "branch.upstream":
		st.Upstream = val
	case "branch.ab":
		// Format: "+3 -1"
		if ahead, behind, ok := strings.Cut(val, " "); ok {
			st.Ahead, _ = strconv.Atoi(strings.TrimPrefix(ahead, "+"))
			st.Behind, _ = strconv.Atoi(strings.TrimPrefix(behind, "-"))
		}
	}
}

// parseOrdinary reads.
func parseOrdinary(e string) (FileStatus, error) {
	f := strings.SplitN(e, " ", 9)
	if len(f) != 9 {
		return FileStatus{}, fmt.Errorf("git status: malformed ordinary entry %q", e)
	}
	staged, unstaged, err := parseXY(f[1])
	if err != nil {
		return FileStatus{}, err
	}
	return FileStatus{Path: f[8], Staged: staged, Unstaged: unstaged}, nil
}

// parseRenamed reads.
func parseRenamed(e, orig string) (FileStatus, error) {
	f := strings.SplitN(e, " ", 10)
	if len(f) != 10 {
		return FileStatus{}, fmt.Errorf("git status: malformed rename entry %q", e)
	}
	staged, unstaged, err := parseXY(f[1])
	if err != nil {
		return FileStatus{}, err
	}
	// f[8] is like "R100" or "C75": a letter followed by a similarity score.
	score, _ := strconv.Atoi(f[8][1:])
	return FileStatus{
		Path:     f[9],
		OrigPath: orig,
		Staged:   staged,
		Unstaged: unstaged,
		Score:    score,
	}, nil
}

// parseUnmerged reads: u <XY> <sub> <m1> <m2> <m3> <mW> <h1> <h2> <h3> <path>
func parseUnmerged(e string) (FileStatus, error) {
	f := strings.SplitN(e, " ", 11)
	if len(f) != 11 {
		return FileStatus{}, fmt.Errorf("git status: malformed unmerged entry %q", e)
	}
	staged, unstaged, err := parseXY(f[1])
	if err != nil {
		return FileStatus{}, err
	}
	return FileStatus{Path: f[10], Staged: staged, Unstaged: unstaged, Unmerged: true}, nil
}

// parseXY splits the two-character status code.
func parseXY(xy string) (staged, unstaged Change, err error) {
	if len(xy) != 2 {
		return "", "", fmt.Errorf("git status: bad status code %q", xy)
	}
	return changeFromByte(xy[0]), changeFromByte(xy[1]), nil
}

func changeFromByte(b byte) Change {
	switch b {
	case 'A':
		return ChangeAdded
	case 'M':
		return ChangeModified
	case 'D':
		return ChangeDeleted
	case 'R':
		return ChangeRenamed
	case 'C':
		return ChangeCopied
	case 'T':
		return ChangeTypeMod
	case 'U':
		return ChangeUnmerged
	default: // '.' and anything unexpected
		return ChangeNone
	}
}
