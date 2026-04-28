# aux4/curl

HTTP client with OAuth2 authentication, stdin body, and NDJSON streaming support. Similar to curl but designed for piping data and OAuth2 workflows: authenticate with any OAuth2 provider, make authenticated requests, read request bodies from stdin, stream NDJSON input where each JSON line triggers a separate HTTP request, and collect results as NDJSON output.

## Installation

```bash
aux4 aux4 pkger install aux4/curl
```

## Quick Start

```bash
# Simple GET request
aux4 curl request https://api.example.com/users

# POST with body from stdin
echo '{"name":"Alice"}' | aux4 curl request --method POST --header "Content-Type: application/json" https://api.example.com/users

# OAuth2 login
aux4 curl oauth login myprovider --clientId abc123 --clientSecret secret --authUrl https://provider.com/oauth --tokenUrl https://provider.com/token --scopes read,write

# Authenticated request (auto-injects Bearer token)
aux4 curl auth-request --provider myprovider https://api.provider.com/v1/me

# Stream NDJSON: each line becomes a request, results streamed as NDJSON
cat users.ndjson | aux4 curl stream https://api.example.com/process
```

## Commands

### `aux4 curl request`

Make an HTTP request. Supports all standard HTTP methods, custom headers, and reading the request body from stdin or the `--body` flag.

```bash
aux4 curl request [--method <METHOD>] [--header <Header: Value>] [--body <data>] [--showHeaders <true|false>] <url>
```

| Flag | Description | Default |
|------|-------------|---------|
| `--method` | HTTP method (GET, POST, PUT, DELETE, PATCH, etc.) | `GET` |
| `--header` | Request header in `Name: Value` format (repeatable) | |
| `--body` | Request body as a string | |
| `--showHeaders` | Include response headers in output | `false` |
| `url` | Request URL (positional argument) | required |

When data is piped to stdin, it is used as the request body. The `--body` flag takes precedence if both are provided.

#### Examples

```bash
# GET request
aux4 curl request https://api.example.com/users

# POST with inline body
aux4 curl request --method POST --body '{"name":"Alice"}' --header "Content-Type: application/json" https://api.example.com/users

# POST with body from stdin
echo '{"name":"Alice"}' | aux4 curl request --method POST --header "Content-Type: application/json" https://api.example.com/users

# Show response headers
aux4 curl request --showHeaders true https://api.example.com/health

# Pipe a file as body
cat payload.json | aux4 curl request --method PUT --header "Content-Type: application/json" https://api.example.com/resource/1
```

### `aux4 curl stream`

Read NDJSON (newline-delimited JSON) from stdin. For each JSON line, make an HTTP request to the given URL with that line as the body. Output results as NDJSON, one JSON object per line.

```bash
aux4 curl stream [--method <METHOD>] [--header <Header: Value>] [--concurrency <N>] <url>
```

| Flag | Description | Default |
|------|-------------|---------|
| `--method` | HTTP method | `POST` |
| `--header` | Request header in `Name: Value` format (repeatable) | |
| `--concurrency` | Number of concurrent requests | `1` |
| `url` | Request URL (positional argument) | required |

Each output line is a JSON object with:
- `status` — HTTP status code (or `0` on connection error)
- `body` — Response body (parsed as JSON if valid, otherwise a string)
- `input` — The original input JSON that triggered this request

#### Examples

```bash
# Process each user record
cat users.ndjson | aux4 curl stream https://api.example.com/process

# With concurrency
cat records.ndjson | aux4 curl stream --concurrency 5 https://api.example.com/batch

# Custom method and headers
cat updates.ndjson | aux4 curl stream --method PUT --header "Authorization: Bearer token123" https://api.example.com/update

# Chain with jq for filtering
cat items.ndjson | aux4 curl stream https://api.example.com/enrich | jq 'select(.status == 200) | .body'
```

## OAuth2 Authentication

The `oauth` command group manages OAuth2 tokens using the authorization code flow. Tokens are stored locally in `.oauth/<provider>.json` by default. Add `.oauth/` to your `.gitignore`.

### `aux4 curl oauth login`

Authenticate with an OAuth2 provider. Opens a local callback server and prints a URL to authorize in the browser.

```bash
aux4 curl oauth login <provider> --clientId <id> --clientSecret <secret> --authUrl <url> --tokenUrl <url> --scopes <scopes> [--callbackPort <port>] [--tokenFile <path>]
```

