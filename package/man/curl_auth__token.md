#### Description

The `token` command prints a valid token for a named auth. If the stored token is still within its lifetime it is returned as-is; if it has expired, or none has been fetched yet, a new one is obtained from the configured endpoint and stored before printing.

The token goes to stdout with no trailing newline, so it can be captured directly in a shell variable.

Concurrent callers are serialized: several processes finding the same expired token produce one fetch, not one each.

This is the same command as `aux4 curl oauth token` and works with both kinds of stored auth.

**Reading the definition from a config file**

When the auth is defined in a config file rather than registered locally, pass `--configFile` and `--config`. The definition is read from the file and a token is fetched on first use, so a machine that has never run `auth add` can still authenticate. The file is the source of truth for how a token is obtained, so a changed endpoint or scope takes effect immediately; only the cached token is local state.

#### Usage

```bash
aux4 curl auth token <name> [--tokenFile <path>]
```

--tokenFile   Custom token file path (default: `.oauth/<name>.json`)
--configFile  Config file holding the auth definitions
--config      Config profile to read the auth definitions from

#### Example

```bash
aux4 curl auth token billing
```

```text
eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...
```

Used in a script:

```bash
TOKEN=$(aux4 curl auth token billing)
aux4 curl request https://api.example.com/v1/invoices --header "Authorization: Bearer $TOKEN"
```
