package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const defaultOAuthDir = ".oauth"

type OAuthToken struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	AuthURL      string `json:"authUrl"`
	TokenURL     string `json:"tokenUrl"`
	Scopes       string `json:"scopes"`
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresAt    string `json:"expiresAt"`
	// Grant records how this token was obtained, which is what decides how to
	// renew it. Empty means the interactive authorization_code flow, so tokens
	// written by earlier versions keep refreshing exactly as they did.
	Grant string      `json:"grant,omitempty"`
	Auth  *AuthConfig `json:"auth,omitempty"`
	// ClientSecretRef names the client secret instead of holding it. The
	// interactive flow needs the secret again at refresh time, so storing the
	// reference rather than the resolved value is what keeps the secret out of
	// the token file entirely.
	ClientSecretRef string `json:"clientSecretRef,omitempty"`
	// SecretProvider and Name record where this token's secrets live. They sit
	// on the token rather than only on Auth because the interactive flow has no
	// auth configuration, and both flows need to find their secrets again.
	SecretProvider string `json:"secretProvider,omitempty"`
	Name           string `json:"name,omitempty"`
	// BrokerURL and Provider are set when Grant == grantBroker: the token was
	// obtained through an OAuth broker that holds the client secret, so renewal
	// goes back to the broker (POST {BrokerURL}/{Provider}/refresh) instead of to
	// the token endpoint with a secret this client never had.
	BrokerURL string `json:"brokerUrl,omitempty"`
	Provider  string `json:"provider,omitempty"`
}

// expandHome turns a leading ~ into the user's home directory. Paths are passed
// straight to os.ReadFile/os.Create, which do not expand it themselves.
func expandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(path, "~"), "/"))
		}
	}
	return path
}

// resolveTokenFile decides where a provider's token lives. An explicit path
// wins, then AUX4_CURL_OAUTH_HOME, then the default directory. Note the default
// is relative to the current directory, so a token saved in one directory is not
// visible from another — set AUX4_CURL_OAUTH_HOME or pass --tokenFile to get a
// single store for a machine.
func resolveTokenFile(provider string, tokenFile string) string {
	if tokenFile != "" {
		return expandHome(tokenFile)
	}
	if home := os.Getenv("AUX4_CURL_OAUTH_HOME"); home != "" {
		return filepath.Join(expandHome(home), provider+".json")
	}
	return filepath.Join(defaultOAuthDir, provider+".json")
}

func loadToken(path string) (*OAuthToken, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var token OAuthToken
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, err
	}

	// The access token was kept out of the file; fetch it back from wherever
	// curl put it. A missing one is not an error -- it simply means there is
	// nothing cached yet, which isExpired already treats as needing a fetch.
	if provider, name := token.secretProvider(); provider != "" && name != "" {
		for _, secret := range providerSecrets {
			if secret.get(&token) != "" {
				continue
			}
			if value, err := readProviderSecret(provider, name, secret.field); err == nil {
				secret.set(&token, value)
			}
		}
	}

	return &token, nil
}

