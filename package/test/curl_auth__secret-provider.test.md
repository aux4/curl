# curl auth with a secret provider

## secrets curl owns

```timeout
30000
```

```beforeAll
aux4 aux4 pkger list 2>/dev/null | grep -q "aux4/secret-aux4" || aux4 aux4 pkger install aux4/secret-aux4 > /dev/null 2>&1
cat > /tmp/aux4-curl-sp-18959.js << 'ENDJS'
const http = require('http');
let issued = 0;
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
      if (p.get('client_secret') !== 'plaintext-canary') {
        return json(401, { error: 'invalid_client', error_description: 'bad credentials' });
      }
      issued++;
      return json(200, { access_token: 'tok-' + issued, expires_in: 3600 });
    }
    if (req.url.startsWith('/api')) return json(200, { auth: req.headers['authorization'] || '' });
    json(404, { error: 'not found' });
  });
}).listen(18959, '127.0.0.1');
ENDJS
nohup node /tmp/aux4-curl-sp-18959.js >/dev/null 2>&1 &
for i in $(seq 1 40); do curl -s -o /dev/null http://127.0.0.1:18959/ping 2>/dev/null && break; sleep 0.25; done
```

```afterAll
aux4 curl auth remove spauth > /dev/null 2>&1 || true
aux4 secret aux4 remove --ref aux4-curl/spauth > /dev/null 2>&1 || true
pkill -f aux4-curl-sp-18959.js
rm -f /tmp/aux4-curl-sp-18959.js
rm -rf .oauth sp-config.yaml
```

### should take a plain secret and store it through the provider

The user never names a reference. curl derives one from the auth name, so
there is nothing to define or maintain.

```execute
aux4 secret aux4 remove --ref aux4-curl/spauth > /dev/null 2>&1 || true
aux4 curl auth add spauth --tokenUrl http://127.0.0.1:18959/token --clientId id1 --clientSecret plaintext-canary --secretProvider aux4 2>&1 | tail -1
aux4 secret aux4 list --vault aux4-curl --withFields true | grep spauth
```

```expect
Auth "spauth" verified and saved to .oauth/spauth.json
secret://aux4/aux4-curl/spauth/accessToken
secret://aux4/aux4-curl/spauth/clientSecret
```

### should keep the secret and the token out of the local file

```execute
grep -cE "plaintext-canary|tok-" .oauth/spauth.json || true
```

```expect
0
```

### should authenticate using the stored secret

```execute
aux4 curl auth-request --provider spauth http://127.0.0.1:18959/api
```

```expect:partial
Bearer tok-
```

### should write a config file with no secret in it

```execute
rm -f sp-config.yaml
printf 'config:\n  prod:\n    host: api.example.com\n' > sp-config.yaml
aux4 secret aux4 remove --ref aux4-curl/spauth > /dev/null 2>&1 || true
aux4 curl auth add spauth --tokenUrl http://127.0.0.1:18959/token --clientId id1 --clientSecret plaintext-canary --secretProvider aux4 --configFile sp-config.yaml --config prod --save true > /dev/null 2>&1
grep -c "plaintext-canary" sp-config.yaml || true
aux4 config get --file sp-config.yaml prod | grep -o '"secretProvider":"[^"]*"'
```

```expect
0
"secretProvider":"aux4"
```

### should keep the local file store when no provider is named

Existing installs are untouched: without --secretProvider the token and the
secret stay in the token file exactly as before.

```execute
rm -rf .oauth
aux4 curl auth add plainauth --tokenUrl http://127.0.0.1:18959/token --clientId id1 --clientSecret plaintext-canary > /dev/null 2>&1
grep -c "plaintext-canary" .oauth/plainauth.json || true
aux4 curl auth remove plainauth > /dev/null 2>&1 || true
```

```expect
1
```

## the interactive flow

```timeout
30000
```

```beforeAll
aux4 aux4 pkger list 2>/dev/null | grep -q "aux4/secret-aux4" || aux4 aux4 pkger install aux4/secret-aux4 > /dev/null 2>&1
cat > /tmp/aux4-curl-oa-18960.js << 'ENDJS'
const http = require('http');
let n = 0;
http.createServer((req, res) => {
  const chunks = [];
  req.on('data', c => chunks.push(c));
  req.on('end', () => {
    const p = new URLSearchParams(Buffer.concat(chunks).toString());
    const json = (c, o) => { res.writeHead(c, { 'Content-Type': 'application/json' }); res.end(JSON.stringify(o)); };
    if (req.url === '/ping') return json(200, { ok: true });
    if (req.url === '/token') {
      if (p.get('refresh_token') !== 'refresh-from-provider') {
        return json(401, { error: 'invalid_grant', error_description: 'bad refresh token' });
      }
      if (p.get('client_secret') !== 'secret-from-provider') {
        return json(401, { error: 'invalid_client', error_description: 'bad client secret' });
      }
      n++;
      return json(200, { access_token: 'fresh-' + n, refresh_token: 'rotated-' + n, expires_in: 3600 });
    }
    json(404, {});
  });
}).listen(18960, '127.0.0.1');
ENDJS
nohup node /tmp/aux4-curl-oa-18960.js >/dev/null 2>&1 &
for i in $(seq 1 40); do curl -s -o /dev/null http://127.0.0.1:18960/ping 2>/dev/null && break; sleep 0.25; done
```

```afterAll
aux4 secret aux4 remove --ref aux4-curl/oaflow > /dev/null 2>&1 || true
pkill -f aux4-curl-oa-18960.js
rm -f /tmp/aux4-curl-oa-18960.js
rm -rf .oauth
```

### should refresh using the refresh token and client secret held by the provider

Neither secret is in the token file, so a successful refresh proves both were
read back from the provider.

```execute
aux4 secret aux4 remove --ref aux4-curl/oaflow > /dev/null 2>&1 || true
aux4 secret aux4 create --vault aux4-curl --item oaflow --fields "clientSecret=secret-from-provider,refreshToken=refresh-from-provider,accessToken=stale" > /dev/null 2>&1
mkdir -p .oauth
cat > .oauth/oaflow.json << 'ENDTOKEN'
{
  "clientId": "id1",
  "clientSecret": "",
  "tokenUrl": "http://127.0.0.1:18960/token",
  "accessToken": "",
  "refreshToken": "",
  "expiresAt": "2020-01-01T00:00:00Z",
  "secretProvider": "aux4",
  "name": "oaflow"
}
ENDTOKEN
aux4 curl oauth token oaflow | grep -o 'fresh-[0-9]*' | sed 's/-[0-9]*/-N/'
```

```expect
fresh-N
```

### should keep the rotated refresh token out of the file and in the provider

A rotated refresh token is long-lived and can mint new access tokens on its
own, so it must land back with the provider rather than on disk.

```execute
grep -cE "fresh-|rotated-|secret-from-provider" .oauth/oaflow.json || true
aux4 secret aux4 get --ref aux4-curl/oaflow --fields refreshToken | grep -o 'rotated-[0-9]*' | sed 's/-[0-9]*/-N/'
```

```expect
0
rotated-N
```
