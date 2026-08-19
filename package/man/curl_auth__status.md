#### Description

The `status` command shows what is stored for a named auth and whether its token is currently usable.

`Grant` reports how the token was obtained: `client_credentials` for an auth registered with `auth add`, `authorization_code` for one from `oauth login`. `Preset` appears only for the former.

`Status` is `valid` or `expired`. A token is treated as expired shortly before its actual deadline, so a token about to lapse reports `expired` and will be renewed on next use.

`Refresh token` and `Can renew` answer different questions and can disagree. A client credentials token never has a refresh token and does not need one — it renews by presenting its credentials again — so it normally reads `Refresh token: no` alongside `Can renew: yes`. A token stored directly with `oauth set-token` reads `no` to both: there is nothing to renew it with.

This is the same command as `aux4 curl oauth status` and works with both kinds of stored auth.

#### Usage

```bash
aux4 curl auth status <name> [--tokenFile <path>]
```

--tokenFile   Custom token file path (default: `.oauth/<name>.json`)

#### Example

```bash
aux4 curl auth status billing
```

```text
Provider:      billing
Status:        valid
Scopes:        invoices:read
Grant:         client_credentials
Preset:        client-credentials
Expires at:    2026-08-18T14:22:05Z
Refresh token: no
Can renew:     yes
Token file:    .oauth/billing.json
```
