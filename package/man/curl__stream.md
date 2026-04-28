#### Description

The `stream` command reads NDJSON (newline-delimited JSON) from stdin. For each JSON line, it makes an HTTP request to the specified URL using that line as the request body. Results are streamed to stdout as NDJSON, one JSON object per response.

Each output line contains:

- **`status`** — The HTTP status code (integer). `0` if the request failed before receiving a response (e.g., connection error).
- **`body`** — The response body. If the response is valid JSON, it is included as a parsed JSON value. Otherwise, it is included as a string.
- **`input`** — The original input JSON object that triggered this request, useful for correlating responses with inputs.

Invalid JSON lines in the input are skipped with an error message on stderr. Empty lines are also skipped.

The `Content-Type: application/json` header is set automatically on every request. Additional headers can be added with `--header`.

Use `--concurrency` to control how many requests are made in parallel. With concurrency greater than 1, output order may differ from input order.

#### Usage

```bash
aux4 curl stream [--method <METHOD>] [--header <Header: Value>] [--concurrency <N>] <url>
```

--method       HTTP method to use (default: POST)
--header       Request header in Name: Value format, can be repeated
--concurrency  Number of concurrent requests (default: 1)
url            The target URL (required, positional)

#### Example

```bash
echo '{"id":1}
{"id":2}' | aux4 curl stream https://api.example.com/process
```

```text
{"body":{"result":"ok"},"input":{"id":1},"status":200}
{"body":{"result":"ok"},"input":{"id":2},"status":200}
```

```bash
cat records.ndjson | aux4 curl stream --concurrency 5 --header "Authorization: Bearer token123" https://api.example.com/batch
```

```text
{"body":{"processed":true},"input":{"id":1},"status":200}
{"body":{"processed":true},"input":{"id":2},"status":200}
```
