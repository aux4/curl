# curl auth status

## with a client credentials auth

```beforeAll
mkdir -p .oauth
cat > .oauth/svcauth.json << 'ENDTOKEN'
{
  "clientId": "id1",
  "tokenUrl": "http://127.0.0.1:18999/token",
  "scopes": "read",
  "accessToken": "tok-1",
  "refreshToken": "",
  "expiresAt": "2099-12-31T23:59:59Z",
  "grant": "client_credentials",
  "auth": {
    "preset": "client-credentials",
    "tokenUrl": "http://127.0.0.1:18999/token",
    "clientId": "id1",
    "clientSecret": "sec1"
  }
}
ENDTOKEN
```

```afterAll
rm -rf .oauth
```

### should report the grant and preset

```execute
aux4 curl auth status svcauth
```

```expect:partial
Grant:         client_credentials
Preset:        client-credentials
```

### should report that it can renew despite having no refresh token

A client credentials token never has a refresh token; it renews by replaying
the credential exchange, so the two lines disagree on purpose.

```execute
aux4 curl auth status svcauth
```

```expect:partial
Refresh token: no
Can renew:     yes
```

## with an interactive oauth token

```beforeAll
mkdir -p .oauth
cat > .oauth/interactive.json << 'ENDTOKEN'
{
  "clientId": "id1",
  "clientSecret": "sec1",
  "authUrl": "http://localhost/auth",
  "tokenUrl": "http://localhost/token",
  "scopes": "read",
  "accessToken": "tok-1",
  "refreshToken": "refresh-1",
  "expiresAt": "2099-12-31T23:59:59Z"
}
ENDTOKEN
```

```afterAll
rm -rf .oauth
```

### should report the authorization_code grant for a token with no grant recorded

Tokens written before this field existed keep refreshing the way they always
did, so an absent grant reads as the interactive flow.

```execute
aux4 curl auth status interactive
```

```expect:partial
Grant:         authorization_code
```

### should report that it can renew from its refresh token

```execute
aux4 curl auth status interactive
```

```expect:partial
Refresh token: yes
Can renew:     yes
```

## with a directly stored api key

```beforeAll
mkdir -p .oauth
cat > .oauth/apikey.json << 'ENDTOKEN'
{
  "accessToken": "static-key",
  "refreshToken": "",
  "expiresAt": ""
}
ENDTOKEN
```

```afterAll
rm -rf .oauth
```

### should report that it cannot renew

```execute
aux4 curl auth status apikey
```

```expect:partial
Can renew:     no
```
