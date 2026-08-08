// Command focagit is, for now, a terminal harness over internal/git.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/dardanbytyqi/focagit/internal/git"
)

const usage = `focagit - git client (stage 1: git layer only)

usage:
  focagit log    [-n N] [-all] [path]   walk history
  focagit status [path]                 working tree state
  focagit refs   [path]                 branches, remotes and tags

path defaults to the current directory.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	// Ctrl-C cancels the context, which kills the git child process.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	var err error
	switch os.Args[1] {
	case "log":
		err = cmdLog(ctx, os.Args[2:])
	case "status":
		err = cmdStatus(ctx, os.Args[2:])
	case "refs":
		err = cmdRefs(ctx, os.Args[2:])
	case "-h", "--help", "help":
		fmt.Print(usage)
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "focagit:", err)
		os.Exit(1)
	}
}

// openRepo resolves the optional trailing path argument.
func openRepo(ctx context.Context, args []string) (*git.Repo, error) {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}
	return git.Open(ctx, path)
}

func cmdLog(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("log", flag.ExitOnError)
	limit := fs.Int("n", 20, "maximum number of commits, 0 for all")
	all := fs.Bool("all", false, "include every ref, not just HEAD")
	if err := fs.Parse(args); err != nil {
		return err
	}

	repo, err := openRepo(ctx, fs.Args())
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	return repo.Log(ctx, git.LogOptions{All: *all, Limit: *limit}, func(c git.Commit) error {
		marker := " "
		if c.IsMerge() {
			marker = "M"
		}
		fmt.Fprintf(w, "%s %s\t%s\t%s\t%s\n",
			marker,
			c.Short(),
			truncate(c.Author.Name, 18),
			c.Author.When.Format(time.DateTime),
			truncate(c.Subject, 72),
		)
		return nil
	})
}

func cmdStatus(ctx context.Context, args []string) error {
	repo, err := openRepo(ctx, args)
	if err != nil {
		return err
	}
	st, err := repo.Status(ctx)
	if err != nil {
		return err
	}

	head := st.Head
	if st.Detached {
		head = "HEAD detached at " + short(st.Oid)
	}
	fmt.Println("on", head)
	if st.Upstream != "" {
		fmt.Printf("tracking %s (ahead %d, behind %d)\n", st.Upstream, st.Ahead, st.Behind)
	}
	if st.IsClean() {
		fmt.Println("working tree clean")
		return nil
	}

	fmt.Println()
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()
	for _, f := range st.Files {
		// Untracked files have no index or worktree change code.
		codes := code(f.Staged) + code(f.Unstaged)
		if f.Untracked {
			codes = "??"
		}
		fmt.Fprintf(w, "%s\t%s\n", codes, describe(f))
	}
	return nil
}

func cmdRefs(ctx context.Context, args []string) error {
	repo, err := openRepo(ctx, args)
	if err != nil {
		return err
	}
	refs, err := repo.Refs(ctx)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()
	for _, r := range refs {
		marker := " "
		if r.Head {
			marker = "*"
		}
		upstream := ""
		if r.Upstream != "" {
			upstream = "-> " + r.Upstream
		}
		fmt.Fprintf(w, "%s %s\t%s\t%s\t%s\n", marker, short(r.Hash), r.Kind, r.Short, upstream)
	}
	return nil
}

// describe renders a status entry the way the UI eventually will: the rename arrow matters.
func describe(f git.FileStatus) string {
	switch {
	case f.Untracked:
		return f.Path + "  (untracked)"
	case f.Unmerged:
		return f.Path + "  (conflict)"
	case f.OrigPath != "":
		return fmt.Sprintf("%s -> %s  (%d%%)", f.OrigPath, f.Path, f.Score)
	default:
		return f.Path
	}
}

func code(c git.Change) string {
	if c == git.ChangeNone {
		return "."
	}
	return string(c)
}

func short(hash string) string {
	if len(hash) < 7 {
		return hash
	}
	return hash[:7]
}

func truncate(s string, n int) string {
	// Count runes, not bytes: commit subjects are full of accents and emoji.
	r := []rune(strings.ReplaceAll(s, "\n", " "))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n-1]) + "…"
}
