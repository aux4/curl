# aux4/curl

HTTP client with OAuth2 authentication and NDJSON streaming support. Similar to curl but designed for OAuth2 workflows: authenticate with any OAuth2 provider, make authenticated requests, upload files and download binary responses, stream NDJSON input where each JSON line triggers a separate HTTP request, and collect results as NDJSON output.

## Installation

```bash
aux4 aux4 pkger install aux4/curl
```

## Quick Start

```bash
# Simple GET request
aux4 curl request https://api.example.com/users

# POST with a body
aux4 curl request --method POST --body '{"name":"Alice"}' --header "Content-Type: application/json" https://api.example.com/users

# OAuth2 login
aux4 curl oauth login myprovider --clientId abc123 --clientSecret secret --authUrl https://provider.com/oauth --tokenUrl https://provider.com/token --scopes read,write

# Authenticated request (auto-injects Bearer token)
aux4 curl auth-request --provider myprovider https://api.provider.com/v1/me

# Stream NDJSON: each line becomes a request, results streamed as NDJSON
cat users.ndjson | aux4 curl stream https://api.example.com/process
```

## Commands

### `aux4 curl request`

Make an HTTP request. Supports all standard HTTP methods, custom headers, request bodies from a string or a file, file uploads, and binary downloads.

```bash
aux4 curl request [--method <METHOD>] [--header <Header: Value>] [--body <data>] [--showHeaders <true|false>] [--upload <path>] [--uploadField <name>] [--output <path>] <url>
```

| Flag | Description | Default |
|------|-------------|---------|
| `--method` | HTTP method (GET, POST, PUT, DELETE, PATCH, etc.) | `GET` |
| `--header` | Request header in `Name: Value` format (repeatable) | |
| `--body` | Request body as a string, or a JSON object of extra form fields when uploading | |
| `--bodyFile` | Send the contents of this file as the raw request body | |
| `--showHeaders` | Include response headers in output | `false` |
| `--upload` | File to send as `multipart/form-data` (repeatable, `field=path` to name the part) | |
| `--uploadField` | Default form field name for uploaded files | `file` |
| `--output` | Write the response body to this file instead of stdout | |
| `url` | Request URL (positional argument) | required |

Piping data into `aux4 curl request` does **not** set the request body — aux4 does not forward stdin to it, and forwarding it would make bodyless requests such as a plain `GET` hang. Use `--bodyFile` to send a file's contents (the way to send binary payloads), or `--body` for a string; `--bodyFile` wins if both are given. For one request per line of piped input, use `aux4 curl stream`.

### Uploading files

`--upload` sends a file as `multipart/form-data` — set automatically, with a generated boundary, so no `Content-Type` is needed. The content type of each part is detected from the file extension, and the method defaults to `POST`. Repeat `--upload` to send several files; by default they share the field name from `--uploadField`, and writing an entry as `field=path` names that part explicitly. When `--body` is given alongside a form-data upload it must be a JSON object, and its entries are sent as additional text fields in the same form.

**`multipart/related`** (for Google Drive/Gmail-style uploads): pass `--header "Content-Type: multipart/related"` and the body is built differently — `--body` becomes a single JSON metadata part (`application/json`, sent verbatim rather than split into fields), followed by one media part per `--upload`. Each media part's `Content-Type` comes from the file extension, or write the entry as `mime/type=path` to set it explicitly (e.g. `--upload "text/markdown=article.md"`). The generated boundary is appended and `Content-Length` is set. This is the shape Drive's `uploadType=multipart` expects for converting a file into a Doc, Sheet or Slides deck.

```bash
# Import Markdown as a Google Doc (Drive converts on upload)
aux4 curl auth-request --provider google --method POST \
  --header "Content-Type: multipart/related" \
  --url "https://www.googleapis.com/upload/drive/v3/files?uploadType=multipart" \
  --body '{"name":"My Doc","mimeType":"application/vnd.google-apps.document"}' \
  --upload "text/markdown=./article.md"
```

### Downloading binary responses

`--output` writes the response body to a file rather than stdout, which is required for images, archives, and other binary payloads. If the server returns an error status, the response is written to stderr and **no file is created**, so a failed download never leaves a truncated or error-filled file behind.

#### Examples

```bash
# GET request
aux4 curl request https://api.example.com/users

# POST with inline body
aux4 curl request --method POST --body '{"name":"Alice"}' --header "Content-Type: application/json" https://api.example.com/users

# Show response headers
aux4 curl request --showHeaders true https://api.example.com/health

# Upload a file, with an extra text field in the same form
aux4 curl request --method POST --upload ./logo.png --uploadField media --body '{"media_category":"tweet_image"}' https://api.example.com/media

# Upload several files, each under its own field name
aux4 curl request --method POST --upload "avatar=./avatar.png" --upload "banner=./banner.png" https://api.example.com/profile

# Download a binary response to a file
aux4 curl request --output ./report.pdf https://api.example.com/reports/1

# Send a file's contents as the raw body
aux4 curl request --method PUT --bodyFile ./payload.json --header "Content-Type: application/json" https://api.example.com/resource/1
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

The `oauth` command group manages OAuth2 tokens using the authorization code flow with PKCE (Proof Key for Code Exchange). PKCE is automatically enabled for all providers, ensuring compatibility with providers like X (Twitter) that require it. Tokens are stored locally in `.oauth/<provider>.json` by default. Add `.oauth/` to your `.gitignore`.

> **Token storage.** Tokens are written as a plaintext JSON file with `0600` permissions (owner read/write only) in a `0700` directory — the same posture as the `gcloud`, `aws` and `gh` CLIs. The file contains the access token, the long-lived refresh token, and the client id/secret, so it should be treated as a credential: keep it out of version control and off shared machines. It is not encrypted at rest; anyone who can read the file as your user can use the tokens.

### `aux4 curl oauth login`

Authenticate with an OAuth2 provider using the authorization code flow with PKCE. Opens a local callback server and prints a URL to authorize in the browser.

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
aux4 curl auth-request --provider <name> [--tokenFile <path>] [--method <METHOD>] [--header <Header: Value>] [--body <data>] [--showHeaders <true|false>] [--upload <path>] [--uploadField <name>] [--output <path>] <url>
```

| Flag | Description | Default |
|------|-------------|---------|
| `--provider` | OAuth provider name | required |
| `--tokenFile` | Custom token file path | `.oauth/<provider>.json` |
| `--method` | HTTP method | `GET` |
| `--header` | Request header (repeatable) | |
| `--body` | Request body, or a JSON object of extra form fields when uploading | |
| `--bodyFile` | Send the contents of this file as the raw request body | |
| `--showHeaders` | Include response headers | `false` |
| `--upload` | File to send as `multipart/form-data` (repeatable, `field=path` to name the part) | |
| `--uploadField` | Default form field name for uploaded files | `file` |
| `--output` | Write the response body to this file instead of stdout | |
| `url` | Request URL (positional argument) | required |

File uploads and binary downloads work exactly as in `request` — see [Uploading files](#uploading-files) and [Downloading binary responses](#downloading-binary-responses).

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
