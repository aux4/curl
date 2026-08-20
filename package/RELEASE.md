# aux4/curl

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


## Machine-to-machine authentication

`aux4 curl auth` registers a credential endpoint under a name. Requests naming it fetch a token, cache it, reuse it while valid, and fetch a new one when it expires — no token handling in the calling script.

```bash
aux4 curl auth add billing --tokenUrl https://auth.example.com/oauth/token --clientId my-client --clientSecret s3cr3t --scopes "invoices:read"
aux4 curl auth-request --provider billing https://api.example.com/v1/invoices
```

### `auth add`

- Registers the endpoint and stores the recipe next to the token it produces, so an expired token can always be re-acquired.
- Calls the endpoint once by default, turning a wrong URL or a bad secret into an immediate error rather than a confusing 401 later. `--verify false` defers the first fetch to first use.
- Shares the token store with `oauth`, so `auth-request`, `auth-stream` and `oauth token` accept either kind of name.

### Presets

Defaults for how credentials are sent and where the token is read from. Every default is individually overridable, so a preset is a starting point, not a mode.

| Preset | Sends | Reads token from | Reads expiry from |
|--------|-------|------------------|-------------------|
| `client-credentials` | form body, `grant_type=client_credentials` | `access_token`, then `token` | `expires_in`, then `exp` |
| `client-credentials-basic` | the same, credentials as HTTP Basic | `access_token`, then `token` | `expires_in`, then `exp` |
| `jwt` | form body | `token`, then `access_token` | `exp`, then `expires_in` |
| `json` | JSON body `{"clientId": ..., "clientSecret": ...}` | `token`, then `access_token` | `expires_in`, then `exp` |

### Field extraction

- `--tokenPath` / `--expiresPath` use the same path syntax as `aux4 json get` (`$.data.credentials.value`).
- Without them, the two well-known names for the preset are tried and nothing further is guessed — responses carrying both `access_token` and `id_token` are why the list stops at two.
- A response with no recognisable token fails with the response body echoed, so it is clear what to point `--tokenPath` at.

### Expiry

First source to answer wins: `--expiresPath`, then `expires_in`/`expires_at`, then the JWT `exp` claim, then `--expiresIn`, then a one hour default. A fetched token is never treated as valid forever.

The `exp` claim is read **without verifying the signature** — the client holds no key and the value is used only to decide when to renew. Anything that is not a JWT, or has no usable `exp`, falls through to the next source.

### Renewal

- Tokens are renewed shortly **before** their deadline, so a token cannot expire in the middle of the request it was fetched for.
- A 401 from the API triggers one renewal and a replay of the request. A 401 that renewal cannot fix is returned unchanged, since not every 401 is about an expired token.
- Renewal is serialized across concurrent callers, in-process and across processes, so a burst of requests needing a new token produces one fetch rather than one each.

### `auth token`, `auth status`, `auth remove`

Read a valid token, inspect what is stored, and delete it. `status` now also reports `Grant`, `Preset` and `Can renew` — a client credentials token reports `Refresh token: no` alongside `Can renew: yes`, because it renews by presenting its credentials again.

## Auth definitions in config.yaml

An auth can be defined in a config file rather than registered per machine, so a checkout authenticates with no setup step:

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

`--configFile` / `--config` are accepted by `auth-request`, `auth-stream` and `auth token`. The file is the source of truth for how a token is obtained — a changed endpoint or scope takes effect at once — while the token itself stays cached locally.

## Secrets by reference

`clientSecretRef` names the secret instead of holding it, resolved through `aux4/secret` when a token is needed. The secret can live in an encrypted local file, the OS keystore, 1Password, or any other provider.

The reference is deliberately not a `secret://` URI: aux4 resolves a parameter whose whole value is one *before* the command runs, so a URI passed to `auth add` would arrive already resolved and be written into the config file in the clear.

`auth add --save true` writes the definition into the config file, preserving the rest of it and storing only the reference. An inline `--clientSecret` combined with `--save` is refused rather than written into a file destined for version control.

`oauth login --clientSecretRef` gives the interactive flow the same treatment. The **reference** is stored in the token file rather than the resolved secret, and refresh resolves it again — so an interactive login no longer keeps its client secret at rest either.

`--clientSecret` on `oauth login` is now optional, since the secret may be supplied by reference instead. Passing it still works exactly as before.

`oauth token` accepts `--configFile` / `--config`, and `oauth status` reports `Grant` and `Can renew` alongside the existing fields.

## curl can hold its own secrets

`--secretProvider` moves both secrets curl deals with out of the local file and into a secret provider:

```bash
aux4 curl auth add billing --tokenUrl https://auth.example.com/oauth/token \
  --clientId my-client --clientSecret s3cr3t --secretProvider aux4
```

The secret is passed in plain. curl stores it and derives the path from the auth name, so nothing has to be named, recorded or maintained:

```text
secret://aux4/aux4-curl/billing/clientSecret
secret://aux4/aux4-curl/billing/accessToken
```

A shared config gains only `secretProvider`, which is not sensitive, and each machine keeps its own credentials at the same path. The token file retains the endpoint, grant and expiry — none of them secret — while the client secret and access token stay with the provider, including on renewal.

Without `--secretProvider` nothing changes. A provider that is not installed fails with a clear message rather than falling back silently.

The interactive flow takes the same flag. There it moves three things — the access token, the **refresh token** and the client secret. The refresh token is the one that matters: long-lived, and able to mint new access tokens on its own, so it is the last thing that should sit in a plain file. Rotated refresh tokens go back to the provider on every renewal.

`--clientSecretRef` is now for the other case only: a secret you already keep somewhere under a path of your own.