func saveToken(path string, token *OAuthToken) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	// With a provider configured the access token is the one thing that must
	// not land in the file. Everything else -- the endpoint, the grant, the
	// deadline -- is not secret and stays here, which keeps the expiry check
	// and the cross-process lock working exactly as before.
	record := *token
	if provider, name := token.secretProvider(); provider != "" && name != "" {
		for _, secret := range providerSecrets {
			value := secret.get(&record)
			if value == "" {
				continue
			}
			if err := storeProviderSecret(provider, name, secret.field, value); err != nil {
				return err
			}
			secret.set(&record, "")
		}
	}

	data, err := json.MarshalIndent(&record, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// secretProvider reports where this token's secrets live, and the name they are
// filed under.
func (t *OAuthToken) secretProvider() (string, string) {
	if t.SecretProvider != "" {
		return t.SecretProvider, t.Name
	}
	if t.Auth == nil {
		return "", ""
	}
	return t.Auth.SecretProvider, t.Auth.Name
}

// providerSecrets are the fields curl hands to a provider rather than writing
// to disk. The refresh token is the most valuable of the three: it is
// long-lived and can mint new access tokens on its own.
var providerSecrets = []struct {
	field string
	get   func(*OAuthToken) string
	set   func(*OAuthToken, string)
}{
	{"accessToken", func(t *OAuthToken) string { return t.AccessToken }, func(t *OAuthToken, v string) { t.AccessToken = v }},
	{"refreshToken", func(t *OAuthToken) string { return t.RefreshToken }, func(t *OAuthToken, v string) { t.RefreshToken = v }},
	{"clientSecret", func(t *OAuthToken) string { return t.ClientSecret }, func(t *OAuthToken, v string) { t.ClientSecret = v }},
}

func (t *OAuthToken) isExpired() bool {
	// Nothing to present. This is the state `auth add --verify false` leaves
	// behind: the recipe is stored but no token has been fetched yet, so the
	// first use has to acquire one.
	if t.AccessToken == "" {
		return true
	}
	if t.ExpiresAt == "" {
		// A directly stored API key (set-token) has no deadline and no way to
		// renew. Only fetched tokens are ever given one.
		return false
	}
	expiresAt, err := time.Parse(time.RFC3339, t.ExpiresAt)
	if err != nil {
		return true
	}
	return time.Now().Add(expirySkew).After(expiresAt)
}

// reacquire renews an expired token using whichever grant produced it.
func (t *OAuthToken) reacquire() error {
	if t.Grant == grantClientCredentials {
		return refreshClientCredentials(t)
	}
	if t.Grant == grantBroker {
		return refreshViaBroker(t)
	}
	return refreshAccessToken(t)
}

// refreshViaBroker renews a broker-obtained token by POSTing the refresh token
// back to the broker's refresh route. The broker holds the client secret, so
// this client never needs it: it hands over the refresh token and gets a fresh
// access token (and possibly a rotated refresh token) in return.
func refreshViaBroker(token *OAuthToken) error {
	if token.RefreshToken == "" {
		return fmt.Errorf("no refresh token available, please login again")
	}
	if token.BrokerURL == "" || token.Provider == "" {
		return fmt.Errorf("broker token is missing its brokerUrl/provider, please login again")
	}

	url := strings.TrimRight(token.BrokerURL, "/") + "/" + token.Provider + "/refresh"
	payload, err := json.Marshal(map[string]string{"refreshToken": token.RefreshToken})
	if err != nil {
		return err
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("broker refresh request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("broker refresh failed: %s returned %d: %s", url, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse broker refresh response: %v", err)
	}

	at, _ := result["accessToken"].(string)
	if at == "" {
		return fmt.Errorf("broker refresh returned no accessToken: %s", strings.TrimSpace(string(body)))
	}
	token.AccessToken = at
	// A provider may or may not rotate the refresh token; the broker returns an
	// empty string when it does not, in which case the existing one is kept.
	if rt, ok := result["refreshToken"].(string); ok && rt != "" {
		token.RefreshToken = rt
	}
	if expiresIn, ok := result["expiresIn"].(float64); ok && expiresIn > 0 {
		token.ExpiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second).Format(time.RFC3339)
	}
	return nil
}

// authHeaderFor renders the outbound header carrying this token. The template
// lives with the auth config, which is what lets a non-bearer API
// (X-API-Key: ${token}) reuse all of the same machinery.
func authHeaderFor(t *OAuthToken) string {
	template := defaultAuthHeader
	if t.Auth != nil {
		template = t.Auth.authHeaderTemplate()
	}
	return applyAuthHeader(template, t.AccessToken)
}

func refreshAccessToken(token *OAuthToken) error {
	if token.RefreshToken == "" {
		return fmt.Errorf("no refresh token available, please login again")
	}

	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {token.RefreshToken},
		"client_id":     {token.ClientID},
	}
	clientSecret := token.ClientSecret
	if clientSecret == "" && token.ClientSecretRef != "" {
		resolved, err := resolveSecretRef(token.ClientSecretRef)
		if err != nil {
			return err
		}
		clientSecret = resolved
	}
	if clientSecret != "" {
		data.Set("client_secret", clientSecret)
	}

	resp, err := http.PostForm(token.TokenURL, data)
	if err != nil {
		return fmt.Errorf("refresh request failed: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse refresh response: %v", err)
	}

	if errMsg, ok := result["error"]; ok {
		if desc, ok := result["error_description"]; ok {
			return fmt.Errorf("refresh failed: %v (%v)", errMsg, desc)
		}
		return fmt.Errorf("refresh failed: %v", errMsg)
	}

	if at, ok := result["access_token"].(string); ok {
		token.AccessToken = at
	}
	if rt, ok := result["refresh_token"].(string); ok {
		token.RefreshToken = rt
	}
	if expiresIn, ok := result["expires_in"].(float64); ok {
		token.ExpiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second).Format(time.RFC3339)
	}

	return nil
}

