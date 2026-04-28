#### Description

The `status` command displays the current OAuth token status for a provider, including whether the token is valid or expired, the scopes granted, the expiry time, and whether a refresh token is available.

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
