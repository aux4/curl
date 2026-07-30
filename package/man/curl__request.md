#### Description

The `request` command makes an HTTP request to the specified URL. It supports all standard HTTP methods (GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS), custom headers, and request bodies.

The request body can be provided in three ways:

- **`--body` flag** — Pass the body as a string argument
- **`--bodyFile` flag** — Send the contents of a file as the raw request body, which is the way to send binary payloads. The method defaults to `POST` when it is used
- **No body** — For methods like GET that typically don't have a body

`--bodyFile` takes precedence over `--body`. Note that piping data into `aux4 curl request` does **not** set the request body: aux4 does not forward stdin to this command, and forwarding it would make bodyless requests such as a plain `GET` hang waiting for input. Use `--bodyFile` for file contents, or `--body` for a string. To make one request per line of piped input, use `aux4 curl stream` instead, which is built for that.

Use `--showHeaders true` to include response status line and headers in the output before the response body.

The `--header` flag can be specified multiple times to set multiple request headers.

Use `--upload` to send a file as `multipart/form-data`. The form field name defaults to `file` and can be changed with `--uploadField`. The `Content-Type` of the file part is detected from its extension, and the method defaults to `POST` when uploading. When `--body` is also given it must be a JSON object, and each of its entries is sent as an additional text field in the same form.

The request is sent as `multipart/form-data` automatically — you do not need to set `Content-Type`, and the boundary is generated for you. If an API expects a different multipart subtype, pass it yourself (for example `--header "Content-Type: multipart/related"`) and the subtype and any of its parameters are kept while the generated boundary is appended; a boundary you supply is ignored, since only the generated one matches the body. A non-multipart `Content-Type` is ignored while uploading, because the body genuinely is multipart.

`--upload` can be repeated to send several files in one request. By default every file uses the same form field name; write an entry as `field=path` to name that part explicitly, which is how you send files under different field names in a single request. A path that itself contains `=` must always be written in the `field=path` form so it cannot be misread.

Use `--output` to write the response body to a file instead of stdout — necessary for binary responses such as images or documents. If the server returns an error status, the response is printed to stderr and **no file is written**, so a failed download never leaves a corrupt file behind.

#### Usage

```bash
aux4 curl request [--method <METHOD>] [--header <Header: Value>] [--body <data>] [--bodyFile <path>] [--showHeaders <true|false>] [--upload <path>] [--uploadField <name>] [--output <path>] <url>
```

--method       HTTP method to use (default: GET)
--header       Request header in Name: Value format, can be repeated
--body         Request body as a string, or JSON object of extra form fields when uploading
--bodyFile     Send the contents of this file as the raw request body
--showHeaders  Show response status and headers (default: false)
--upload       File to send as multipart/form-data, can be repeated; use field=path to name the part
--uploadField  Default form field name for uploaded files (default: file)
--output       Write the response body to this file instead of stdout
url            The target URL (required, positional)

#### Example

```bash
aux4 curl request https://httpbin.org/get
```

```text
{"args":{},"headers":{"Host":"httpbin.org"},"url":"https://httpbin.org/get"}
```

```bash
aux4 curl request --method POST --body '{"name":"Alice"}' --header "Content-Type: application/json" https://httpbin.org/post
```

```text
{"args":{},"data":"{\"name\":\"Alice\"}","json":{"name":"Alice"},"url":"https://httpbin.org/post"}
```

Send a file's contents as the raw body:

```bash
aux4 curl request --method PUT --bodyFile ./payload.json --header "Content-Type: application/json" https://httpbin.org/put
```

```bash
aux4 curl request --showHeaders true https://httpbin.org/get
```

```text
HTTP/1.1 200 OK
Content-Type: application/json

{"args":{},"headers":{"Host":"httpbin.org"},"url":"https://httpbin.org/get"}
```

Upload a file as `multipart/form-data`, with an extra text field in the same form:

```bash
aux4 curl request --method POST --upload ./logo.png --uploadField media --body '{"media_category":"tweet_image"}' https://httpbin.org/post
```

Send several files in one request, each under its own field name:

```bash
aux4 curl request --method POST --upload "avatar=./avatar.png" --upload "banner=./banner.png" https://httpbin.org/post
```

Download a binary response to a file:

```bash
aux4 curl request --output ./logo.png https://httpbin.org/image/png
```
