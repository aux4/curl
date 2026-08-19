#### Description

The `token` command retrieves a valid access token for the specified OAuth provider. If the stored token has expired and a refresh token is available, it automatically refreshes the token and updates the stored file.

The access token is printed to stdout, making it easy to use in scripts or pipe to other commands.

When the auth is defined in a config file rather than registered locally, pass `--configFile` and `--config`. The definition is read from the file and a token is fetched on first use, so a machine that has never run `oauth login` or `auth add` can still obtain one.

#### Usage

```bash
aux4 curl oauth token <provider> [--tokenFile <path>]
```

--tokenFile   Custom path to the token file (default: .oauth/<provider>.json)
--configFile  Config file holding the auth definitions
--config      Config profile to read the auth definitions from

#### Example

```bash
aux4 curl oauth token pinterest
```

```text
pina_ABCDEF123456...
```

```bash
# Use in a script
TOKEN=$(aux4 curl oauth token pinterest)
curl -H "Authorization: Bearer $TOKEN" https://api.pinterest.com/v5/user_account
```
