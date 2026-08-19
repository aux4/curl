package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	// Re-acquire slightly before the deadline. A token that expires in flight
	// fails the very call it was fetched for, and client/server clocks drift.
	expirySkew = 60 * time.Second
	// Used when the endpoint reports no lifetime and the token is not a JWT.
	// A fetched token is never treated as valid forever.
	defaultTokenTTL = 3600 * time.Second
	// Grant recorded on tokens obtained from client credentials, as opposed to
	// the interactive authorization_code flow that `oauth login` performs.
	grantClientCredentials = "client_credentials"
	defaultAuthHeader      = "Authorization: Bearer ${token}"
)

// AuthConfig is the recipe for obtaining a token from a credential endpoint:
// how to ask, and where the answer lives in the response. It is stored next to
// the token it produced, so a token that expires can always be re-acquired.
type AuthConfig struct {
	Preset        string   `json:"preset,omitempty"`
	TokenURL      string   `json:"tokenUrl"`
	ClientID      string   `json:"clientId,omitempty"`
	ClientSecret  string   `json:"clientSecret,omitempty"`
	Scopes        string   `json:"scopes,omitempty"`
	Audience      string   `json:"audience,omitempty"`
	Method        string   `json:"method,omitempty"`
	ContentType   string   `json:"contentType,omitempty"`
	CredentialsIn string   `json:"credentialsIn,omitempty"`
	Headers       []string `json:"headers,omitempty"`
	Body          string   `json:"body,omitempty"`
	TokenPath     string   `json:"tokenPath,omitempty"`
	ExpiresPath   string   `json:"expiresPath,omitempty"`
	ExpiresIn     int      `json:"expiresIn,omitempty"`
	AuthHeader    string   `json:"authHeader,omitempty"`
	// ClientSecretRef names the secret instead of holding it, so a definition
	// can be committed to a shared config file. See resolveSecretRef.
	ClientSecretRef string `json:"clientSecretRef,omitempty"`
}

// preset is a bundle of defaults, not a separate code path: every field it sets
// can be overridden individually, so there is exactly one request builder and
// one extraction routine no matter which preset was chosen.
type preset struct {
	contentType   string
	credentialsIn string
	// tokenPaths are tried in order. Two well-known names at most: responses
	// that carry both access_token and id_token would otherwise let a longer
	// probe list hand back a token that authenticates nothing.
	tokenPaths []string
	// expirySources are tried in order: "field" (expires_in/expires_at),
	// "jwt" (the exp claim), "static" (--expiresIn, then the default TTL).
	expirySources []string
}

var presets = map[string]preset{
	"client-credentials": {
		contentType:   "form",
		credentialsIn: "body",
		tokenPaths:    []string{"$.access_token", "$.token"},
		expirySources: []string{"field", "jwt", "static"},
	},
	"client-credentials-basic": {
		contentType:   "form",
		credentialsIn: "header",
		tokenPaths:    []string{"$.access_token", "$.token"},
		expirySources: []string{"field", "jwt", "static"},
	},
	"jwt": {
		contentType:   "form",
		credentialsIn: "body",
		tokenPaths:    []string{"$.token", "$.access_token"},
		expirySources: []string{"jwt", "field", "static"},
	},
	"json": {
		contentType:   "json",
		credentialsIn: "body",
		tokenPaths:    []string{"$.token", "$.access_token"},
		expirySources: []string{"field", "jwt", "static"},
	},
}

func presetNames() []string {
	return []string{"client-credentials", "client-credentials-basic", "jwt", "json"}
}

// resolve merges the preset defaults with the explicit overrides on the config.
func (c *AuthConfig) resolve() preset {
	name := c.Preset
	if name == "" {
		name = "client-credentials"
	}
	p, ok := presets[name]
	if !ok {
		p = presets["client-credentials"]
	}

	if c.ContentType != "" {
		p.contentType = c.ContentType
	}
	if c.CredentialsIn != "" {
		p.credentialsIn = c.CredentialsIn
	}
	// An explicit path replaces the candidate list entirely: the caller knows
	// where the token is, so a fallback could only ever pick the wrong field.
	if c.TokenPath != "" {
		p.tokenPaths = []string{c.TokenPath}
	}
	if c.ExpiresPath != "" {
		p.expirySources = append([]string{"path"}, p.expirySources...)
	}
	return p
}

