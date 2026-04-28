# curl oauth logout

## with existing token

```beforeAll
mkdir -p .oauth
cat > .oauth/testprovider.json << 'ENDTOKEN'
{
  "clientId": "test-client",
  "clientSecret": "test-secret",
  "accessToken": "test-token"
}
ENDTOKEN
```

```afterAll
rm -rf .oauth
```

### should remove the token file

```execute
aux4 curl oauth logout testprovider 2>/dev/null && test ! -f .oauth/testprovider.json && echo "token removed"
```

```expect
token removed
```

## with missing provider

### should show error

```execute
rm -rf .oauth
aux4 curl oauth logout nonexistent
```

```error:partial
No token found for provider "nonexistent"
```
