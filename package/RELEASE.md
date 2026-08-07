# aux4/curl 1.2.0

## Status-only output and request timeouts

Two new capabilities on the HTTP primitives, aimed at health probing (used by `aux4/uptime`).

### `request --status`

- Prints **only** the numeric HTTP status code (e.g. `200`) and nothing else — no body, no headers, no output file.
- Exits `0` for every status class, so `200`, `404` and `503` all succeed. Usable in pipelines and as a health check that captures "down" codes.
- Overrides `--showHeaders` and `--output`.
- A transport failure (DNS, connection refused, TLS, or a `--maxTime` timeout) prints nothing to stdout, writes the error to stderr, and exits `1` — distinguishing "unreachable" from a real status code.

```bash
aux4 curl request --status true https://example.com
# -> 200
```

`--status` is a boolean flag: pass `--status true`, or write it bare after the URL (`aux4 curl request https://example.com --status`). A bare `--status` before the URL would consume the URL as its value (same as `--showHeaders`).

### `--maxTime` on `request` and `stream`

- Timeout in **seconds**, decimals allowed (e.g. `2.5`). `0` or absent means no timeout (backward compatible; previously both commands could hang forever on a bad host).
- `request`: exceeding the timeout fails like a transport error (stderr + exit `1`; with `--status`, empty stdout + exit `1`).
- `stream`: keeps the resilient contract — a timed-out line emits `{"error":"…timeout…","input":…,"status":0}` and the stream still exits `0`.

```bash
aux4 curl request --maxTime 2.5 https://example.com
cat records.ndjson | aux4 curl stream --maxTime 5 https://api.example.com/batch
```
