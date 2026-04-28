#### Description

The `curl` command group provides HTTP client functionality with stdin body support and NDJSON streaming. It contains two subcommands:

- **request** — Make a single HTTP request with optional stdin body
- **stream** — Read NDJSON from stdin, call a URL per line, output results as NDJSON

#### Usage

```bash
aux4 curl <command>
```

#### Example

```bash
aux4 curl request https://api.example.com/health
```

```text
{"status":"ok"}
```
