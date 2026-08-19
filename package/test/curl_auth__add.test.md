# curl auth add

## with a standard client_credentials endpoint

```timeout
30000
```

```beforeAll
cat > /tmp/aux4-curl-authadd-18951.js << 'ENDJS'
const http = require('http');
function b64(o) { return Buffer.from(JSON.stringify(o)).toString('base64url'); }
function jwt(seconds) {
  return b64({ alg: 'HS256', typ: 'JWT' }) + '.' +
         b64({ sub: 'svc', exp: Math.floor(Date.now() / 1000) + seconds }) + '.sig';
}
http.createServer((req, res) => {
  const chunks = [];
  req.on('data', c => chunks.push(c));
  req.on('end', () => {
    const raw = Buffer.concat(chunks).toString();
    const json = (code, obj) => {
      res.writeHead(code, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify(obj));
    };
    if (req.url === '/ping') return json(200, { ok: true });
    if (req.url === '/token') {
      const p = new URLSearchParams(raw);
      if (p.get('grant_type') !== 'client_credentials') return json(400, { error: 'unsupported_grant_type' });
      if (p.get('client_id') !== 'id1' || p.get('client_secret') !== 'sec1') {
        return json(401, { error: 'invalid_client', error_description: 'bad credentials' });
      }
      return json(200, { access_token: 'svc-token', expires_in: 3600 });
    }
    if (req.url === '/token-basic') {
      const expected = 'Basic ' + Buffer.from('id1:sec1').toString('base64');
      if ((req.headers['authorization'] || '') !== expected) {
        return json(401, { error: 'invalid_client', error_description: 'basic required' });
      }
      if (new URLSearchParams(raw).get('client_secret')) {
        return json(400, { error: 'invalid_request', error_description: 'secret must not be in the body' });
      }
      return json(200, { access_token: 'basic-tok', expires_in: 3600 });
    }
    if (req.url === '/login') {
      let body = {};
      try { body = JSON.parse(raw); } catch (e) {}
      if (body.clientId !== 'id1' || body.clientSecret !== 'sec1') return json(401, { message: 'nope' });
      return json(200, { token: jwt(7200) });
    }
    if (req.url === '/login-short') return json(200, { token: jwt(5) });
    if (req.url === '/nested') return json(200, { data: { credentials: { value: 'nested-tok' }, ttl: 900 } });
    if (req.url === '/no-token') return json(200, { message: 'all good', id: 7 });
    if (req.url === '/html') {
      res.writeHead(500, { 'Content-Type': 'text/html' });
      return res.end('<html><body>Gateway Error</body></html>');
    }
    if (req.url.startsWith('/api')) {
      return json(200, { auth: req.headers['authorization'] || '', key: req.headers['x-api-key'] || '' });
    }
    json(404, { error: 'not found' });
  });
}).listen(18951, '127.0.0.1');
ENDJS
nohup node /tmp/aux4-curl-authadd-18951.js >/dev/null 2>&1 &
for i in $(seq 1 40); do curl -s -o /dev/null http://127.0.0.1:18951/ping 2>/dev/null && break; sleep 0.25; done
```

```afterAll
pkill -f aux4-curl-authadd-18951.js
rm -f /tmp/aux4-curl-authadd-18951.js
rm -rf .oauth
```

### should fetch a token to verify the configuration

```execute
aux4 curl auth add svc --tokenUrl http://127.0.0.1:18951/token --clientId id1 --clientSecret sec1 2>&1
```

```expect:partial
Auth "svc" verified and saved to .oauth/svc.json
```

### should use the stored auth on an API call

```execute
aux4 curl auth add svc --tokenUrl http://127.0.0.1:18951/token --clientId id1 --clientSecret sec1
aux4 curl auth-request --provider svc http://127.0.0.1:18951/api
```

```expect:json
{
  "auth": "Bearer svc-token",
  "key": ""
}
```

### should report a rejected credential instead of storing it

```execute
aux4 curl auth add svc --tokenUrl http://127.0.0.1:18951/token --clientId id1 --clientSecret wrong
```

```error:partial
token request failed: invalid_client (bad credentials)
```

### should skip the check with --verify false

```execute
aux4 curl auth add later --tokenUrl http://127.0.0.1:18951/token --clientId id1 --clientSecret wrong --verify false 2>&1
```

```expect:partial
Auth "later" saved to .oauth/later.json
```

### should send the credentials as HTTP Basic with the basic preset

```execute
aux4 curl auth add basic --preset client-credentials-basic --tokenUrl http://127.0.0.1:18951/token-basic --clientId id1 --clientSecret sec1
aux4 curl auth-request --provider basic http://127.0.0.1:18951/api
```

