package git

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrNotARepository is returned by Open when the given path is not inside a git working.
var ErrNotARepository = errors.New("not a git repository")

// CommandError carries everything needed to understand a failed git invocation.
type CommandError struct {
	Args   []string
	Stderr string
	Err    error
}

func (e *CommandError) Error() string {
	msg := fmt.Sprintf("git %s: %v", strings.Join(e.Args, " "), e.Err)
	if e.Stderr != "" {
		msg += ": " + e.Stderr
	}
	return msg
}

func (e *CommandError) Unwrap() error { return e.Err }

// ExitCode reports git's exit status, or -1 if the process never ran.
func (e *CommandError) ExitCode() int {
	var ee *exec.ExitError
	if errors.As(e.Err, &ee) {
		return ee.ExitCode()
	}
	return -1
}
