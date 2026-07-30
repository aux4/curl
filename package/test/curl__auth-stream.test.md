# curl auth-stream

## with valid token

```timeout
20000
```

```beforeAll
cat > /tmp/aux4-curl-srv-18932.js << 'ENDJS'
const http = require('http');
http.createServer((req, res) => {
  const auth = req.headers['authorization'] || '';
  const chunks = [];
  req.on('data', c => chunks.push(c));
  req.on('end', () => {
    const b = Buffer.concat(chunks).toString();
    let received = null;
    try { received = b ? JSON.parse(b) : null; } catch (e) { received = null; }
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ auth: auth, received: received }));
  });
}).listen(18932, '127.0.0.1');
ENDJS
nohup node /tmp/aux4-curl-srv-18932.js >/dev/null 2>&1 &
echo $! > /tmp/aux4-curl-auth-stream-server.pid
for i in $(seq 1 40); do curl -s -o /dev/null http://127.0.0.1:18932/ 2>/dev/null && break; sleep 0.25; done

mkdir -p .oauth
cat > .oauth/testprovider.json << 'ENDTOKEN'
{
  "clientId": "test-client",
  "clientSecret": "test-secret",
  "authUrl": "http://localhost/auth",
  "tokenUrl": "http://localhost/token",
  "scopes": "read,write",
  "accessToken": "stream-token-123",
  "refreshToken": "my-refresh-token",
  "expiresAt": "2099-12-31T23:59:59Z"
}
ENDTOKEN
```

```afterAll
kill $(cat /tmp/aux4-curl-auth-stream-server.pid) 2>/dev/null
rm -f /tmp/aux4-curl-auth-stream-server.pid
rm -rf .oauth
```

### should stream with auth header

```execute
echo '{"id":1}' | aux4 curl auth-stream --provider testprovider --header "Content-Type: application/json" http://127.0.0.1:18932/
```

```expect:json:partial
{
  "body": {
    "auth": "Bearer stream-token-123",
    "received": {
      "id": 1
    }
  },
  "input": {
    "id": 1
  },
  "status": 200
}
```