```expect:json
{
  "auth": "Bearer basic-tok",
  "key": ""
}
```

### should post JSON credentials with the json preset

```execute
aux4 curl auth add loginapi --preset json --tokenUrl http://127.0.0.1:18951/login --clientId id1 --clientSecret sec1 2>&1
```

```expect:partial
Auth "loginapi" verified and saved to .oauth/loginapi.json
```

### should read the token from an explicit tokenPath

```execute
aux4 curl auth add nested --preset json --tokenUrl http://127.0.0.1:18951/nested --tokenPath '$.data.credentials.value'
aux4 curl auth token nested
```

```expect
nested-tok
```

### should apply a non-bearer header template

```execute
aux4 curl auth add keyed --tokenUrl http://127.0.0.1:18951/token --clientId id1 --clientSecret sec1 --authHeader 'X-API-Key: ${token}'
aux4 curl auth-request --provider keyed http://127.0.0.1:18951/api
```

```expect:json
{
  "auth": "",
  "key": "svc-token"
}
```

### should report where the token was looked for when it is absent

```execute
aux4 curl auth add missing --preset json --tokenUrl http://127.0.0.1:18951/no-token
```

```error:partial
no token found at $.token or $.access_token in the response
```

### should report a non-JSON response with its status

```execute
aux4 curl auth add broken --tokenUrl http://127.0.0.1:18951/html --clientId id1 --clientSecret sec1
```

```error:partial
token endpoint returned HTTP 500 with a non-JSON body
```

### should reject an unknown preset

```execute
aux4 curl auth add bogus --preset nope --tokenUrl http://127.0.0.1:18951/token
```

```error:partial
unknown preset "nope" (available: client-credentials, client-credentials-basic, jwt, json)
```

### should require a tokenUrl

```execute
aux4 curl auth add nourl --tokenUrl ""
```

```error:partial
tokenUrl is required
```

## expiry resolution

```timeout
30000
```

```beforeAll
cat > /tmp/aux4-curl-authexp-18952.js << 'ENDJS'
const http = require('http');
function b64(o) { return Buffer.from(JSON.stringify(o)).toString('base64url'); }
function jwt(seconds) {
  return b64({ alg: 'HS256', typ: 'JWT' }) + '.' +
         b64({ sub: 'svc', exp: Math.floor(Date.now() / 1000) + seconds }) + '.sig';
}
http.createServer((req, res) => {
  const json = (code, obj) => {
    res.writeHead(code, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify(obj));
  };
  if (req.url === '/ping') return json(200, { ok: true });
  // A JWT expiring inside the refresh skew, with no expiry field of its own.
  if (req.url === '/jwt-short') return json(200, { token: jwt(5) });
  // A JWT good for two hours, again with no expiry field.
  if (req.url === '/jwt-long') return json(200, { token: jwt(7200) });
  // Nothing anywhere reports a lifetime.
  if (req.url === '/bare') return json(200, { token: 'bare-tok' });
  json(404, { error: 'not found' });
}).listen(18952, '127.0.0.1');
ENDJS
nohup node /tmp/aux4-curl-authexp-18952.js >/dev/null 2>&1 &
for i in $(seq 1 40); do curl -s -o /dev/null http://127.0.0.1:18952/ping 2>/dev/null && break; sleep 0.25; done
```

```afterAll
pkill -f aux4-curl-authexp-18952.js
rm -f /tmp/aux4-curl-authexp-18952.js
rm -rf .oauth
```

### should read the deadline from the jwt exp claim

A JWT expiring in five seconds is already inside the refresh skew, so reading
`exp` is the only way to know this token is due for renewal.

```execute
aux4 curl auth add shortjwt --preset jwt --tokenUrl http://127.0.0.1:18952/jwt-short --verify true
aux4 curl auth status shortjwt
```

```expect:partial
Status:        expired
```

### should treat a long-lived jwt as valid

```execute
aux4 curl auth add longjwt --preset jwt --tokenUrl http://127.0.0.1:18952/jwt-long --verify true
aux4 curl auth status longjwt
```

```expect:partial
Status:        valid
```

### should fall back to the configured lifetime

```execute
aux4 curl auth add ttl --preset jwt --tokenUrl http://127.0.0.1:18952/bare --expiresIn 5
aux4 curl auth status ttl
```

```expect:partial
Status:        expired
```

### should fall back to the default lifetime when nothing reports one

```execute
aux4 curl auth add nottl --preset jwt --tokenUrl http://127.0.0.1:18952/bare
aux4 curl auth status nottl
```

```expect:partial
Status:        valid
```
