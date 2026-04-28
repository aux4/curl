#### Description

The `oauth` command group manages OAuth2 authentication for HTTP requests. It supports the standard authorization code flow with a local callback server to receive the authorization code from the browser.

Tokens are stored locally in `.oauth/<provider>.json` by default, or in a custom path specified with `--tokenFile`. Add `.oauth/` to your `.gitignore` to avoid committing credentials.

Available subcommands:

- `login` — Authenticate with an OAuth2 provider
- `token` — Get a valid access token (auto-refreshes if expired)
- `status` — Show token status and expiry
- `logout` — Remove stored token

#### Usage

```bash
aux4 curl oauth <subcommand> [options]
```

#### Example

```bash
aux4 curl oauth login pinterest --clientId abc123 --clientSecret secret --authUrl https://www.pinterest.com/oauth/ --tokenUrl https://api.pinterest.com/v5/oauth/token --scopes boards:read,pins:read
```
