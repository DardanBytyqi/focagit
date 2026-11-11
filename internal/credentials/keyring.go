package credentials

import (
	"encoding/json"
	"errors"

	"github.com/zalando/go-keyring"
)

// keyringService groups focagit's entries in the system keyring.
const keyringService = "focagit"

// Keyring keeps logins in the system keyring: the Secret Service on Linux, Keychain on macOS.
type Keyring struct{}

func (Keyring) Get(origin string) (Login, error) {
	secret, err := keyring.Get(keyringService, origin)
	if errors.Is(err, keyring.ErrNotFound) {
		return Login{}, ErrNotFound
	}
	if err != nil {
		return Login{}, err
	}
	var l Login
	if err := json.Unmarshal([]byte(secret), &l); err != nil {
		return Login{}, ErrNotFound
	}
	return l, nil
}

func (Keyring) Set(origin string, l Login) error {
	b, _ := json.Marshal(l)
	return keyring.Set(keyringService, origin, string(b))
}

func (Keyring) Delete(origin string) error {
	err := keyring.Delete(keyringService, origin)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}