func ensureValidToken(provider string, tokenFile string, definition *AuthConfig) (*OAuthToken, string, error) {
	path := resolveTokenFile(provider, tokenFile)
	token, err := loadToken(path)
	if err != nil {
		if definition == nil {
			return nil, path, fmt.Errorf("no token found for provider %q. Run oauth login or auth add first", provider)
		}
		// Nothing stored yet, but the shared configuration describes how to get
		// one. This is what lets a fresh checkout authenticate with no setup
		// step: the definition travels in config, the token is fetched on use.
		token = tokenFromConfig(definition)
	} else if definition != nil {
		// The configuration is the source of truth for *how* to obtain a token,
		// so a changed endpoint or scope in the shared file takes effect at
		// once. Only the cached token itself is kept from local state.
		token.Grant = grantClientCredentials
		token.Auth = definition
	}
	if token.Auth != nil && token.Auth.Name == "" {
		token.Auth.Name = provider
	}

	if !token.isExpired() {
		return token, path, nil
	}

	var refreshErr error
	lockErr := withTokenLock(path, func() error {
		// Re-read inside the lock: another process may have refreshed while we
		// were waiting for it, and its token is then the one to use.
		if current, loadErr := loadToken(path); loadErr == nil {
			if definition != nil {
				current.Grant = grantClientCredentials
				current.Auth = definition
			}
			token = current
			if !token.isExpired() {
				return nil
			}
		}

		if err := token.reacquire(); err != nil {
			refreshErr = err
			return nil
		}
		if err := saveToken(path, token); err != nil {
			refreshErr = fmt.Errorf("error saving refreshed token: %v", err)
		}
		return nil
	})
	if lockErr != nil {
		return nil, path, lockErr
	}
	if refreshErr != nil {
		return nil, path, refreshErr
	}

	return token, path, nil
}

// reacquireToken forces a new token regardless of the cached deadline. It backs
// the 401 retry: the skew handles clock drift, but a token revoked or
// invalidated server-side is only ever detectable from the API's own answer.
//
// usedToken is the token that just failed. If the stored token has since moved
// on, another request already renewed it and that one is returned instead --
// which is what stops a batch of concurrent 401s from stampeding the endpoint.
func reacquireToken(provider string, tokenFile string, usedToken string, definition *AuthConfig) (*OAuthToken, error) {
	path := resolveTokenFile(provider, tokenFile)

	var result *OAuthToken
	var opErr error

	lockErr := withTokenLock(path, func() error {
		token, err := loadToken(path)
		if err != nil {
			if definition == nil {
				opErr = fmt.Errorf("no token found for provider %q", provider)
				return nil
			}
			token = tokenFromConfig(definition)
		} else if definition != nil {
			token.Grant = grantClientCredentials
			token.Auth = definition
		}

		if token.AccessToken != usedToken && !token.isExpired() {
			result = token
			return nil
		}

		if err := token.reacquire(); err != nil {
			opErr = err
			return nil
		}
		if err := saveToken(path, token); err != nil {
			opErr = fmt.Errorf("error saving refreshed token: %v", err)
			return nil
		}
		result = token
		return nil
	})
	if lockErr != nil {
		return nil, lockErr
	}
	if opErr != nil {
		return nil, opErr
	}
	return result, nil
}

func generateCodeVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func generateCodeChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// args: provider clientId clientSecret authUrl tokenUrl scopes callbackPort tokenFile clientSecretRef secretProvider broker brokerToken
func runOAuthLogin(args []string) {
	if len(args) < 6 {
		fmt.Fprintf(os.Stderr, "Error: provider, clientId, authUrl, tokenUrl, and scopes are required\n")
		os.Exit(1)
	}

	provider := args[0]
	clientID := args[1]
	clientSecret := args[2]
	authURL := args[3]
	tokenURL := args[4]
	scopes := args[5]

	callbackPort := "9876"
	if len(args) > 6 && args[6] != "" {
		callbackPort = args[6]
	}

	tokenFile := ""
	if len(args) > 7 && args[7] != "" {
		tokenFile = args[7]
	}

	// The interactive flow gets the same treatment as client credentials: the
	// secret may be named instead of inlined, so an oauth definition can live
	// in a shared config file too.
	secretProvider := ""
	if len(args) > 9 {
		secretProvider = strings.TrimSpace(args[9])
	}

	clientSecretRef := ""
	if clientSecret == "" && len(args) > 8 && strings.TrimSpace(args[8]) != "" {
		clientSecretRef = strings.TrimSpace(args[8])
		if err := validateSecretRef(clientSecretRef); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		resolved, err := resolveSecretRef(clientSecretRef)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		clientSecret = resolved
	}

	path := resolveTokenFile(provider, tokenFile)
	redirectURI := fmt.Sprintf("http://localhost:%s/callback", callbackPort)

	// Broker mode: an OAuth broker (arg 10) holds the client secret, so the
	// authorize-url and code exchange happen there. The loopback is identical —
	// only the two crypto steps move server-side, and this client never sees a
	// secret. clientId/authUrl/tokenUrl are unused in this mode.
	if len(args) > 10 && strings.TrimSpace(args[10]) != "" {
		brokerToken := ""
		if len(args) > 11 {
			brokerToken = strings.TrimSpace(args[11])
		}
		brokerLogin(provider, strings.TrimSpace(args[10]), scopes, brokerToken, path)
		return
	}

	state := fmt.Sprintf("%d", time.Now().UnixNano())

	codeVerifier, err := generateCodeVerifier()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating PKCE verifier: %v\n", err)
		os.Exit(1)
	}
	codeChallenge := generateCodeChallenge(codeVerifier)

	authParams := url.Values{
		"client_id":             {clientID},
		"redirect_uri":          {redirectURI},
		"response_type":         {"code"},
		"scope":                 {scopes},
		"state":                 {state},
		"code_challenge":        {codeChallenge},
		"code_challenge_method": {"S256"},
	}

	// The caller may pass extra provider-specific parameters on authUrl itself
	// (Google, for example, needs access_type=offline to return a refresh
	// token), so join with & when a query string is already present.
	separator := "?"
	if strings.Contains(authURL, "?") {
		separator = "&"
	}
	fullAuthURL := authURL + separator + authParams.Encode()
	fmt.Fprintf(os.Stderr, "Open this URL in your browser to authorize:\n\n%s\n\n", fullAuthURL)
	fmt.Fprintf(os.Stderr, "Waiting for callback on port %s...\n", callbackPort)

	code, err := waitForCallbackCode(callbackPort, state)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Exchange code for tokens
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {clientID},
		"code_verifier": {codeVerifier},
	}
	if clientSecret != "" {
		data.Set("client_secret", clientSecret)
	}

	resp, err := http.PostForm(tokenURL, data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error exchanging code: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing token response: %v\n", err)
		os.Exit(1)
	}

	if errMsg, ok := result["error"]; ok {
		// error alone is rarely actionable; error_description is where the
		// provider explains what was actually wrong.
		fmt.Fprintf(os.Stderr, "Error: %v\n", errMsg)
		if desc, ok := result["error_description"]; ok {
			fmt.Fprintf(os.Stderr, "Details: %v\n", desc)
		}
		if uri, ok := result["error_uri"]; ok {
			fmt.Fprintf(os.Stderr, "More: %v\n", uri)
		}
		os.Exit(1)
	}

	token := &OAuthToken{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		AuthURL:      authURL,
		TokenURL:     tokenURL,
		Scopes:       scopes,
	}

	// With a provider named, the access token, the refresh token and the client
	// secret all go there instead of into the file. saveToken does the moving;
	// this only records where.
	if secretProvider != "" {
		token.SecretProvider = secretProvider
		token.Name = provider
	}

	// When the secret came from a reference, keep the reference and drop the
	// value. Refresh resolves it again, so nothing is lost and the token file
	// never holds the secret.
	if clientSecretRef != "" {
		token.ClientSecret = ""
		token.ClientSecretRef = clientSecretRef
	}

	if at, ok := result["access_token"].(string); ok {
		token.AccessToken = at
	}
	if rt, ok := result["refresh_token"].(string); ok {
		token.RefreshToken = rt
	}
	if expiresIn, ok := result["expires_in"].(float64); ok {
		token.ExpiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second).Format(time.RFC3339)
	}

	if err := saveToken(path, token); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving token: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Login successful! Token saved to %s\n", path)
}

