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
