#### Description

The `remove` command deletes a stored auth: both its cached token and the configuration used to obtain it. The containing directory is removed too if nothing else is left in it.

Removing an auth does not revoke the token at the provider — it only forgets it locally. Revoke it with the provider directly if that matters.

#### Usage

```bash
aux4 curl auth remove <name> [--tokenFile <path>]
```

--tokenFile   Custom token file path (default: `.oauth/<name>.json`)

#### Example

```bash
aux4 curl auth remove billing
```

```text
Logged out from billing
```
