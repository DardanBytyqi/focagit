package credentials

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// Main runs the askpass or credential-helper side when git started this binary; false otherwise.
func Main(args []string) (int, bool) {
	path := os.Getenv(envSocket)
	if path == "" {
		return 0, false
	}
	return run(path, os.Getenv(envToken), args, os.Stdin, os.Stdout), true
}

func run(path, token string, args []string, in io.Reader, out io.Writer) int {
	// As a helper: "--credential get|store|erase" with key=value lines on stdin.
	if len(args) == 2 && args[0] == helperFlag {
		resp, err := call(path, Request{Token: token, Action: args[1], Fields: readFields(in)})
		// A helper that cannot help stays quiet, so git carries on to the next one.
		if err != nil {
			return 0
		}
		for _, k := range []string{"username", "password"} {
			if v, ok := resp.Fields[k]; ok {
				fmt.Fprintf(out, "%s=%s\n", k, v)
			}
		}
		return 0
	}

	// As askpass: the prompt is the only argument and the answer goes to stdout.
	if len(args) != 1 {
		return 1
	}
	resp, err := call(path, Request{Token: token, Action: actionPrompt, Prompt: args[0], Parent: os.Getppid()})
	if err != nil || !resp.OK {
		return 1
	}
	fmt.Fprintln(out, resp.Answer)
	return 0
}

// readFields parses the key=value lines git writes to a helper, up to the blank line.
func readFields(in io.Reader) map[string]string {
	f := map[string]string{}
	sc := bufio.NewScanner(in)
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			break
		}
		if k, v, ok := strings.Cut(line, "="); ok {
			f[k] = v
		}
	}
	return f
}
