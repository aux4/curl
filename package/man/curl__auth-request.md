#### Description

The `auth-request` command works like `request` but automatically injects the OAuth2 Bearer token into the `Authorization` header. It reads the stored token for the specified provider, refreshes it if expired, and adds the header before making the request.

This is the primary way to make authenticated API calls through `aux4 curl`. Any additional headers specified with `--header` are preserved alongside the injected authorization header.

#### Usage

```bash
aux4 curl auth-request --provider <name> [--tokenFile <path>] [--method <METHOD>] [--header <Header: Value>] [--body <data>] [--bodyFile <path>] [--showHeaders <true|false>] [--upload <path>] [--uploadField <name>] [--output <path>] <url>
```

--provider      OAuth provider name (must have a stored token from `oauth login`)
--tokenFile     Custom token file path (default: .oauth/<provider>.json)
--method        HTTP method to use (default: GET)
--header        Request header in Name: Value format, can be repeated
--body          Request body as a string, or JSON object of extra form fields when uploading
--bodyFile      Send the contents of this file as the raw request body
--showHeaders   Show response status and headers (default: false)
--upload        File to send as multipart/form-data, can be repeated; use field=path to name the part
--uploadField   Default form field name for uploaded files (default: file)
--output        Write the response body to this file instead of stdout
url             The target URL (required, positional)

File uploads and binary downloads behave exactly as in `curl request`; the only difference is that the OAuth `Authorization` header is added for you. Uploading defaults the method to `POST`, and a failed download writes no file.

#### Example

```bash
aux4 curl auth-request --provider pinterest https://api.pinterest.com/v5/user_account
```

```text
{"username":"johndoe","account_type":"BUSINESS"}
```

```bash
aux4 curl auth-request --provider pinterest --method POST --body '{"title":"My Pin","board_id":"123"}' --header "Content-Type: application/json" https://api.pinterest.com/v5/pins
```

```text
{"id":"456","title":"My Pin","board_id":"123"}
```
