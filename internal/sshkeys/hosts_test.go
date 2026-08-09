package sshkeys

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// hostLine builds a real known_hosts line, so nothing here needs a network.
func hostLine(t *testing.T, host string) string {
	t.Helper()
	s := store(t)
	key, err := s.Generate(context.Background(), "host_key", "")
	if err != nil {
		t.Fatal(err)
	}
	return host + " " + key.Public
}

func TestTrustHost(t *testing.T) {
	s := store(t)
	ctx := context.Background()
	line := hostLine(t, "esempio.test")

	key, err := fingerprintLine(ctx, line)
	if err != nil {
		t.Fatalf("fingerprintLine: %v", err)
	}
	if !strings.HasPrefix(key.Fingerprint, "SHA256:") || key.Type != "ED25519" {
		t.Fatalf("key = %+v", key)
	}

	if s.IsHostKnown(ctx, "esempio.test") {
		t.Fatal("a fresh known_hosts already trusts the host")
	}
	if err := s.TrustHost(ctx, "esempio.test", []HostKey{key}); err != nil {
		t.Fatalf("TrustHost: %v", err)
	}
	if !s.IsHostKnown(ctx, "esempio.test") {
		t.Error("the host is still not trusted after TrustHost")
	}

	t.Run("known_hosts is not readable by anyone else", func(t *testing.T) {
		st, err := os.Stat(filepath.Join(s.dir, "known_hosts"))
		if err != nil {
			t.Fatal(err)
		}
		if perm := st.Mode().Perm(); perm&0o077 != 0 {
			t.Errorf("mode = %o", perm)
		}
	})

	t.Run("trusting again appends without losing the first", func(t *testing.T) {
		other, err := fingerprintLine(ctx, hostLine(t, "altro.test"))
		if err != nil {
			t.Fatal(err)
		}
		if err := s.TrustHost(ctx, "altro.test", []HostKey{other}); err != nil {
			t.Fatal(err)
		}
		if !s.IsHostKnown(ctx, "esempio.test") || !s.IsHostKnown(ctx, "altro.test") {
			t.Error("one of the two hosts was lost")
		}
	})

	// A line for another host would trust a different server entirely.
	t.Run("a key belonging to another host is refused", func(t *testing.T) {
		wrong, err := fingerprintLine(ctx, hostLine(t, "cattivo.test"))
		if err != nil {
			t.Fatal(err)
		}
		if err := s.TrustHost(ctx, "buono.test", []HostKey{wrong}); !errors.Is(err, ErrNoHostKeys) {
			t.Errorf("TrustHost: got %v, want a refusal", err)
		}
		if s.IsHostKnown(ctx, "buono.test") {
			t.Error("the host was trusted despite the refusal")
		}
	})

	t.Run("nothing to trust is refused", func(t *testing.T) {
		if err := s.TrustHost(ctx, "vuoto.test", nil); !errors.Is(err, ErrNoHostKeys) {
			t.Errorf("TrustHost: got %v, want ErrNoHostKeys", err)
		}
	})

	t.Run("host names that are really options are refused", func(t *testing.T) {
		for _, host := range []string{"", "  ", "-oProxyCommand=rm -rf /", "due parole"} {
			if err := s.TrustHost(ctx, host, []HostKey{key}); err == nil {
				t.Errorf("TrustHost(%q) was accepted", host)
			}
			if s.IsHostKnown(ctx, host) {
				t.Errorf("IsHostKnown(%q) = true", host)
			}
		}
	})
}
