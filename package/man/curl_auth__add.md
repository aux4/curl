#### Description

The `add` command registers a credential endpoint under a name and stores how to talk to it. Nothing else has to be repeated afterwards: `aux4 curl auth-request --provider <name>` and `aux4 curl auth-stream --provider <name>` obtain, cache and renew the token on their own.

By default the endpoint is called once during `add` so a wrong URL or a bad secret fails immediately rather than surfacing as a puzzling 401 on the first real call. Pass `--verify false` to register without calling out; the first request that needs a token will fetch it.

**Presets**

A preset chooses the defaults for how credentials are sent and where the token is read from. Every one of its defaults can be overridden individually, so a preset is a starting point rather than a mode:

- **`client-credentials`** (default) — form-encoded body with `grant_type=client_credentials`, credentials in the body. Reads `access_token`, then `token`.
- **`client-credentials-basic`** — the same request with the credentials sent as HTTP Basic instead. Several providers accept only this form.
- **`jwt`** — for endpoints that return a JWT. Reads `token`, then `access_token`, and prefers the token's own `exp` claim over any expiry field.
- **`json`** — posts `{"clientId": ..., "clientSecret": ...}` as JSON, for login endpoints that are not OAuth at all. Reads `token`, then `access_token`.

**Reading the token**

`--tokenPath` names where the token is in the response, using the same path syntax as `aux4 json get` (`$.data.credentials.value`). Without it, the two well-known names for the preset are tried in order. Nothing further is probed: responses that carry both an `access_token` and an `id_token` would otherwise let a longer guess list return a token that authenticates nothing. If neither name is present the command fails and prints the response, so you can see what to point `--tokenPath` at.

**Working out when it expires**

The first of these that yields an answer wins:

1. `--expiresPath`, when given
2. `expires_in` / `expires_at` in the response
3. the `exp` claim, when the token is a JWT
4. `--expiresIn`, as a fixed lifetime
5. a one hour default

A JWT's `exp` is read without verifying its signature — the token is opaque to the client and there is no key to verify against. It is used only to know when to renew, and a token that is not a JWT, or carries no usable `exp`, simply falls through to the next source. Tokens are renewed shortly before the deadline rather than after it, so a token cannot expire in the middle of the request it was fetched for.

**Sending the token**

`--authHeader` is the header added to API calls, defaulting to `Authorization: Bearer ${token}`. Set it to something like `X-API-Key: ${token}` for APIs that do not use bearer tokens.

**Storing the definition in config.yaml**

`--save true` writes the definition into a config file instead of the local token store, under `<config>/auth/<name>`. The rest of the file is preserved. From then on `auth-request`, `auth-stream` and `auth token` can read it with `--configFile` and `--config`, so a machine that has the file needs no registration step at all.

`--clientSecretRef` names the secret rather than holding it, in the form `<provider>/<vault>/<item>/<field>`. It is resolved through `aux4/secret` when a token is needed, so the config file stays safe to commit. It is deliberately not written as a `secret://` URI: aux4 resolves a parameter whose whole value is one before the command runs, so a URI would arrive here already resolved and be written into the file in the clear.

Combining `--save` with an inline `--clientSecret` is refused, as is giving both `--clientSecret` and `--clientSecretRef`.

#### Usage

```bash
aux4 curl auth add <name> --tokenUrl <url> [--preset <preset>] [--clientId <id>] [--clientSecret <secret>] [--scopes <scopes>] [--audience <audience>] [--method <method>] [--contentType <form|json>] [--credentialsIn <body|header>] [--header <header>] [--body <body>] [--tokenPath <path>] [--expiresPath <path>] [--expiresIn <seconds>] [--authHeader <template>] [--verify <true|false>] [--tokenFile <path>]
```

--tokenUrl        URL of the endpoint that issues the token (required)
--preset          Defaults for the request and the response: `client-credentials`, `client-credentials-basic`, `jwt`, `json` (default: `client-credentials`)
--clientId        Client ID sent to the token endpoint
--clientSecret    Client secret sent to the token endpoint
--scopes          Space-separated scopes to request
--audience        Audience to request, for providers that require one
--method          HTTP method for the token request (default: `POST`)
--contentType     How credentials are encoded: `form` or `json`. Overrides the preset
--credentialsIn   Send credentials in the `body` or as HTTP Basic in the `header`. Overrides the preset
--header          Extra header on the token request (Name: Value). Repeatable
--body            Verbatim token request body, replacing the generated one
--tokenPath       Path to the token in the response, e.g. `$.data.access_token`
--expiresPath     Path to the expiry in the response
--expiresIn       Fallback lifetime in seconds, used when nothing else reports one
--authHeader      Header template for API calls (default: `Authorization: Bearer ${token}`)
--clientSecretRef Secret reference holding the client secret, `<provider>/<vault>/<item>/<field>`
--save            Write the definition to the config file instead of the local token store (default: `false`)
--configFile      Config file to write the definition to
--config          Config profile to write the definition under
--verify          Fetch a token immediately to check the configuration (default: `true`)
--tokenFile       Custom token file path (default: `.oauth/<name>.json`)

#### Example

A standard OAuth2 client credentials endpoint:

```bash
aux4 curl auth add billing --tokenUrl https://auth.example.com/oauth/token --clientId my-client --clientSecret s3cr3t --scopes "invoices:read"
```

```text
Auth "billing" verified and saved to .oauth/billing.json
```

A provider that requires HTTP Basic:

```bash
aux4 curl auth add payments --preset client-credentials-basic --tokenUrl https://api.payments.example/oauth2/token --clientId pk_live_123 --clientSecret sk_live_456
```

A login endpoint that is not OAuth and returns a JWT with no expiry field:

```bash
aux4 curl auth add legacy --preset jwt --contentType json --tokenUrl https://legacy.example.com/api/login --clientId svc-account --clientSecret hunter2
```

A response whose token is nested:

```bash
aux4 curl auth add nested --preset json --tokenUrl https://api.example.com/session --clientId id --clientSecret secret --tokenPath '$.data.credentials.accessToken' --expiresPath '$.data.credentials.ttlSeconds'
```

An API that expects a key header rather than a bearer token:

```bash
aux4 curl auth add keyed --tokenUrl https://auth.example.com/token --clientId id --clientSecret secret --authHeader 'X-API-Key: ${token}'
```

Storing the definition in a config file, with the secret kept by reference:

```bash
aux4 curl auth add billing \
  --tokenUrl https://auth.example.com/oauth/token \
  --clientId my-client \
  --clientSecretRef 'aux4/Work/Billing API/clientSecret' \
  --scopes "invoices:read" \
  --configFile config.yaml --config prod --save true
```

```text
Auth "billing" verified and saved to config.yaml
```