| Flag | Description | Default |
|------|-------------|---------|
| `provider` | Provider name (positional argument) | required |
| `--clientId` | OAuth client ID | required |
| `--clientSecret` | OAuth client secret | required |
| `--authUrl` | Authorization endpoint URL | required |
| `--tokenUrl` | Token exchange endpoint URL | required |
| `--scopes` | Comma-separated scopes | required |
| `--callbackPort` | Local callback server port | `9876` |
| `--tokenFile` | Custom token file path | `.oauth/<provider>.json` |

#### Example

```bash
aux4 curl oauth login pinterest \
  --clientId abc123 \
  --clientSecret mysecret \
  --authUrl https://www.pinterest.com/oauth/ \
  --tokenUrl https://api.pinterest.com/v5/oauth/token \
  --scopes boards:read,pins:read,pins:write
```

### `aux4 curl oauth token`

Print a valid access token to stdout. Automatically refreshes if expired.

```bash
aux4 curl oauth token <provider> [--tokenFile <path>]
```

#### Example

```bash
# Get token
aux4 curl oauth token pinterest

# Use in a script
TOKEN=$(aux4 curl oauth token pinterest)
curl -H "Authorization: Bearer $TOKEN" https://api.pinterest.com/v5/user_account
```

### `aux4 curl oauth status`

Show token status, scopes, expiry, and whether a refresh token is available.

```bash
aux4 curl oauth status <provider> [--tokenFile <path>]
```

#### Example

```bash
aux4 curl oauth status pinterest
```

```text
Provider:      pinterest
Status:        valid
Scopes:        boards:read,pins:read,pins:write
Expires at:    2026-04-28T12:30:00Z
Refresh token: yes
Token file:    .oauth/pinterest.json
```

### `aux4 curl oauth logout`

Remove the stored token file for a provider. Does not revoke the token on the provider side.

```bash
aux4 curl oauth logout <provider> [--tokenFile <path>]
```

## Authenticated Requests

### `aux4 curl auth-request`

Same as `request` but automatically injects the `Authorization: Bearer <token>` header. Reads the stored token for the provider, refreshes if expired, and adds the header before making the request.

```bash
aux4 curl auth-request --provider <name> [--tokenFile <path>] [--method <METHOD>] [--header <Header: Value>] [--body <data>] [--showHeaders <true|false>] <url>
```

| Flag | Description | Default |
|------|-------------|---------|
| `--provider` | OAuth provider name | required |
| `--tokenFile` | Custom token file path | `.oauth/<provider>.json` |
| `--method` | HTTP method | `GET` |
| `--header` | Request header (repeatable) | |
| `--body` | Request body | |
| `--showHeaders` | Include response headers | `false` |
| `url` | Request URL (positional argument) | required |

#### Examples

```bash
# GET with auth
aux4 curl auth-request --provider pinterest https://api.pinterest.com/v5/user_account

# POST with auth
aux4 curl auth-request --provider pinterest --method POST \
  --body '{"title":"My Pin","board_id":"123"}' \
  --header "Content-Type: application/json" \
  https://api.pinterest.com/v5/pins
```

### `aux4 curl auth-stream`

Same as `stream` but automatically injects the `Authorization: Bearer <token>` header for each request. Reads NDJSON from stdin, makes one authenticated HTTP request per line, outputs NDJSON results.

```bash
echo '<ndjson>' | aux4 curl auth-stream --provider <name> [--tokenFile <path>] [--method <METHOD>] [--header <Header: Value>] [--concurrency <n>] <url>
```

| Flag | Description | Default |
|------|-------------|---------|
| `--provider` | OAuth provider name | required |
| `--tokenFile` | Custom token file path | `.oauth/<provider>.json` |
| `--method` | HTTP method | `POST` |
| `--header` | Request header (repeatable) | |
| `--concurrency` | Number of concurrent requests | `1` |
| `url` | Request URL (positional argument) | required |

#### Example

```bash
echo '{"title":"Pin 1","board_id":"123"}
{"title":"Pin 2","board_id":"123"}' | aux4 curl auth-stream --provider pinterest \
  --header "Content-Type: application/json" \
  https://api.pinterest.com/v5/pins
```

## Token File

By default, tokens are stored in `.oauth/<provider>.json` in the current directory. Override with `--tokenFile` on any command:

```bash
aux4 curl oauth login myprovider --tokenFile /custom/path/token.json ...
aux4 curl auth-request --provider myprovider --tokenFile /custom/path/token.json https://api.example.com/data
```
