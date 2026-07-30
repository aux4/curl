# curl oauth login

## login and token exchange

```timeout
20000
```

```beforeAll
cat > /tmp/aux4-curl-srv-18930.js << 'ENDJS'
const http = require('http');
http.createServer((req, res) => {
  const chunks = [];
  req.on('data', c => chunks.push(c));
  req.on('end', () => {
    const body = Buffer.concat(chunks).toString();
    const params = {};
    body.split('&').forEach(p => { const i = p.indexOf('='); params[p.slice(0, i)] = p.slice(i + 1); });
    if (params.grant_type === 'authorization_code') {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ access_token: 'test-access-token', refresh_token: 'test-refresh-token', expires_in: 3600 }));
    } else {
      res.writeHead(400);
      res.end();
    }
  });
}).listen(18930, '127.0.0.1');
ENDJS
nohup node /tmp/aux4-curl-srv-18930.js >/dev/null 2>&1 &
echo $! > /tmp/aux4-curl-oauth-server.pid
for i in $(seq 1 40); do curl -s -o /dev/null http://127.0.0.1:18930/ 2>/dev/null && break; sleep 0.25; done
```

```afterAll
kill $(cat /tmp/aux4-curl-oauth-server.pid) 2>/dev/null
rm -f /tmp/aux4-curl-oauth-server.pid
rm -rf .oauth
```

### should save token after login

```execute
mkdir -p .oauth
cat > .oauth/testprovider.json << 'ENDTOKEN'
{
  "clientId": "test-client",
  "clientSecret": "test-secret",
  "authUrl": "http://127.0.0.1:18930/auth",
  "tokenUrl": "http://127.0.0.1:18930/token",
  "scopes": "read,write",
  "accessToken": "test-access-token",
  "refreshToken": "test-refresh-token",
  "expiresAt": "2099-12-31T23:59:59Z"
}
ENDTOKEN
aux4 curl oauth status testprovider
```

```expect:partial
Provider:      testprovider
Status:        valid
Scopes:        read,write
```

## authorize url construction

### should append params with & when authUrl already has a query string

```execute
aux4 curl oauth login gtestprov --clientId CID --clientSecret "" --authUrl "https://accounts.example.com/auth?access_type=offline&prompt=consent" --tokenUrl https://accounts.example.com/token --scopes "openid email" --callbackPort 18939 --tokenFile /tmp/aux4-authurl-token.json >/dev/null 2>/tmp/aux4-authurl.txt &
LOGIN_PID=$!
sleep 2
kill $LOGIN_PID 2>/dev/null
wait $LOGIN_PID 2>/dev/null
grep -oE "auth\?access_type=offline&prompt=consent&client_id=CID" /tmp/aux4-authurl.txt | head -1
rm -f /tmp/aux4-authurl.txt /tmp/aux4-authurl-token.json
```

```expect
auth?access_type=offline&prompt=consent&client_id=CID
```
