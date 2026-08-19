#### Description

The `auth` command group handles machine-to-machine authentication: a service that issues a token to a client presenting credentials, with no browser and no user involved. Use it whenever an API expects a token that your script has to obtain for itself.

An auth is registered once under a name. From then on that name is all any request needs — the token is fetched on first use, cached, reused while it is valid, and fetched again once it expires.

Auths share the token store with `aux4 curl oauth`, so `auth-request`, `auth-stream` and `oauth token` all work with either kind. The difference is only in how a token is obtained: `oauth login` opens a browser, `auth add` presents credentials.

Available subcommands:

- `add` — Register a credential endpoint under a name
- `token` — Get a valid token, fetching a new one if needed
- `status` — Show the stored configuration and token state
- `remove` — Remove a stored auth and its cached token

Tokens and their configuration are stored in `.oauth/<name>.json` by default, or in a custom path given with `--tokenFile`. The file contains the client secret, so add `.oauth/` to your `.gitignore`.

#### Usage

```bash
aux4 curl auth <subcommand> [options]
```

#### Example

```bash
aux4 curl auth add billing --tokenUrl https://auth.example.com/oauth/token --clientId my-client --clientSecret s3cr3t --scopes "invoices:read"
aux4 curl auth-request --provider billing https://api.example.com/v1/invoices
```

```text
{"invoices":[{"id":"inv_1","total":4200}]}
```
