package git

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// ResetMode is how much of the working state a reset takes with it.
type ResetMode string

const (
	// ResetSoft moves the branch and leaves the difference staged.
	ResetSoft ResetMode = "soft"
	// ResetMixed moves the branch and unstages, keeping the files as they are.
	ResetMixed ResetMode = "mixed"
	// ResetHard moves the branch and throws the working tree away with it.
	ResetHard ResetMode = "hard"
)

// ResetPreview says what moving the branch would cost, for a window that has to warn.
type ResetPreview struct {
	// Behind counts the commits the branch would give up.
	Behind int `json:"behind"`
	// Ahead counts the ones it would gain, when the target is further along.
	Ahead int `json:"ahead"`
	// Dirty is true when tracked files have changes a hard reset would destroy.
	Dirty bool `json:"dirty"`
}

// PreviewReset answers what a reset would do, without doing it.
func (r *Repo) PreviewReset(ctx context.Context, rev string) (ResetPreview, error) {
	hash, err := r.resolve(ctx, rev)
	if err != nil {
		return ResetPreview{}, err
	}

	out, err := r.run.output(ctx, "rev-list", "--left-right", "--count", "HEAD..."+hash)
	if err != nil {
		return ResetPreview{}, err
	}
	f := strings.Fields(string(out))
	if len(f) != 2 {
		return ResetPreview{}, fmt.Errorf("rev-list answered %q", out)
	}
	behind, _ := strconv.Atoi(f[0])
	ahead, _ := strconv.Atoi(f[1])

	st, err := r.Status(ctx)
	if err != nil {
		return ResetPreview{}, err
	}
	// Untracked files survive even a hard reset, so they are not what to warn about.
	dirty := false
	for _, file := range st.Files {
		if !file.Untracked {
			dirty = true
			break
		}
	}
	return ResetPreview{Behind: behind, Ahead: ahead, Dirty: dirty}, nil
}

// Reset moves the current branch onto another commit.
func (r *Repo) Reset(ctx context.Context, rev string, mode ResetMode) error {
	switch mode {
	case ResetSoft, ResetMixed, ResetHard:
	default:
		return fmt.Errorf("%q is not a reset mode", mode)
	}
	hash, err := r.resolve(ctx, rev)
	if err != nil {
		return err
	}
	_, err = r.run.output(ctx, "reset", "--"+string(mode), hash)
	return err
}