func (c *AuthConfig) authHeaderTemplate() string {
	if strings.TrimSpace(c.AuthHeader) != "" {
		return c.AuthHeader
	}
	return defaultAuthHeader
}

// applyAuthHeader renders the outbound auth header for a token. The template is
// what allows a non-bearer API (X-API-Key: ${token}) to use the same machinery.
func applyAuthHeader(template string, token string) string {
	if strings.TrimSpace(template) == "" {
		template = defaultAuthHeader
	}
	return strings.ReplaceAll(template, "${token}", token)
}

// jsonPathGet resolves the subset of the aux4/json path syntax that a token
// response needs: a leading "$", dot-separated fields, and numeric array
// indices (negative counts from the end). Wildcards and filters are absent on
// purpose -- an unsupported expression fails here rather than quietly
// selecting the wrong value.
func jsonPathGet(data interface{}, path string) (interface{}, bool) {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "$")
	path = strings.TrimPrefix(path, ".")
	if path == "" {
		return data, true
	}

	current := data
	for _, segment := range strings.Split(path, ".") {
		if segment == "" {
			continue
		}
		switch node := current.(type) {
		case map[string]interface{}:
			value, ok := node[segment]
			if !ok {
				return nil, false
			}
			current = value
		case []interface{}:
			index, err := strconv.Atoi(segment)
			if err != nil {
				return nil, false
			}
			if index < 0 {
				index += len(node)
			}
			if index < 0 || index >= len(node) {
				return nil, false
			}
			current = node[index]
		default:
			return nil, false
		}
	}
	return current, true
}

// jsonPathString resolves a path and coerces the result to a non-empty string.
func jsonPathString(data interface{}, path string) (string, bool) {
	value, ok := jsonPathGet(data, path)
	if !ok || value == nil {
		return "", false
	}
	text, ok := value.(string)
	if !ok {
		return "", false
	}
	if strings.TrimSpace(text) == "" {
		return "", false
	}
	return text, true
}

// jwtExpiry reads the exp claim out of a JWT without verifying its signature.
//
// We are the client: the token is opaque to us and we hold no key, so there is
// nothing to verify against and nothing to gain by trying. exp is read purely
// as a cache hint, and it is a better one than expires_in because it is
// absolute -- it survives a slow token request instead of being measured from
// a moment we would have to guess.
//
// Every failure is soft. A token that is not a JWT, or is one without a usable
// exp, is still perfectly valid to the API; it just tells us nothing about its
// own lifetime, so the caller falls through to the next expiry source.
func jwtExpiry(token string, now time.Time) (time.Time, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}, false
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// Not every encoder omits the padding the JWT spec says to omit.
		payload, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return time.Time{}, false
		}
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return time.Time{}, false
	}

	seconds, ok := numericValue(claims["exp"])
	if !ok || seconds <= 0 {
		return time.Time{}, false
	}

	expiry := time.Unix(int64(seconds), 0).UTC()
	// An exp already in the past is not a useful cache hint -- treat it as
	// absent so a static TTL takes over rather than forcing a refetch loop.
	if !expiry.After(now) {
		return time.Time{}, false
	}
	return expiry, true
}

// numericValue accepts the shapes a JSON number arrives in, including the
// string form some endpoints emit for numeric fields.
func numericValue(value interface{}) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case int:
		return float64(typed), true
	case json.Number:
		if parsed, err := typed.Float64(); err == nil {
			return parsed, true
		}
	case string:
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64); err == nil {
			return parsed, true
		}
	}
	return 0, false
}

// absoluteTime parses the absolute-deadline forms seen in the wild: RFC3339
// and epoch seconds, each of which may arrive as a string or a number.
func absoluteTime(value interface{}) (time.Time, bool) {
	if text, ok := value.(string); ok {
		if parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(text)); err == nil {
			return parsed.UTC(), true
		}
	}
	if seconds, ok := numericValue(value); ok && seconds > 0 {
		return time.Unix(int64(seconds), 0).UTC(), true
	}
	return time.Time{}, false
}

