package git

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// Field and record separators for the custom log format.
const (
	fieldSep  = '\x1f'
	recordSep = '\x1e'
)

// logFields must stay in sync with the parsing order in parseCommit.
var logFields = []string{
	"%H",  // full hash
	"%P",  // parent hashes, space separated
	"%an", // author name
	"%ae", // author email
	"%aI", // author date, strict ISO 8601
	"%cn", // committer name
	"%ce", // committer email
	"%cI", // committer date, strict ISO 8601
	"%s",  // subject
	"%b",  // body
}

var logFormat = strings.Join(logFields, string(fieldSep)) + string(recordSep)

// maxRecordSize caps a single commit record.
const maxRecordSize = 4 << 20 // 4 MiB

// LogOptions selects which commits to walk.
type LogOptions struct {
	All   bool     // include every ref, not just HEAD
	Revs  []string // explicit revisions or ranges; ignored when All is set
	Limit int      // stop after N commits, 0 means no limit
	Skip  int      // skip the first N commits
	Paths []string // restrict history to these paths
}

func (o LogOptions) args() []string {
	var a []string
	if o.Limit > 0 {
		a = append(a, "-n", strconv.Itoa(o.Limit))
	}
	if o.Skip > 0 {
		a = append(a, "--skip", strconv.Itoa(o.Skip))
	}
	if o.All {
		// A stash is three commits under refs/, and --all would draw all three.
		a = append(a, "--exclude=refs/stash", "--all")
	} else {
		a = append(a, o.Revs...)
	}
	if len(o.Paths) > 0 {
		a = append(a, "--")
		a = append(a, o.Paths...)
	}
	return a
}

// Log streams commits to fn in topological order as git produces them.
func (r *Repo) Log(ctx context.Context, opts LogOptions, fn func(Commit) error) error {
	args := append([]string{
		"log",
		"--topo-order",
		"--format=" + logFormat,
	}, opts.args()...)

	return r.run.stream(ctx, args, func(rd io.Reader) error {
		sc := bufio.NewScanner(rd)
		sc.Buffer(make([]byte, 0, 64*1024), maxRecordSize)
		sc.Split(splitRecords)
		return scanCommits(sc, fn)
	})
}

// Commits collects the walk into a slice.
func (r *Repo) Commits(ctx context.Context, opts LogOptions) ([]Commit, error) {
	out := []Commit{}
	err := r.Log(ctx, opts, func(c Commit) error {
		out = append(out, c)
		return nil
	})
	return out, err
}

// splitRecords is a bufio.SplitFunc that cuts the stream on 0x1E. git emits a newline.
func splitRecords(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if i := bytes.IndexByte(data, recordSep); i >= 0 {
		return i + 1, bytes.TrimLeft(data[:i], "\n"), nil
	}
	if atEOF {
		if rest := bytes.TrimLeft(data, "\n"); len(rest) > 0 {
			return len(data), rest, nil
		}
		return len(data), nil, nil
	}
	return 0, nil, nil // ask for more data
}

func parseCommit(rec []byte) (Commit, error) {
	f := bytes.Split(rec, []byte{fieldSep})
	if len(f) != len(logFields) {
		return Commit{}, fmt.Errorf("git log: want %d fields, got %d in %q", len(logFields), len(f), rec)
	}

	authorWhen, err := time.Parse(time.RFC3339, string(f[4]))
	if err != nil {
		return Commit{}, fmt.Errorf("git log: author date: %w", err)
	}
	commitWhen, err := time.Parse(time.RFC3339, string(f[7]))
	if err != nil {
		return Commit{}, fmt.Errorf("git log: committer date: %w", err)
	}

	return Commit{
		Hash:      string(f[0]),
		Parents:   strings.Fields(string(f[1])), // empty for a root commit
		Author:    Person{Name: string(f[2]), Email: string(f[3]), When: authorWhen},
		Committer: Person{Name: string(f[5]), Email: string(f[6]), When: commitWhen},
		Subject:   string(f[8]),
		// git terminates a non-empty body with a newline; the UI adds its own spacing.
		Body: strings.TrimRight(string(f[9]), "\n"),
	}, nil
}

// scanCommits drives the scanner over a stream of records.
func scanCommits(rd *bufio.Scanner, fn func(Commit) error) error {
	for rd.Scan() {
		c, err := parseCommit(rd.Bytes())
		if err != nil {
			return err
		}
		if err := fn(c); err != nil {
			return err
		}
	}
	return rd.Err()
}

// UnpushedCount is how many commits on HEAD are on no remote.
func (r *Repo) UnpushedCount(ctx context.Context) int {
	// With no remote configured every commit qualifies, which says nothing.
	refs, err := r.run.outputQuiet(ctx, "for-each-ref", "--format=%(refname)", "refs/remotes")
	if err != nil || len(bytes.TrimSpace(refs)) == 0 {
		return 0
	}

	// The order matters: --not inverts everything after it, HEAD included.
	out, err := r.run.outputQuiet(ctx, "rev-list", "--count", "HEAD", "--not", "--remotes")
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	return n
}
