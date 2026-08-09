package sshkeys

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ErrNoHostKeys means the server answered with nothing we can trust.
var ErrNoHostKeys = errors.New("the server offered no host key")

// HostKey is one key a server presented, as it would be stored and as it reads.
type HostKey struct {
	// Type is ED25519, RSA, ECDSA.
	Type string `json:"type"`
	// Fingerprint is what the user compares against the host's published list.
	Fingerprint string `json:"fingerprint"`
	// Line is the entry itself, so what is trusted is what was shown.
	Line string `json:"line"`
}

func (s *Store) knownHosts() string { return filepath.Join(s.dir, "known_hosts") }

// IsHostKnown reports whether ssh would already talk to this host.
func (s *Store) IsHostKnown(ctx context.Context, host string) bool {
	if err := checkHost(host); err != nil {
		return false
	}
	// -F exits 1 when the host is absent, which is an answer rather than a failure.
	cmd := exec.CommandContext(ctx, "ssh-keygen", "-F", host, "-f", s.knownHosts())
	cmd.Stdout, cmd.Stderr = nil, nil
	return cmd.Run() == nil
}

// ScanHost asks the server for its keys. Nothing is trusted by this.
func (s *Store) ScanHost(ctx context.Context, host string) ([]HostKey, error) {
	if err := checkHost(host); err != nil {
		return nil, err
	}

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "ssh-keyscan", "-T", "10", "-t", "rsa,ecdsa,ed25519", "--", host)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ssh-keyscan: %s", strings.TrimSpace(stderr.String()))
	}

	var keys []HostKey
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		// Comments start with #, and ssh-keyscan emits several.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, err := fingerprintLine(ctx, line)
		if err != nil {
			continue
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrNoHostKeys, host)
	}
	return keys, nil
}

// fingerprintLine lets ssh-keygen compute the fingerprint, not us.
func fingerprintLine(ctx context.Context, line string) (HostKey, error) {
	cmd := exec.CommandContext(ctx, "ssh-keygen", "-l", "-f", "-")
	cmd.Stdin = strings.NewReader(line + "\n")
	out, err := cmd.Output()
	if err != nil {
		return HostKey{}, err
	}

	_, fingerprint, _, kind := parseFingerprint(string(out))
	if fingerprint == "" {
		return HostKey{}, fmt.Errorf("could not fingerprint %q", line)
	}
	return HostKey{Type: kind, Fingerprint: fingerprint, Line: line}, nil
}

// TrustHost appends the keys the user approved, exactly as they were shown.
func (s *Store) TrustHost(ctx context.Context, host string, keys []HostKey) error {
	if err := checkHost(host); err != nil {
		return err
	}
	if len(keys) == 0 {
		return ErrNoHostKeys
	}

	var b strings.Builder
	for _, k := range keys {
		line := strings.TrimSpace(k.Line)
		// A line for some other host would trust the wrong server entirely.
		if line == "" || !strings.HasPrefix(line, host+" ") {
			return fmt.Errorf("%w: %q is not a key for %s", ErrNoHostKeys, line, host)
		}
		b.WriteString(line + "\n")
	}

	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(s.knownHosts(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(b.String())
	return err
}

// checkHost keeps a host name from being read as an option or a second argument.
func checkHost(host string) error {
	switch {
	case strings.TrimSpace(host) == "":
		return errors.New("no host given")
	case strings.HasPrefix(host, "-"):
		return fmt.Errorf("%q is not a host name", host)
	case strings.ContainsAny(host, " \t\n\r"):
		return fmt.Errorf("%q is not a host name", host)
	}
	return nil
}