// waitForCallbackCode runs the loopback OAuth redirect listener: it serves the
// callback endpoint on the given port, validates the returned state, and returns
// the authorization code. Shared by the local and broker login flows.
func waitForCallbackCode(callbackPort, state string) (string, error) {
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			errCh <- fmt.Errorf("state mismatch")
			http.Error(w, "State mismatch", http.StatusBadRequest)
			return
		}
		if errParam := r.URL.Query().Get("error"); errParam != "" {
			errCh <- fmt.Errorf("authorization error: %s", errParam)
			fmt.Fprintf(w, "Authorization failed: %s. You can close this window.", errParam)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			errCh <- fmt.Errorf("no code in callback")
			http.Error(w, "No code received", http.StatusBadRequest)
			return
		}
		fmt.Fprintf(w, "Authorization successful! You can close this window.")
		codeCh <- code
	})

	listener, err := net.Listen("tcp", ":"+callbackPort)
	if err != nil {
		return "", fmt.Errorf("could not listen on port %s: %v", callbackPort, err)
	}

	server := &http.Server{Handler: mux}
	go server.Serve(listener)
	defer server.Shutdown(context.Background())

	select {
	case code := <-codeCh:
		return code, nil
	case err := <-errCh:
		return "", err
	case <-time.After(5 * time.Minute):
		return "", fmt.Errorf("timed out waiting for callback")
	}
}

