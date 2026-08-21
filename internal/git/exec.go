package git

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"
)

// runner invokes the git binary inside a fixed directory.
type runner struct {
	bin string
	dir string
	log *slog.Logger
}

// env returns the environment every git child process inherits.
func env() []string {
	out := append(os.Environ(),
		"LC_ALL=C",              // stable, parseable messages regardless of locale
		"GIT_PAGER=cat",         // never hand output to less and hang forever
		"GIT_TERMINAL_PROMPT=0", // fail fast instead of blocking on a credential prompt
		"GIT_OPTIONAL_LOCKS=0",  // read-only commands must not take index.lock
		"GIT_EDITOR=true",       // --continue would open one and wait forever
	)

	// GIT_TERMINAL_PROMPT misses ssh, which asks on /dev/tty and blocks forever.
	if os.Getenv("GIT_SSH_COMMAND") == "" {
		out = append(out, "GIT_SSH_COMMAND=ssh -o BatchMode=yes")
	}
	return out
}

func (r runner) command(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, r.bin, args...)
	cmd.Dir = r.dir
	cmd.Env = env()
	return cmd
}

// finish records how an invocation went.
func (r runner) finish(args []string, start time.Time, stderr string, err error) {
	attrs := []any{
		slog.String("cmd", strings.Join(args, " ")),
		slog.Duration("dur", time.Since(start)),
		slog.String("dir", r.dir),
	}
	if err == nil {
		r.log.Debug("git", attrs...)
		return
	}
	r.log.Error("git failed", append(attrs,
		slog.String("err", err.Error()),
		slog.String("stderr", stderr),
	)...)
}

// outputOK runs git and treats the listed exit codes as success, including in the log.
func (r runner) outputOK(ctx context.Context, ok []int, args ...string) ([]byte, error) {
	out, err := r.outputQuiet(ctx, args...)
	if err == nil {
		return out, nil
	}
	var ce *CommandError
	if errors.As(err, &ce) {
		for _, code := range ok {
			if ce.ExitCode() == code {
				return out, nil
			}
		}
	}
	r.log.Error("git failed", slog.String("cmd", strings.Join(args, " ")), slog.String("err", err.Error()))
	return out, err
}

// output runs git to completion and returns stdout. stderr is folded into the returned.
func (r runner) output(ctx context.Context, args ...string) ([]byte, error) {
	out, err := r.outputQuiet(ctx, args...)
	if err != nil {
		var ce *CommandError
		if errors.As(err, &ce) {
			r.log.Error("git failed",
				slog.String("cmd", strings.Join(args, " ")),
				slog.String("stderr", ce.Stderr),
			)
		}
	}
	return out, err
}

// outputQuiet is output without the error-level log, for callers that expect failure.
func (r runner) outputQuiet(ctx context.Context, args ...string) ([]byte, error) {
	start := time.Now()

	cmd := r.command(ctx, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	r.log.Debug("git",
		slog.String("cmd", strings.Join(args, " ")),
		slog.Duration("dur", time.Since(start)),
		slog.String("dir", r.dir),
	)

	if err != nil {
		return stdout.Bytes(), &CommandError{
			Args:   args,
			Stderr: strings.TrimSpace(stderr.String()),
			Err:    err,
		}
	}
	return stdout.Bytes(), nil
}

// input runs git with stdin fed from a string.
func (r runner) input(ctx context.Context, stdin string, args ...string) ([]byte, error) {
	start := time.Now()

	cmd := r.command(ctx, args...)
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	r.finish(args, start, stderr.String(), err)

	if err != nil {
		return nil, &CommandError{
			Args:   args,
			Stderr: strings.TrimSpace(stderr.String()),
			Err:    err,
		}
	}
	return stdout.Bytes(), nil
}

// progress runs git and hands each stderr line to fn as it arrives.
func (r runner) progress(ctx context.Context, fn func(string), args ...string) ([]byte, error) {
	start := time.Now()

	cmd := r.command(ctx, args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	pipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, &CommandError{Args: args, Err: err}
	}
	if err := cmd.Start(); err != nil {
		return nil, &CommandError{Args: args, Err: err}
	}

	var stderr strings.Builder
	sc := bufio.NewScanner(pipe)
	sc.Split(scanProgress)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		stderr.WriteString(line + "\n")
		if fn != nil {
			fn(line)
		}
	}

	waitErr := cmd.Wait()
	r.finish(args, start, stderr.String(), waitErr)
	if waitErr != nil {
		return stdout.Bytes(), &CommandError{
			Args:   args,
			Stderr: strings.TrimSpace(stderr.String()),
			Err:    waitErr,
		}
	}
	return stdout.Bytes(), nil
}

// scanProgress splits on carriage returns too: that is how git redraws.
func scanProgress(data []byte, atEOF bool) (int, []byte, error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexAny(data, "\r\n"); i >= 0 {
		return i + 1, data[:i], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// stream runs git and hands stdout to fn while git is still producing it.
func (r runner) stream(ctx context.Context, args []string, fn func(io.Reader) error) error {
	start := time.Now()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	cmd := r.command(ctx, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		r.finish(args, start, "", err)
		return &CommandError{Args: args, Err: err}
	}
	if err := cmd.Start(); err != nil {
		r.finish(args, start, stderr.String(), err)
		return &CommandError{Args: args, Err: err}
	}

	fnErr := fn(stdout)
	if fnErr != nil {
		cancel()
	}

	// Drain whatever is left, otherwise git blocks writing into a full pipe and Wait never.
	_, _ = io.Copy(io.Discard, stdout)
	waitErr := cmd.Wait()

	// The consumer's error is the interesting one.
	if fnErr != nil {
		// Not an error from git's point of view: we asked it to stop.
		r.log.Debug("git cancelled by consumer",
			slog.String("cmd", strings.Join(args, " ")),
			slog.Duration("dur", time.Since(start)),
			slog.String("reason", fnErr.Error()),
		)
		return fnErr
	}

	r.finish(args, start, stderr.String(), waitErr)
	if waitErr != nil {
		return &CommandError{
			Args:   args,
			Stderr: strings.TrimSpace(stderr.String()),
			Err:    waitErr,
		}
	}
	return nil
}
