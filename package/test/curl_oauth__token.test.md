# curl oauth token

## get stored token

```beforeAll
mkdir -p .oauth
cat > .oauth/testprovider.json << 'ENDTOKEN'
{
  "clientId": "test-client",
  "clientSecret": "test-secret",
  "authUrl": "http://localhost/auth",
  "tokenUrl": "http://localhost/token",
  "scopes": "read,write",
  "accessToken": "my-test-access-token-123",
  "refreshToken": "my-test-refresh-token",
  "expiresAt": "2099-12-31T23:59:59Z"
}
ENDTOKEN
```

```afterAll
rm -rf .oauth
```

### should print the access token

```execute
aux4 curl oauth token testprovider
```

```expect
my-test-access-token-123
```

## with missing provider

### should show error

```execute
aux4 curl oauth token nonexistent
```

```error:partial
no token found for provider "nonexistent"
```
