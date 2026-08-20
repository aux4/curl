# aux4/curl

HTTP client with OAuth2 authentication and NDJSON streaming support. Similar to curl but designed for authenticated workflows: log in to any OAuth2 provider interactively or register a machine-to-machine credential endpoint, make authenticated requests without ever handling the token yourself, upload files and download binary responses, stream NDJSON input where each JSON line triggers a separate HTTP request, and collect results as NDJSON output.

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

# OAuth2 login (interactive, opens a browser)
aux4 curl oauth login myprovider --clientId abc123 --clientSecret secret --authUrl https://provider.com/oauth --tokenUrl https://provider.com/token --scopes read,write

# Machine-to-machine auth (no browser, token fetched and cached automatically)
aux4 curl auth add myapi --tokenUrl https://auth.example.com/oauth/token --clientId abc123 --clientSecret secret

# Authenticated request (auto-injects Bearer token, renews it when it expires)
aux4 curl auth-request --provider myapi https://api.example.com/v1/me

# Stream NDJSON: each line becomes a request, results streamed as NDJSON
cat users.ndjson | aux4 curl stream https://api.example.com/process
```

## Commands

### `aux4 curl request`

Make an HTTP request. Supports all standard HTTP methods, custom headers, request bodies from a string or a file, file uploads, and binary downloads.

```bash
aux4 curl request [--method <METHOD>] [--header <Header: Value>] [--body <data>] [--showHeaders <true|false>] [--upload <path>] [--uploadField <name>] [--output <path>] [--status <true|false>] [--maxTime <seconds>] <url>
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
| `--status` | Print only the numeric HTTP status code and exit 0 (transport failure prints nothing and exits 1) | `false` |
| `--maxTime` | Request timeout in seconds (decimals allowed); `0` means no timeout | `0` |
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

### Health checks and timeouts

`--status` prints **only** the numeric HTTP status code and nothing else — no body, headers, or output file. It exits `0` for any status class, so `200`, `404` and `503` all succeed and you can capture a "down" code in a pipeline or a monitor. `--status` overrides `--showHeaders` and `--output`. A **transport failure** — DNS, connection refused, TLS, or a `--maxTime` timeout — prints nothing to stdout, writes the error to stderr, and exits `1`, distinguishing an unreachable host from a real status code.

`--maxTime` bounds the request with a timeout in **seconds** (decimals allowed, e.g. `2.5`). `0` or omitting it means no timeout (the default). Exceeding it fails like any transport error: an error on stderr and exit `1` (with `--status`, empty stdout + exit `1`). This is the primitive `aux4/uptime` uses for command-based health probes.

```bash
# Print just the status code
aux4 curl request --status true https://example.com
# -> 200

# Fail fast if the host is slow or hung
aux4 curl request --maxTime 2.5 https://example.com

# Bounded health probe
aux4 curl request --status true --maxTime 2 https://example.com
# -> 200
```

**Note:** `--status` is a boolean flag — pass it as `--status true`, or write it bare **after** the URL (`aux4 curl request https://example.com --status`). A bare `--status` before the URL would consume the URL as its value (the same rule that applies to `--showHeaders`).

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
aux4 curl stream [--method <METHOD>] [--header <Header: Value>] [--concurrency <N>] [--maxTime <seconds>] <url>
```

| Flag | Description | Default |
|------|-------------|---------|
| `--method` | HTTP method | `POST` |
| `--header` | Request header in `Name: Value` format (repeatable) | |
| `--concurrency` | Number of concurrent requests | `1` |
| `--maxTime` | Per-request timeout in seconds (decimals allowed); `0` means no timeout | `0` |
| `url` | Request URL (positional argument) | required |

Each output line is a JSON object with:
- `status` — HTTP status code (or `0` on connection error or timeout)
- `body` — Response body (parsed as JSON if valid, otherwise a string)
- `input` — The original input JSON that triggered this request

`--maxTime` sets a per-request timeout. A timed-out line is treated as a transport error — it emits `{"error":"…timeout…","input":…,"status":0}` and the stream still exits `0`, so one slow upstream never aborts the batch.

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

# Bound each request with a timeout (slow lines become status:0 error objects)
cat records.ndjson | aux4 curl stream --maxTime 5 https://api.example.com/batch
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
| `--clientSecret` | OAuth client secret | required unless `--clientSecretRef` is given |
| `--clientSecretRef` | Secret reference holding the client secret, `<provider>/<vault>/<item>/<field>` | |
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

Show token status, scopes, expiry, how the token was obtained, and whether it can be renewed.

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
Grant:         authorization_code
Expires at:    2026-04-28T12:30:00Z
Refresh token: yes
Can renew:     yes
Token file:    .oauth/pinterest.json
```

