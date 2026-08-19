# curl auth-request

## with valid token

```timeout
20000
```

```beforeAll
cat > /tmp/aux4-curl-srv-18931.js << 'ENDJS'
const http = require('http');
http.createServer((req, res) => {
  const auth = req.headers['authorization'] || '';
  if (req.method === 'GET') {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ auth: auth, path: req.url }));
    return;
  }
  const chunks = [];
  req.on('data', c => chunks.push(c));
  req.on('end', () => {
    const b = Buffer.concat(chunks).toString();
    let body = null;
    try { body = b ? JSON.parse(b) : null; } catch (e) { body = null; }
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ auth: auth, body: body }));
  });
}).listen(18931, '127.0.0.1');
ENDJS
nohup node /tmp/aux4-curl-srv-18931.js >/dev/null 2>&1 &
echo $! > /tmp/aux4-curl-auth-request-server.pid
for i in $(seq 1 40); do curl -s -o /dev/null http://127.0.0.1:18931/ 2>/dev/null && break; sleep 0.25; done

mkdir -p .oauth
cat > .oauth/testprovider.json << 'ENDTOKEN'
{
  "clientId": "test-client",
  "clientSecret": "test-secret",
  "authUrl": "http://localhost/auth",
  "tokenUrl": "http://localhost/token",
  "scopes": "read,write",
  "accessToken": "my-secret-token",
  "refreshToken": "my-refresh-token",
  "expiresAt": "2099-12-31T23:59:59Z"
}
ENDTOKEN
```

```afterAll
kill $(cat /tmp/aux4-curl-auth-request-server.pid) 2>/dev/null
rm -f /tmp/aux4-curl-auth-request-server.pid
rm -rf .oauth
```

### should inject authorization header on GET

```execute
aux4 curl auth-request --provider testprovider http://127.0.0.1:18931/test
```

```expect:json
{
  "auth": "Bearer my-secret-token",
  "path": "/test"
}
```

### should inject authorization header on POST

```execute
aux4 curl auth-request --provider testprovider --method POST --body '{"name":"test"}' --header "Content-Type: application/json" http://127.0.0.1:18931/data
```

```expect:json
{
  "auth": "Bearer my-secret-token",
  "body": {
    "name": "test"
  }
}
```

## with missing provider

### should show error

```execute
aux4 curl auth-request --provider nonexistent http://127.0.0.1:18931/test
```

```error:partial
no token found for provider "nonexistent"
```

## when the api rejects the stored token

```timeout
30000
```

```beforeAll
cat > /tmp/aux4-curl-retry-18954.js << 'ENDJS'
const http = require('http');
let issued = 0;
let accepted = 0;
http.createServer((req, res) => {
  const chunks = [];
  req.on('data', c => chunks.push(c));
  req.on('end', () => {
    const json = (code, obj) => {
      res.writeHead(code, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify(obj));
    };
    if (req.url === '/ping') return json(200, { ok: true });
    if (req.url === '/issued') return json(200, { issued: issued });
    if (req.url === '/token') {
      issued++;
      accepted = issued;
      return json(200, { access_token: 'tok-' + issued, expires_in: 3600 });
    }
    // Invalidate the outstanding token server-side, the way a revocation or an
    // early expiry would, without touching what the client has cached.
    if (req.url === '/revoke') {
      accepted++;
      return json(200, { accepted: accepted });
    }
    if (req.url.startsWith('/api')) {
      const auth = req.headers['authorization'] || '';
      if (auth !== 'Bearer tok-' + accepted) return json(401, { error: 'unauthorized' });
      return json(200, { ok: true, auth: auth });
    }
    json(404, { error: 'not found' });
  });
}).listen(18954, '127.0.0.1');
ENDJS
nohup node /tmp/aux4-curl-retry-18954.js >/dev/null 2>&1 &
for i in $(seq 1 40); do curl -s -o /dev/null http://127.0.0.1:18954/ping 2>/dev/null && break; sleep 0.25; done
```

```afterAll
pkill -f aux4-curl-retry-18954.js
rm -f /tmp/aux4-curl-retry-18954.js
rm -rf .oauth
```

### should renew and replay the request on a 401

The cached token is still inside its lifetime, so only the API's answer can
reveal that it is no longer accepted.

```execute
aux4 curl auth add revoked --tokenUrl http://127.0.0.1:18954/token >/dev/null 2>&1
aux4 curl request http://127.0.0.1:18954/revoke >/dev/null
aux4 curl auth-request --provider revoked http://127.0.0.1:18954/api
```

```expect:json
{
  "ok": true,
  "auth": "Bearer tok-2"
}
```

### should store the renewed token for the next call

```execute
aux4 curl auth add revoked2 --tokenUrl http://127.0.0.1:18954/token >/dev/null 2>&1
aux4 curl request http://127.0.0.1:18954/revoke >/dev/null
aux4 curl auth-request --provider revoked2 http://127.0.0.1:18954/api >/dev/null
before=$(aux4 curl request http://127.0.0.1:18954/issued | aux4 json get '$.issued')
aux4 curl auth-request --provider revoked2 http://127.0.0.1:18954/api >/dev/null
after=$(aux4 curl request http://127.0.0.1:18954/issued | aux4 json get '$.issued')
echo "extra fetches: $((after - before))"
```

```expect
extra fetches: 0
```

### should return the api response when the token cannot be renewed

A 401 is not always about an expired token, so a failed renewal must not
replace the API's own answer.

```execute
mkdir -p .oauth
cat > .oauth/norenew.json << 'ENDTOKEN'
{
  "accessToken": "not-accepted",
  "refreshToken": "",
  "expiresAt": "2099-12-31T23:59:59Z"
}
ENDTOKEN
aux4 curl auth-request --provider norenew http://127.0.0.1:18954/api
```

```expect:json
{
  "error": "unauthorized"
}
```
