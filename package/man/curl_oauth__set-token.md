#### Description

The `set-token` command stores an API key or an already-issued access token directly, without any login flow. Use it for services that hand out a long-lived key rather than running OAuth.

The stored value is used by `auth-request` and `auth-stream` exactly like a token obtained through a login, so the rest of the tooling does not care where it came from.

A token stored this way carries no expiry and no means of renewal, so it is never treated as expired and never refreshed — `status` reports `Can renew: no`. When the service rotates the key, run `set-token` again.

#### Usage

```bash
aux4 curl oauth set-token <provider> --accessToken <value> [--tokenFile <path>]
```

--accessToken  The API key or access token to store (required)
--tokenFile    Custom token file path (default: `.oauth/<provider>.json`)

#### Example

```bash
aux4 curl oauth set-token openai --accessToken sk-abc123
```

```text
Token saved to .oauth/openai.json
```

```bash
aux4 curl auth-request --provider openai https://api.openai.com/v1/models
```
