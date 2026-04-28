#### Description

The `logout` command removes the stored OAuth token for a provider. It deletes the token file from `.oauth/<provider>.json` (or the custom path specified by `--tokenFile`). If the `.oauth/` directory becomes empty after removal, it is also cleaned up.

This does not revoke the token on the provider's side. To fully revoke access, visit the provider's application settings.

#### Usage

```bash
aux4 curl oauth logout <provider> [--tokenFile <path>]
```

--tokenFile   Custom path to the token file (default: .oauth/<provider>.json)

#### Example

```bash
aux4 curl oauth logout pinterest
```

```text
Logged out from pinterest
```