var relativeExpiryFields = []string{"expires_in", "expiresIn"}
var absoluteExpiryFields = []string{"expires_at", "expiresAt", "expiry", "expiration"}

// expiryFromFields reads the lifetime the endpoint reported. Relative
// (expires_in) and absolute (expires_at) forms both occur, so both are probed.
func expiryFromFields(result map[string]interface{}, now time.Time) (time.Time, bool) {
	for _, field := range relativeExpiryFields {
		if value, ok := result[field]; ok {
			if seconds, ok := numericValue(value); ok && seconds > 0 {
				return now.Add(time.Duration(seconds) * time.Second), true
			}
		}
	}
	for _, field := range absoluteExpiryFields {
		if value, ok := result[field]; ok {
			if parsed, ok := absoluteTime(value); ok {
				return parsed, true
			}
		}
	}
	return time.Time{}, false
}

// resolveExpiry walks the configured sources in order and returns the first
// deadline any of them yields. The final source is always a static TTL, so
// this never reports "no expiry" for a freshly fetched token.
func resolveExpiry(cfg *AuthConfig, p preset, result map[string]interface{}, token string, now time.Time) time.Time {
	for _, source := range p.expirySources {
		switch source {
		case "path":
			if value, ok := jsonPathGet(result, cfg.ExpiresPath); ok {
				// An explicit path may point at either form.
				if seconds, ok := numericValue(value); ok && seconds > 0 && seconds < 1e9 {
					return now.Add(time.Duration(seconds) * time.Second)
				}
				if parsed, ok := absoluteTime(value); ok {
					return parsed
				}
			}
		case "field":
			if parsed, ok := expiryFromFields(result, now); ok {
				return parsed
			}
		case "jwt":
			if parsed, ok := jwtExpiry(token, now); ok {
				return parsed
			}
		case "static":
			if cfg.ExpiresIn > 0 {
				return now.Add(time.Duration(cfg.ExpiresIn) * time.Second)
			}
			return now.Add(defaultTokenTTL)
		}
	}
	if cfg.ExpiresIn > 0 {
		return now.Add(time.Duration(cfg.ExpiresIn) * time.Second)
	}
	return now.Add(defaultTokenTTL)
}

// buildTokenRequest assembles the credential exchange described by the config.
func buildTokenRequest(cfg *AuthConfig, p preset) (*http.Request, error) {
	method := strings.ToUpper(strings.TrimSpace(cfg.Method))
	if method == "" {
		method = "POST"
	}

	basicAuth := p.credentialsIn == "header"

	var payload string
	contentType := ""

	switch {
	case strings.TrimSpace(cfg.Body) != "":
		// An explicit body wins outright; the caller is describing an endpoint
		// whose shape we could not have guessed.
		payload = cfg.Body
		if p.contentType == "json" {
			contentType = "application/json"
		} else {
			contentType = "application/x-www-form-urlencoded"
		}
	case p.contentType == "json":
		fields := map[string]string{}
		if !basicAuth {
			if cfg.ClientID != "" {
				fields["clientId"] = cfg.ClientID
			}
			if cfg.ClientSecret != "" {
				fields["clientSecret"] = cfg.ClientSecret
			}
		}
		if cfg.Scopes != "" {
			fields["scope"] = cfg.Scopes
		}
		if cfg.Audience != "" {
			fields["audience"] = cfg.Audience
		}
		encoded, err := json.Marshal(fields)
		if err != nil {
			return nil, err
		}
		payload = string(encoded)
		contentType = "application/json"
	default:
		form := url.Values{"grant_type": {grantClientCredentials}}
		if !basicAuth {
			if cfg.ClientID != "" {
				form.Set("client_id", cfg.ClientID)
			}
			if cfg.ClientSecret != "" {
				form.Set("client_secret", cfg.ClientSecret)
			}
		}
		if cfg.Scopes != "" {
			form.Set("scope", cfg.Scopes)
		}
		if cfg.Audience != "" {
			form.Set("audience", cfg.Audience)
		}
		payload = form.Encode()
		contentType = "application/x-www-form-urlencoded"
	}

	request, err := http.NewRequest(method, cfg.TokenURL, strings.NewReader(payload))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("Accept", "application/json")

	if basicAuth {
		// RFC 6749 section 2.3.1: form-urlencode each half before joining, and
		// send them as HTTP Basic. Several providers accept only this form.
		credentials := url.QueryEscape(cfg.ClientID) + ":" + url.QueryEscape(cfg.ClientSecret)
		request.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(credentials)))
	}

	for _, header := range cfg.Headers {
		header = strings.TrimSpace(header)
		if header == "" {
			continue
		}
		parts := strings.SplitN(header, ":", 2)
		if len(parts) == 2 {
			request.Header.Set(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		}
	}

	return request, nil
}

