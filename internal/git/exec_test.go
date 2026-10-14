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

	// With askpass in place ssh may ask for a passphrase there, but still never on /dev/tty or about hosts.
	t.Run("askpass lets passphrases through", func(t *testing.T) {
		SetEnv([]string{"SSH_ASKPASS=/bin/focagit", "SSH_ASKPASS_REQUIRE=force"})
		t.Cleanup(func() { extraEnv.Store(nil) })
		var cmd string
		for _, kv := range env() {
			if v, ok := strings.CutPrefix(kv, "GIT_SSH_COMMAND="); ok {
				cmd = v
			}
		}
		if strings.Contains(cmd, "BatchMode") || !strings.Contains(cmd, "StrictHostKeyChecking=yes") {
			t.Errorf("ssh command is %q", cmd)
		}
	})

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