// brokerLogin performs the loopback authorization-code flow against an OAuth
// broker that holds the client secret. It asks the broker to build the
// authorization URL (which returns the PKCE verifier), runs the same local
// callback listener, then hands the code back to the broker to exchange for
// tokens. The stored token records a grantBroker grant + the broker URL, so
// future refreshes route back to the broker too — the client never sees a secret.
// brokerGet / brokerPost issue the broker HTTP calls, attaching the aux4 identity
// token as a Bearer header when one is provided. A secured broker (the default:
// OAUTH_APP_PUBLIC unset on the VM) requires it; a public broker ignores it.
func brokerGet(rawurl, token string) (*http.Response, error) {
	req, err := http.NewRequest("GET", rawurl, nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return http.DefaultClient.Do(req)
}

func brokerPost(rawurl, token, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest("POST", rawurl, body)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", contentType)
	return http.DefaultClient.Do(req)
}

func brokerLogin(provider, brokerURL, scopes, brokerToken, path string) {
	base := strings.TrimRight(brokerURL, "/")

	// The session id is a high-entropy value only this client knows; it is both
	// the OAuth state and the poll key. The parked code cannot be redeemed without
	// the PKCE verifier held here, so even a guessed session id yields nothing.
	session, err := generateCodeVerifier()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating session id: %v\n", err)
		os.Exit(1)
	}

	// 1. Ask the broker to build the authorization URL. It holds the client id,
	// sets its OWN public callback as the redirect (so no loopback is needed), and
	// returns the PKCE code verifier we must keep for the exchange.
	authQuery := url.Values{
		"scopes": {scopes},
		"state":  {session},
	}
	authResp, err := brokerGet(base+"/"+provider+"/authorize-url?"+authQuery.Encode(), brokerToken)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: broker authorize-url request failed: %v\n", err)
		os.Exit(1)
	}
	authBody, _ := io.ReadAll(authResp.Body)
	authResp.Body.Close()
	if authResp.StatusCode == 401 || authResp.StatusCode == 403 {
		fmt.Fprintf(os.Stderr, "Error: the broker at %s requires authentication (HTTP %d) but no valid broker token was supplied. Pass --brokerToken (the calling package normally supplies this — e.g. your aux4 identity for an aux4 broker).\n", base, authResp.StatusCode)
		os.Exit(1)
	}
	if authResp.StatusCode < 200 || authResp.StatusCode >= 300 {
		fmt.Fprintf(os.Stderr, "Error: broker authorize-url returned %d: %s\n", authResp.StatusCode, strings.TrimSpace(string(authBody)))
		os.Exit(1)
	}
	var authResult struct {
		URL          string `json:"url"`
		CodeVerifier string `json:"codeVerifier"`
		State        string `json:"state"`
	}
	if err := json.Unmarshal(authBody, &authResult); err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not parse broker authorize-url response: %v\n", err)
		os.Exit(1)
	}
	if authResult.URL == "" || authResult.CodeVerifier == "" {
		fmt.Fprintf(os.Stderr, "Error: broker authorize-url response missing url/codeVerifier\n")
		os.Exit(1)
	}

	// 2. Send the user through consent on ANY device. The provider redirects the
	// browser to the broker's own callback (not a loopback), which parks the code;
	// we poll the broker for it. This is what makes phone/second-device approval work.
	fmt.Fprintf(os.Stderr, "Open this URL in your browser to authorize:\n\n%s\n\n", authResult.URL)
	fmt.Fprintf(os.Stderr, "Waiting for authorization...\n")
	code := ""
	deadline := time.Now().Add(10 * time.Minute)
	for time.Now().Before(deadline) {
		time.Sleep(2 * time.Second)
		pollResp, perr := brokerGet(base+"/session/"+session, brokerToken)
		if perr != nil {
			continue
		}
		pollBody, _ := io.ReadAll(pollResp.Body)
		pollResp.Body.Close()
		if pollResp.StatusCode < 200 || pollResp.StatusCode >= 300 {
			continue
		}
		var poll struct {
			Status string `json:"status"`
			Code   string `json:"code"`
			Error  string `json:"error"`
		}
		if err := json.Unmarshal(pollBody, &poll); err != nil {
			continue
		}
		switch poll.Status {
		case "ready":
			code = poll.Code
		case "error":
			fmt.Fprintf(os.Stderr, "Error: authorization failed: %s\n", poll.Error)
			os.Exit(1)
		case "expired":
			fmt.Fprintf(os.Stderr, "Error: login session expired, please try again\n")
			os.Exit(1)
		}
		if code != "" {
			break
		}
	}
	if code == "" {
		fmt.Fprintf(os.Stderr, "Error: timed out waiting for authorization\n")
		os.Exit(1)
	}

	// 3. Hand the code to the broker to exchange for tokens (it holds the secret).
	// The redirect URI is the broker's own callback, so this client never sends one.
	exchangePayload, _ := json.Marshal(map[string]string{
		"code":         code,
		"codeVerifier": authResult.CodeVerifier,
	})
	exResp, err := brokerPost(base+"/"+provider+"/exchange", brokerToken, "application/json", bytes.NewReader(exchangePayload))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: broker exchange request failed: %v\n", err)
		os.Exit(1)
	}
	exBody, _ := io.ReadAll(exResp.Body)
	exResp.Body.Close()
	if exResp.StatusCode < 200 || exResp.StatusCode >= 300 {
		fmt.Fprintf(os.Stderr, "Error: broker exchange returned %d: %s\n", exResp.StatusCode, strings.TrimSpace(string(exBody)))
		os.Exit(1)
	}
	var tokens struct {
		AccessToken  string  `json:"accessToken"`
		RefreshToken string  `json:"refreshToken"`
		ExpiresIn    float64 `json:"expiresIn"`
	}
	if err := json.Unmarshal(exBody, &tokens); err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not parse broker exchange response: %v\n", err)
		os.Exit(1)
	}
	if tokens.AccessToken == "" {
		fmt.Fprintf(os.Stderr, "Error: broker exchange returned no accessToken: %s\n", strings.TrimSpace(string(exBody)))
		os.Exit(1)
	}

	token := &OAuthToken{
		Grant:        grantBroker,
		BrokerURL:    base,
		Provider:     provider,
		Scopes:       scopes,
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
	}
	if tokens.ExpiresIn > 0 {
		token.ExpiresAt = time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second).Format(time.RFC3339)
	}

	if err := saveToken(path, token); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving token: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "Login successful! Token saved to %s\n", path)
}