// truncate keeps an error message readable when a token endpoint answers with
// an HTML error page instead of the JSON it promised.
func truncate(text string, limit int) string {
	text = strings.TrimSpace(text)
	if len(text) <= limit {
		return text
	}
	return text[:limit] + "..."
}

// fetchToken performs the credential exchange and extracts the token and its
// deadline. It is the single re-acquisition path for client-credential grants,
// used both for the first fetch and for every refresh after it.
func fetchToken(cfg *AuthConfig) (string, time.Time, error) {
	if strings.TrimSpace(cfg.TokenURL) == "" {
		return "", time.Time{}, fmt.Errorf("tokenUrl is required")
	}

	cfg, err := cfg.withResolvedCredentials()
	if err != nil {
		return "", time.Time{}, err
	}

	p := cfg.resolve()

	request, err := buildTokenRequest(cfg, p)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("could not build token request: %v", err)
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("token request failed: %v", err)
	}
	defer response.Body.Close()

	raw, err := io.ReadAll(response.Body)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("could not read token response: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", time.Time{}, fmt.Errorf("token endpoint returned HTTP %d with a non-JSON body: %s",
			response.StatusCode, truncate(string(raw), 200))
	}

	// OAuth reports failures in the body, so check it before the status code:
	// error_description is where the provider actually explains what is wrong.
	if errMsg, ok := result["error"]; ok && errMsg != nil && fmt.Sprintf("%v", errMsg) != "" {
		if desc, ok := result["error_description"]; ok {
			return "", time.Time{}, fmt.Errorf("token request failed: %v (%v)", errMsg, desc)
		}
		return "", time.Time{}, fmt.Errorf("token request failed: %v", errMsg)
	}

	if response.StatusCode >= 400 {
		return "", time.Time{}, fmt.Errorf("token endpoint returned HTTP %d: %s",
			response.StatusCode, truncate(string(raw), 200))
	}

	token := ""
	for _, path := range p.tokenPaths {
		if found, ok := jsonPathString(result, path); ok {
			token = found
			break
		}
	}
	if token == "" {
		// Echo the response: the field is somewhere in there, and the caller
		// needs to see where in order to pass the right --tokenPath.
		return "", time.Time{}, fmt.Errorf("no token found at %s in the response: %s",
			strings.Join(p.tokenPaths, " or "), truncate(string(raw), 200))
	}

	return token, resolveExpiry(cfg, p, result, token, time.Now().UTC()), nil
}

// refreshClientCredentials re-runs the credential exchange for an expired
// token and updates it in place.
func refreshClientCredentials(token *OAuthToken) error {
	if token.Auth == nil {
		return fmt.Errorf("no auth configuration stored, run auth add again")
	}
	accessToken, expiresAt, err := fetchToken(token.Auth)
	if err != nil {
		return err
	}
	token.AccessToken = accessToken
	token.ExpiresAt = expiresAt.Format(time.RFC3339)
	return nil
}

