package sshkeys

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// store points at a throwaway .ssh directory, never the real one.
func store(t *testing.T) *Store {
	t.Helper()
	return &Store{dir: filepath.Join(t.TempDir(), ".ssh")}
}

func TestGenerate(t *testing.T) {
	s := store(t)
	ctx := context.Background()

	key, err := s.Generate(ctx, "id_focagit", "dardan@example.com")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if key.Type != "ED25519" {
		t.Errorf("type = %q, want ED25519", key.Type)
	}
	if !strings.HasPrefix(key.Fingerprint, "SHA256:") {
		t.Errorf("fingerprint = %q", key.Fingerprint)
	}
	if key.Comment != "dardan@example.com" {
		t.Errorf("comment = %q", key.Comment)
	}
	if !strings.HasPrefix(key.Public, "ssh-ed25519 ") {
		t.Errorf("public = %q, want the pasteable line", key.Public)
	}
	// Documented, not incidental: see the comment on Generate.
	if key.Encrypted {
		t.Error("Encrypted = true, but Generate never sets a passphrase")
	}

	t.Run("the private key is not readable by anyone else", func(t *testing.T) {
		st, err := os.Stat(key.Path)
		if err != nil {
			t.Fatal(err)
		}
		if perm := st.Mode().Perm(); perm&0o077 != 0 {
			t.Errorf("mode = %o, want no group or other bits", perm)
		}
		dir, err := os.Stat(s.dir)
		if err != nil {
			t.Fatal(err)
		}
		// ssh refuses to use keys from a directory others can read.
		if perm := dir.Mode().Perm(); perm&0o077 != 0 {
			t.Errorf(".ssh mode = %o, want 0700", perm)
		}
	})

	t.Run("an existing key is never overwritten", func(t *testing.T) {
		before, err := os.ReadFile(key.Path + ".pub")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.Generate(ctx, "id_focagit", "altro"); !errors.Is(err, ErrKeyExists) {
			t.Fatalf("Generate: got %v, want ErrKeyExists", err)
		}
		after, err := os.ReadFile(key.Path + ".pub")
		if err != nil {
			t.Fatal(err)
		}
		if string(before) != string(after) {
			t.Error("the key was replaced despite the refusal")
		}
	})

	t.Run("names that would escape .ssh are refused", func(t *testing.T) {
		for _, name := range []string{"", "  ", "../fuori", "sotto/chiave", ".nascosta", "x.pub"} {
			if _, err := s.Generate(ctx, name, ""); !errors.Is(err, ErrInvalidName) {
				t.Errorf("Generate(%q): got %v, want ErrInvalidName", name, err)
			}
		}
	})
}

func TestList(t *testing.T) {
	s := store(t)
	ctx := context.Background()

	for _, name := range []string{"id_uno", "id_due"} {
		if _, err := s.Generate(ctx, name, name+"@example.com"); err != nil {
			t.Fatal(err)
		}
	}

	keys, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("got %d keys, wanted 2: %+v", len(keys), keys)
	}
	// Sorted by name, so the order does not wander between runs.
	if keys[0].Name != "id_due" || keys[1].Name != "id_uno" {
		t.Errorf("order = %q, %q", keys[0].Name, keys[1].Name)
	}

	t.Run("a public key with no private half is skipped", func(t *testing.T) {
		orphan := filepath.Join(s.dir, "orfana.pub")
		if err := os.WriteFile(orphan, []byte("ssh-ed25519 AAAA orfana\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		keys, err := s.List(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, k := range keys {
			if k.Name == "orfana" {
				t.Error("a key with no private half was listed as usable")
			}
		}
	})

	t.Run("an unreadable file does not hide the rest", func(t *testing.T) {
		junk := filepath.Join(s.dir, "spazzatura.pub")
		if err := os.WriteFile(junk, []byte("questa non e una chiave\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(s.dir, "spazzatura"), []byte("nemmeno\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		keys, err := s.List(ctx)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(keys) != 2 {
			t.Errorf("got %d keys, want the 2 real ones", len(keys))
		}
	})
}

func TestEncryptedIsDetected(t *testing.T) {
	s := store(t)
	ctx := context.Background()

	plain, err := s.Generate(ctx, "id_semplice", "")
	if err != nil {
		t.Fatal(err)
	}
	if plain.Encrypted {
		t.Error("a key generated without a passphrase reports as encrypted")
	}

	// Put a passphrase on a copy, the way a user would from a terminal.
	locked := filepath.Join(s.dir, "id_protetta")
	copyFile(t, plain.Path, locked)
	copyFile(t, plain.Path+".pub", locked+".pub")
	cmd := exec.Command("ssh-keygen", "-p", "-P", "", "-N", "unapassphrase", "-f", locked)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ssh-keygen -p: %v\n%s", err, out)
	}

	keys, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range keys {
		if k.Name == "id_protetta" && !k.Encrypted {
			t.Error("Encrypted = false for a key with a passphrase")
		}
		if k.Name == "id_semplice" && k.Encrypted {
			t.Error("Encrypted = true for a key without one")
		}
	}
}

func copyFile(t *testing.T, from, to string) {
	t.Helper()
	b, err := os.ReadFile(from)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(to, b, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestParseFingerprint(t *testing.T) {
	tests := []struct {
		line    string
		bits    int
		print   string
		comment string
		kind    string
	}{
		{
			"256 SHA256:J7ysv3Ygxa dardan@example.com (ED25519)",
			256, "SHA256:J7ysv3Ygxa", "dardan@example.com", "ED25519",
		},
		{
			// ssh-add reports the path where the comment would be.
			"2048 SHA256:RLWDAv8mp2 /home/me/.ssh/id_rsa (RSA)",
			2048, "SHA256:RLWDAv8mp2", "/home/me/.ssh/id_rsa", "RSA",
		},
		{
			// A comment is free text and often has spaces in it.
			"256 SHA256:abc portatile di lavoro (ED25519)",
			256, "SHA256:abc", "portatile di lavoro", "ED25519",
		},
		{
			// ssh-keygen writes this when a key has no comment at all.
			"256 SHA256:abc no comment (ED25519)",
			256, "SHA256:abc", "no comment", "ED25519",
		},
	}

	for _, tt := range tests {
		bits, print, comment, kind := parseFingerprint(tt.line)
		if bits != tt.bits || print != tt.print || comment != tt.comment || kind != tt.kind {
			t.Errorf("parseFingerprint(%q) = %d, %q, %q, %q", tt.line, bits, print, comment, kind)
		}
	}
}
