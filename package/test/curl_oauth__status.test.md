# curl oauth status

## with valid token

```beforeAll
mkdir -p .oauth
cat > .oauth/testprovider.json << 'ENDTOKEN'
{
  "clientId": "test-client",
  "clientSecret": "test-secret",
  "authUrl": "http://localhost/auth",
  "tokenUrl": "http://localhost/token",
  "scopes": "read,write",
  "accessToken": "test-token",
  "refreshToken": "test-refresh",
  "expiresAt": "2099-12-31T23:59:59Z"
}
ENDTOKEN
```

```afterAll
rm -rf .oauth
```

### should show valid status

```execute
aux4 curl oauth status testprovider
```

```expect:partial
Provider:      testprovider
Status:        valid
Scopes:        read,write
```

### should show refresh token available

```execute
aux4 curl oauth status testprovider
```

```expect:partial
Refresh token: yes
```

## with expired token

```beforeAll
mkdir -p .oauth
cat > .oauth/expiredprovider.json << 'ENDTOKEN'
{
  "clientId": "test-client",
  "clientSecret": "test-secret",
  "authUrl": "http://localhost/auth",
  "tokenUrl": "http://localhost/token",
  "scopes": "read",
  "accessToken": "old-token",
  "refreshToken": "",
  "expiresAt": "2020-01-01T00:00:00Z"
}
ENDTOKEN
```

```afterAll
rm -rf .oauth
```

### should show expired status

```execute
aux4 curl oauth status expiredprovider
```

```expect:partial
Provider:      expiredprovider
Status:        expired
```

### should show no refresh token

```execute
aux4 curl oauth status expiredprovider
```

```expect:partial
Refresh token: no
```

## with a token file outside the current directory

```beforeAll
mkdir -p "$HOME/.aux4-curl-tokentest"
cat > "$HOME/.aux4-curl-tokentest/tildeprov.json" << 'ENDTOKEN'
{
  "clientId": "test-client",
  "accessToken": "test-access-token",
  "refreshToken": "test-refresh-token",
  "scopes": "read",
  "expiresAt": "2099-12-31T23:59:59Z"
}
ENDTOKEN
cp "$HOME/.aux4-curl-tokentest/tildeprov.json" "$HOME/.aux4-curl-tokentest/envprov.json"
```

```afterAll
rm -rf "$HOME/.aux4-curl-tokentest"
```

### should expand a leading tilde in tokenFile

```execute
aux4 curl oauth status tildeprov --tokenFile '~/.aux4-curl-tokentest/tildeprov.json'
```

```expect:partial
Provider:      tildeprov
Status:        valid
```

### should read the token from AUX4_CURL_OAUTH_HOME

```execute
AUX4_CURL_OAUTH_HOME="$HOME/.aux4-curl-tokentest" aux4 curl oauth status envprov
```

```expect:partial
Provider:      envprov
Status:        valid
```