// args: name tokenUrl preset clientId clientSecret scopes audience method
//
//	contentType credentialsIn header body tokenPath expiresPath expiresIn
//	authHeader verify tokenFile
func runAuthAdd(args []string) {
	if len(args) < 2 || args[0] == "" {
		fmt.Fprintf(os.Stderr, "Error: name is required\n")
		os.Exit(1)
	}

	name := args[0]

	at := func(index int) string {
		if len(args) > index {
			return args[index]
		}
		return ""
	}

	if strings.TrimSpace(at(1)) == "" {
		fmt.Fprintf(os.Stderr, "Error: tokenUrl is required\n")
		os.Exit(1)
	}

	presetName := strings.TrimSpace(at(2))
	if presetName == "" {
		presetName = "client-credentials"
	}
	if _, ok := presets[presetName]; !ok {
		fmt.Fprintf(os.Stderr, "Error: unknown preset %q (available: %s)\n",
			presetName, strings.Join(presetNames(), ", "))
		os.Exit(1)
	}

	credentialsIn := strings.TrimSpace(at(9))
	if credentialsIn != "" && credentialsIn != "body" && credentialsIn != "header" {
		fmt.Fprintf(os.Stderr, "Error: credentialsIn must be body or header\n")
		os.Exit(1)
	}

	contentType := strings.TrimSpace(at(8))
	if contentType != "" && contentType != "form" && contentType != "json" {
		fmt.Fprintf(os.Stderr, "Error: contentType must be form or json\n")
		os.Exit(1)
	}

	expiresIn := 0
	if raw := strings.TrimSpace(at(14)); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			fmt.Fprintf(os.Stderr, "Error: expiresIn must be a positive number of seconds\n")
			os.Exit(1)
		}
		expiresIn = parsed
	}

	config := &AuthConfig{
		Preset:        presetName,
		TokenURL:      strings.TrimSpace(at(1)),
		ClientID:      at(3),
		ClientSecret:  at(4),
		Scopes:        at(5),
		Audience:      at(6),
		Method:        strings.TrimSpace(at(7)),
		ContentType:   contentType,
		CredentialsIn: credentialsIn,
		Headers:       parseMultiValue(at(10)),
		Body:          at(11),
		TokenPath:     strings.TrimSpace(at(12)),
		ExpiresPath:   strings.TrimSpace(at(13)),
		ExpiresIn:     expiresIn,
		AuthHeader:    strings.TrimSpace(at(15)),
	}

	config.ClientSecretRef = strings.TrimSpace(at(16))
	if config.ClientSecret != "" && config.ClientSecretRef != "" {
		fmt.Fprintf(os.Stderr, "Error: use either clientSecret or clientSecretRef, not both\n")
		os.Exit(1)
	}

	if config.ClientSecretRef != "" {
		if err := validateSecretRef(config.ClientSecretRef); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}

	verify := at(17) != "false"
	tokenFile := at(18)
	configFile := at(19)
	configName := at(20)
	save := at(21) == "true"

	// Writing the definition to config only makes sense if it can be shared,
	// and an inline secret is exactly what makes it unshareable. Refusing is
	// better than silently writing the secret into a file destined for git.
	if save && config.ClientSecret != "" {
		fmt.Fprintf(os.Stderr, "Error: refusing to save an inline clientSecret to %s. Use --clientSecretRef so the file stays shareable\n", configFile)
		os.Exit(1)
	}

	path := resolveTokenFile(name, tokenFile)

	token := &OAuthToken{
		Grant:    grantClientCredentials,
		TokenURL: config.TokenURL,
		ClientID: config.ClientID,
		Scopes:   config.Scopes,
		Auth:     config,
	}

	// Fetching once at registration turns a typo in the URL or a bad secret
	// into an error here, instead of a puzzling 401 on the first real call.
	if verify {
		accessToken, expiresAt, err := fetchToken(config)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		token.AccessToken = accessToken
		token.ExpiresAt = expiresAt.Format(time.RFC3339)
	}

	if err := saveToken(path, token); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving auth configuration: %v\n", err)
		os.Exit(1)
	}

	if save {
		if err := saveAuthToConfig(configFile, configName, name, config); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		target := configFile
		if target == "" {
			target = "config.yaml"
		}
		if verify {
			fmt.Fprintf(os.Stderr, "Auth %q verified and saved to %s\n", name, target)
		} else {
			fmt.Fprintf(os.Stderr, "Auth %q saved to %s\n", name, target)
		}
		return
	}

	if verify {
		fmt.Fprintf(os.Stderr, "Auth %q verified and saved to %s\n", name, path)
	} else {
		fmt.Fprintf(os.Stderr, "Auth %q saved to %s\n", name, path)
	}
}
