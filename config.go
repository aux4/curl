package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// providerMissingError marks the one resolution failure that is about setup
// rather than about the reference, so callers can pass it through instead of
// appending advice that does not apply.
type providerMissingError struct{ message string }

func (e providerMissingError) Error() string { return e.message }

const secretURIPrefix = "secret://"

// A definition can name its secret instead of carrying it. The reference is
// deliberately NOT a secret:// URI: the aux4 core resolves any parameter whose
// whole value is one, so a URI handed to `auth add` would arrive here already
// resolved and get written into config.yaml in the clear -- the precise leak
// this field exists to prevent. Without the scheme nothing intercepts it, and
// it is resolved here, at the moment it is needed.
//
// Format: <provider>/<vault>/<item>/<field>, e.g. aux4/Work/Billing API/clientSecret
func resolveSecretRef(ref string) (string, error) {
	ref = strings.Trim(strings.TrimSpace(ref), "/")
	parts := strings.Split(ref, "/")
	if len(parts) < 3 {
		return "", fmt.Errorf("secret reference %q must be <provider>/<vault>/<item>/<field>", ref)
	}

	provider := parts[0]
	field := parts[len(parts)-1]
	itemRef := strings.Join(parts[1:len(parts)-1], "/")

	command := exec.Command("aux4", "secret", provider, "get", "--ref", itemRef, "--fields", field)

	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		if hint := missingProviderMessage(provider, message); hint != "" {
			return "", providerMissingError{message: hint}
		}
		// The provider prefixes its own failures, so keeping ours as well would
		// read as "Error: could not resolve x: Error: ...".
		return "", fmt.Errorf("could not resolve %s: %s", ref, strings.TrimPrefix(message, "Error: "))
	}

	var fields map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &fields); err != nil {
		return "", fmt.Errorf("secret provider %q returned an unexpected response for %s", provider, ref)
	}

	value, ok := fields[field]
	if !ok || value == "" {
		return "", fmt.Errorf("secret %s resolved to an empty value", ref)
	}
	return value, nil
}

// validateSecretRef checks that a value really is a reference before it is
// written anywhere.
//
// This guards a leak that is easy to walk into: `secret://` is the reference
// syntax everywhere else in aux4, so reaching for it here is the natural move
// -- but the core resolves any parameter whose whole value is such a URI before
// the command ever runs. What arrives is then the secret itself, and writing it
// out as though it were a reference puts the plaintext into a file destined for
// version control.
//
// Two checks, because neither alone is sufficient. The shape catches the common
// case (a resolved secret rarely looks like provider/vault/item/field), and
// resolving it proves the value names something real rather than merely
// resembling a path.
func validateSecretRef(ref string) error {
	ref = strings.TrimSpace(ref)

	if strings.HasPrefix(ref, secretURIPrefix) {
		return fmt.Errorf("pass the reference without the %s prefix (%s), otherwise aux4 resolves it to the secret itself before this command runs and the secret would be written to the config file",
			secretURIPrefix, strings.TrimPrefix(ref, secretURIPrefix))
	}

	if len(strings.Split(strings.Trim(ref, "/"), "/")) < 3 {
		// The value is deliberately not echoed. If this check is failing it is
		// most likely because the value IS the secret, and repeating it to
		// stderr would put it into terminal scrollback and CI logs.
		return fmt.Errorf("the secret reference must be <provider>/<vault>/<item>/<field>. If you passed a %s URI, drop the prefix: aux4 resolves those to the secret itself before this command runs, so what arrived here was not a reference",
			secretURIPrefix)
	}

	if _, err := resolveSecretRef(ref); err != nil {
		// A missing provider is already actionable; appending advice about the
		// value would point at the wrong problem.
		var missing providerMissingError
		if errors.As(err, &missing) {
			return err
		}
		return fmt.Errorf("%v. The value must name a secret, not be one", err)
	}

	return nil
}

