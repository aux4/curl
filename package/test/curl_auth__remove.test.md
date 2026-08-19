# curl auth remove

## with a stored auth

```beforeAll
mkdir -p .oauth
cat > .oauth/removeme.json << 'ENDTOKEN'
{
  "accessToken": "tok-1",
  "expiresAt": "2099-12-31T23:59:59Z",
  "grant": "client_credentials",
  "auth": {
    "preset": "client-credentials",
    "tokenUrl": "http://127.0.0.1:18999/token"
  }
}
ENDTOKEN
```

```afterAll
rm -rf .oauth
```

### should remove the stored configuration and its token

```execute
aux4 curl auth remove removeme 2>&1
aux4 curl auth status removeme 2>&1 || true
```

```expect:partial
Logged out from removeme
No token found for provider "removeme"
```

## with an unknown name

### should show error

```execute
aux4 curl auth remove nosuchauth 2>&1 || true
```

```expect:partial
No token found for provider "nosuchauth"
```
