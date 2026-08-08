package git

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
)

// FileChange is one entry in a commit's list of touched files.
type FileChange struct {
	Path    string `json:"path"`
	OldPath string `json:"oldPath,omitempty"` // renames and copies only
	Status  Change `json:"status"`
	Score   int    `json:"score,omitempty"` // rename/copy similarity, 0-100
}

// CommitFiles lists the files a commit changed.
func (r *Repo) CommitFiles(ctx context.Context, hash string) ([]FileChange, error) {
	out, err := r.run.output(ctx,
		"show", "--format=", "--name-status", "-z", "-M", "--first-parent", hash)
	if err != nil {
		return nil, err
	}
	return parseNameStatus(out)
}

// parseNameStatus reads the NUL-separated output of --name-status.
func parseNameStatus(data []byte) ([]FileChange, error) {
	out := []FileChange{}

	entries := bytes.Split(data, []byte{0})
	for i := 0; i < len(entries); i++ {
		status := string(entries[i])
		if status == "" {
			continue
		}

		// Renames and copies carry a similarity score: "R100", "C75".
		kind := Change(status[:1])
		score := 0
		if len(status) > 1 {
			score, _ = strconv.Atoi(status[1:])
		}

		need := 1
		if kind == ChangeRenamed || kind == ChangeCopied {
			need = 2
		}
		if i+need >= len(entries) {
			return nil, fmt.Errorf("git show --name-status: entry %q is missing its path", status)
		}

		fc := FileChange{Status: kind, Score: score}
		if need == 2 {
			fc.OldPath = string(entries[i+1])
			fc.Path = string(entries[i+2])
		} else {
			fc.Path = string(entries[i+1])
		}
		if fc.Path == "" {
			return nil, fmt.Errorf("git show --name-status: entry %q has an empty path", status)
		}

		i += need
		out = append(out, fc)
	}
	return out, nil
}
