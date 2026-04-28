#### Description

The `request` command makes an HTTP request to the specified URL. It supports all standard HTTP methods (GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS), custom headers, and request bodies.

The request body can be provided in three ways:

- **`--body` flag** — Pass the body as a string argument
- **stdin** — Pipe data into the command; stdin is automatically used as the request body
- **No body** — For methods like GET that typically don't have a body

When both stdin and `--body` are provided, the `--body` flag takes precedence.

Use `--showHeaders true` to include response status line and headers in the output before the response body.

The `--header` flag can be specified multiple times to set multiple request headers.

#### Usage

```bash
aux4 curl request [--method <METHOD>] [--header <Header: Value>] [--body <data>] [--showHeaders <true|false>] <url>
```

--method       HTTP method to use (default: GET)
--header       Request header in Name: Value format, can be repeated
--body         Request body as a string
--showHeaders  Show response status and headers (default: false)
url            The target URL (required, positional)

#### Example

```bash
aux4 curl request https://httpbin.org/get
```

```text
{"args":{},"headers":{"Host":"httpbin.org"},"url":"https://httpbin.org/get"}
```

```bash
echo '{"name":"Alice"}' | aux4 curl request --method POST --header "Content-Type: application/json" https://httpbin.org/post
```

```text
{"args":{},"data":"{\"name\":\"Alice\"}","json":{"name":"Alice"},"url":"https://httpbin.org/post"}
```

```bash
aux4 curl request --showHeaders true https://httpbin.org/get
```

```text
HTTP/1.1 200 OK
Content-Type: application/json

{"args":{},"headers":{"Host":"httpbin.org"},"url":"https://httpbin.org/get"}
```