// configuredAuth reads the auth definitions that came from config.yaml and
// picks the one matching this provider. aux4 delivers an object-valued
// parameter as a JSON string, so nothing has to be parsed but JSON. A missing
// or empty value simply means "not configured", and the stored token is used.
func configuredAuth(args []string, index int, provider string) (*AuthConfig, error) {
	if len(args) <= index {
		return nil, nil
	}
	return authFromConfig(args[index], provider)
}

// args: provider tokenFile auth
func runOAuthToken(args []string) {
	if len(args) < 1 || args[0] == "" {
		fmt.Fprintf(os.Stderr, "Error: provider is required\n")
		os.Exit(1)
	}

	provider := args[0]
	tokenFile := ""
	if len(args) > 1 && args[1] != "" {
		tokenFile = args[1]
	}

	definition, err := configuredAuth(args, 2, provider)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	token, _, err := ensureValidToken(provider, tokenFile, definition)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprint(os.Stdout, token.AccessToken)
}

// args: provider tokenFile
func runOAuthStatus(args []string) {
	if len(args) < 1 || args[0] == "" {
		fmt.Fprintf(os.Stderr, "Error: provider is required\n")
		os.Exit(1)
	}

	provider := args[0]
	tokenFile := ""
	if len(args) > 1 && args[1] != "" {
		tokenFile = args[1]
	}

	path := resolveTokenFile(provider, tokenFile)
	token, err := loadToken(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "No token found for provider %q\n", provider)
		os.Exit(1)
	}

	status := "valid"
	if token.isExpired() {
		status = "expired"
	}

	hasRefresh := "no"
	if token.RefreshToken != "" {
		hasRefresh = "yes"
	}

	scopes := token.Scopes
	if scopes == "" {
		scopes = "(none)"
	}

	grant := token.Grant
	if grant == "" {
		grant = "authorization_code"
	}

	// "Refresh token: no" alone would be misleading for a client-credentials
	// token: it has no refresh token and never needs one, because it renews by
	// replaying the credential exchange.
	canRenew := "no"
	if token.Grant == grantClientCredentials {
		if token.Auth != nil && token.Auth.TokenURL != "" {
			canRenew = "yes"
		}
	} else if token.RefreshToken != "" {
		canRenew = "yes"
	}

	preset := ""
	if token.Auth != nil && token.Auth.Preset != "" {
		preset = token.Auth.Preset
	}

	fmt.Fprintf(os.Stdout, "Provider:      %s\n", provider)
	fmt.Fprintf(os.Stdout, "Status:        %s\n", status)
	fmt.Fprintf(os.Stdout, "Scopes:        %s\n", scopes)
	fmt.Fprintf(os.Stdout, "Grant:         %s\n", grant)
	if preset != "" {
		fmt.Fprintf(os.Stdout, "Preset:        %s\n", preset)
	}
	fmt.Fprintf(os.Stdout, "Expires at:    %s\n", token.ExpiresAt)
	fmt.Fprintf(os.Stdout, "Refresh token: %s\n", hasRefresh)
	fmt.Fprintf(os.Stdout, "Can renew:     %s\n", canRenew)
	fmt.Fprintf(os.Stdout, "Token file:    %s\n", path)
}

