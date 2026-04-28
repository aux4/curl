#### Description

The `auth-stream` command works like `stream` but automatically injects the OAuth2 Bearer token into the `Authorization` header for each request. It reads NDJSON from stdin, makes one authenticated HTTP request per JSON line, and outputs NDJSON results.

This is useful for bulk API operations that require authentication, such as creating multiple pins or updating multiple boards.

#### Usage

```bash
echo '<ndjson>' | aux4 curl auth-stream --provider <name> [--tokenFile <path>] [--method <METHOD>] [--header <Header: Value>] [--concurrency <n>] <url>
```

--provider      OAuth provider name (must have a stored token from `oauth login`)
--tokenFile     Custom token file path (default: .oauth/<provider>.json)
--method        HTTP method to use (default: POST)
--header        Request header in Name: Value format, can be repeated
--concurrency   Number of concurrent requests (default: 1)
url             The target URL (required, positional)

#### Example

```bash
echo '{"title":"Pin 1","board_id":"123"}
{"title":"Pin 2","board_id":"123"}' | aux4 curl auth-stream --provider pinterest --header "Content-Type: application/json" https://api.pinterest.com/v5/pins
```

```text
{"body":{"id":"456","title":"Pin 1"},"input":{"title":"Pin 1","board_id":"123"},"status":200}
{"body":{"id":"789","title":"Pin 2"},"input":{"title":"Pin 2","board_id":"123"},"status":200}
```
