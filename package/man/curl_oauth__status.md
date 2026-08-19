#### Description

The `status` command displays the current OAuth token status for a provider, including whether the token is valid or expired, the scopes granted, the expiry time, and whether a refresh token is available.

`Grant` reports how the token was obtained: `authorization_code` for an interactive login, `client_credentials` for an auth registered with `auth add`. A token written before this field existed reads as `authorization_code`, which is how it still refreshes.

`Refresh token` and `Can renew` answer different questions and can disagree. A client credentials token never has a refresh token and does not need one — it renews by presenting its credentials again — so it reads `Refresh token: no` alongside `Can renew: yes`. A token stored directly with `set-token` reads `no` to both, since there is nothing to renew it with.

A token whose client secret is held by reference reports `Can renew: yes` as well: the reference is resolved again at refresh time.

#### Usage

```bash
aux4 curl oauth status <provider> [--tokenFile <path>]
```

--tokenFile   Custom path to the token file (default: .oauth/<provider>.json)

#### Example

```bash
aux4 curl oauth status pinterest
```

```text
Provider:      pinterest
Status:        valid
Scopes:        boards:read,pins:read,pins:write
Expires at:    2026-04-28T12:30:00Z
Refresh token: yes
Token file:    .oauth/pinterest.json
```
