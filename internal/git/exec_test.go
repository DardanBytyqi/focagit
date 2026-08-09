package git

import (
	"slices"
	"strings"
	"testing"
)

// TestEnvDisablesSSHPrompts guards the fetch that hung: GIT_TERMINAL_PROMPT
// silences git, not ssh, which asks on /dev/tty and waits forever.
func TestEnvDisablesSSHPrompts(t *testing.T) {
	t.Setenv("GIT_SSH_COMMAND", "")
	if !slices.Contains(env(), "GIT_SSH_COMMAND=ssh -o BatchMode=yes") {
		t.Error("ssh can still block on a prompt")
	}

	// Someone who set their own command means it, keys and all.
	t.Run("a command the user set wins", func(t *testing.T) {
		const mine = "GIT_SSH_COMMAND=ssh -i /my/key"
		t.Setenv("GIT_SSH_COMMAND", "ssh -i /my/key")
		for _, kv := range env() {
			if strings.HasPrefix(kv, "GIT_SSH_COMMAND=") && kv != mine {
				t.Errorf("env replaced it with %q", kv)
			}
		}
	})
}
