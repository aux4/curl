# curl oauth set-token

## storing a key directly

```afterAll
rm -rf .oauth
```

### should store the token and report where

```execute
rm -rf .oauth
aux4 curl oauth set-token directkey --accessToken sk-abc123 2>&1
```

```expect
Token saved to .oauth/directkey.json
```

### should hand the stored value back

```execute
rm -rf .oauth
aux4 curl oauth set-token directkey --accessToken sk-abc123 > /dev/null 2>&1
aux4 curl oauth token directkey
```

```expect
sk-abc123
```

### should never treat a directly stored key as expired

There is no expiry and no way to renew one, so it must be used as-is rather
than triggering a refresh that cannot succeed.

```execute
rm -rf .oauth
aux4 curl oauth set-token directkey --accessToken sk-abc123 > /dev/null 2>&1
aux4 curl oauth status directkey | grep -E "^Status:|^Can renew:"
```

```expect
Status:        valid
Can renew:     no
```

### should replace the value when the key is rotated

```execute
rm -rf .oauth
aux4 curl oauth set-token directkey --accessToken sk-old > /dev/null 2>&1
aux4 curl oauth set-token directkey --accessToken sk-new > /dev/null 2>&1
aux4 curl oauth token directkey
```

```expect
sk-new
```

### should require an access token

```execute
aux4 curl oauth set-token directkey --accessToken ""
```

```error:partial
provider and accessToken are required
```
