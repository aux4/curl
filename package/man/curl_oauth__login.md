#### Description

The `login` command authenticates with an OAuth2 provider using the authorization code flow with PKCE (Proof Key for Code Exchange). It starts a temporary local HTTP server to receive the callback, then prints the authorization URL for the user to open in their browser.

PKCE is automatically enabled for all providers. A random code verifier and S256 code challenge are generated per login attempt. This is required by providers like X (Twitter) and recommended by the OAuth 2.0 Security Best Current Practice for all authorization code flows.

After the user authorizes the application, the provider redirects to the local server with an authorization code. The command exchanges this code (along with the PKCE code verifier) for access and refresh tokens, and saves them to `.oauth/<provider>.json` (or the path specified by `--tokenFile`).

The callback server times out after 5 minutes if no authorization is received.

If the token exchange fails, the provider's `error_description` is printed alongside the error code, so the reason (for example a missing client secret or an invalid grant) is visible rather than just a bare `invalid_request`.

Some providers require extra parameters on the authorization request. Include them directly on `--authUrl` and they are preserved — the standard OAuth parameters are appended with `&` when the URL already has a query string. Google, for example, only returns a refresh token when `access_type=offline` is present:

```bash
--authUrl "https://accounts.google.com/o/oauth2/v2/auth?access_type=offline&prompt=consent"
```

**Keeping the client secret out of the token file**

`--clientSecretRef` names the client secret instead of taking it on the command line, in the form `<provider>/<vault>/<item>/<field>`. It is resolved through `aux4/secret` for the initial exchange, and the **reference** rather than the secret is written to the token file. Refresh resolves it again, so the secret is never stored at rest and never appears in shell history or in `ps`.

It is deliberately not written as a `secret://` URI: aux4 resolves a parameter whose whole value is one before the command runs, so a URI would arrive here already resolved and be stored as though it were a reference.

**Letting curl hold the secrets**

`--secretProvider` stores the access token, the refresh token and the client secret in a secret provider rather than in the token file, at paths curl derives from the provider name. The refresh token is the important one: it is long-lived and can obtain new access tokens by itself, so it is the last thing that should sit in a plain file. A rotated refresh token is written back to the provider on each renewal.

The token file keeps the endpoints, scopes and expiry, none of which are secret. Without `--secretProvider` behaviour is unchanged.

#### Usage

```bash
aux4 curl oauth login <provider> --clientId <id> --clientSecret <secret> --authUrl <url> --tokenUrl <url> --scopes <scopes> [--callbackPort <port>] [--tokenFile <path>]
```

--clientId       OAuth client ID from the provider's developer console
--clientSecret   OAuth client secret from the provider's developer console
--authUrl        The provider's authorization endpoint URL, optionally with extra query parameters
--tokenUrl       The provider's token exchange endpoint URL
--scopes         Comma-separated list of scopes to request
--callbackPort   Port for the local callback server (default: 9876)
--tokenFile      Custom path to store the token file (default: .oauth/<provider>.json)

#### Example

```bash
aux4 curl oauth login pinterest \
  --clientId abc123 \
  --clientSecret mysecret \
  --authUrl https://www.pinterest.com/oauth/ \
  --tokenUrl https://api.pinterest.com/v5/oauth/token \
  --scopes boards:read,pins:read,pins:write
```

```text
Open this URL in your browser to authorize:

https://www.pinterest.com/oauth/?client_id=abc123&redirect_uri=http://localhost:9876/callback&response_type=code&scope=boards:read,pins:read,pins:write&state=1234567890

Waiting for callback on port 9876...
Login successful! Token saved to .oauth/pinterest.json
```

Taking the client secret from a secret provider instead of the command line:

```bash
aux4 curl oauth login pinterest \
  --clientId abc123 \
  --clientSecretRef 'aux4/Work/Pinterest/clientSecret' \
  --authUrl https://www.pinterest.com/oauth/ \
  --tokenUrl https://api.pinterest.com/v5/oauth/token \
  --scopes boards:read,pins:read
```
