#### Description

The `login` command authenticates with an OAuth2 provider using the authorization code flow. It starts a temporary local HTTP server to receive the callback, then prints the authorization URL for the user to open in their browser.

After the user authorizes the application, the provider redirects to the local server with an authorization code. The command exchanges this code for access and refresh tokens, and saves them to `.oauth/<provider>.json` (or the path specified by `--tokenFile`).

The callback server times out after 5 minutes if no authorization is received.

#### Usage

```bash
aux4 curl oauth login <provider> --clientId <id> --clientSecret <secret> --authUrl <url> --tokenUrl <url> --scopes <scopes> [--callbackPort <port>] [--tokenFile <path>]
```

--clientId       OAuth client ID from the provider's developer console
--clientSecret   OAuth client secret from the provider's developer console
--authUrl        The provider's authorization endpoint URL
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