// missingProviderMessage turns "Command not found" into something actionable.
//
// aux4 exits 127 with that text when a command does not exist, which is what
// happens both when no secret provider is installed and when aux4/secret itself
// is absent. Passing the raw text through would tell the user only that some
// word was not a command.
//
// What it deliberately does NOT do is name a package to install. The provider
// in a reference is a command name, and nothing about it reveals the scope or
// package name behind it -- "1password" comes from aux4/secret-1password, and a
// third-party provider could sit in any scope under any name. Guessing
// "aux4/secret-<provider>" would hand the user an install command that is very
// likely wrong.
func missingProviderMessage(provider string, stderr string) string {
	if !strings.Contains(stderr, "Command not found") {
		return ""
	}

	if strings.Contains(stderr, "Command not found: secret") {
		return "aux4/secret is not installed"
	}

	if !strings.Contains(stderr, "Command not found: "+provider) {
		return ""
	}

	return fmt.Sprintf("no secret provider %q is installed", provider)
}

// withResolvedCredentials returns a copy carrying the client secret that was
// stored by reference. An inline secret always wins, so a machine that has one
// configured locally does not need the secret provider at all.
//
// It returns a copy rather than filling in the receiver on purpose. This config
// is persisted alongside the cached token, and writing the resolved secret into
// it would put the secret at rest in the token file -- undoing the one thing
// the reference exists to achieve. The resolved value lives only as long as the
// request that needs it.
func (c *AuthConfig) withResolvedCredentials() (*AuthConfig, error) {
	if c.ClientSecret != "" || c.ClientSecretRef == "" {
		return c, nil
	}

	secret, err := resolveSecretRef(c.ClientSecretRef)
	if err != nil {
		return nil, err
	}

	resolved := *c
	resolved.ClientSecret = secret
	return &resolved, nil
}

// authFromConfig picks one named definition out of the `auth` map that arrived
// from config.yaml. aux4 hands an object-valued parameter over as a JSON
// string, so there is nothing to parse but JSON.
func authFromConfig(raw string, name string) (*AuthConfig, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	definitions := map[string]*AuthConfig{}
	if err := json.Unmarshal([]byte(raw), &definitions); err != nil {
		return nil, fmt.Errorf("the auth configuration is not a valid object: %v", err)
	}

	definition, ok := definitions[name]
	if !ok {
		available := make([]string, 0, len(definitions))
		for key := range definitions {
			available = append(available, key)
		}
		if len(available) == 0 {
			return nil, nil
		}
		return nil, fmt.Errorf("no auth named %q in the configuration (found: %s)", name, strings.Join(available, ", "))
	}

	return definition, nil
}

// tokenFromConfig builds a token entry from a configured definition, so a
// machine that has never run `auth add` can still authenticate: the definition
// comes from the shared config and the token is fetched on first use.
func tokenFromConfig(definition *AuthConfig) *OAuthToken {
	return &OAuthToken{
		Grant:    grantClientCredentials,
		ClientID: definition.ClientID,
		TokenURL: definition.TokenURL,
		Scopes:   definition.Scopes,
		Auth:     definition,
	}
}

// saveAuthToConfig writes a definition into config.yaml under
// <config>/auth/<name>, delegating to aux4/config so the file keeps its own
// format and the rest of its contents.
//
// The client secret is never written here. Only clientSecretRef is, which is
// what keeps the file safe to commit and share.
func saveAuthToConfig(configFile string, configName string, name string, definition *AuthConfig) error {
	shareable := *definition
	shareable.ClientSecret = ""

	payload, err := json.Marshal(&shareable)
	if err != nil {
		return err
	}

	path := name
	if configName != "" {
		path = configName + "/auth/" + name
	} else {
		path = "auth/" + name
	}

	args := []string{"config", "merge", "--name", path, "--save", "true"}
	if configFile != "" {
		args = append(args, "--file", configFile)
	}

	command := exec.Command("aux4", args...)
	command.Stdin = bytes.NewReader(payload)

	var stderr bytes.Buffer
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("could not write the auth to %s: %s", configFile, message)
	}
	return nil
}
