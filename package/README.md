# aux4/curl

HTTP client with stdin body and NDJSON streaming support. Similar to curl but designed for piping data: read request bodies from stdin, stream NDJSON input where each JSON line triggers a separate HTTP request, and collect results as NDJSON output.

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
