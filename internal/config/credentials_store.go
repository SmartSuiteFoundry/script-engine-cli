package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/zalando/go-keyring"
)

// ErrCredentialStoreFallback is returned when credentials were written to the
// config file because the OS secret store could not be used.
var ErrCredentialStoreFallback = errors.New("credentials saved in config file; OS secret store unavailable")

// credService is the OS credential store service name (visible in Keychain / Credential Manager).
const credService = "sse-cli"

func keyringAccount(absConfigPath, kind string) string {
	return absConfigPath + "\x1e" + kind
}

func absConfigPath(configPath string) (string, error) {
	if strings.TrimSpace(configPath) == "" {
		return "", errors.New("config path is empty")
	}
	return filepath.Abs(configPath)
}

// SecretsFromKeyring loads token or api_key stored for this config file path.
// On lookup errors (e.g. keychain unavailable), it returns "", "" so callers can fall back to file/env.
func SecretsFromKeyring(configPath string) (token, apiKey string) {
	abs, err := absConfigPath(configPath)
	if err != nil {
		return "", ""
	}
	t, errT := keyring.Get(credService, keyringAccount(abs, "token"))
	if errT == nil && strings.TrimSpace(t) != "" {
		return strings.TrimSpace(t), ""
	}
	if errT != nil && !errors.Is(errT, keyring.ErrNotFound) {
		return "", ""
	}
	k, errK := keyring.Get(credService, keyringAccount(abs, "api_key"))
	if errK == nil && strings.TrimSpace(k) != "" {
		return "", strings.TrimSpace(k)
	}
	return "", ""
}

func saveSecretsKeyring(absConfigPath, token, apiKey string) error {
	tok := strings.TrimSpace(token)
	key := strings.TrimSpace(apiKey)
	if tok != "" && key != "" {
		return errors.New("token and api_key cannot both be set")
	}
	_ = keyring.Delete(credService, keyringAccount(absConfigPath, "token"))
	_ = keyring.Delete(credService, keyringAccount(absConfigPath, "api_key"))
	if tok != "" {
		return keyring.Set(credService, keyringAccount(absConfigPath, "token"), tok)
	}
	if key != "" {
		return keyring.Set(credService, keyringAccount(absConfigPath, "api_key"), key)
	}
	return nil
}

// ReadMerged loads the config file and overlays credentials from the OS secret store when present.
func ReadMerged(path string) (File, error) {
	f, err := ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			f = File{}
		} else {
			return File{}, err
		}
	}
	if kt, ka := SecretsFromKeyring(path); kt != "" || ka != "" {
		f.Token, f.APIKey = kt, ka
	}
	return f, nil
}
