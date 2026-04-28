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
