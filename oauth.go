package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
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
}

func resolveTokenFile(provider string, tokenFile string) string {
	if tokenFile != "" {
		return tokenFile
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
	return &token, nil
}

func saveToken(path string, token *OAuthToken) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func (t *OAuthToken) isExpired() bool {
	if t.ExpiresAt == "" {
		return false
	}
	expiresAt, err := time.Parse(time.RFC3339, t.ExpiresAt)
	if err != nil {
		return true
	}
	return time.Now().After(expiresAt)
}

func refreshAccessToken(token *OAuthToken) error {
	if token.RefreshToken == "" {
		return fmt.Errorf("no refresh token available, please login again")
	}

	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {token.RefreshToken},
		"client_id":     {token.ClientID},
		"client_secret": {token.ClientSecret},
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

func ensureValidToken(provider string, tokenFile string) (*OAuthToken, string, error) {
	path := resolveTokenFile(provider, tokenFile)
	token, err := loadToken(path)
	if err != nil {
		return nil, path, fmt.Errorf("no token found for provider %q. Run oauth login first", provider)
	}

	if token.isExpired() {
		if err := refreshAccessToken(token); err != nil {
			return nil, path, err
		}
		if err := saveToken(path, token); err != nil {
			return nil, path, fmt.Errorf("error saving refreshed token: %v", err)
		}
	}

	return token, path, nil
}

// args: provider clientId clientSecret authUrl tokenUrl scopes callbackPort tokenFile
func runOAuthLogin(args []string) {
	if len(args) < 6 {
		fmt.Fprintf(os.Stderr, "Error: provider, clientId, clientSecret, authUrl, tokenUrl, and scopes are required\n")
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

	path := resolveTokenFile(provider, tokenFile)
	redirectURI := fmt.Sprintf("http://localhost:%s/callback", callbackPort)

	state := fmt.Sprintf("%d", time.Now().UnixNano())

	authParams := url.Values{
		"client_id":     {clientID},
		"redirect_uri":  {redirectURI},
		"response_type": {"code"},
		"scope":         {scopes},
		"state":         {state},
	}

	fullAuthURL := authURL + "?" + authParams.Encode()
	fmt.Fprintf(os.Stderr, "Open this URL in your browser to authorize:\n\n%s\n\n", fullAuthURL)
	fmt.Fprintf(os.Stderr, "Waiting for callback on port %s...\n", callbackPort)

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
		fmt.Fprintf(os.Stderr, "Error: could not listen on port %s: %v\n", callbackPort, err)
		os.Exit(1)
	}

	server := &http.Server{Handler: mux}
	go server.Serve(listener)

	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		server.Shutdown(context.Background())
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	case <-time.After(5 * time.Minute):
		server.Shutdown(context.Background())
		fmt.Fprintf(os.Stderr, "Error: timed out waiting for callback\n")
		os.Exit(1)
	}

	server.Shutdown(context.Background())

	// Exchange code for tokens
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
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
		fmt.Fprintf(os.Stderr, "Error: %v\n", errMsg)
		os.Exit(1)
	}

	token := &OAuthToken{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		AuthURL:      authURL,
		TokenURL:     tokenURL,
		Scopes:       scopes,
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

// args: provider tokenFile
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

	token, _, err := ensureValidToken(provider, tokenFile)
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

	fmt.Fprintf(os.Stdout, "Provider:      %s\n", provider)
	fmt.Fprintf(os.Stdout, "Status:        %s\n", status)
	fmt.Fprintf(os.Stdout, "Scopes:        %s\n", scopes)
	fmt.Fprintf(os.Stdout, "Expires at:    %s\n", token.ExpiresAt)
	fmt.Fprintf(os.Stdout, "Refresh token: %s\n", hasRefresh)
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

	token, _, err := ensureValidToken(provider, tokenFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// args[2:] = method, url, header, body, showHeaders
	requestArgs := args[2:]

	existingHeaders := ""
	if len(requestArgs) > 2 {
		existingHeaders = requestArgs[2]
	}

	authHeader := "Authorization: Bearer " + token.AccessToken
	if existingHeaders != "" {
		existingHeaders = existingHeaders + "\n" + authHeader
	} else {
		existingHeaders = authHeader
	}

	// Rebuild args for runRequest: method url header body showHeaders
	newArgs := make([]string, 5)
	newArgs[0] = requestArgs[0] // method
	newArgs[1] = requestArgs[1] // url
	newArgs[2] = existingHeaders
	if len(requestArgs) > 3 {
		newArgs[3] = requestArgs[3] // body
	}
	if len(requestArgs) > 4 {
		newArgs[4] = requestArgs[4] // showHeaders
	}

	runRequest(newArgs)
}

// args: provider tokenFile method url header concurrency
func runAuthStream(args []string) {
	if len(args) < 4 {
		fmt.Fprintf(os.Stderr, "Error: provider, tokenFile, method, and url are required\n")
		os.Exit(1)
	}

	provider := args[0]
	tokenFile := args[1]

	token, _, err := ensureValidToken(provider, tokenFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// args[2:] = method, url, header, concurrency
	requestArgs := args[2:]

	existingHeaders := ""
	if len(requestArgs) > 2 {
		existingHeaders = requestArgs[2]
	}

	authHeader := "Authorization: Bearer " + token.AccessToken
	if existingHeaders != "" {
		existingHeaders = existingHeaders + "\n" + authHeader
	} else {
		existingHeaders = authHeader
	}

	newArgs := make([]string, 4)
	newArgs[0] = requestArgs[0] // method
	newArgs[1] = requestArgs[1] // url
	newArgs[2] = existingHeaders
	if len(requestArgs) > 3 {
		newArgs[3] = requestArgs[3] // concurrency
	}

	runStream(newArgs)
}