// args: provider tokenFile
func runOAuthLogout(args []string) {
	if len(args) < 1 || args[0] == "" {
		fmt.Fprintf(os.Stderr, "Error: provider is required\n")
		os.Exit(1)
	}

	provider := args[0]
	tokenFile := ""
	if len(args) > 1 && args[1] != "" {
		tokenFile = args[1]
	}

	path := resolveTokenFile(provider, tokenFile)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "No token found for provider %q\n", provider)
		} else {
			fmt.Fprintf(os.Stderr, "Error removing token: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Remove parent dir if empty
	dir := filepath.Dir(path)
	entries, _ := os.ReadDir(dir)
	if len(entries) == 0 {
		os.Remove(dir)
	}

	fmt.Fprintf(os.Stderr, "Logged out from %s\n", provider)
}

// args: provider accessToken tokenFile
func runOAuthSetToken(args []string) {
	if len(args) < 2 || args[0] == "" || args[1] == "" {
		fmt.Fprintf(os.Stderr, "Error: provider and accessToken are required\n")
		os.Exit(1)
	}

	provider := args[0]
	accessToken := args[1]

	tokenFile := ""
	if len(args) > 2 && args[2] != "" {
		tokenFile = args[2]
	}

	path := resolveTokenFile(provider, tokenFile)

	token := &OAuthToken{
		AccessToken: accessToken,
	}

	if err := saveToken(path, token); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving token: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Token saved to %s\n", path)
}

// args: provider tokenFile method url header body showHeaders
func runAuthRequest(args []string) {
	if len(args) < 4 {
		fmt.Fprintf(os.Stderr, "Error: provider, tokenFile, method, and url are required\n")
		os.Exit(1)
	}

	provider := args[0]
	tokenFile := args[1]

	// The configured definition, when present, arrives after the request
	// arguments so the existing positions are untouched.
	definition, err := configuredAuth(args, 11, provider)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	token, _, err := ensureValidToken(provider, tokenFile, definition)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// args[2:] = method, url, header, body, showHeaders
	requestArgs := args[2:]

	// Headers may arrive as a JSON array (repeatable variable); normalize to a
	// newline-separated list so the auth header can simply be appended.
	var headerList []string
	if len(requestArgs) > 2 {
		headerList = parseMultiValue(requestArgs[2])
	}
	baseHeaders := append([]string{}, headerList...)
	headerList = append(headerList, authHeaderFor(token))
	existingHeaders := strings.Join(headerList, "\n")

	// One retry with a fresh token if the API rejects this one. The skew covers
	// clock drift; a token revoked or invalidated server-side is only ever
	// visible from the API's own answer.
	usedToken := token.AccessToken
	reauth := func() ([]string, error) {
		refreshed, err := reacquireToken(provider, tokenFile, usedToken, definition)
		if err != nil {
			return nil, err
		}
		usedToken = refreshed.AccessToken
		return append(append([]string{}, baseHeaders...), authHeaderFor(refreshed)), nil
	}

	// Rebuild args for runRequest: method url header body showHeaders upload uploadField output bodyFile
	newArgs := make([]string, 9)
	newArgs[0] = requestArgs[0] // method
	newArgs[1] = requestArgs[1] // url
	newArgs[2] = existingHeaders
	if len(requestArgs) > 3 {
		newArgs[3] = requestArgs[3] // body
	}
	if len(requestArgs) > 4 {
		newArgs[4] = requestArgs[4] // showHeaders
	}
	if len(requestArgs) > 5 {
		newArgs[5] = requestArgs[5] // upload
	}
	if len(requestArgs) > 6 {
		newArgs[6] = requestArgs[6] // uploadField
	}
	if len(requestArgs) > 7 {
		newArgs[7] = requestArgs[7] // output
	}
	if len(requestArgs) > 8 {
		newArgs[8] = requestArgs[8] // bodyFile
	}

	runRequestWithAuth(newArgs, reauth)
}

// args: provider tokenFile method url header concurrency
func runAuthStream(args []string) {
	if len(args) < 4 {
		fmt.Fprintf(os.Stderr, "Error: provider, tokenFile, method, and url are required\n")
		os.Exit(1)
	}

	provider := args[0]
	tokenFile := args[1]

	definition, err := configuredAuth(args, 6, provider)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	token, _, err := ensureValidToken(provider, tokenFile, definition)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// args[2:] = method, url, header, concurrency
	requestArgs := args[2:]

	// Headers may arrive as a JSON array (repeatable variable); normalize to a
	// newline-separated list so the auth header can simply be appended.
	var headerList []string
	if len(requestArgs) > 2 {
		headerList = parseMultiValue(requestArgs[2])
	}
	auth := &streamAuth{
		provider:   provider,
		tokenFile:  tokenFile,
		definition: definition,
		header:     authHeaderFor(token),
		token:      token.AccessToken,
	}
	existingHeaders := strings.Join(headerList, "\n")

	newArgs := make([]string, 4)
	newArgs[0] = requestArgs[0] // method
	newArgs[1] = requestArgs[1] // url
	newArgs[2] = existingHeaders
	if len(requestArgs) > 3 {
		newArgs[3] = requestArgs[3] // concurrency
	}

	runStreamWithAuth(newArgs, auth)
}
