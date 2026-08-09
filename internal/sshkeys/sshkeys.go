// Package sshkeys reads and creates SSH keys; it never reads private key material.
package sshkeys

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

var (
	// ErrKeyExists means the name is taken; overwriting locks the user out.
	ErrKeyExists = errors.New("a key by that name already exists")
	// ErrInvalidName means the name would escape the .ssh directory or hide the key.
	ErrInvalidName = errors.New("invalid key name")
	// ErrNoAgent means no ssh-agent is running to hold the key.
	ErrNoAgent = errors.New("no ssh-agent is running")
)

// Key is one keypair as seen from outside: nothing here is secret.
type Key struct {
	// Name is the file name, e.g. "id_ed25519".
	Name string `json:"name"`
	Path string `json:"path"`
	// Type is ED25519, RSA, ECDSA — whatever ssh-keygen reports.
	Type        string `json:"type"`
	Bits        int    `json:"bits"`
	Fingerprint string `json:"fingerprint"`
	Comment     string `json:"comment"`
	// Public is the whole public key line, the thing you paste into GitHub.
	Public string `json:"public"`
	// InAgent means the agent holds it, so git can use it without asking anything.
	InAgent bool `json:"inAgent"`
	// Encrypted means the private key has a passphrase.
	Encrypted bool `json:"encrypted"`
}

// Store is the .ssh directory.
type Store struct{ dir string }

// Open uses the real ~/.ssh.
func Open() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return &Store{dir: filepath.Join(home, ".ssh")}, nil
}

// Dir is where the keys live, for showing in the UI.
func (s *Store) Dir() string { return s.dir }

// List returns every keypair with a readable .pub file, oldest name first.
func (s *Store) List(ctx context.Context) ([]Key, error) {
	matches, err := filepath.Glob(filepath.Join(s.dir, "*.pub"))
	if err != nil {
		return nil, err
	}

	loaded := agentFingerprints(ctx)

	keys := make([]Key, 0, len(matches))
	for _, pub := range matches {
		key, err := s.read(ctx, pub)
		if err != nil {
			// A stray or unreadable .pub is not a reason to show nothing.
			continue
		}
		key.InAgent = loaded[key.Fingerprint]
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].Name < keys[j].Name })
	return keys, nil
}

// read builds a Key from a public key file and its private counterpart.
func (s *Store) read(ctx context.Context, pubPath string) (Key, error) {
	priv := strings.TrimSuffix(pubPath, ".pub")
	if _, err := os.Stat(priv); err != nil {
		return Key{}, fmt.Errorf("%s has no private key beside it", pubPath)
	}

	content, err := os.ReadFile(pubPath)
	if err != nil {
		return Key{}, err
	}

	key := Key{
		Name:   filepath.Base(priv),
		Path:   priv,
		Public: strings.TrimSpace(string(content)),
	}

	out, err := exec.CommandContext(ctx, "ssh-keygen", "-l", "-f", pubPath).Output()
	if err != nil {
		return Key{}, err
	}
	key.Bits, key.Fingerprint, key.Comment, key.Type = parseFingerprint(string(out))
	key.Encrypted = isEncrypted(ctx, priv)
	return key, nil
}

// parseFingerprint reads "256 SHA256:xxx a comment here (ED25519)".
func parseFingerprint(line string) (bits int, fingerprint, comment, kind string) {
	line = strings.TrimSpace(line)

	// The type is the last parenthesised word.
	if i := strings.LastIndex(line, "("); i >= 0 {
		kind = strings.TrimSuffix(line[i+1:], ")")
		line = strings.TrimSpace(line[:i])
	}

	first, rest, ok := strings.Cut(line, " ")
	if !ok {
		return 0, "", "", kind
	}
	bits, _ = strconv.Atoi(first)

	fingerprint, comment, _ = strings.Cut(rest, " ")
	return bits, fingerprint, strings.TrimSpace(comment), kind
}

// isEncrypted asks whether the private key opens with an empty passphrase.
func isEncrypted(ctx context.Context, priv string) bool {
	cmd := exec.CommandContext(ctx, "ssh-keygen", "-y", "-P", "", "-f", priv)
	cmd.Stdout, cmd.Stderr = nil, nil
	return cmd.Run() != nil
}

// agentFingerprints is the set of keys the agent holds, empty if none is running.
func agentFingerprints(ctx context.Context) map[string]bool {
	loaded := map[string]bool{}

	out, err := exec.CommandContext(ctx, "ssh-add", "-l").Output()
	if err != nil {
		// Exit 1 means no identities, exit 2 no agent: both mean nothing is loaded.
		return loaded
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if _, fingerprint, _, _ := parseFingerprint(line); fingerprint != "" {
			loaded[fingerprint] = true
		}
	}
	return loaded
}

// Generate creates an ed25519 keypair. It never sets a passphrase, and that is
// deliberate: ssh-keygen cannot be given one without either putting it in argv,
// where every process on the machine can read it, or an askpass helper we do not
// ship. Worse, fed one on stdin it silently generates an unprotected key and
// exits 0 — a key the user would believe was protected. So the honest options
// are none at all, or the terminal.
func (s *Store) Generate(ctx context.Context, name, comment string) (Key, error) {
	if err := checkName(name); err != nil {
		return Key{}, err
	}
	if strings.ContainsAny(comment, "\n\r") {
		return Key{}, fmt.Errorf("%w: the comment has a line break", ErrInvalidName)
	}

	priv := filepath.Join(s.dir, name)
	// ssh-keygen would ask whether to overwrite, and with no terminal that is a hang.
	if _, err := os.Stat(priv); err == nil {
		return Key{}, fmt.Errorf("%w: %s", ErrKeyExists, name)
	}
	// 0700, because ssh refuses to use keys in a world-readable directory.
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return Key{}, err
	}

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "ssh-keygen",
		"-t", "ed25519", "-f", priv, "-N", "", "-C", comment, "-q")
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return Key{}, fmt.Errorf("ssh-keygen: %s", strings.TrimSpace(stderr.String()))
	}
	return s.read(ctx, priv+".pub")
}

// checkName keeps the name a plain file inside .ssh.
func checkName(name string) error {
	switch {
	case strings.TrimSpace(name) == "":
		return fmt.Errorf("%w: it is empty", ErrInvalidName)
	case name != filepath.Base(name), strings.Contains(name, "/"):
		return fmt.Errorf("%w: %q is a path, not a name", ErrInvalidName, name)
	case strings.HasPrefix(name, "."):
		return fmt.Errorf("%w: %q would be hidden", ErrInvalidName, name)
	case strings.HasSuffix(name, ".pub"):
		return fmt.Errorf("%w: %q is the public half's name", ErrInvalidName, name)
	}
	return nil
}

// AddToAgent hands the key to the running agent so git can use it unattended.
func (s *Store) AddToAgent(ctx context.Context, path string) error {
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "ssh-add", path)
	cmd.Stderr = &stderr
	// A key with a passphrase would ask for it, and there is nothing here to ask with.
	cmd.Env = append(os.Environ(), "SSH_ASKPASS_REQUIRE=never", "DISPLAY=")

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if strings.Contains(msg, "Could not open a connection") {
			return ErrNoAgent
		}
		return fmt.Errorf("ssh-add: %s", msg)
	}
	return nil
}