### `aux4 curl oauth logout`

Remove the stored token file for a provider. Does not revoke the token on the provider side.

```bash
aux4 curl oauth logout <provider> [--tokenFile <path>]
```

## Machine-to-Machine Authentication

The `auth` command group covers the case where a service issues a token to a client presenting credentials — no browser, no user. Register the credential endpoint once under a name, and every request that names it obtains, caches and renews its own token.

```bash
aux4 curl auth add billing --tokenUrl https://auth.example.com/oauth/token --clientId my-client --clientSecret s3cr3t --scopes "invoices:read"
aux4 curl auth-request --provider billing https://api.example.com/v1/invoices
```

The token is fetched once, reused for every call while it is still valid, and fetched again once it expires. Nothing in your scripts ever touches it.

Auths share the token store with `oauth`, so `auth-request`, `auth-stream` and `oauth token` accept either kind of name. The only difference is how the token was obtained.

> **Token storage.** The stored file holds the client secret as well as the token, with the same `0600` posture described under [OAuth2 Authentication](#oauth2-authentication). Keep `.oauth/` out of version control.

### `aux4 curl auth add`

Register a credential endpoint under a name.

```bash
aux4 curl auth add <name> --tokenUrl <url> [--preset <preset>] [--clientId <id>] [--clientSecret <secret>] [--scopes <scopes>] [--audience <audience>] [--method <METHOD>] [--contentType <form|json>] [--credentialsIn <body|header>] [--header <Header: Value>] [--body <data>] [--tokenPath <path>] [--expiresPath <path>] [--expiresIn <seconds>] [--authHeader <template>] [--verify <true|false>] [--tokenFile <path>]
```

| Flag | Description | Default |
|------|-------------|---------|
| `name` | Name used to reference this auth (positional argument) | required |
| `--tokenUrl` | URL of the endpoint that issues the token | required |
| `--preset` | Defaults for the request and the response | `client-credentials` |
| `--clientId` | Client ID sent to the token endpoint | |
| `--clientSecret` | Client secret sent to the token endpoint | |
| `--scopes` | Space-separated scopes to request | |
| `--audience` | Audience to request, for providers that require one | |
| `--method` | HTTP method for the token request | `POST` |
| `--contentType` | How credentials are encoded: `form` or `json` | from preset |
| `--credentialsIn` | Send credentials in the `body` or as HTTP Basic in the `header` | from preset |
| `--header` | Extra header on the token request (repeatable) | |
| `--body` | Verbatim token request body, replacing the generated one | |
| `--tokenPath` | Path to the token in the response | from preset |
| `--expiresPath` | Path to the expiry in the response | |
| `--expiresIn` | Fallback lifetime in seconds | |
| `--authHeader` | Header template for API calls | `Authorization: Bearer ${token}` |
| `--verify` | Fetch a token immediately to check the configuration | `true` |
| `--tokenFile` | Custom token file path | `.oauth/<name>.json` |

By default `add` calls the endpoint once, so a wrong URL or a bad secret fails right there instead of surfacing as a confusing 401 later. With `--verify false` nothing is requested until the first call that needs a token.

### Presets

A preset picks the defaults for how credentials are sent and where the token is read from. Each of its defaults can be overridden on its own, so a preset is a starting point rather than a mode.

| Preset | Sends | Reads token from | Reads expiry from |
|--------|-------|------------------|-------------------|
| `client-credentials` | form body, `grant_type=client_credentials` | `access_token`, then `token` | `expires_in`, then `exp` |
| `client-credentials-basic` | the same, credentials as HTTP Basic | `access_token`, then `token` | `expires_in`, then `exp` |
| `jwt` | form body | `token`, then `access_token` | `exp`, then `expires_in` |
| `json` | JSON body `{"clientId": ..., "clientSecret": ...}` | `token`, then `access_token` | `expires_in`, then `exp` |

```bash
# A provider that accepts only HTTP Basic
aux4 curl auth add payments --preset client-credentials-basic \
  --tokenUrl https://api.payments.example/oauth2/token \
  --clientId pk_live_123 --clientSecret sk_live_456

# A login endpoint that is not OAuth at all and returns a JWT
aux4 curl auth add legacy --preset jwt --contentType json \
  --tokenUrl https://legacy.example.com/api/login \
  --clientId svc-account --clientSecret hunter2
```

### Reading the token from the response

`--tokenPath` says where the token is, using the same path syntax as `aux4 json get`:

```bash
aux4 curl auth add nested --preset json \
  --tokenUrl https://api.example.com/session \
  --clientId id --clientSecret secret \
  --tokenPath '$.data.credentials.accessToken' \
  --expiresPath '$.data.credentials.ttlSeconds'
```

Without it, the two names for the chosen preset are tried in order and nothing further is guessed. If neither is present the command fails and prints the response so you can see what to point `--tokenPath` at.

**Note:** responses that carry both an `access_token` and an `id_token` are why the list stops at two. A longer guess list would eventually return a token that authenticates nothing.

### Expiry

The first source that yields an answer decides when the token is renewed:

1. `--expiresPath`, when given
2. `expires_in` / `expires_at` in the response
3. the `exp` claim, when the token is a JWT
4. `--expiresIn`, as a fixed lifetime
5. a one hour default

A JWT's `exp` is read without verifying its signature — the token is opaque to the client and there is no key to verify against, and it is used only to decide when to renew. Anything that is not a JWT, or carries no usable `exp`, falls through to the next source.

Renewal happens shortly before the deadline rather than after it, so a token cannot expire in the middle of the request it was fetched for. If an API rejects a token anyway — revoked early, or a clock further out of step than expected — the request is retried once with a freshly fetched token. Concurrent callers are serialized, so a burst of requests that all need a new token produces one fetch rather than one each.

### Sending the token

APIs that do not use bearer tokens can set the header template:

```bash
aux4 curl auth add keyed --tokenUrl https://auth.example.com/token \
  --clientId id --clientSecret secret \
  --authHeader 'X-API-Key: ${token}'
```

### Defining auths in config.yaml

An auth can live in a config file instead of being registered on each machine. Put the definition under a config profile and every machine that has the file can authenticate — no `auth add`, no setup step:

```yaml
config:
  prod:
    auth:
      billing:
        tokenUrl: https://auth.example.com/oauth/token
        preset: client-credentials
        clientId: my-client
        clientSecretRef: aux4/Work/Billing API/clientSecret
        scopes: invoices:read
```

```bash
aux4 curl auth-request --provider billing --configFile config.yaml --config prod https://api.example.com/v1/invoices
```

The definition comes from the file; the token is fetched on first use and cached locally exactly as before. The config file is the source of truth for *how* to obtain a token, so changing an endpoint or a scope there takes effect immediately — only the cached token is local state.

`--configFile` and `--config` work the same way on `auth-request`, `auth-stream` and `auth token`.

### Keeping the secret out of the file

`clientSecretRef` names the secret instead of holding it, so the file is safe to commit:

```yaml
clientSecretRef: aux4/Work/Billing API/clientSecret
#                └──┬─┘ └───────┬──────┘ └────┬────┘
#              provider    vault/item        field
```

It is resolved through [`aux4/secret`](https://hub.aux4.io/r/public/packages/aux4/secret) when a token is needed, so the secret can live wherever you keep secrets — an encrypted local file (`aux4/secret-aux4`), the OS keystore (`aux4/secret-os`), 1Password, or anything else implementing the provider contract.

**Note:** the reference is deliberately not written as a `secret://` URI. aux4 resolves any parameter whose whole value is one *before* the command runs, so a URI passed to `auth add` would arrive already resolved and be written into the config file in the clear — the exact leak this field prevents.

`auth add --save true` writes the definition into the config file for you:

```bash
aux4 curl auth add billing \
  --tokenUrl https://auth.example.com/oauth/token \
  --clientId my-client \
  --clientSecretRef 'aux4/Work/Billing API/clientSecret' \
  --scopes "invoices:read" \
  --configFile config.yaml --config prod --save true
```

The rest of the file is preserved, and only the reference is written — never a secret. Passing an inline `--clientSecret` together with `--save` is refused rather than silently writing a credential into a file destined for version control.

The same reference works for the interactive flow:

```bash
aux4 curl oauth login pinterest --clientSecretRef 'aux4/Work/Pinterest/clientSecret' --clientId abc123 ...
```

In both flows the **reference** is what gets stored, never the resolved secret. A client-credentials token re-resolves it on every renewal; an interactive login re-resolves it on refresh. So the secret exists only for the duration of the request that needs it — it is not written into `.oauth/<name>.json`, into config.yaml, into shell history, or into `ps`.

### Letting curl hold the secrets

`--secretProvider` puts both secrets curl deals with into a secret provider instead of a file:

```bash
aux4 curl auth add billing \
  --tokenUrl https://auth.example.com/oauth/token \
  --clientId my-client \
  --clientSecret s3cr3t \
  --secretProvider aux4 \
  --configFile config.yaml --config prod --save true
```

You pass the secret in plain — there is no reference to invent. curl stores it and derives the path itself:

```text
secret://aux4/aux4-curl/billing/clientSecret     the credential you supplied
secret://aux4/aux4-curl/billing/accessToken      the token curl fetched
```

Because the path is derived from the auth name, nothing records it and nothing has to be maintained. The config file gains only `secretProvider: aux4`, which is not a secret, so it stays shareable — and each machine holds its own credentials at the same path.

The local token file keeps the endpoint, the grant and the expiry, which are not sensitive. Neither the client secret nor the access token is written to it, and a renewed token goes back to the provider.

**Note:** without `--secretProvider` nothing changes — the token file works exactly as it always has. Naming a provider that is not installed fails with a clear message rather than falling back silently.

`--secretProvider` works the same on the interactive flow:

```bash
aux4 curl oauth login pinterest --clientId abc123 --clientSecret s3cr3t --secretProvider aux4 ...
```

There it moves three things: the access token, the **refresh token** and the client secret. The refresh token matters most — it is long-lived and can mint new access tokens on its own, so it is the last thing that should sit in a plain file. A rotated refresh token goes back to the provider on every renewal.

`--clientSecretRef` remains for the other case: a secret you already keep somewhere, under a path of your own choosing.

### `aux4 curl auth token`

Print a valid token to stdout, fetching a new one first if the stored one has expired.

```bash
aux4 curl auth token <name> [--tokenFile <path>]
```

### `aux4 curl auth status`

Show the stored configuration and token state.

```bash
aux4 curl auth status <name> [--tokenFile <path>]
```

```text
Provider:      billing
Status:        valid
Scopes:        invoices:read
Grant:         client_credentials
Preset:        client-credentials
Expires at:    2026-08-18T14:22:05Z
Refresh token: no
Can renew:     yes
Token file:    .oauth/billing.json
```

`Refresh token` and `Can renew` answer different questions. A client credentials token never has a refresh token and does not need one — it renews by presenting its credentials again.

### `aux4 curl auth remove`

Remove a stored auth and its cached token. Does not revoke the token on the provider side.

```bash
aux4 curl auth remove <name> [--tokenFile <path>]
```

## Authenticated Requests

### `aux4 curl auth-request`

Same as `request` but automatically injects the auth header. Reads the stored token for the given name, renews it if it has expired, and adds the header before making the request. `--provider` accepts either an `oauth login` provider or an `auth add` name.

If the API rejects the token with a 401, the token is renewed and the request replayed once. A 401 that renewal cannot fix is returned as the response, unchanged.

```bash
aux4 curl auth-request --provider <name> [--tokenFile <path>] [--method <METHOD>] [--header <Header: Value>] [--body <data>] [--showHeaders <true|false>] [--upload <path>] [--uploadField <name>] [--output <path>] <url>
```

| Flag | Description | Default |
|------|-------------|---------|
| `--provider` | OAuth provider or auth name | required |
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

Same as `stream` but automatically injects the auth header for each request. Reads NDJSON from stdin, makes one authenticated HTTP request per line, outputs NDJSON results. `--provider` accepts either an `oauth login` provider or an `auth add` name.

Lines rejected with a 401 are retried once with a renewed token. Renewal is shared across the concurrent requests, so a stream whose token expires mid-run fetches one replacement, not one per line.

```bash
echo '<ndjson>' | aux4 curl auth-stream --provider <name> [--tokenFile <path>] [--method <METHOD>] [--header <Header: Value>] [--concurrency <n>] <url>
```

| Flag | Description | Default |
|------|-------------|---------|
| `--provider` | OAuth provider or auth name | required |
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

By default, tokens are stored in `.oauth/<name>.json` in the current directory. Override with `--tokenFile` on any command:

```bash
aux4 curl oauth login myprovider --tokenFile /custom/path/token.json ...
aux4 curl auth add myapi --tokenFile /custom/path/token.json ...
aux4 curl auth-request --provider myapi --tokenFile /custom/path/token.json https://api.example.com/data
```

The file holds the cached token together with whatever is needed to obtain another one — a refresh token for an interactive login, the credential endpoint and client secret for an `auth add` registration. Both are credentials: keep the directory out of version control.

The default path is relative to the current directory, so a token saved in one directory is not visible from another. Set `AUX4_CURL_OAUTH_HOME` to keep a single store per machine.
