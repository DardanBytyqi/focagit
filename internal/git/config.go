package git

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidSignature means the name or email cannot be written to a config file.
var ErrInvalidSignature = errors.New("invalid name or email")

// Signature is who commits made from this repository are attributed to.
type Signature struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	// Scope is where the values come from: local, global, system, or empty when unset.
	// Name and email are separate keys and can genuinely come from different files.
	NameScope  string `json:"nameScope"`
	EmailScope string `json:"emailScope"`
}

// IsLocal reports whether this repository sets both values itself.
func (s Signature) IsLocal() bool { return s.NameScope == "local" && s.EmailScope == "local" }

// IsComplete reports whether git has enough to make a commit.
func (s Signature) IsComplete() bool { return s.Name != "" && s.Email != "" }

// Signature reads the identity git would use for the next commit here.
func (r *Repo) Signature(ctx context.Context) (Signature, error) {
	var sig Signature
	sig.Name, sig.NameScope = r.configScoped(ctx, "user.name")
	sig.Email, sig.EmailScope = r.configScoped(ctx, "user.email")
	return sig, nil
}

// GlobalSignature reads the identity a repository falls back to when it sets none.
func (r *Repo) GlobalSignature(ctx context.Context) (Signature, error) {
	var sig Signature
	sig.Name, sig.NameScope = r.configScoped(ctx, "user.name", "--global")
	sig.Email, sig.EmailScope = r.configScoped(ctx, "user.email", "--global")
	return sig, nil
}

// configScoped returns a config value and which file it came from.
func (r *Repo) configScoped(ctx context.Context, key string, where ...string) (value, scope string) {
	args := append([]string{"config"}, where...)
	args = append(args, "--show-scope", "--get", key)

	// An unset key exits 1, which is an answer rather than a failure.
	out, err := r.run.outputQuiet(ctx, args...)
	if err != nil {
		return "", ""
	}
	// The format is "<scope>\t<value>".
	scope, value, found := strings.Cut(strings.TrimRight(string(out), "\n"), "\t")
	if !found {
		return "", ""
	}
	return value, scope
}

// SetSignature writes the identity into this repository's own config.
func (r *Repo) SetSignature(ctx context.Context, name, email string) error {
	if err := checkConfigValue(name); err != nil {
		return err
	}
	if err := checkConfigValue(email); err != nil {
		return err
	}

	// -- so a value starting with "-" is not read as an option.
	if _, err := r.run.output(ctx, "config", "--local", "--", "user.name", name); err != nil {
		return err
	}
	_, err := r.run.output(ctx, "config", "--local", "--", "user.email", email)
	return err
}

// ClearSignature drops the local identity so the global one applies again.
func (r *Repo) ClearSignature(ctx context.Context) error {
	// --unset exits 5 when the key was not there, which is the state we wanted anyway.
	if _, err := r.run.outputOK(ctx, []int{5}, "config", "--local", "--unset", "user.name"); err != nil {
		return err
	}
	_, err := r.run.outputOK(ctx, []int{5}, "config", "--local", "--unset", "user.email")
	return err
}

// checkConfigValue rejects what would corrupt a config file or make no sense in one.
func checkConfigValue(v string) error {
	if strings.TrimSpace(v) == "" {
		return fmt.Errorf("%w: the value is empty", ErrInvalidSignature)
	}
	if strings.ContainsAny(v, "\n\r") {
		return fmt.Errorf("%w: it contains a line break", ErrInvalidSignature)
	}
	return nil
}
