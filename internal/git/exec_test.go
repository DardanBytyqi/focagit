package git

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// TestEnvDisablesSSHPrompts guards the fetch that hung: GIT_TERMINAL_PROMPT
// silences git, not ssh, which asks on /dev/tty and waits forever.
func TestEnvDisablesSSHPrompts(t *testing.T) {
	t.Setenv("GIT_SSH_COMMAND", "")
	if !slices.Contains(env(true), "GIT_SSH_COMMAND=ssh -o BatchMode=yes") {
		t.Error("ssh can still block on a prompt")
	}

	// With askpass in place ssh may ask for a passphrase there, but still never on /dev/tty or about hosts.
	t.Run("askpass lets passphrases through", func(t *testing.T) {
		SetEnv([]string{"SSH_ASKPASS=/bin/focagit", "SSH_ASKPASS_REQUIRE=force"})
		t.Cleanup(func() { extraEnv.Store(nil) })
		var cmd string
		for _, kv := range env(true) {
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
		for _, kv := range env(true) {
			if strings.HasPrefix(kv, "GIT_SSH_COMMAND=") && kv != mine {
				t.Errorf("env replaced it with %q", kv)
			}
		}
	})
}

// TestRemoteCommandsKeepTheRepositorySSHCommand guards core.sshCommand, which GIT_SSH_COMMAND would outrank.
func TestRemoteCommandsKeepTheRepositorySSHCommand(t *testing.T) {
	t.Setenv("GIT_SSH_COMMAND", "")
	repo := testRepo(t)
	ctx := context.Background()
	ours := func(args ...string) bool {
		// The empty one t.Setenv leaves behind is not ours.
		return slices.ContainsFunc(repo.run.command(ctx, args...).Env, func(v string) bool {
			return strings.HasPrefix(v, "GIT_SSH_COMMAND=") && v != "GIT_SSH_COMMAND="
		})
	}

	if !ours("fetch") {
		t.Error("with no core.sshCommand, fetch should get our safe ssh command")
	}

	if out, err := repo.run.command(ctx, "config", "core.sshCommand", "ssh -i /my/work/key").CombinedOutput(); err != nil {
		t.Fatalf("git config: %v\n%s", err, out)
	}
	if ours("fetch") || ours("push") {
		t.Error("our ssh command would replace the repository's core.sshCommand")
	}
	if !ours("status") {
		t.Error("local commands should not look up the ssh command at all")
	}
}
